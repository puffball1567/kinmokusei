package project

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/puffball1567/kinmokusei/internal/product"
)

// AddAutoDependency keeps existing Go packages working, while modules with an
// explicit [package] declaration become ordinary Kinmokusei dependencies.
func AddAutoDependency(root, module, version, replacement string, offline bool) error {
	if err := validatePackageRequest(module, version); err != nil {
		return err
	}
	manifest, err := ReadManifest(root)
	if err != nil {
		return err
	}
	var directory string
	if replacement != "" {
		if isPortableAbsolutePath(replacement) {
			return fmt.Errorf("replacement must be project-relative")
		}
		directory = filepath.Join(manifest.Root, filepath.FromSlash(replacement))
	} else {
		directory, err = downloadSourcePackage(module, version, offline)
		if err != nil {
			return err
		}
	}
	contents, err := os.ReadFile(filepath.Join(directory, product.ProjectFileName))
	if os.IsNotExist(err) {
		return AddDependency(root, module, version, replacement, offline)
	}
	if err != nil {
		return err
	}
	dependency, err := ParseManifest(filepath.Join(directory, product.ProjectFileName), contents)
	if err != nil {
		return err
	}
	if dependency.Package.Entry == "" {
		return AddDependency(root, module, version, replacement, offline)
	}
	if _, exists := manifest.Packages[module]; exists {
		return fmt.Errorf("Kinmokusei dependency %q already exists", module)
	}
	manifest.Packages[module] = version
	if replacement != "" {
		manifest.PackageReplacements[module] = filepath.ToSlash(filepath.Clean(filepath.FromSlash(replacement)))
	}
	return commitManifestAndLock(manifest, offline)
}

// FetchDependencies restores artifacts from an unchanged lock, never resolves
// latest versions, and verifies downloaded sources before publishing state.
func FetchDependencies(root string, offline bool) error {
	manifest, err := ReadManifest(root)
	if err != nil {
		return err
	}
	lock, err := ReadLock(root)
	if err != nil {
		return err
	}
	if lock.ManifestHash != Hash(manifest.Contents) {
		return fmt.Errorf("manifest does not match lock; run keika deps lock for intentional changes")
	}
	target, err := ResolveTarget(manifest.Target)
	if err != nil {
		return err
	}
	if !sameBuildTarget(target, lock.Target) {
		return fmt.Errorf("target does not match lock")
	}
	if Hash([]byte(lock.GoMod)) != lock.GoModHash || Hash([]byte(lock.GoSum)) != lock.GoSumHash {
		return fmt.Errorf("lock lacks reproducible Go module contents; run keika deps lock")
	}
	for _, pkg := range lock.Packages {
		if pkg.ReplacePath == "" {
			if _, err := downloadSourcePackage(pkg.Path, pkg.Version, offline); err != nil {
				return err
			}
		}
	}
	if _, err := ReadPackageGraph(manifest, lock); err != nil {
		return err
	}
	state := filepath.Dir(product.DependencyDirectory(root))
	if err := os.MkdirAll(state, 0o755); err != nil {
		return err
	}
	directory, err := os.MkdirTemp(state, ".deps-fetch-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte(lock.GoMod), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, "go.sum"), []byte(lock.GoSum), 0o644); err != nil {
		return err
	}
	if err := runGo(directory, offline, target, "mod", "download", "all"); err != nil {
		return err
	}
	for name, want := range map[string]string{"go.mod": lock.GoModHash, "go.sum": lock.GoSumHash} {
		contents, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return err
		}
		if Hash(contents) != want {
			return fmt.Errorf("fetch changed locked %s", name)
		}
	}
	destination := product.DependencyDirectory(root)
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	for name, contents := range map[string]string{"go.mod": lock.GoMod, "go.sum": lock.GoSum} {
		if err := writeAtomic(filepath.Join(destination, name), []byte(contents), 0o644); err != nil {
			return err
		}
	}
	return CheckDependencies(root)
}

// Latest selection is explicit, transactional, and limited to declared source
// packages. Exact module@version remains available for upgrades and downgrades.
func UpdateSourcePackages(root, selected string, offline bool) error {
	manifest, err := ReadManifest(root)
	if err != nil {
		return err
	}
	modules := sortedKeys(manifest.Packages)
	if selected != "" {
		if _, ok := manifest.Packages[selected]; !ok {
			return fmt.Errorf("Kinmokusei dependency %q is not declared; use module@version for an exact Go update", selected)
		}
		modules = []string{selected}
	}
	for _, module := range modules {
		if replacement := manifest.PackageReplacements[module]; replacement != "" {
			pkg, err := ReadManifest(filepath.Join(manifest.Root, filepath.FromSlash(replacement)))
			if err != nil {
				return err
			}
			manifest.Packages[module] = "v" + pkg.Project.Version
			continue
		}
		if offline {
			return fmt.Errorf("cannot select latest version of %s offline; use module@version", module)
		}
		directory, err := os.MkdirTemp("", "kinmokusei-package-version-")
		if err != nil {
			return err
		}
		command := exec.Command("go", "list", "-m", "-json", module+"@latest")
		command.Dir = directory
		command.Env = packageCommandEnvironment(false)
		output, runErr := command.Output()
		removeErr := os.RemoveAll(directory)
		if runErr != nil {
			return fmt.Errorf("cannot resolve latest version of %s: %w", module, runErr)
		}
		if removeErr != nil {
			return removeErr
		}
		var info struct{ Path, Version string }
		if err := json.Unmarshal(output, &info); err != nil {
			return err
		}
		if info.Path != module || !goModuleVersionPattern.MatchString(info.Version) {
			return fmt.Errorf("invalid latest version for %s", module)
		}
		manifest.Packages[module] = info.Version
	}
	return commitManifestAndLock(manifest, offline)
}
