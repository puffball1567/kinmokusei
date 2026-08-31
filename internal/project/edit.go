package project

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/puffball1567/onsentamago/internal/product"
)

func RenderManifest(manifest Manifest) ([]byte, error) {
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	var result bytes.Buffer
	result.WriteString("[project]\n")
	fmt.Fprintf(&result, "name = %s\n", strconv.Quote(manifest.Project.Name))
	fmt.Fprintf(&result, "version = %s\n", strconv.Quote(manifest.Project.Version))
	fmt.Fprintf(&result, "go-module = %s\n", strconv.Quote(manifest.Project.GoModule))
	fmt.Fprintf(&result, "go-version = %s\n", strconv.Quote(manifest.Project.GoVersion))
	if manifest.Target.GOOS != "" || manifest.Target.GOARCH != "" || manifest.Target.CGO != "" || len(manifest.Target.Tags) != 0 {
		result.WriteString("\n[target]\n")
		if manifest.Target.GOOS != "" {
			fmt.Fprintf(&result, "goos = %s\n", strconv.Quote(manifest.Target.GOOS))
		}
		if manifest.Target.GOARCH != "" {
			fmt.Fprintf(&result, "goarch = %s\n", strconv.Quote(manifest.Target.GOARCH))
		}
		if manifest.Target.CGO != "" {
			fmt.Fprintf(&result, "cgo = %s\n", strconv.Quote(manifest.Target.CGO))
		}
		if len(manifest.Target.Tags) != 0 {
			fmt.Fprintf(&result, "tags = %s\n", strconv.Quote(strings.Join(manifest.Target.Tags, ",")))
		}
	}
	if manifest.GoInterop.Unsafe != "" {
		result.WriteString("\n[go.interop]\n")
		fmt.Fprintf(&result, "unsafe = %s\n", strconv.Quote(manifest.GoInterop.Unsafe))
	}
	dependencies := sortedKeys(manifest.Dependencies)
	if len(dependencies) != 0 {
		result.WriteString("\n[go.dependencies]\n")
		for _, path := range dependencies {
			fmt.Fprintf(&result, "%s = %s\n", strconv.Quote(path), strconv.Quote(manifest.Dependencies[path]))
		}
	}
	replacements := sortedKeys(manifest.Replacements)
	if len(replacements) != 0 {
		result.WriteString("\n[go.replacements]\n")
		for _, path := range replacements {
			fmt.Fprintf(&result, "%s = %s\n", strconv.Quote(path), strconv.Quote(filepath.ToSlash(filepath.Clean(filepath.FromSlash(manifest.Replacements[path])))))
		}
	}
	return result.Bytes(), nil
}

func ParseDependencyArgument(argument string) (string, string, error) {
	separator := strings.LastIndex(argument, "@")
	if separator <= 0 || separator == len(argument)-1 {
		return "", "", fmt.Errorf("dependency must have the form <module>@<version>")
	}
	path, version := argument[:separator], argument[separator+1:]
	if err := validateModulePath(path); err != nil {
		return "", "", err
	}
	if !goModuleVersionPattern.MatchString(version) {
		return "", "", fmt.Errorf("dependency version %q is not a complete Go module version", version)
	}
	return path, version, nil
}

func AddDependency(root, path, version, replacement string, offline bool) error {
	manifest, err := ReadManifest(root)
	if err != nil {
		return err
	}
	if _, exists := manifest.Dependencies[path]; exists {
		return fmt.Errorf("Go dependency %q already exists; remove it before changing its version", path)
	}
	manifest.Dependencies[path] = version
	if replacement != "" {
		manifest.Replacements[path] = filepath.ToSlash(filepath.Clean(filepath.FromSlash(replacement)))
	}
	return commitManifestAndLock(manifest, offline)
}

func RemoveDependency(root, path string, offline bool) error {
	manifest, err := ReadManifest(root)
	if err != nil {
		return err
	}
	if _, exists := manifest.Dependencies[path]; !exists {
		return fmt.Errorf("Go dependency %q is not declared", path)
	}
	delete(manifest.Dependencies, path)
	delete(manifest.Replacements, path)
	return commitManifestAndLock(manifest, offline)
}

