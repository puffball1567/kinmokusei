package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestWorkspaceInitialization(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	canonical := canonicalWorkspacePath(root)
	for _, test := range []struct {
		name   string
		params any
		want   []string
	}{
		{"root URI", map[string]any{"rootUri": fileURI(root)}, []string{canonical}},
		{"legacy root path", map[string]any{"rootPath": root}, []string{canonical}},
		{"folders override root", map[string]any{"rootUri": fileURI(root), "workspaceFolders": []workspaceFolder{}}, []string{}},
		{"duplicate folders", map[string]any{"workspaceFolders": []workspaceFolder{{URI: fileURI(root)}, {URI: fileURI(root)}}}, []string{canonical}},
		{"unsupported roots", map[string]any{"workspaceFolders": []workspaceFolder{{URI: "https://example.invalid/project"}, {URI: "file:relative"}}}, []string{}},
		{"no root", nil, []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.params)
			if err != nil {
				t.Fatal(err)
			}
			s := newServer(strings.NewReader(""), &bytes.Buffer{})
			if err := s.initializeWorkspace(encoded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(s.workspaceRoots, test.want) {
				t.Fatalf("roots=%v want=%v", s.workspaceRoots, test.want)
			}
		})
	}
}

func workspacePackageFixture(t *testing.T) (string, string, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "app")
	library := filepath.Join(base, "library")
	input := `export class Box{constructor(public value:int){} public function read():int{return this.value;}}`
	files := map[string]string{
		"app/kinmokusei.toml":     "[project]\nname = \"app\"\nversion = \"0.1.0\"\ngo-module = \"app.test/main\"\ngo-version = \"1.23\"\n[dependencies]\n\"pkg.test/library\" = \"v0.1.0\"\n[replace]\n\"pkg.test/library\" = \"../library\"\n",
		"app/main.km":             `import {Box} from "pkg.test/library";`,
		"library/kinmokusei.toml": "[project]\nname = \"library\"\nversion = \"0.1.0\"\ngo-module = \"pkg.test/library\"\ngo-version = \"1.23\"\n[package]\nentry = \"index.km\"\nmin-kinmokusei = \"0.4.0\"\nbackend = \"go\"\nlicense = \"MIT\"\n",
		"library/index.km":        input,
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
	return canonicalWorkspacePath(root), canonicalWorkspacePath(filepath.Join(library, "index.km")), input
}

