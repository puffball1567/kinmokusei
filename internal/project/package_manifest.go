package project

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/product"
)

// Package metadata extends the existing project manifest. The project
// go-module is also the source package's public module identity.
type PackageConfig struct {
	Entry          string
	MinimumVersion string
	Backend        string
	License        string
}

func (p *PackageConfig) set(key, value string) error {
	switch key {
	case "entry":
		p.Entry = value
	case "min-kinmokusei":
		p.MinimumVersion = value
	case "backend":
		p.Backend = value
	case "license":
		p.License = value
	default:
		return fmt.Errorf("unknown package key %q", key)
	}
	return nil
}

func (m Manifest) validatePackageConfig() error {
	if m.Package != (PackageConfig{}) || len(m.Exports) != 0 {
		if !validPackageSource(m.Package.Entry) {
			return fmt.Errorf("package entry must be a canonical relative .km source path")
		}
		if !versionPattern.MatchString(m.Package.MinimumVersion) {
			return fmt.Errorf("package min-kinmokusei must be a complete version")
		}
		if m.Package.Backend != "go" {
			return fmt.Errorf("unsupported package backend %q; expected go", m.Package.Backend)
		}
		if strings.TrimSpace(m.Package.License) == "" {
			return fmt.Errorf("package license identifier must not be empty")
		}
	}
	for submodule, source := range m.Exports {
		if !canonicalPackageSubpath(submodule) || !validPackageSource(source) {
			return fmt.Errorf("invalid package export %q = %q", submodule, source)
		}
	}
	for module, version := range m.Packages {
		if err := validateModulePath(module); err != nil {
			return err
		}
		if !canonicalPackageSubpath(module) {
			return fmt.Errorf("noncanonical package module %q", module)
		}
		if module == m.Project.GoModule {
			return fmt.Errorf("package %q cannot depend on itself", module)
		}
		if !goModuleVersionPattern.MatchString(version) {
			return fmt.Errorf("package %q requires a complete version, got %q", module, version)
		}
	}
	for module, replacement := range m.PackageReplacements {
		if err := validateModulePath(module); err != nil {
			return err
		}
		// Consumer-owned replacements may include siblings, including overrides
		// for transitive dependencies. Dependency manifests never grant this access.
		if replacement == "" || isPortableAbsolutePath(replacement) || strings.ContainsAny(replacement, "\\\x00\r\n") {
			return fmt.Errorf("package replacement %q must be a portable relative path", module)
		}
	}
	return nil
}

func canonicalPackageSubpath(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.HasPrefix(value, "../") && !strings.HasPrefix(value, "/") && path.Clean(value) == value && !strings.ContainsAny(value, "\\:\x00\r\n")
}

func validPackageSource(value string) bool {
	if !canonicalPackageSubpath(value) || filepath.Ext(value) != ".km" {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if excludedPackageDirectory(component) {
			return false
		}
	}
	return true
}

// Source resolution and hashing must agree: code outside the content hash
// must never enter a dependency through its entry, exports, or relative imports.
func excludedPackageDirectory(name string) bool {
	// Match case-insensitive filesystems and Windows' trailing-dot/space aliases.
	name = strings.TrimRight(name, ". ")
	return strings.EqualFold(name, ".git") || strings.EqualFold(name, product.StateDirectoryName)
}
