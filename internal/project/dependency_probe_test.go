package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourcePackageProbeUsesPublicSourceGraph(t *testing.T) {
	t.Parallel()
	for _, replacement := range []string{"../library", "./vendor/library"} {
		t.Run(replacement, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "app")
			library := filepath.Join(root, filepath.FromSlash(replacement))
			packageFiles(t, root, map[string]string{
				"kinmokusei.toml": packageManifest("app.test/main") + fmt.Sprintf("[dependencies]\n\"pkg.test/library\" = \"v0.1.0\"\n[replace]\n\"pkg.test/library\" = %q\n", replacement),
				"index.km":        `import {value} from "pkg.test/library";`,
			})
			packageFiles(t, library, map[string]string{
				"kinmokusei.toml": packageManifest("pkg.test/library") + "[exports]\n\"extra\" = \"src/extra.km\"\n",
				"index.km":        `export {value} from "./src/value";import {extra} from "pkg.test/library/extra";`,
				"src/value.km":    `import go fmt from "fmt";import {helper} from "./shared.km";export function value():int{return helper();}`,
				"src/extra.km":    `import go strconv from "strconv";export {helper as extra} from "./shared";`,
				"src/shared.km":   `import go strings from "strings";import {fetch} from "kinmokusei/http";export function helper():int{return 1;}`,
				// Unpublished files must not acquire dependencies or block locking,
				// even when the replacement lives inside the application directory.
				"examples/remote.km":  `import go example from "missing.test/example";`,
				"tests/unfinished.km": `export function (`,
			})
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
			destination := t.TempDir()
			if err := writeDependencyProbe(root, destination, graph); err != nil {
				t.Fatal(err)
			}
			contents, err := os.ReadFile(filepath.Join(destination, "kinmokusei_dependencies.go"))
			if err != nil {
				t.Fatal(err)
			}
			for _, imported := range []string{"fmt", "strconv", "strings", "net/http"} {
				if strings.Count(string(contents), fmt.Sprintf("_ %q", imported)) != 1 {
					t.Fatalf("missing or repeated %s: %s", imported, contents)
				}
			}
			if strings.Contains(string(contents), "missing.test") {
				t.Fatalf("example leaked into probe: %s", contents)
			}
			if err := CheckDependencies(root); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSourcePackageProbeValidatesReachableFiles(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, extra, want string }{
		{"missing import", `import {value} from "./missing";`, "", "cannot read package source"},
		{"missing reexport", `export {value} from "./missing";`, "", "cannot read package source"},
		{"escape", `export {value} from "../outside";`, "", "invalid package source path"},
		{"cycle", `export {value} from "./other";`, `export {value} from "./index";`, "source import cycle"},
		{"syntax", `export {value} from "./other";`, `export function (`, "invalid package source"},
		{"undeclared source", `export {value} from "pkg.test/undeclared";`, "", "not declared"},
		{"undeclared nested Go", `export {value} from "./other";`, `import go helper from "go.test/undeclared";`, "[go.dependencies]"},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "app")
			packageFiles(t, root, map[string]string{"kinmokusei.toml": packageManifest("app.test/main"), "index.km": ""})
			packageFiles(t, filepath.Join(base, "library"), map[string]string{"kinmokusei.toml": packageManifest("pkg.test/library"), "index.km": test.source, "other.km": test.extra})
			if _, err := LockDependencies(root, true); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(root, "kinmokusei.lock"))
			if err != nil {
				t.Fatal(err)
			}
			if err := AddAutoDependency(root, "pkg.test/library", "v0.1.0", "../library", true); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v want=%s", err, test.want)
			}
			after, err := os.ReadFile(filepath.Join(root, "kinmokusei.lock"))
			if err != nil || string(before) != string(after) {
				t.Fatal("failed package addition changed lock", err)
			}
			if err := CheckDependencies(root); err != nil {
				t.Fatal("failed package addition broke previous graph", err)
			}
		})
	}
}
