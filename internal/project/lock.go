package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/puffball1567/onsentamago/internal/product"
)

const LockVersion = 3

type LockedLicenseFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type LockedModule struct {
	Path         string              `json:"path"`
	Version      string              `json:"version,omitempty"`
	Sum          string              `json:"sum,omitempty"`
	GoModSum     string              `json:"goModSum,omitempty"`
	ReplacePath  string              `json:"replacePath,omitempty"`
	LicenseFiles []LockedLicenseFile `json:"licenseFiles"`
}

type Lock struct {
	LockVersion  int            `json:"lockVersion"`
	ManifestHash string         `json:"manifestHash"`
	GoVersion    string         `json:"goVersion"`
	Target       BuildTarget    `json:"target"`
	GoModHash    string         `json:"goModHash"`
	GoSumHash    string         `json:"goSumHash"`
	Modules      []LockedModule `json:"modules"`
}

func Hash(contents []byte) string {
	sum := sha256.Sum256(contents)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (m Manifest) NewLock(goMod, goSum []byte, target BuildTarget, modules []LockedModule) Lock {
	modules = append([]LockedModule(nil), modules...)
	for index := range modules {
		if modules[index].LicenseFiles == nil {
			modules[index].LicenseFiles = []LockedLicenseFile{}
		}
	}
	sort.Slice(modules, func(i, j int) bool {
		if modules[i].Path != modules[j].Path {
			return modules[i].Path < modules[j].Path
		}
		return modules[i].Version < modules[j].Version
	})
	return Lock{
		LockVersion: LockVersion, ManifestHash: Hash(m.Contents), GoVersion: m.Project.GoVersion, Target: target,
		GoModHash: Hash(goMod), GoSumHash: Hash(goSum), Modules: modules,
	}
}

func ReadLock(root string) (Lock, error) {
	path := filepath.Join(root, product.LockFileName)
	contents, err := os.ReadFile(path)
	if err != nil {
		return Lock{}, fmt.Errorf("cannot read dependency lock %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var lock Lock
	if err = decoder.Decode(&lock); err != nil {
		return Lock{}, fmt.Errorf("invalid dependency lock %s: %w", path, err)
	}
	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		return Lock{}, fmt.Errorf("invalid dependency lock %s: trailing JSON content", path)
	}
	if err = lock.Validate(); err != nil {
		return Lock{}, fmt.Errorf("invalid dependency lock %s: %w", path, err)
	}
	return lock, nil
}

func (l Lock) Validate() error {
	if l.LockVersion != LockVersion {
		return fmt.Errorf("unsupported lockVersion %d; expected %d", l.LockVersion, LockVersion)
	}
	if !goVersionPattern.MatchString(l.GoVersion) {
		return fmt.Errorf("goVersion %q must have the form 1.N or 1.N.P", l.GoVersion)
	}
	if err := l.Target.Validate(); err != nil {
		return err
	}
	for name, value := range map[string]string{"manifestHash": l.ManifestHash, "goModHash": l.GoModHash, "goSumHash": l.GoSumHash} {
		if len(value) != len("sha256:")+sha256.Size*2 || value[:len("sha256:")] != "sha256:" {
			return fmt.Errorf("%s is not a SHA-256 digest", name)
		}
		if _, err := hex.DecodeString(value[len("sha256:"):]); err != nil {
			return fmt.Errorf("%s is not a SHA-256 digest", name)
		}
	}
	for i, module := range l.Modules {
		if module.Path == "" {
			return fmt.Errorf("modules[%d] has an empty path", i)
		}
		if i > 0 && (l.Modules[i-1].Path > module.Path || l.Modules[i-1].Path == module.Path && l.Modules[i-1].Version >= module.Version) {
			return fmt.Errorf("modules must be uniquely sorted by path and version")
		}
		if isPortableAbsolutePath(module.ReplacePath) {
			return fmt.Errorf("module %q contains an absolute replacement path", module.Path)
		}
		cleanReplacement := filepath.Clean(filepath.FromSlash(module.ReplacePath))
		if module.ReplacePath != "" && (cleanReplacement == ".." || strings.HasPrefix(cleanReplacement, ".."+string(filepath.Separator))) {
			return fmt.Errorf("module %q replacement path escapes the project root", module.Path)
		}
		if module.LicenseFiles == nil {
			return fmt.Errorf("module %q is missing licenseFiles metadata", module.Path)
		}
		for licenseIndex, license := range module.LicenseFiles {
			if license.Path == "" || isPortableAbsolutePath(license.Path) || filepath.Base(license.Path) != license.Path {
				return fmt.Errorf("module %q licenseFiles[%d] has an invalid path", module.Path, licenseIndex)
			}
			if licenseIndex > 0 && module.LicenseFiles[licenseIndex-1].Path >= license.Path {
				return fmt.Errorf("module %q licenseFiles must be uniquely sorted by path", module.Path)
			}
			if !validSHA256(license.SHA256) {
				return fmt.Errorf("module %q license file %q has an invalid SHA-256 digest", module.Path, license.Path)
			}
		}
	}
	return nil
}

func sameBuildTarget(left, right BuildTarget) bool {
	return left.GOOS == right.GOOS && left.GOARCH == right.GOARCH && left.CGOEnabled == right.CGOEnabled && slices.Equal(left.Tags, right.Tags)
}

func validSHA256(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(value[len("sha256:"):])
	return err == nil
}

func WriteLock(root string, lock Lock) error {
	if err := lock.Validate(); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return writeAtomic(filepath.Join(root, product.LockFileName), contents, 0o644)
}

func writeAtomic(path string, contents []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".ontama-write-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(contents); err == nil {
		err = temporary.Chmod(mode)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
