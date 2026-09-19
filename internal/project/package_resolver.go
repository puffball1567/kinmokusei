package project

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (g *PackageGraph) owner(file string) *SourcePackage {
	file = canonicalPackageFile(file)
	var found *SourcePackage
	for _, pkg := range g.Packages {
		if pathWithin(pkg.Directory, file) && (found == nil || len(pkg.Directory) > len(found.Directory)) {
			found = pkg
		}
	}
	return found
}

// ResolveImport checks the importing package's own declared dependency edges,
// not merely membership somewhere in the consumer's transitive graph.
func (g *PackageGraph) ResolveImport(importer, imported string) (string, error) {
	manifest := g.Root
	owner := g.owner(importer)
	if owner != nil {
		manifest = owner.Manifest
	}
	imported = manifest.expandSourceImport(imported)
	dependencies := manifest.Packages
	module := ""
	for candidate := range dependencies {
		if (imported == candidate || strings.HasPrefix(imported, candidate+"/")) && len(candidate) > len(module) {
			module = candidate
		}
	}
	if owner != nil {
		self := owner.Manifest.Project.GoModule
		if (imported == self || strings.HasPrefix(imported, self+"/")) && len(self) > len(module) {
			module = self
		}
	}
	if module == "" {
		return "", fmt.Errorf("Kinmokusei package import %q is not declared in [dependencies]", imported)
	}
	pkg := g.Packages[module]
	if pkg == nil {
		return "", fmt.Errorf("Kinmokusei package %q is missing from lock", module)
	}
	source := pkg.Manifest.Package.Entry
	if imported != module {
		var found bool
		source, found = pkg.Manifest.Exports[strings.TrimPrefix(imported, module+"/")]
		if !found {
			return "", fmt.Errorf("package %q does not export submodule %q", module, imported)
		}
	}
	return packageSourceFile(pkg.Directory, source)
}

func (g *PackageGraph) ValidateRelativeImport(importer, target string) error {
	if pkg := g.owner(importer); pkg != nil {
		relative, err := filepath.Rel(pkg.Directory, canonicalPackageFile(target))
		if err != nil {
			return err
		}
		if !validPackageSource(filepath.ToSlash(relative)) {
			return fmt.Errorf("invalid package source path %q", relative)
		}
	}
	return nil
}

func (g *PackageGraph) ValidateGoImport(importer, imported string) error {
	if pkg := g.owner(importer); pkg != nil {
		return pkg.validateGoImport(imported)
	}
	return nil
}

// Public module identity, not an absolute cache/replacement path, determines
// generated symbol names. Moving a checkout does not change generated APIs.
func (g *PackageGraph) SourceIdentity(file string) string {
	file = canonicalPackageFile(file)
	if pkg := g.owner(file); pkg != nil {
		relative, err := filepath.Rel(pkg.Directory, file)
		if err == nil {
			return pkg.Manifest.Project.GoModule + "/" + filepath.ToSlash(relative)
		}
	}
	return ""
}

func canonicalPackageFile(file string) string {
	if absolute, err := filepath.Abs(file); err == nil {
		file = absolute
	}
	if resolved, err := filepath.EvalSymlinks(file); err == nil {
		return resolved
	}
	if parent, err := filepath.EvalSymlinks(filepath.Dir(file)); err == nil {
		return filepath.Join(parent, filepath.Base(file))
	}
	return file
}
