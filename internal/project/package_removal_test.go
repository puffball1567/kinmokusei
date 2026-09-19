package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSourcePackageRemovalRetainsSharedAndPrunesUnusedReplacements(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "app")
	packageFiles(t, root, map[string]string{"kinmokusei.toml": packageManifest("app.test/main") + `[dependencies]
"pkg.test/a" = "v0.1.0"
"pkg.test/b" = "v0.1.0"
"pkg.test/shared" = "v0.1.0"
[imports]
"a" = "pkg.test/a"
"b" = "pkg.test/b"
"shared" = "pkg.test/shared"
[replace]
"pkg.test/a" = "../a"
"pkg.test/b" = "../b"
"pkg.test/shared" = "../shared"
"pkg.test/private" = "../private"
`, "index.km": ""})
	for _, name := range []string{"a", "b", "shared", "private"} {
		manifest := packageManifest("pkg.test/" + name)
		if name == "a" {
			manifest += "[dependencies]\n\"pkg.test/private\" = \"v0.1.0\"\n"
		}
		if name == "b" {
			manifest += "[dependencies]\n\"pkg.test/shared\" = \"v0.1.0\"\n"
		}
		packageFiles(t, filepath.Join(base, name), map[string]string{"kinmokusei.toml": manifest, "index.km": ""})
	}
	if _, err := LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		remove string
		want   []string
	}{
		{"shared", []string{"pkg.test/a", "pkg.test/b", "pkg.test/private", "pkg.test/shared"}},
		{"a", []string{"pkg.test/b", "pkg.test/shared"}},
		{"b", []string{}},
	} {
		if step.remove == "a" {
			// Removing a dependency must not need to read its lost checkout.
			if err := os.Rename(filepath.Join(base, "a"), filepath.Join(base, "a-unavailable")); err != nil {
				t.Fatal(err)
			}
		}
		if err := RemoveDependency(root, "pkg.test/"+step.remove, true); err != nil {
			t.Fatalf("remove %s: %v", step.remove, err)
		}
		manifest, err := ReadManifest(root)
		if err != nil {
			t.Fatal(err)
		}
		if got := sortedKeys(manifest.PackageReplacements); !reflect.DeepEqual(got, step.want) {
			t.Fatalf("after removing %s replacements=%v want=%v", step.remove, got, step.want)
		}
		if len(manifest.Imports) != len(manifest.Packages) {
			t.Fatalf("aliases not retained/pruned with direct dependencies: %v", manifest.Imports)
		}
		for alias, module := range manifest.Imports {
			if manifest.Packages[module] == "" {
				t.Fatalf("dangling alias %s -> %s", alias, module)
			}
		}
		if err := CheckDependencies(root); err != nil {
			t.Fatal(err)
		}
	}
}
