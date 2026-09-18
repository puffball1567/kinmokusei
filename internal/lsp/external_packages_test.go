package lsp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestExternalPackageEditor(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "app")
	library := filepath.Join(base, "library")
	input := `import {Box} from "pkg.test/library";function run():int{const box=new Box(42);return box.value;}`
	files := map[string]string{
		"app/kinmokusei.toml":     "[project]\nname = \"app\"\nversion = \"0.1.0\"\ngo-module = \"app.test/main\"\ngo-version = \"1.23\"\n[dependencies]\n\"pkg.test/library\" = \"v0.1.0\"\n[replace]\n\"pkg.test/library\" = \"../library\"\n",
		"app/main.km":             input,
		"library/kinmokusei.toml": "[project]\nname = \"library\"\nversion = \"0.1.0\"\ngo-module = \"pkg.test/library\"\ngo-version = \"1.23\"\n[package]\nentry = \"index.km\"\nmin-kinmokusei = \"0.4.0\"\nbackend = \"go\"\nlicense = \"MIT\"\n",
		"library/index.km":        `export class Box{constructor(public value:int){}}`,
	}
	for name, contents := range files {
		file := filepath.Join(base, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	uri := fileURI(filepath.Join(root, "main.km"))
	messages := serveExternalPackageMessages(t, openDocument(uri, input), requestAt("textDocument/definition", 2, uri, positionOf(input, "Box(42)", 0), ""), requestAt("textDocument/completion", 3, uri, positionOf(input, "value;", 0), ""))
	definition, ok := messages[2]["result"].(map[string]any)
	canonical, err := filepath.EvalSymlinks(filepath.Join(library, "index.km"))
	if err != nil {
		t.Fatal(err)
	}
	if !ok || definition["uri"] != fileURI(canonical) {
		t.Fatalf("definition=%v", messages[2])
	}
	found := false
	for _, raw := range messages[3]["result"].([]any) {
		found = found || raw.(map[string]any)["label"] == "value"
	}
	if !found {
		t.Fatalf("completion=%v", messages[3])
	}
	// Following the definition opens a library without its own lock. It must
	// retain the consumer's dependency context, including unsaved edits.
	libraryURI := fileURI(canonical)
	libraryInput := `export class Box{constructor(public value:int){} function read():int{return this.value;}}`
	s := &Server{documents: map[string]document{
		uri:        {Path: filepath.Join(root, "main.km"), Text: input},
		libraryURI: {Path: canonical, Text: libraryInput},
	}}
	result, checkErr := s.checkDocument(s.documents[libraryURI], map[string]string{canonical: libraryInput})
	if checkErr != nil || len(result.Diagnostics) != 0 {
		t.Fatalf("library context: %v %v", checkErr, result.Diagnostics)
	}
	messages = serveExternalPackageMessages(t, openDocument(uri, input), openDocument(libraryURI, libraryInput), requestAt("textDocument/completion", 4, libraryURI, positionOf(libraryInput, "value;", 0), ""))
	found = false
	for _, raw := range messages[4]["result"].([]any) {
		found = found || raw.(map[string]any)["label"] == "value"
	}
	if !found {
		t.Fatalf("library completion=%v", messages)
	}
}

func serveExternalPackageMessages(t *testing.T, requests ...string) map[float64]map[string]any {
	t.Helper()
	return serveExternalPackageMessagesInitialized(t, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, requests...)
}

func serveExternalPackageMessagesInitialized(t *testing.T, initialize string, requests ...string) map[float64]map[string]any {
	t.Helper()
	requests = append([]string{initialize}, requests...)
	requests = append(requests, `{"jsonrpc":"2.0","id":900,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(requests...)), &output); err != nil {
		t.Fatal(err)
	}
	result := map[float64]map[string]any{}
	diagnostics := 0
	for _, message := range decodeMessages(t, output.String()) {
		if message["method"] == "textDocument/publishDiagnostics" {
			diagnostics++
			if len(message["params"].(map[string]any)["diagnostics"].([]any)) != 0 {
				t.Fatalf("diagnostics=%v", message)
			}
		}
		if id, ok := message["id"].(float64); ok {
			result[id] = message
		}
	}
	if diagnostics == 0 {
		t.Fatal("no diagnostics published")
	}
	return result
}
