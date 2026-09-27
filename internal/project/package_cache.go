package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func packageCommandEnvironment(offline bool) []string {
	env := os.Environ()
	for key, value := range map[string]string{"GOWORK": "off", "GO111MODULE": "on", "GOTOOLCHAIN": "local", "GOFLAGS": ""} {
		env = environmentWith(env, key, value)
	}
	if offline {
		for key, value := range map[string]string{"GOPROXY": "off", "GOSUMDB": "off", "GONOPROXY": "none", "GOVCS": "*:off"} {
			env = environmentWith(env, key, value)
		}
	}
	return env
}

// Only explicit dependency commands call download. It runs outside any user
// module, so obtaining an archive cannot rewrite a project's go.mod/go.sum.
func downloadSourcePackage(module, version string, offline bool) (string, error) {
	if err := validatePackageRequest(module, version); err != nil {
		return "", err
	}
	if offline {
		return cachedSourcePackage(module, version)
	}
	directory, err := os.MkdirTemp("", "kinmokusei-package-download-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(directory)
	command := exec.Command("go", "mod", "download", "-json", module+"@"+version)
	command.Dir = directory
	command.Env = packageCommandEnvironment(false)
	output, err := command.Output()
	var result struct{ Path, Version, Dir, Error string }
	decodeErr := json.Unmarshal(output, &result)
	if err != nil || result.Error != "" {
		if result.Error != "" {
			return "", fmt.Errorf("%s", result.Error)
		}
		return "", fmt.Errorf("cannot download %s@%s: %w", module, version, err)
	}
	if decodeErr != nil {
		return "", decodeErr
	}
	if result.Path != module || result.Version != version || result.Dir == "" {
		return "", fmt.Errorf("download returned unexpected package identity for %s@%s", module, version)
	}
	return result.Dir, nil
}

func cachedSourcePackage(module, version string) (string, error) {
	if err := validatePackageRequest(module, version); err != nil {
		return "", err
	}
	command := exec.Command("go", "env", "GOMODCACHE")
	command.Env = packageCommandEnvironment(true)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("cannot locate Go module cache: %w", err)
	}
	// Go's module cache escapes ASCII uppercase as ! followed by lowercase.
	escape := func(value string) string {
		var result strings.Builder
		for _, ch := range value {
			if ch >= 'A' && ch <= 'Z' {
				result.WriteByte('!')
				ch += 'a' - 'A'
			}
			result.WriteRune(ch)
		}
		return result.String()
	}
	directory := filepath.Join(strings.TrimSpace(string(output)), filepath.FromSlash(escape(module)+"@"+escape(version)))
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("cache miss for %s@%s; run keika deps fetch with network access", module, version)
	}
	return directory, nil
}

func validatePackageRequest(module, version string) error {
	if err := validateModulePath(module); err != nil {
		return err
	}
	if !canonicalPackageSubpath(module) || !goModuleVersionPattern.MatchString(version) {
		return fmt.Errorf("invalid package request %q@%q", module, version)
	}
	return nil
}

func hashSourcePackage(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if file != root && excludedPackageDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("package contains non-regular file %s", file)
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		fmt.Fprintf(hash, "%d:%s:%d:", len(relative), relative, info.Size())
		input, err := os.Open(file)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(hash, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}
