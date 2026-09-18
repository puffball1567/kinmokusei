package project

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/product"
)

type LockedPackage struct {
	Path           string            `json:"path"`
	Version        string            `json:"version"`
	Hash           string            `json:"hash"`
	ManifestHash   string            `json:"manifestHash"`
	ReplacePath    string            `json:"replacePath,omitempty"`
	License        string            `json:"license"`
	Dependencies   map[string]string `json:"dependencies"`
	GoDependencies map[string]string `json:"goDependencies"`
}

type SourcePackage struct {
	Manifest  Manifest
	Directory string
	Lock      LockedPackage
}

type PackageGraph struct {
	Root     Manifest
	Packages map[string]*SourcePackage
}

// ResolvePackageGraph is an explicit dependency operation. ReadPackageGraph
// instead verifies locked sources without fetching, changing a graph or files.
func ResolvePackageGraph(root Manifest, target BuildTarget, offline bool) (*PackageGraph, error) {
	return packageGraph(root, target, nil, offline)
}

func ReadPackageGraph(root Manifest, lock Lock) (*PackageGraph, error) {
	return packageGraph(root, lock.Target, &lock, true)
}

func packageGraph(root Manifest, target BuildTarget, lock *Lock, offline bool) (*PackageGraph, error) {
	graph := &PackageGraph{Root: root, Packages: map[string]*SourcePackage{}}
	locked := map[string]LockedPackage{}
	if lock != nil {
		for _, item := range lock.Packages {
			locked[item.Path] = item
		}
	}
	visiting := map[string]bool{}
	var visit func(string, string) error
	visit = func(module, version string) error {
		if visiting[module] {
			return fmt.Errorf("Kinmokusei package dependency cycle through %q", module)
		}
		if prior := graph.Packages[module]; prior != nil {
			if prior.Lock.Version != version {
				return fmt.Errorf("conflicting Kinmokusei package versions for %s: %s and %s", module, prior.Lock.Version, version)
			}
			return nil
		}
		replacement := root.PackageReplacements[module]
		if lock != nil {
			item, ok := locked[module]
			if !ok || item.Version != version || item.ReplacePath != replacement {
				return fmt.Errorf("package %s@%s does not match lock; run keika deps lock", module, version)
			}
		}
		var directory string
		var err error
		if replacement != "" {
			directory, err = filepath.EvalSymlinks(filepath.Join(root.Root, filepath.FromSlash(replacement)))
		} else if lock == nil {
			directory, err = downloadSourcePackage(module, version, offline)
		} else {
			directory, err = cachedSourcePackage(module, version)
		}
		if err != nil {
			return fmt.Errorf("package %s@%s unavailable: %w", module, version, err)
		}
		directory, err = filepath.EvalSymlinks(directory)
		if err != nil {
			return err
		}
		manifest, err := ReadManifest(directory)
		if err != nil {
			return fmt.Errorf("package %s@%s: %w", module, version, err)
		}
		if manifest.Package.Entry == "" {
			return fmt.Errorf("module %s@%s does not declare [package]", module, version)
		}
		if manifest.Project.GoModule != module {
			return fmt.Errorf("package module identity %q does not match requested %q", manifest.Project.GoModule, module)
		}
		if "v"+manifest.Project.Version != version {
			return fmt.Errorf("package %s version %s does not match requested %s", module, manifest.Project.Version, version)
		}
		if err := compatibleSourcePackage(manifest, root, target); err != nil {
			return err
		}
		for _, entry := range append([]string{manifest.Package.Entry}, mapValues(manifest.Exports)...) {
			if _, err := packageSourceFile(directory, entry); err != nil {
				return fmt.Errorf("package %s: %w", module, err)
			}
		}
		item := LockedPackage{Path: module, Version: version, ManifestHash: Hash(manifest.Contents), ReplacePath: replacement, License: manifest.Package.License, Dependencies: manifest.Packages, GoDependencies: manifest.Dependencies}
		item.Hash = item.ManifestHash // Local edits are live; graph/metadata edits require relocking.
		if replacement == "" {
			item.Hash, err = hashSourcePackage(directory)
			if err != nil {
				return err
			}
		}
		if lock != nil && !reflect.DeepEqual(item, locked[module]) {
			return fmt.Errorf("package %s@%s content or manifest does not match lock; run keika deps lock only for intentional changes", module, version)
		}
		graph.Packages[module] = &SourcePackage{Manifest: manifest, Directory: directory, Lock: item}
		visiting[module] = true
		for _, dependency := range sortedKeys(manifest.Packages) {
			if err := visit(dependency, manifest.Packages[dependency]); err != nil {
				return err
			}
		}
		delete(visiting, module)
		return nil
	}
	for _, module := range sortedKeys(root.Packages) {
		if err := visit(module, root.Packages[module]); err != nil {
			return nil, err
		}
	}
	if lock != nil && len(locked) != len(graph.Packages) {
		return nil, fmt.Errorf("package lock contains unreachable dependencies; run keika deps lock")
	}
	for module := range root.PackageReplacements {
		if graph.Packages[module] == nil {
			return nil, fmt.Errorf("package replacement %q is not in the dependency graph", module)
		}
	}
	return graph, nil
}