func TestWorkspaceExternalPackageWithoutOpenConsumer(t *testing.T) {
	t.Parallel()
	root, library, input := workspacePackageFixture(t)
	uri := fileURI(library)
	for _, folderMode := range []bool{false, true} {
		for _, closeConsumer := range []bool{false, true} {
			t.Run(fmt.Sprintf("folders=%t/close=%t", folderMode, closeConsumer), func(t *testing.T) {
				params := fmt.Sprintf(`{"rootUri":%q}`, fileURI(root))
				if folderMode {
					params = fmt.Sprintf(`{"workspaceFolders":[{"uri":%q,"name":"app"}]}`, fileURI(root))
				}
				initialize := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":%s}`, params)
				var messages []string
				if closeConsumer {
					consumer := fileURI(filepath.Join(root, "main.km"))
					messages = append(messages, openDocument(consumer, `import {Box} from "pkg.test/library";`), openDocument(uri, input), fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didClose","params":{"textDocument":{"uri":%q}}}`, consumer))
				} else {
					messages = append(messages, openDocument(uri, input))
				}
				messages = append(messages, requestAt("textDocument/completion", 2, uri, positionOf(input, "value;", 0), ""), requestAt("textDocument/definition", 3, uri, positionOf(input, "value;", 0), ""))
				responses := serveExternalPackageMessagesInitialized(t, initialize, messages...)
				found := false
				for _, item := range responses[2]["result"].([]any) {
					found = found || item.(map[string]any)["label"] == "value"
				}
				if !found {
					t.Fatalf("completion=%v", responses[2])
				}
				definition, ok := responses[3]["result"].(map[string]any)
				if !ok || definition["uri"] != uri {
					t.Fatalf("definition=%v", responses[3])
				}
			})
		}
	}
}

func workspaceChange(added, removed []workspaceFolder) json.RawMessage {
	value, _ := json.Marshal(map[string]any{"event": map[string]any{"added": added, "removed": removed}})
	return value
}

func TestWorkspaceChangesRefreshExternalDiagnosticsAndSnapshots(t *testing.T) {
	t.Parallel()
	root, library, input := workspacePackageFixture(t)
	uri := fileURI(library)
	var output bytes.Buffer
	s := newServer(strings.NewReader(""), &output)
	s.documents[uri] = document{URI: uri, Path: library, Text: input, Version: 1}
	assertDiagnostics := func(wantError bool) {
		t.Helper()
		got := s.diagnostics[uri][uri]
		if (len(got) != 0) != wantError {
			t.Fatalf("diagnostics=%v wantError=%t", got, wantError)
		}
		if wantError && !strings.Contains(got[0].Message, "lock is missing") {
			t.Fatalf("unexpected error=%v", got)
		}
	}
	if err := s.publishFor(uri); err != nil {
		t.Fatal(err)
	}
	assertDiagnostics(true)
	folders := []workspaceFolder{{URI: fileURI(root)}}
	if err := s.didChangeWorkspaceFolders(workspaceChange(folders, nil)); err != nil {
		t.Fatal(err)
	}
	assertDiagnostics(false)
	snapshot := s.requestSnapshot()
	generation := s.generation
	for _, noChange := range []json.RawMessage{workspaceChange(folders, nil), []byte(`{"event":"invalid"}`), []byte(`{}`)} {
		if err := s.didChangeWorkspaceFolders(noChange); err != nil {
			t.Fatal(err)
		}
	}
	if generation != s.generation {
		t.Fatal("no-op workspace event invalidated requests")
	}
	if err := s.didChangeWorkspaceFolders(workspaceChange(nil, folders)); err != nil {
		t.Fatal(err)
	}
	assertDiagnostics(true)
	if s.generation != generation+1 || len(snapshot.workspaceRoots) != 1 || len(s.workspaceRoots) != 0 {
		t.Fatal("workspace snapshot/generation was not preserved")
	}
	result, err := snapshot.checkDocument(snapshot.documents[uri], map[string]string{library: input})
	if err != nil || len(result.Diagnostics) != 0 {
		t.Fatalf("snapshot analysis=%v err=%v", result.Diagnostics, err)
	}
	if err := s.didChangeWorkspaceFolders(workspaceChange(folders, nil)); err != nil {
		t.Fatal(err)
	}
	assertDiagnostics(false)
}

func TestWorkspaceProtocolLifecycle(t *testing.T) {
	t.Parallel()
	root, library, input := workspacePackageFixture(t)
	uri := fileURI(library)
	folders := []workspaceFolder{{URI: fileURI(root)}}
	notification := func(params json.RawMessage) string {
		return fmt.Sprintf(`{"jsonrpc":"2.0","method":"workspace/didChangeWorkspaceFolders","params":%s}`, params)
	}
	messages, err := serveAsyncTest(t, nil,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"workspaceFolders":"invalid"}}`,
		fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"rootUri":%q}}`, fileURI(root)),
		`{"jsonrpc":"2.0","id":3,"method":"workspace/didChangeWorkspaceFolders","params":{}}`,
		openDocument(uri, input),
		notification(workspaceChange(nil, folders)),
		notification(workspaceChange(folders, nil)),
		requestAt("textDocument/completion", 4, uri, positionOf(input, "value;", 0), ""),
		`{"jsonrpc":"2.0","id":90,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]float64{"1": -32602, "3": -32600} {
		responses := responsesForRawID(messages, id)
		if len(responses) != 1 || responses[0]["error"].(map[string]any)["code"] != want {
			t.Fatalf("response %s=%v", id, responses)
		}
	}
	capabilities := responsesForRawID(messages, "2")[0]["result"].(map[string]any)["capabilities"].(map[string]any)
	workspace := capabilities["workspace"].(map[string]any)["workspaceFolders"].(map[string]any)
	if workspace["supported"] != true || workspace["changeNotifications"] != true {
		t.Fatalf("capabilities=%v", workspace)
	}
	var counts []int
	for _, message := range messages {
		if message["method"] == "textDocument/publishDiagnostics" {
			params := message["params"].(map[string]any)
			if params["uri"] == uri {
				counts = append(counts, len(params["diagnostics"].([]any)))
			}
		}
	}
	if !reflect.DeepEqual(counts, []int{0, 1, 0}) {
		t.Fatalf("diagnostic transitions=%v", counts)
	}
	completion := responsesForRawID(messages, "4")
	if len(completion) != 1 || len(completion[0]["result"].([]any)) == 0 {
		t.Fatalf("completion after workspace change=%v", completion)
	}
}

func TestWorkspaceChangeSuppressesStaleRequest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	uri := fileURI(filepath.Join(root, "value.km"))
	input := `function value(input:int):int{return input;}`
	change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"workspace/didChangeWorkspaceFolders","params":%s}`, workspaceChange([]workspaceFolder{{URI: fileURI(root)}}, nil))
	var output bytes.Buffer
	s := newServer(strings.NewReader(framed(
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "value", 0), ""),
		change,
		requestAt("textDocument/hover", 3, uri, positionOf(input, "value", 0), ""),
		`{"jsonrpc":"2.0","id":90,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	)), &output)
	s.beforeRequest = func(ctx context.Context, _ request) {
		for {
			s.requestMu.Lock()
			generation := s.generation
			s.requestMu.Unlock()
			if generation >= 2 {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Millisecond):
			}
		}
	}
	if err := s.serve(); err != nil {
		t.Fatal(err)
	}
	messages := decodeMessages(t, output.String())
	stale := responsesForRawID(messages, "2")
	if len(stale) != 1 || stale[0]["error"].(map[string]any)["code"] != float64(contentModifiedCode) {
		t.Fatalf("stale response=%v", stale)
	}
	current := responsesForRawID(messages, "3")
	if len(current) != 1 || current[0]["error"] != nil || current[0]["result"] == nil {
		t.Fatalf("current response=%v", current)
	}
}
