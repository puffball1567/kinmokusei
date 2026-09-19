package project

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultSourceImportAlias(t *testing.T) {
	t.Parallel()
	for module, want := range map[string]string{
		"github.com/puffball1567/kinmokusei-cli": "kinmokusei-cli",
		"example.com/cli/v2":                     "cli", "example.com/cli/v10": "cli",
		"example.com/cli/v999999999999999999999": "cli",
		"example.com/cli/v1":                     "v1", "example.com/cli/v02": "v02",
		"example.com/cli/version": "version", "gopkg.in/yaml.v3": "yaml.v3",
	} {
		if got := defaultSourceImportAlias(module); got != want {
			t.Fatalf("%s: got %q want %q", module, got, want)
		}
	}
}

func TestSourceImportInstallIsTransactional(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "app")
	packageFiles(t, root, map[string]string{"kinmokusei.toml": packageManifest("app.test/main"), "index.km": ""})
	for _, name := range []string{"first", "second", "broken"} {
		packageFiles(t, filepath.Join(base, name), map[string]string{
			"kinmokusei.toml": packageManifest(name + ".test/tool"),
			"index.km":        `export function value():int{return 42;}`,
		})
	}
	if err := AddAutoDependency(root, "first.test/tool", "v0.1.0", "../first", true); err != nil {
		t.Fatal(err)
	}
	before := map[string]string{}
	for _, name := range []string{"kinmokusei.toml", "kinmokusei.lock", ".kinmokusei/deps/go.mod", ".kinmokusei/deps/go.sum"} {
		contents, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		before[name] = string(contents)
	}
	unchanged := func() {
		t.Helper()
		for name, want := range before {
			contents, err := os.ReadFile(filepath.Join(root, name))
			if err != nil || string(contents) != want {
				t.Fatalf("failed add changed %s: %v", name, err)
			}
		}
	}
	for _, alias := range []string{"", "tool", "tool/sub", "kinmokusei", "../bad", "app.test", "second.test/tool"} {
		if err := AddAutoDependencyWithAlias(root, "second.test/tool", "v0.1.0", "../second", alias, true); err == nil || !strings.Contains(err.Error(), "--alias") {
			t.Fatalf("alias=%q err=%v", alias, err)
		}
		unchanged()
	}
	// Failure after alias registration must roll back both the alias and dependency.
	if err := os.Remove(filepath.Join(base, "broken", "index.km")); err != nil {
		t.Fatal(err)
	}
	if err := AddAutoDependencyWithAlias(root, "broken.test/tool", "v0.1.0", "../broken", "broken", true); err == nil || !strings.Contains(err.Error(), "missing package source") {
		t.Fatalf("missing source err=%v", err)
	}
	unchanged()
	if err := AddAutoDependencyWithAlias(root, "second.test/tool", "v0.1.0", "../second", "other-tool", true); err != nil {
		t.Fatal(err)
	}
	if err := UpdateSourcePackages(root, "", true); err != nil {
		t.Fatal(err)
	}
	manifest, err := ReadManifest(root)
	want := map[string]string{"tool": "first.test/tool", "other-tool": "second.test/tool"}
	if err != nil || !reflect.DeepEqual(manifest.Imports, want) {
		t.Fatalf("aliases after update=%v err=%v", manifest.Imports, err)
	}
}

func TestSourceImportInstallRejectsGoOnlyAlias(t *testing.T) {
	t.Parallel()
	for _, withManifest := range []bool{false, true} {
		root := t.TempDir()
		initial := packageManifest("app.test/main")
		files := map[string]string{"kinmokusei.toml": initial, "index.km": "", "helper/go.mod": "module go.test/helper\n\ngo 1.23\n"}
		if withManifest {
			files["helper/kinmokusei.toml"] = strings.Split(packageManifest("go.test/helper"), "[package]")[0]
		}
		packageFiles(t, root, files)
		if err := AddAutoDependencyWithAlias(root, "go.test/helper", "v0.1.0", "./helper", "short", true); err == nil || !strings.Contains(err.Error(), "only applies to Kinmokusei") {
			t.Fatalf("Go-only alias: %v", err)
		}
		contents, err := os.ReadFile(filepath.Join(root, "kinmokusei.toml"))
		if err != nil || string(contents) != initial {
			t.Fatalf("Go-only rejection changed manifest: %v", err)
		}
	}
}

func TestSourceImportInstallMajorVersion(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "app")
	packageFiles(t, root, map[string]string{"kinmokusei.toml": packageManifest("app.test/main"), "index.km": `import {value} from "tool";`})
	packageFiles(t, filepath.Join(base, "library"), map[string]string{
		"kinmokusei.toml": strings.Replace(packageManifest("pkg.test/tool/v2"), "version = \"0.1.0\"", "version = \"2.1.0\"", 1),
		"index.km":        `export function value():int{return 42;}`,
	})
	if err := AddAutoDependency(root, "pkg.test/tool/v2", "v2.1.0", "../library", true); err != nil {
		t.Fatal(err)
	}
	manifest, lock, err := ValidateLockedFiles(root)
	if err != nil || manifest.Imports["tool"] != "pkg.test/tool/v2" {
		t.Fatalf("major-version alias: %v %v", manifest.Imports, err)
	}
	graph, err := ReadPackageGraph(manifest, lock)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := graph.ResolveImport(filepath.Join(root, "index.km"), "tool")
	if err != nil || graph.SourceIdentity(resolved) != "pkg.test/tool/v2/index.km" {
		t.Fatalf("major-version resolution: %s %v", resolved, err)
	}
}