func (g *PackageGraph) LockedPackages() []LockedPackage {
	result := make([]LockedPackage, 0, len(g.Packages))
	for _, pkg := range g.Packages {
		result = append(result, pkg.Lock)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

func mapValues(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for _, key := range sortedKeys(values) {
		result = append(result, values[key])
	}
	return result
}

func (g *PackageGraph) GoManifest() (Manifest, error) {
	result := g.Root
	result.Dependencies = map[string]string{}
	for path, version := range g.Root.Dependencies {
		result.Dependencies[path] = version
	}
	for _, pkg := range g.Packages {
		for path := range pkg.Manifest.Replacements {
			if g.Root.Replacements[path] == "" {
				return Manifest{}, fmt.Errorf("package %s requests Go replacement %s; declare replacements explicitly in the consuming project", pkg.Lock.Path, path)
			}
		}
		for path, version := range pkg.Manifest.Dependencies {
			if prior := result.Dependencies[path]; prior == "" || comparePackageVersions(prior, version) < 0 {
				result.Dependencies[path] = version
			}
		}
	}
	return result, nil
}

// A library cannot silently acquire a latest Go dependency through the
// consumer's tidy operation. Standard-library paths have no dotted first part.
func (p *SourcePackage) validateGoImport(imported string) error {
	if !strings.Contains(strings.SplitN(imported, "/", 2)[0], ".") {
		return nil
	}
	for module := range p.Manifest.Dependencies {
		if imported == module || strings.HasPrefix(imported, module+"/") {
			return nil
		}
	}
	return fmt.Errorf("package %s imports Go package %q without a [go.dependencies] declaration; put Go helpers in a separately declared Go module", p.Lock.Path, imported)
}

func compatibleSourcePackage(pkg, root Manifest, target BuildTarget) error {
	version := strings.TrimPrefix(product.VersionString(), "v")
	if version == "devel" {
		version = product.DevelopmentCompatibilityVersion
	}
	if comparePackageVersions(version, pkg.Package.MinimumVersion) < 0 {
		return fmt.Errorf("package %s requires Kinmokusei %s; compiler is %s", pkg.Project.GoModule, pkg.Package.MinimumVersion, version)
	}
	if comparePackageVersions(root.Project.GoVersion, pkg.Project.GoVersion) < 0 {
		return fmt.Errorf("package %s requires Go %s", pkg.Project.GoModule, pkg.Project.GoVersion)
	}
	if pkg.Target.GOOS != "" && pkg.Target.GOOS != target.GOOS || pkg.Target.GOARCH != "" && pkg.Target.GOARCH != target.GOARCH || pkg.Target.CGO == "enabled" && !target.CGOEnabled {
		return fmt.Errorf("package %s is incompatible with target %s/%s (cgo=%t)", pkg.Project.GoModule, target.GOOS, target.GOARCH, target.CGOEnabled)
	}
	for _, tag := range pkg.Target.Tags {
		found := false
		for _, active := range target.Tags {
			found = found || tag == active
		}
		if !found {
			return fmt.Errorf("package %s requires build tag %s", pkg.Project.GoModule, tag)
		}
	}
	return nil
}

// Export paths are explicit. A dependency cannot reach arbitrary files in its
// cache, a sibling package, or the consumer through relative paths/symlinks.
func packageSourceFile(root, relative string) (string, error) {
	if !validPackageSource(relative) {
		return "", fmt.Errorf("invalid package source path %q", relative)
	}
	file := filepath.Join(root, filepath.FromSlash(relative))
	resolved, err := filepath.EvalSymlinks(file)
	if err != nil {
		return "", fmt.Errorf("missing package source %s: %w", relative, err)
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if !pathWithin(base, resolved) {
		return "", fmt.Errorf("package source %q escapes its package root", relative)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("package source %q is not a regular file", relative)
	}
	return resolved, nil
}

func pathWithin(root, file string) bool {
	relative, err := filepath.Rel(root, file)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
