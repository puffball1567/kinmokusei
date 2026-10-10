package compiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/stdlib"
)

func TestGoSourceMapStableAcrossEntryPoints(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":         "module sourcemapfixture\n\ngo 1.23\n",
		"shared.km":      "export function choose(flag: boolean): int {\n if (flag) {\n return 1;\n }\n return 2;\n}\n",
		"first.test.km":  "import { choose } from \"./shared\"; function first(): int { return choose(true); }",
		"second.test.km": "import { choose } from \"./shared\"; function second(): int { return choose(false); }",
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var previous map[string]bool
	for _, name := range []string{"first.test.km", "second.test.km"} {
		path := filepath.Join(root, name)
		if name == "second.test.km" {
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			path, err = filepath.Rel(cwd, path)
			if err != nil {
				t.Fatal(err)
			}
		}
		artifacts, diagnostics, err := EmitGoWithSourceMap([]string{path}, "sample")
		if err != nil || len(diagnostics) > 0 {
			t.Fatalf("emit: %v %v", err, diagnostics)
		}
		plain, _, err := EmitGo([]string{path}, "sample")
		if err != nil || !bytes.Equal(plain, artifacts.GoSource) {
			t.Fatalf("generated output differs: %v", err)
		}
		validateSourceMap(t, artifacts, files)
		sharedID := ""
		for _, entry := range artifacts.SourceMap.Sources {
			if entry.Path == "shared.km" {
				sharedID = entry.ID
			}
		}
		origins := map[string]bool{}
		for _, mapping := range artifacts.SourceMap.Mappings {
			if mapping.SourceID == sharedID {
				origins[mapping.OriginID] = true
			}
		}
		if len(origins) != 3 {
			t.Fatalf("expected if and two returns: %v", origins)
		}
		if previous != nil {
			if len(previous) != len(origins) {
				t.Fatal("different origin sets")
			}
			for id := range previous {
				if !origins[id] {
					t.Fatal("entry point changed origin identity")
				}
			}
		}
		previous = origins
	}
	otherRoot := t.TempDir()
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(otherRoot, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	relocated, diagnostics, err := EmitGoWithSourceMap([]string{filepath.Join(otherRoot, "first.test.km")}, "sample")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("relocated emit: %v %v", err, diagnostics)
	}
	for id := range previous {
		found := false
		for _, mapping := range relocated.SourceMap.Mappings {
			if mapping.OriginID == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("checkout relocation changed origin %s", id)
		}
	}
}

func TestGoSourceMapEmbeddedInputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "entry.km")
	input := `import { Response } from "kinmokusei/http"; function identity(value: Response): Response { return value; }`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts, diagnostics, err := EmitGoWithSourceMap([]string{path}, "sample")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	standard, ok := stdlib.Lookup("kinmokusei/http")
	if !ok {
		t.Fatal("missing embedded module")
	}
	validateSourceMap(t, artifacts, map[string]string{"entry.km": input, standard.VirtualPath: standard.Contents})
	found := false
	for _, entry := range artifacts.SourceMap.Sources {
		if entry.Path == standard.VirtualPath {
			found = entry.Embedded
		}
	}
	if !found {
		t.Fatal("embedded source was presented as a disk file")
	}
	plain, _, err := EmitGo([]string{path}, "sample")
	if err != nil || !bytes.Equal(plain, artifacts.GoSource) {
		t.Fatalf("embedded imports changed generated output: %v", err)
	}
}

func validateSourceMap(t *testing.T, artifacts GoArtifacts, inputs map[string]string) {
	t.Helper()
	metadata := artifacts.SourceMap
	if metadata.Version != 1 || metadata.Generated.SHA256 != mapHash(artifacts.GoSource) || metadata.Generated.ByteLength != len(artifacts.GoSource) {
		t.Fatalf("invalid header: %+v", metadata)
	}
	sources := map[string]MapSource{}
	for _, entry := range metadata.Sources {
		input, ok := inputs[entry.Path]
		if !ok || entry.SHA256 != mapHash([]byte(input)) || entry.ByteLength != len(input) || filepath.IsAbs(entry.Path) {
			t.Fatalf("invalid source: %+v", entry)
		}
		sources[entry.ID] = entry
	}
	for i, mapping := range metadata.Mappings {
		entry, ok := sources[mapping.SourceID]
		if !ok || len(mapping.OriginID) != 64 {
			t.Fatalf("invalid identity: %+v", mapping)
		}
		for _, part := range []struct {
			r    MapRange
			text string
		}{{mapping.Generated, string(artifacts.GoSource)}, {mapping.Source, inputs[entry.Path]}} {
			if part.r.Start.Offset >= part.r.End.Offset || part.r.Start.Offset < 0 || part.r.End.Offset > len(part.text) {
				t.Fatalf("invalid range: %+v", part.r)
			}
			for _, position := range []MapPosition{part.r.Start, part.r.End} {
				prefix := part.text[:position.Offset]
				if position.Line != strings.Count(prefix, "\n")+1 || position.Column != len(prefix)-strings.LastIndex(prefix, "\n") {
					t.Fatalf("invalid coordinates: %+v", position)
				}
			}
		}
		if i > 0 && metadata.Mappings[i-1].Generated.End.Offset > mapping.Generated.Start.Offset {
			t.Fatal("overlapping ranges")
		}
	}
}

