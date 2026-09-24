package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/product"
)

func TestSourcePackageDependencyBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, manifest, source, want string }{
		{"missing entry", strings.Replace(packageManifest("pkg.test/lib"), "index.km", "missing.km", 1), "", "missing package source"},
		{"missing submodule", packageManifest("pkg.test/lib") + "[exports]\n\"sub\" = \"missing.km\"\n", "", "missing package source"},
		{"undeclared Go", packageManifest("pkg.test/lib"), `import go helper from "go.test/helper";`, "without a [go.dependencies] declaration"},
		{"own Go helpers", packageManifest("pkg.test/lib"), `import go helper from "pkg.test/lib/helper";`, "separately declared Go module"},
		{"private replacement", packageManifest("pkg.test/lib") + "[dependencies]\n\"pkg.test/private\" = \"v0.1.0\"\n[replace]\n\"pkg.test/private\" = \"../secret\"\n", "", "cache miss for pkg.test/private@v0.1.0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "app")
			packageFiles(t, root, map[string]string{"kinmokusei.toml": packageManifest("app.test/main") + "[dependencies]\n\"pkg.test/lib\" = \"v0.1.0\"\n[replace]\n\"pkg.test/lib\" = \"../lib\"\n", "index.km": ""})
			packageFiles(t, filepath.Join(base, "lib"), map[string]string{"kinmokusei.toml": test.manifest, "index.km": test.source})
			packageFiles(t, filepath.Join(base, "secret"), map[string]string{"kinmokusei.toml": packageManifest("pkg.test/private"), "index.km": ""})
			if _, err := LockDependencies(root, true); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v want=%s", err, test.want)
			}
		})
	}
}

func TestSourcePackageExcludedDirectoryBoundary(t *testing.T) {
	t.Parallel()
	for _, directory := range []string{".git", product.StateDirectoryName, "nested/.git", "nested/" + product.StateDirectoryName, ".GIT", ".KINMOKUSEI", ".git.", ".kinmokusei "} {
		t.Run(directory, func(t *testing.T) {
			root := t.TempDir()
			source := directory + "/hidden.km"
			packageFiles(t, root, map[string]string{"index.km": "", source: "original"})
			before, err := hashSourcePackage(root)
			if err != nil {
				t.Fatal(err)
			}
			packageFiles(t, root, map[string]string{source: "changed"})
			after, err := hashSourcePackage(root)
			if err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Fatal("metadata directories must remain excluded from the package hash")
			}
			if validPackageSource(source) {
				t.Error("unhashed source accepted by manifest validation")
			}
			if _, err := packageSourceFile(root, source); err == nil {
				t.Error("unhashed source accepted as an entry/export")
			}
			canonical, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			graph := &PackageGraph{Packages: map[string]*SourcePackage{"pkg.test/lib": {Directory: canonical}}}
			if err := graph.ValidateRelativeImport(filepath.Join(root, "index.km"), filepath.Join(root, filepath.FromSlash(source))); err == nil {
				t.Error("relative import of unhashed source accepted")
			}
			link := filepath.Join(root, "alias.km")
			if err := os.Symlink(filepath.Join(root, filepath.FromSlash(source)), link); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			if _, err := packageSourceFile(root, "alias.km"); err == nil {
				t.Error("symlink to unhashed source accepted")
			}
			if err := graph.ValidateRelativeImport(filepath.Join(root, "index.km"), link); err == nil {
				t.Error("relative symlink to unhashed source accepted")
			}
		})
	}
}

func TestSourcePackageConflictingVersions(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "app")
	packageFiles(t, root, map[string]string{"kinmokusei.toml": packageManifest("app.test/main") + "[dependencies]\n\"pkg.test/a\" = \"v0.1.0\"\n\"pkg.test/b\" = \"v0.1.0\"\n[replace]\n\"pkg.test/a\" = \"../a\"\n\"pkg.test/b\" = \"../b\"\n", "index.km": ""})
	packageFiles(t, filepath.Join(base, "a"), map[string]string{"kinmokusei.toml": packageManifest("pkg.test/a"), "index.km": ""})
	packageFiles(t, filepath.Join(base, "b"), map[string]string{"kinmokusei.toml": packageManifest("pkg.test/b") + "[dependencies]\n\"pkg.test/a\" = \"v0.2.0\"\n", "index.km": ""})
	if _, err := LockDependencies(root, true); err == nil || !strings.Contains(err.Error(), "conflicting Kinmokusei package versions for pkg.test/a: v0.1.0 and v0.2.0") {
		t.Fatalf("conflict=%v", err)
	}
}

func TestSourcePackageSymlinkBoundary(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	library := filepath.Join(base, "lib")
	packageFiles(t, base, map[string]string{"outside.km": "", "lib/index.km": ""})
	link := filepath.Join(library, "escape.km")
	if err := os.Symlink(filepath.Join(base, "outside.km"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := packageSourceFile(library, "escape.km"); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("export escape=%v", err)
	}
	canonical, err := filepath.EvalSymlinks(library)
	if err != nil {
		t.Fatal(err)
	}
	graph := &PackageGraph{Packages: map[string]*SourcePackage{"pkg.test/lib": {Directory: canonical}}}
	if err := graph.ValidateRelativeImport(filepath.Join(library, "index.km"), link); err == nil {
		t.Fatal("relative symlink escape accepted")
	}
}
