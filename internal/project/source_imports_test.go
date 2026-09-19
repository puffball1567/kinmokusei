package project

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSourceImportManifest(t *testing.T) {
	t.Parallel()
	base := packageManifest("app.test/main") + "[dependencies]\n\"pkg.test/library\" = \"v0.1.0\"\n"
	for _, test := range []struct{ name, section, want string }{
		{"simple", `[imports]
"cli" = "pkg.test/library"`, ""},
		{"scoped", `[imports]
"@tools/cli" = "pkg.test/library"`, ""},
		{"missing dependency", `[imports]
"cli" = "pkg.test/other"`, "direct [dependencies]"},
		{"chain", `[imports]
"cli" = "pkg.test/library"
"other" = "cli"`, "direct [dependencies]"},
		{"submodule target", `[imports]
"cli" = "pkg.test/library/sub"`, "direct [dependencies]"},
		{"reserved", `[imports]
"kinmokusei/http" = "pkg.test/library"`, "reserved"},
		{"reserved root", `[imports]
"kinmokusei" = "pkg.test/library"`, "reserved"},
		{"canonical", `[imports]
"pkg.test/library" = "pkg.test/library"`, "conflicts with canonical"},
		{"canonical child", `[imports]
"pkg.test/library/sub" = "pkg.test/library"`, "conflicts with canonical"},
		{"canonical parent", `[imports]
"pkg.test" = "pkg.test/library"`, "conflicts with canonical"},
		{"self", `[imports]
"app.test/main" = "pkg.test/library"`, "conflicts with canonical"},
		{"unquoted", `[imports]
cli = "pkg.test/library"`, "quoted strings"},
		{"duplicate", `[imports]
"cli" = "pkg.test/library"
"cli" = "pkg.test/library"`, "duplicate"},
		{"duplicate section", "[imports]\n[imports]", "duplicate section"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest, err := ParseManifest(filepath.Join(t.TempDir(), "kinmokusei.toml"), []byte(base+test.section))
			if test.want != "" {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("err=%v want=%s", err, test.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			rendered, err := RenderManifest(manifest)
			if err != nil {
				t.Fatal(err)
			}
			again, err := ParseManifest(manifest.Path, rendered)
			if err != nil || !reflect.DeepEqual(manifest.Imports, again.Imports) {
				t.Fatalf("round trip: %v %v", again.Imports, err)
			}
		})
	}
	for _, alias := range []string{"", ".hidden", "./cli", "../cli", "/cli", "cli/", "cli//sub", "cli/../sub", "cli/./sub", `cli\sub`, "C:/cli", "two words", "cli\nsub", "cli\x00sub", "cli?"} {
		_, err := ParseManifest(filepath.Join(t.TempDir(), "kinmokusei.toml"), []byte(base+fmt.Sprintf("[imports]\n%q = \"pkg.test/library\"\n", alias)))
		if err == nil || !strings.Contains(err.Error(), "invalid source import alias") {
			t.Fatalf("alias=%q err=%v", alias, err)
		}
	}
}

func TestSourceImportResolutionIsPackageScoped(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root, library, utility := filepath.Join(base, "app"), filepath.Join(base, "library"), filepath.Join(base, "utility")
	rootManifest := packageManifest("app.test/main") + `[dependencies]
"pkg.test/library" = "v0.1.0"
[imports]
"short" = "pkg.test/library"
"root-only" = "pkg.test/library"
"@tools/lib" = "pkg.test/library"
[replace]
"pkg.test/library" = "../library"
"pkg.test/utility" = "../utility"
`
	packageFiles(t, root, map[string]string{"kinmokusei.toml": rootManifest, "index.km": `export {value} from "short";`})
	packageFiles(t, library, map[string]string{"kinmokusei.toml": packageManifest("pkg.test/library") + `[dependencies]
"pkg.test/utility" = "v0.1.0"
[imports]
"short" = "pkg.test/utility"
"library-only" = "pkg.test/utility"
[exports]
"sub" = "sub.km"
`, "index.km": `export {value} from "short";`, "sub.km": `export {value} from "library-only";`})
	packageFiles(t, utility, map[string]string{"kinmokusei.toml": packageManifest("pkg.test/utility"), "index.km": `export function value():int{return 42;}`})
	lock, err := LockDependencies(root, true)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := ReadPackageGraph(manifest, lock)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ importer, imported, want string }{
		{root, "short", filepath.Join(library, "index.km")},
		{root, "short/sub", filepath.Join(library, "sub.km")},
		{root, "@tools/lib/sub", filepath.Join(library, "sub.km")},
		{root, "pkg.test/library/sub", filepath.Join(library, "sub.km")},
		{library, "short", filepath.Join(utility, "index.km")},
		{library, "pkg.test/library", filepath.Join(library, "index.km")},
	} {
		got, err := graph.ResolveImport(filepath.Join(test.importer, "index.km"), test.imported)
		if err != nil || got != canonicalPackageFile(test.want) {
			t.Fatalf("import %q from %s: %s %v", test.imported, test.importer, got, err)
		}
		if !strings.HasPrefix(graph.SourceIdentity(got), "pkg.test/") {
			t.Fatalf("noncanonical identity: %s", graph.SourceIdentity(got))
		}
	}
	for _, test := range []struct{ importer, imported string }{
		{library, "root-only"}, {root, "library-only"}, {utility, "short"},
		{root, "pkg.test/utility"}, {root, "shorter"}, {root, "short/private"},
		{root, "short/../utility"}, {root, `short\sub`},
	} {
		if _, err := graph.ResolveImport(filepath.Join(test.importer, "index.km"), test.imported); err == nil {
			t.Fatalf("unexpected resolution: %+v", test)
		}
	}
	for _, pkg := range lock.Packages {
		if !strings.HasPrefix(pkg.Path, "pkg.test/") {
			t.Fatalf("alias in lock: %v", pkg)
		}
	}
	// Changing only the aliases is a manifest change, never an implicit relock.
	packageFiles(t, root, map[string]string{"kinmokusei.toml": strings.Replace(rootManifest, "root-only", "renamed", 1)})
	if err := CheckDependencies(root); err == nil || !strings.Contains(err.Error(), "does not match kinmokusei.toml") {
		t.Fatalf("changed alias accepted by old lock: %v", err)
	}
}

func TestSourceImportLongestPrefix(t *testing.T) {
	t.Parallel()
	m := Manifest{Imports: map[string]string{"tool": "pkg.test/a", "tool/special": "pkg.test/b"}}
	for imported, want := range map[string]string{
		"tool": "pkg.test/a", "tool/sub": "pkg.test/a/sub",
		"tool/special": "pkg.test/b", "tool/special/sub": "pkg.test/b/sub",
		"tool/specialized": "pkg.test/a/specialized", "tools": "tools", "./tool": "./tool",
	} {
		if got := m.expandSourceImport(imported); got != want {
			t.Fatalf("%s -> %s, want %s", imported, got, want)
		}
	}
}