func TestGoSourceMapCapturesCheckedInput(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "text.km")
	input := "function Text(): string {\r\n let text: string = \"金木犀\";\r\n return text;\r\n}\r\n"
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := CheckFiles([]string{path})
	if err != nil || len(result.Diagnostics) > 0 {
		t.Fatalf("check: %v %v", err, result.Diagnostics)
	}
	if err := os.WriteFile(path, []byte("changed after checking"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts, _, err := emitCheckedGoWithSourceMap(result, "sample", root)
	if err != nil {
		t.Fatal(err)
	}
	validateSourceMap(t, artifacts, map[string]string{"text.km": input})
	goPath := filepath.Join(root, "output.go")
	mapPath := filepath.Join(root, "maps", "output.json")
	data, err := artifacts.SourceMapJSON(goPath, mapPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata GoSourceMap
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.SourceRoot != ".." || metadata.Generated.File != "../output.go" || strings.Contains(string(data), root) {
		t.Fatalf("nonportable paths:\n%s", data)
	}
}

func TestGoSourceMapGeneratedModuleAndSafeOutputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "main.km")
	input := "function main(): void { let value = 1; }"
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	directory, diagnostics, err := WriteGeneratedModule([]string{path}, "main")
	if err != nil || len(diagnostics) > 0 {
		t.Fatalf("module: %v %v", err, diagnostics)
	}
	generated, err := os.ReadFile(filepath.Join(directory, "generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "generated.go.map.json"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata GoSourceMap
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Generated.File != "generated.go" || metadata.Generated.SHA256 != mapHash(generated) {
		t.Fatal("sidecar does not match generated module")
	}
	rebased := filepath.Clean(filepath.Join(directory, filepath.FromSlash(metadata.SourceRoot)))
	if rebased != root {
		t.Fatalf("source root = %s, want %s", rebased, root)
	}
	artifacts, _, err := EmitGoWithSourceMap([]string{path}, "main")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "output.go")
	for _, pair := range [][2]string{{output, output}, {path, output}, {output, path}} {
		if err := artifacts.WriteFiles(pair[0], pair[1]); err == nil {
			t.Fatalf("unsafe paths accepted: %v", pair)
		}
	}
	alias := filepath.Join(root, "alias.km")
	if err := os.Link(path, alias); err == nil {
		if err := artifacts.WriteFiles(output, alias); err == nil {
			t.Fatal("hard-linked source overwritten")
		}
	}
	after, _ := os.ReadFile(path)
	if string(after) != input {
		t.Fatal("input changed")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("failed validation wrote output")
	}
}

func TestGoSourceMapFailureAndCoverage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "user.test.km")
	input := "function Choose(flag: boolean): int {\n if (flag) {\n return 1;\n }\n return 2;\n}\nfunction Crash(): int {\n let values: int[] = [];\n return values[0];\n}\n"
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts, diagnostics, err := EmitGoWithSourceMap([]string{path}, "sample")
	if err != nil || len(diagnostics) > 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
func Choose(flag bool) int { if flag { return 1 }; return 2 }
func Crash() int { values := []int{}; return values[0] }
`
	test := `package sample
import ("testing"; "runtime"; "os"; "strconv"; "strings"; "sourcemapruntime/reference")
func panicValue(run func() int) (caught bool) { defer func() { caught = recover() != nil }(); run(); return }
func captureCrash() { defer func() {
 if recover() == nil { panic("missing panic") }
 pcs := make([]uintptr, 32); count := runtime.Callers(0, pcs); frames := runtime.CallersFrames(pcs[:count])
 for { frame, more := frames.Next(); if strings.HasSuffix(frame.Function, ".Crash") {
  if err := os.WriteFile("failure.line", []byte(strconv.Itoa(frame.Line)), 0600); err != nil { panic(err) }; return
 }; if !more { panic("missing source frame") } }
 }(); Crash() }
func TestBehavior(t *testing.T) {
 if got, want := Choose(true), reference.Choose(true); got != want { t.Fatalf("choose = %d, want %d", got, want) }
 if got, want := panicValue(Crash), panicValue(reference.Crash); got != want { t.Fatalf("panic = %v, want %v", got, want) }
 captureCrash()
}
`
	runGeneratedGoDifferentialTestConfigured(t, root, "sourcemapruntime", stringPointer("module sourcemapruntime\n\ngo 1.23\n"), artifacts.GoSource, reference, test, []string{"test", "-coverprofile=coverage.out", "./..."}, nil)
	lineText, err := os.ReadFile(filepath.Join(root, "failure.line"))
	if err != nil {
		t.Fatal(err)
	}
	line, err := strconv.Atoi(string(lineText))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, mapping := range artifacts.SourceMap.Mappings {
		if mapping.Generated.Start.Line <= line && mapping.Generated.End.Line >= line && mapping.Source.Start.Line == 9 {
			found = true
		}
	}
	if !found {
		t.Fatalf("runtime failure line %d not mapped to source line 9", line)
	}
	profile, err := os.ReadFile(filepath.Join(root, "coverage.out"))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[int]int{}
	for _, record := range strings.Split(string(profile), "\n") {
		var sl, sc, el, ec, statements, count int
		if _, err := fmt.Sscanf(record, "sourcemapruntime/generated.go:%d.%d,%d.%d %d %d", &sl, &sc, &el, &ec, &statements, &count); err != nil {
			continue
		}
		for _, mapping := range artifacts.SourceMap.Mappings {
			// Test only contained return fragments; consumers must split blocks
			// by generated ranges, not mark every line in a source if-span.
			start, end := mapping.Generated.Start, mapping.Generated.End
			if (start.Line > sl || start.Line == sl && start.Column >= sc) && (end.Line < el || end.Line == el && end.Column <= ec) {
				counts[mapping.Source.Start.Line] += count
			}
		}
	}
	if counts[3] == 0 {
		t.Fatalf("executed return not covered: %s", profile)
	}
	if count, exists := counts[5]; !exists || count != 0 {
		t.Fatalf("unexecuted return incorrectly covered: %v\n%s", counts, profile)
	}
}

func stringPointer(value string) *string { return &value }