func UpdateDependency(root, path, version string, offline bool) error {
	manifest, err := ReadManifest(root)
	if err != nil {
		return err
	}
	current, exists := manifest.Dependencies[path]
	if !exists {
		return fmt.Errorf("Go dependency %q is not declared", path)
	}
	if current == version {
		return fmt.Errorf("Go dependency %q already uses version %q", path, version)
	}
	manifest.Dependencies[path] = version
	return commitManifestAndLock(manifest, offline)
}

func commitManifestAndLock(manifest Manifest, offline bool) error {
	snapshot, err := captureDependencyState(manifest.Root)
	if err != nil {
		return err
	}
	contents, err := RenderManifest(manifest)
	if err != nil {
		return err
	}
	path := filepath.Join(manifest.Root, product.ProjectFileName)
	if err = writeAtomic(path, contents, 0o644); err != nil {
		return err
	}
	if _, err = LockDependencies(manifest.Root, offline); err != nil {
		if restoreErr := snapshot.restore(); restoreErr != nil {
			return fmt.Errorf("dependency resolution failed (%v) and the original dependency state could not be restored: %w", err, restoreErr)
		}
		return fmt.Errorf("dependency resolution failed; original dependency state restored: %w", err)
	}
	return nil
}

type dependencyFileSnapshot struct {
	path     string
	contents []byte
	mode     os.FileMode
	exists   bool
}

type dependencyStateSnapshot struct {
	files                      []dependencyFileSnapshot
	stateDirectoryExisted      bool
	dependencyDirectoryExisted bool
}

func captureDependencyState(root string) (dependencyStateSnapshot, error) {
	stateDirectory := filepath.Join(root, product.StateDirectoryName)
	dependencyDirectory := product.DependencyDirectory(root)
	stateDirectoryExisted, err := existingDirectory(stateDirectory)
	if err != nil {
		return dependencyStateSnapshot{}, err
	}
	dependencyDirectoryExisted, err := existingDirectory(dependencyDirectory)
	if err != nil {
		return dependencyStateSnapshot{}, err
	}
	snapshot := dependencyStateSnapshot{
		stateDirectoryExisted:      stateDirectoryExisted,
		dependencyDirectoryExisted: dependencyDirectoryExisted,
	}
	for _, path := range []string{
		filepath.Join(root, product.ProjectFileName),
		filepath.Join(root, product.LockFileName),
		filepath.Join(dependencyDirectory, "go.mod"),
		filepath.Join(dependencyDirectory, "go.sum"),
	} {
		file := dependencyFileSnapshot{path: path}
		contents, err := os.ReadFile(path)
		if err == nil {
			info, statErr := os.Stat(path)
			if statErr != nil {
				return dependencyStateSnapshot{}, statErr
			}
			file.contents, file.mode, file.exists = contents, info.Mode().Perm(), true
		} else if !os.IsNotExist(err) {
			return dependencyStateSnapshot{}, err
		}
		snapshot.files = append(snapshot.files, file)
	}
	return snapshot, nil
}

func existingDirectory(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("dependency state path %q is not a directory", path)
	}
	return true, nil
}

func (snapshot dependencyStateSnapshot) restore() error {
	var restoreErrors []error
	for _, file := range snapshot.files {
		if file.exists {
			if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
				restoreErrors = append(restoreErrors, err)
				continue
			}
			if err := writeAtomic(file.path, file.contents, file.mode); err != nil {
				restoreErrors = append(restoreErrors, err)
			}
			continue
		}
		if err := os.Remove(file.path); err != nil && !os.IsNotExist(err) {
			restoreErrors = append(restoreErrors, err)
		}
	}
	dependencyDirectory := filepath.Dir(snapshot.files[len(snapshot.files)-1].path)
	stateDirectory := filepath.Dir(dependencyDirectory)
	if !snapshot.dependencyDirectoryExisted {
		if err := os.Remove(dependencyDirectory); err != nil && !os.IsNotExist(err) {
			restoreErrors = append(restoreErrors, err)
		}
	}
	if !snapshot.stateDirectoryExisted {
		if err := os.Remove(stateDirectory); err != nil && !os.IsNotExist(err) {
			restoreErrors = append(restoreErrors, err)
		}
	}
	return errors.Join(restoreErrors...)
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
