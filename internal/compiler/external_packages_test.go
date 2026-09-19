package compiler

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func externalManifest(module string, library bool) string {
	text := fmt.Sprintf("[project]\nname = \"fixture\"\nversion = \"0.1.0\"\ngo-module = %q\ngo-version = \"1.23\"\n", module)
	if library {
		text += "[package]\nentry = \"index.km\"\nmin-kinmokusei = \"0.4.0\"\nbackend = \"go\"\nlicense = \"MIT\"\n"
	}
	return text
}

func TestExternalSourcePackagesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	var previous []byte
	for attempt := 0; attempt < 2; attempt++ {
		base := t.TempDir()
		if attempt == 1 {
			alias := filepath.Join(t.TempDir(), "checkout")
			if err := os.Symlink(base, alias); err == nil {
				base = alias
			} else {
				t.Logf("checkout symlink unavailable: %v", err)
			}
		}
		root := filepath.Join(base, "app")
		files := map[string]string{
			"app/kinmokusei.toml":            externalManifest("external-source-packages.test", false) + "[dependencies]\n\"pkg.test/first\" = \"v0.1.0\"\n\"pkg.test/second\" = \"v0.1.0\"\n[replace]\n\"pkg.test/first\" = \"../first\"\n\"pkg.test/second\" = \"../second\"\n",
			"first/kinmokusei.toml":          externalManifest("pkg.test/first", true) + "[exports]\n\"api\" = \"src/api.km\"\n",
			"first/index.km":                 `export {Box} from "./src/api";`,
			"first/src/api.km":               `import {value} from "./value";export class Box{public function read():int{return value();}}`,
			"first/src/value.km":             `export function value():int{return 20;}`,
			"first/examples/unconfigured.km": `import go sample from "missing.test/sample";`,
			"second/kinmokusei.toml":         externalManifest("pkg.test/second", true) + "[dependencies]\n\"pkg.test/first\" = \"v0.1.0\"\n[imports]\n\"second\" = \"pkg.test/first\"\n",
			"second/index.km":                `import {Box} from "second";function value():int{return new Box().read()+2;}export function second():int{return value();}`,
			"app/reexports.km":               `export {Box} from "first/api";`,
			"app/identity.km":                `import {Box} from "./reexports";export function identity(value:Box):Box{return value;}`,
			"app/main.km":                    `import {Box} from "pkg.test/first/api";import {second} from "pkg.test/second";import {identity} from "./identity";export function Answer():int{return identity(new Box()).read()+second();}`,
		}
		files["app/kinmokusei.toml"] += "[imports]\n\"first\" = \"pkg.test/first\"\n\"second\" = \"pkg.test/second\"\n"
		if attempt == 1 {
			files["app/main.km"] = strings.ReplaceAll(strings.ReplaceAll(files["app/main.km"], "pkg.test/first", "first"), "pkg.test/second", "second")
		}
		for name, source := range files {
			file := filepath.Join(base, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := project.LockDependencies(root, true); err != nil {
			t.Fatal(err)
		}
		entry := filepath.Join(root, "main.km")
		directory, diagnostics, err := WriteGeneratedModule([]string{entry}, "externalpackages")
		if err != nil || len(diagnostics) != 0 {
			t.Fatalf("diagnostics=%v err=%v", diagnostics, err)
		}
		generated, err := os.ReadFile(filepath.Join(directory, "generated.go"))
		if err != nil {
			t.Fatal(err)
		}
		if attempt != 0 && !bytes.Equal(generated, previous) {
			t.Fatalf("generated code depends on checkout path or import alias\n%s\n%s", generated, previous)
		}
		previous = generated
		reference := `package reference
type box struct{}
func (box)read()int{return 20}
func Answer()int{return box{}.read()+22}
`
		comparison := `package externalpackages_test
import("testing";g "external-source-packages.test";r "external-source-packages.test/reference")
func TestAnswer(t *testing.T){if got,want:=g.Answer(),r.Answer();got!=want{t.Fatalf("got %d want %d",got,want)}}
`
		runGeneratedGoDifferentialTestInExistingModule(t, directory, "external-source-packages.test", generated, reference, comparison, []string{"test", "-mod=readonly", "./..."}, []string{"GOPROXY=off"})
		result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{filepath.Join(base, "first", "src", "value.km"): `export function value():int{return "wrong";}`})
		if err != nil || len(result.Diagnostics) == 0 {
			t.Fatalf("external overlay diagnostics=%v err=%v", result.Diagnostics, err)
		}
		for _, bad := range []string{
			`import {Box} from "pkg.test/first/private";`,
			`import {value} from "pkg.test/second";`,
			`import {Box} from "pkg.test/unregistered";`,
		} {
			result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: bad})
			if err != nil || len(result.Diagnostics) == 0 {
				t.Fatalf("invalid import accepted: %s err=%v", bad, err)
			}
		}
		result, err = CheckFilesWithOverlay([]string{entry}, map[string]string{filepath.Join(base, "second", "index.km"): `import {Box} from "first";export function second():int{return new Box().read();}`})
		if err != nil || len(result.Diagnostics) == 0 || !strings.Contains(result.Diagnostics[0].Message, "not declared") {
			t.Fatalf("consumer alias leaked into dependency: diagnostics=%v err=%v", result.Diagnostics, err)
		}
		result, err = CheckFilesWithOverlay([]string{entry}, map[string]string{filepath.Join(base, "first", "src", "value.km"): `import {second} from "../../second/index";export function value():int{return second();}`})
		if err != nil || len(result.Diagnostics) == 0 || !strings.Contains(result.Diagnostics[0].Message, "source path") {
			t.Fatalf("escaping import diagnostics=%v err=%v", result.Diagnostics, err)
		}
		result, err = CheckFilesWithOverlay([]string{entry}, map[string]string{filepath.Join(base, "first", "src", "value.km"): `import go helper from "pkg.test/helper";export function value():int{return helper.Value();}`})
		if err != nil || len(result.Diagnostics) == 0 || !strings.Contains(result.Diagnostics[0].Message, "[go.dependencies]") {
			t.Fatalf("undeclared Go import diagnostics=%v err=%v", result.Diagnostics, err)
		}
	}
}
