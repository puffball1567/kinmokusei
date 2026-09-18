package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/product"
)

func TestNewProjectTemplatesAreOfflineLocked(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"app", "library"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "greeting")
			if err := NewProject(kind, root, NewProjectOptions{}); err != nil {
				t.Fatal(err)
			}
			manifest, lock, err := ValidateLockedFiles(root)
			if err != nil {
				t.Fatal(err)
			}
			if manifest.Project.Name != "greeting" || manifest.Project.GoModule != "example.com/greeting" || manifest.Project.Version != "0.1.0" {
				t.Fatalf("manifest=%+v", manifest.Project)
			}
			if len(lock.Modules) != 0 || len(lock.Packages) != 0 || strings.Contains(lock.GoMod, ".keika-new-") {
				t.Fatalf("unexpected dependencies or staging path: %+v", lock)
			}
			entry := "main.km"
			if kind == "library" {
				entry = "index.km"
				if manifest.Package.License != "UNLICENSED" || manifest.Package.Entry != entry || manifest.Package.MinimumVersion != product.DevelopmentCompatibilityVersion {
					t.Fatalf("package=%+v", manifest.Package)
				}
				goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
				if err != nil || string(goMod) != "module example.com/greeting\n\ngo 1.23\n" {
					t.Fatalf("go.mod=%s err=%v", goMod, err)
				}
			}
			for _, name := range []string{entry, "README.md", ".gitignore", product.LockFileName} {
				if info, err := os.Stat(filepath.Join(root, name)); err != nil || !info.Mode().IsRegular() {
					t.Fatalf("missing generated file %s: %v", name, err)
				}
			}
			before, err := os.ReadFile(filepath.Join(root, product.LockFileName))
			if err != nil {
				t.Fatal(err)
			}
			// Relocation/restoration must not depend on the staging directory.
			if err := os.Remove(filepath.Join(product.DependencyDirectory(root), "go.mod")); err != nil {
				t.Fatal(err)
			}
			if err := FetchDependencies(root, true); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(filepath.Join(root, product.LockFileName))
			if err != nil || string(before) != string(after) {
				t.Fatal("fetch changed lock", err)
			}
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 1 {
				t.Fatalf("staging leaked: %v %v", entries, err)
			}
		})
	}
}

func TestNewProjectOptionsAndValidation(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "directory with spaces")
	if err := NewProject("library", root, NewProjectOptions{Name: "library-name", Module: "pkg.test/library", License: "MIT"}); err != nil {
		t.Fatal(err)
	}
	manifest, err := ReadManifest(root)
	if err != nil || manifest.Project.Name != "library-name" || manifest.Project.GoModule != "pkg.test/library" || manifest.Package.License != "MIT" {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
	for _, test := range []struct {
		kind    string
		options NewProjectOptions
		want    string
	}{
		{"unknown", NewProjectOptions{}, "unknown project template"},
		{"app", NewProjectOptions{License: "MIT"}, "only to the library"},
		{"app", NewProjectOptions{Name: "bad name"}, "project name"},
		{"library", NewProjectOptions{Module: "not-a-module"}, "go-module"},
		{"library", NewProjectOptions{License: "   "}, "license"},
	} {
		destination := filepath.Join(t.TempDir(), "new-project")
		if err := NewProject(test.kind, destination, test.options); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%+v: %v", test, err)
		}
		if _, err := os.Lstat(destination); !os.IsNotExist(err) {
			t.Fatalf("invalid options created destination: %v", err)
		}
	}
	if err := NewProject("app", "", NewProjectOptions{}); err == nil {
		t.Fatal("accepted empty destination")
	}
	if err := NewProject("app", filepath.Join(t.TempDir(), "missing", "child"), NewProjectOptions{}); err == nil {
		t.Fatal("accepted missing parent")
	}
}

func TestNewProjectNeverOverwritesDestination(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"empty directory", "populated directory", "file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "existing")
			switch kind {
			case "empty directory", "populated directory":
				if err := os.Mkdir(root, 0o755); err != nil {
					t.Fatal(err)
				}
				if kind == "populated directory" {
					if err := os.WriteFile(filepath.Join(root, "main.km"), []byte("user source"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			case "file":
				if err := os.WriteFile(root, []byte("user file"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(filepath.Join(parent, "absent"), root); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			if err := NewProject("app", root, NewProjectOptions{}); err == nil || !strings.Contains(err.Error(), "already exists") {
				t.Fatalf("overwrite check=%v", err)
			}
			if kind == "populated directory" {
				contents, err := os.ReadFile(filepath.Join(root, "main.km"))
				if err != nil || string(contents) != "user source" {
					t.Fatal("source changed", err)
				}
			}
			if kind == "file" {
				contents, err := os.ReadFile(root)
				if err != nil || string(contents) != "user file" {
					t.Fatal("file changed", err)
				}
			}
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 1 {
				t.Fatalf("unexpected writes: %v %v", entries, err)
			}
		})
	}
}

func TestNewProjectToolchainFailureLeavesNoDestination(t *testing.T) {
	parent := t.TempDir()
	// Initialize the process-wide target inventory before simulating a missing
	// toolchain; do not cache a deliberate failure for unrelated tests.
	if _, err := ResolveTarget(TargetConfig{}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	if err := NewProject("app", filepath.Join(parent, "app"), NewProjectOptions{}); err == nil || !strings.Contains(err.Error(), "destination was not created") {
		t.Fatalf("missing toolchain error=%v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed staging leaked: %v %v", entries, err)
	}
}

func TestNewProjectPublicationRechecksDestination(t *testing.T) {
	t.Parallel()
	staging := t.TempDir()
	if err := os.WriteFile(filepath.Join(staging, "main.km"), []byte("new source"), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := publishNewProject(staging, destination); err == nil {
		t.Fatal("replaced a destination created after preflight")
	}
	entries, err := os.ReadDir(destination)
	if err != nil || len(entries) != 0 {
		t.Fatalf("destination changed: %v %v", entries, err)
	}
}

func TestNewProjectPublicationFailureRetainsPartialOutput(t *testing.T) {
	t.Parallel()
	staging := t.TempDir()
	if err := os.WriteFile(filepath.Join(staging, "a.km"), []byte("prepared source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.km", filepath.Join(staging, "z.km")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	destination := filepath.Join(t.TempDir(), "project")
	err := publishNewProject(staging, destination)
	if err == nil || !strings.Contains(err.Error(), "partial output retained") || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("publication error=%v", err)
	}
	contents, err := os.ReadFile(filepath.Join(destination, "a.km"))
	if err != nil || string(contents) != "prepared source" {
		t.Fatalf("partial output=%q err=%v", contents, err)
	}
	if _, err := os.Lstat(filepath.Join(destination, "z.km")); !os.IsNotExist(err) {
		t.Fatalf("published unexpected staged link: %v", err)
	}
}
