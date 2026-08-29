package lsp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/source"
)

func requestAt(method string, id int, uri string, at position, extra string) string {
	if extra != "" {
		extra = "," + extra
	}
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%q,"params":{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}%s}}`, id, method, uri, at.Line, at.Character, extra)
}

func positionOf(text, needle string, occurrence int) position {
	offset := -1
	remaining := text
	base := 0
	for index := 0; index <= occurrence; index++ {
		found := strings.Index(remaining, needle)
		if found < 0 {
			return position{Line: -1, Character: -1}
		}
		offset = base + found
		base = offset + len(needle)
		remaining = text[base:]
	}
	line := strings.Count(text[:offset], "\n")
	lineStart := strings.LastIndex(text[:offset], "\n") + 1
	character := len(utf16.Encode([]rune(text[lineStart:offset])))
	return position{Line: line, Character: character}
}

func serveMessages(t *testing.T, messages ...string) map[float64]map[string]any {
	t.Helper()
	all := append([]string{`{"jsonrpc":"2.0","id":1,"method":"initialize"}`}, messages...)
	all = append(all, `{"jsonrpc":"2.0","id":900,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(all...)), &output); err != nil {
		t.Fatal(err)
	}
	result := map[float64]map[string]any{}
	for _, message := range decodeMessages(t, output.String()) {
		if id, ok := message["id"].(float64); ok {
			result[id] = message
		}
	}
	return result
}

func openDocument(uri, text string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":%q}}}`, uri, text)
}

func TestReferencesUseSemanticIdentityAcrossShadowing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shadow.otm")
	uri := fileURI(path)
	text := `function compute(value: int): int {
  const first: int = value;
  if (true) { const value: int = 2; const nested: int = value; }
  return value + first;
}`
	outer := positionOf(text, "value", 0)
	inner := positionOf(text, "value", 2)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/references", 2, uri, outer, `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/references", 3, uri, outer, `"context":{"includeDeclaration":false}`),
		requestAt("textDocument/references", 4, uri, inner, `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/definition", 5, uri, positionOf(text, "value", 3), ""),
	)
	if got := len(messages[2]["result"].([]any)); got != 3 {
		t.Fatalf("outer references including declaration = %d, want 3: %#v", got, messages[2])
	}
	if got := len(messages[3]["result"].([]any)); got != 2 {
		t.Fatalf("outer references excluding declaration = %d, want 2: %#v", got, messages[3])
	}
	if got := len(messages[4]["result"].([]any)); got != 2 {
		t.Fatalf("inner references = %d, want 2: %#v", got, messages[4])
	}
	definition := messages[5]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if definition["line"] != float64(2) {
		t.Fatalf("shadowed definition = %#v, want inner declaration on line 2", definition)
	}
}

func TestInheritanceNavigationAndOverrideRenameFamily(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inheritance.otm")
	uri := fileURI(path)
	text := `class Base {
  public virtual function read(): int { return 1; }
}
class Child extends Base {
  public override function read(): int { return super.read() + 1; }
}
function use(): int { return new Child().read(); }
`
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/definition", 2, uri, positionOf(text, "Base", 1), ""),
		requestAt("textDocument/references", 3, uri, positionOf(text, "read", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 4, uri, positionOf(text, "read", 1), `"newName":"speak"`),
	)
	definition := messages[2]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if definition["line"] != float64(0) {
		t.Fatalf("base definition = %#v, want line 0", definition)
	}
	if got := len(messages[3]["result"].([]any)); got != 4 {
		t.Fatalf("override family references = %d, want base/override/super/call: %#v", got, messages[3])
	}
	changes := messages[4]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 4 {
		t.Fatalf("override family rename edits = %d, want 4: %#v", got, changes)
	}
}

func TestRenameProducesPreciseUnicodeWorkspaceEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unicode.otm")
	uri := fileURI(path)
	text := `function total(温泉: int): int {
  const label: string = "温泉";
  // 温泉 in a comment is not a symbol.
  return 温泉 + 1;
}`
	at := positionOf(text, "温泉", 3)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/prepareRename", 2, uri, at, ""),
		requestAt("textDocument/rename", 3, uri, at, `"newName":"たまご"`),
	)
	prepared := messages[2]["result"].(map[string]any)
	if prepared["placeholder"] != "温泉" {
		t.Fatalf("prepareRename = %#v", prepared)
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	edits := changes[uri].([]any)
	if len(edits) != 2 {
		t.Fatalf("rename edits = %#v, want declaration and semantic use only", edits)
	}
	for _, raw := range edits {
		if raw.(map[string]any)["newText"] != "たまご" {
			t.Fatalf("rename edit = %#v", raw)
		}
	}
}

func TestRenameRejectsInvalidAndCapturingNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reject.otm")
	uri := fileURI(path)
	text := `function value(input: int): int {
  if (true) { const occupied: int = 1; const captured: int = input; }
  return input;
}
function other(available: int): int { return available; }`
	at := positionOf(text, "input", 0)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/rename", 2, uri, at, `"newName":"return"`),
		requestAt("textDocument/rename", 3, uri, at, `"newName":"occupied"`),
		requestAt("textDocument/rename", 4, uri, positionOf(text, "int", 0), `"newName":"integer"`),
		requestAt("textDocument/rename", 5, uri, at, `"newName":"available"`),
		requestAt("textDocument/rename", 6, uri, at, `"newName":"int"`),
	)
	for _, id := range []float64{2, 3, 6} {
		errorValue, ok := messages[id]["error"].(map[string]any)
		if !ok || errorValue["code"] != float64(-32602) {
			t.Fatalf("rename id %v = %#v, want invalid-params error", id, messages[id])
		}
	}
	if messages[4]["result"] != nil {
		t.Fatalf("unsupported type rename = %#v, want null", messages[4])
	}
	if _, ok := messages[5]["result"].(map[string]any); !ok {
		t.Fatalf("non-capturing rename into a name used by another scope = %#v", messages[5])
	}
}

func TestReferencesAndRenameCrossRelativeImport(t *testing.T) {
	directory := t.TempDir()
	dependency := filepath.Join(directory, "dependency.otm")
	entry := filepath.Join(directory, "main.otm")
	dependencyText := `function helper(value: int): int { return value + 1; }`
	entryText := `import { helper } from "./dependency";
function main(): int { return helper(41); }`
	if err := os.WriteFile(dependency, []byte(`function stale(): int { return 0; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	entryURI := fileURI(entry)
	dependencyURI := fileURI(dependency)
	at := positionOf(entryText, "helper", 1)
	messages := serveMessages(t,
		openDocument(dependencyURI, dependencyText),
		openDocument(entryURI, entryText),
		requestAt("textDocument/references", 2, entryURI, at, `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 3, entryURI, at, `"newName":"increment"`),
	)
	if got := len(messages[2]["result"].([]any)); got != 3 {
		t.Fatalf("cross-file references = %d, want declaration, import, call: %#v", got, messages[2])
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[entryURI].([]any)); got != 2 {
		t.Fatalf("entry edits = %d, want import and call", got)
	}
	if got := len(changes[dependencyURI].([]any)); got != 1 {
		t.Fatalf("dependency edits = %d, want declaration", got)
	}
}

func TestValidRenameIdentifierMatrix(t *testing.T) {
	for _, name := range []string{"value", "_private", "温泉卵", "value2"} {
		if !validRenameIdentifier(name) {
			t.Errorf("validRenameIdentifier(%q) = false", name)
		}
	}
	for _, name := range []string{"", "2value", "with.dot", "two words", "return", "this", "value;"} {
		if validRenameIdentifier(name) {
			t.Errorf("validRenameIdentifier(%q) = true", name)
		}
	}
}

func TestReferencesAndRenameGoImportAlias(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go_alias.otm")
	uri := fileURI(path)
	text := `import go words from "strings";
function clean(value: string): string { return words.TrimSpace(value); }`
	at := positionOf(text, "words", 1)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/references", 2, uri, at, `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 3, uri, at, `"newName":"textutil"`),
	)
	if got := len(messages[2]["result"].([]any)); got != 2 {
		t.Fatalf("Go alias references = %d, want import and use", got)
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 2 {
		t.Fatalf("Go alias edits = %d, want import and use", got)
	}
}

func TestReferencesBindingKindsMatrix(t *testing.T) {
	tests := []struct {
		name string
		text string
		term string
		use  int
		want int
	}{
		{"arrow parameter", `function apply(): int { const fn = (item: int): int => item + item; return fn(1); }`, "item", 1, 3},
		{"multiple assignment", `import go strconv from "strconv"; function parse(): int { let value: int = 0; let err: error = nil; [value, err] = strconv.Atoi("1"); return value; }`, "value", 1, 3},
		{"range binding", `function sum(items: int[]): int { let total: int = 0; for (const item: int of items) { total += item; } return total; }`, "item", 1, 2},
		{"select binding", `function read(input: GoReceiveChannel<int>): int { select { case const selected = <-input { return selected; } } }`, "selected", 1, 2},
		{"value switch case", `function classify(input: int, expected: int): boolean { switch (input) { case expected { return true; } default { return false; } } }`, "expected", 1, 2},
		{"type switch binding", `function inspect(input: error): boolean { switch (input) { case const typed as error { return typed != nil; } default { return false; } } }`, "typed", 1, 2},
		{"propagation operand", `function source(): Result<int> { return ok(1); } function use(): Result<int> { const value = source()?; return ok(value); }`, "source", 1, 2},
		{"catch binding", `function inspect(input: error): string { let result = ""; try { throw input; } catch (caught: error) { result = caught.Error(); } return result; }`, "caught", 1, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "binding.otm")
			uri := fileURI(path)
			messages := serveMessages(t,
				openDocument(uri, test.text),
				requestAt("textDocument/references", 2, uri, positionOf(test.text, test.term, test.use), `"context":{"includeDeclaration":true}`),
			)
			if got := len(messages[2]["result"].([]any)); got != test.want {
				t.Fatalf("references = %d, want %d: %#v", got, test.want, messages[2])
			}
		})
	}
}

func TestRefactorInvalidRequestMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.otm")
	uri := fileURI(path)
	text := `function value(): int { return 1; }`
	unopened := serveMessages(t,
		requestAt("textDocument/references", 2, uri, position{}, `"context":{"includeDeclaration":true}`),
	)
	if got := len(unopened[2]["result"].([]any)); got != 0 {
		t.Fatalf("unopened references = %#v", unopened[2])
	}
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/references", 3, uri, position{Line: 99}, `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/prepareRename", 4, uri, positionOf(text, "return", 0), ""),
	)
	if got := len(messages[3]["result"].([]any)); got != 0 {
		t.Fatalf("references id 3 = %#v", messages[3])
	}
	if messages[4]["result"] != nil {
		t.Fatalf("keyword prepareRename = %#v, want null", messages[4])
	}
}

func TestRenameClassFieldAndMethodMatrix(t *testing.T) {
	text := `class Box {
  public value: int;
  constructor(value: int) { this.value = value; }
  public function get(): int { return this.value; }
  public static function make(value: int): Box { return new Box(value); }
}
function read(box: Box): int { const next: Box = Box.make(box.get()); return next.value; }
class Other { public value: int; public function get(): int { return this.value; } }`
	tests := []struct {
		name       string
		term       string
		occurrence int
		newName    string
		wantEdits  int
	}{
		{"class", "Box", 0, "Crate", 6},
		{"field excludes same spelling in another class and parameters", "value", 0, "amount", 4},
		{"instance method excludes same spelling in another class", "get", 0, "readValue", 2},
		{"static method", "make", 0, "create", 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "members.otm")
			uri := fileURI(path)
			at := positionOf(text, test.term, test.occurrence)
			messages := serveMessages(t,
				openDocument(uri, text),
				requestAt("textDocument/prepareRename", 2, uri, at, ""),
				requestAt("textDocument/rename", 3, uri, at, `"newName":"`+test.newName+`"`),
			)
			prepared, ok := messages[2]["result"].(map[string]any)
			if !ok || prepared["placeholder"] != test.term {
				t.Fatalf("prepareRename = %#v", messages[2])
			}
			changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
			edits := changes[uri].([]any)
			if len(edits) != test.wantEdits {
				t.Fatalf("rename edits = %d, want %d: %#v", len(edits), test.wantEdits, edits)
			}
			for _, raw := range edits {
				if raw.(map[string]any)["newText"] != test.newName {
					t.Fatalf("rename edit = %#v", raw)
				}
			}
		})
	}
}

func TestRenameNativeStructTypeAndFieldIncludesLiteralKeys(t *testing.T) {
	text := `struct Point { public value: int; }
function make(): Point { return Point { value: 1 }; }
function read(point: Point): int { return point.value; }`
	tests := []struct {
		name      string
		term      string
		newName   string
		wantEdits int
	}{
		{"type", "Point", "Coordinate", 4},
		{"field", "value", "amount", 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "struct.otm")
			uri := fileURI(path)
			messages := serveMessages(t,
				openDocument(uri, text),
				requestAt("textDocument/references", 2, uri, positionOf(text, test.term, 0), `"context":{"includeDeclaration":true}`),
				requestAt("textDocument/rename", 3, uri, positionOf(text, test.term, 0), `"newName":"`+test.newName+`"`),
			)
			if got := len(messages[2]["result"].([]any)); got != test.wantEdits {
				t.Fatalf("references = %d, want %d: %#v", got, test.wantEdits, messages[2])
			}
			changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
			if got := len(changes[uri].([]any)); got != test.wantEdits {
				t.Fatalf("rename edits = %d, want %d: %#v", got, test.wantEdits, changes)
			}
		})
	}
}

func TestRenameNativeStructMethodUsesResolvedReceiverIdentity(t *testing.T) {
	text := `struct Counter {
  public value: int;
  public function read(): int { return this.value; }
  public pointer function add(delta: int): void { this.value += delta; }
}

function use(value: Counter, pointer: *Counter): int {
  value.add(1);
  pointer.add(2);
  const method = value.add;
  method(3);
  return value.read();
}`
	path := filepath.Join(t.TempDir(), "struct_method.otm")
	uri := fileURI(path)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/references", 2, uri, positionOf(text, "add", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 3, uri, positionOf(text, "add", 0), `"newName":"increase"`),
		requestAt("textDocument/definition", 4, uri, positionOf(text, "add", 2), ""),
	)
	if got := len(messages[2]["result"].([]any)); got != 4 {
		t.Fatalf("method references = %d, want 4: %#v", got, messages[2])
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 4 {
		t.Fatalf("method edits = %d, want 4: %#v", got, changes)
	}
	definition := messages[4]["result"].(map[string]any)
	start := definition["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(3) {
		t.Fatalf("method definition = %#v", definition)
	}
}

func TestExternalNativeStructReceiverMethodNavigationAndRename(t *testing.T) {
	text := `struct Counter { public value: int; }
public function add(this: *Counter, delta: int): void {
  this.value += delta;
}
function use(value: *Counter): void { value.add(2); }`
	path := filepath.Join(t.TempDir(), "external_receiver.otm")
	uri := fileURI(path)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/references", 2, uri, positionOf(text, "add", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 3, uri, positionOf(text, "delta", 0), `"newName":"amount"`),
		requestAt("textDocument/definition", 4, uri, positionOf(text, "add", 1), ""),
		requestAt("textDocument/references", 5, uri, positionOf(text, "this", 1), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/definition", 6, uri, positionOf(text, "this", 1), ""),
		requestAt("textDocument/prepareRename", 7, uri, positionOf(text, "this", 1), ""),
	)
	if got := len(messages[2]["result"].([]any)); got != 2 {
		t.Fatalf("method references = %d, want 2: %#v", got, messages[2])
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 2 {
		t.Fatalf("parameter edits = %d, want 2: %#v", got, changes)
	}
	definition := messages[4]["result"].(map[string]any)
	start := definition["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(1) {
		t.Fatalf("method definition = %#v", definition)
	}
	if got := len(messages[5]["result"].([]any)); got != 2 {
		t.Fatalf("receiver references = %d, want 2: %#v", got, messages[5])
	}
	receiverDefinition := messages[6]["result"].(map[string]any)
	receiverStart := receiverDefinition["range"].(map[string]any)["start"].(map[string]any)
	if receiverStart["line"] != float64(1) {
		t.Fatalf("receiver definition = %#v", receiverDefinition)
	}
	if messages[7]["result"] != nil {
		t.Fatalf("receiver prepareRename = %#v, want null for fixed 'this' syntax", messages[7])
	}
}

func TestRenameInterfaceMethodUpdatesImplementationFamily(t *testing.T) {
	path := filepath.Join(t.TempDir(), "interface.otm")
	uri := fileURI(path)
	text := `interface Reader { function read(value: int): int; }
interface Source { function read(value: int): int; }
class First implements Reader, Source { public function read(value: int): int { return value; } }
class Second implements Reader { public function read(value: int): int { return value + 1; } }
function all(contract: Reader, source: Source, first: First, second: Second): int {
  return contract.read(1) + source.read(2) + first.read(3) + second.read(4);
}`
	at := positionOf(text, "read", 0)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/references", 2, uri, at, `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 3, uri, at, `"newName":"fetch"`),
	)
	if got := len(messages[2]["result"].([]any)); got != 8 {
		t.Fatalf("method family references = %d, want 8: %#v", got, messages[2])
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 8 {
		t.Fatalf("method family edits = %d, want 8: %#v", got, changes)
	}
}

func TestRenameConstructorParameterFieldAsSingleDeclaration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "parameter_field.otm")
	uri := fileURI(path)
	text := `class Box {
  constructor(public value: int) { this.value = value; }
}
function read(box: Box): int { return box.value; }`
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/rename", 2, uri, positionOf(text, "value", 0), `"newName":"amount"`),
	)
	changes := messages[2]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 4 {
		t.Fatalf("parameter-field edits = %d, want one declaration and three uses: %#v", got, changes[uri])
	}
}

func TestRenameClassAcrossUnsavedRelativeImport(t *testing.T) {
	directory := t.TempDir()
	dependency := filepath.Join(directory, "box.otm")
	entry := filepath.Join(directory, "main.otm")
	dependencyText := `class Box { constructor(public value: int) {} }`
	entryText := `import { Box } from "./box";
function make(value: int): Box { return new Box(value); }`
	if err := os.WriteFile(dependency, []byte(`class Stale {}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dependencyURI := fileURI(dependency)
	entryURI := fileURI(entry)
	messages := serveMessages(t,
		openDocument(dependencyURI, dependencyText),
		openDocument(entryURI, entryText),
		requestAt("textDocument/rename", 2, entryURI, positionOf(entryText, "Box", 2), `"newName":"Crate"`),
	)
	changes := messages[2]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[dependencyURI].([]any)); got != 1 {
		t.Fatalf("dependency edits = %d, want class declaration", got)
	}
	if got := len(changes[entryURI].([]any)); got != 3 {
		t.Fatalf("entry edits = %d, want import, result type, and construction: %#v", got, changes[entryURI])
	}
}

func TestRenameGoAliasIncludesTypeQualifiersButNotExternalType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go_type_alias.otm")
	uri := fileURI(path)
	text := `import go clock from "time";
function identity(value: clock.Duration): clock.Duration { return value; }`
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/rename", 2, uri, positionOf(text, "clock", 1), `"newName":"times"`),
		requestAt("textDocument/prepareRename", 3, uri, positionOf(text, "Duration", 0), ""),
	)
	changes := messages[2]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 3 {
		t.Fatalf("Go alias edits = %d, want import and two type qualifiers: %#v", got, changes[uri])
	}
	if messages[3]["result"] != nil {
		t.Fatalf("external Go type prepareRename = %#v, want null", messages[3])
	}
}

func TestRenameTypeAndMemberCollisionMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collisions.otm")
	uri := fileURI(path)
	text := `class Box {
  public value: int;
  public other: int;
  public function get(): int { return this.value; }
  public function otherMethod(): int { return this.other; }
}
class Existing {}`
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/rename", 2, uri, positionOf(text, "Box", 0), `"newName":"Existing"`),
		requestAt("textDocument/rename", 3, uri, positionOf(text, "value", 0), `"newName":"other"`),
		requestAt("textDocument/rename", 4, uri, positionOf(text, "get", 0), `"newName":"otherMethod"`),
	)
	for _, id := range []float64{2, 3, 4} {
		errorValue, ok := messages[id]["error"].(map[string]any)
		if !ok || errorValue["code"] != float64(-32602) {
			t.Fatalf("rename id %v = %#v, want invalid-params error", id, messages[id])
		}
	}
}

func TestDefinitionTypeConstructionAndMemberMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "definition_members.otm")
	uri := fileURI(path)
	text := `class Box {
  public value: int;
  public function get(): int { return this.value; }
}
function use(box: Box): int {
  const copy: Box = new Box();
  return copy.value + copy.get();
}`
	tests := []struct {
		name       string
		term       string
		occurrence int
		wantLine   float64
	}{
		{"parameter type", "Box", 1, 0},
		{"local type", "Box", 2, 0},
		{"construction", "Box", 3, 0},
		{"field", "value", 2, 1},
		{"method", "get", 1, 2},
	}
	requests := []string{openDocument(uri, text)}
	for index, test := range tests {
		requests = append(requests, requestAt("textDocument/definition", index+2, uri, positionOf(text, test.term, test.occurrence), ""))
	}
	messages := serveMessages(t, requests...)
	for index, test := range tests {
		result, ok := messages[float64(index+2)]["result"].(map[string]any)
		if !ok {
			t.Errorf("%s definition = %#v", test.name, messages[float64(index+2)])
			continue
		}
		start := result["range"].(map[string]any)["start"].(map[string]any)
		if start["line"] != test.wantLine {
			t.Errorf("%s definition line = %v, want %v", test.name, start["line"], test.wantLine)
		}
	}
}

func FuzzRelatedDeclarationsNeverPanics(f *testing.F) {
	f.Add("read", "read", true, true)
	f.Add("read", "write", true, false)
	f.Fuzz(func(t *testing.T, primaryName, secondaryName string, bridgeSecondary, includeSecondClass bool) {
		span := func(offset int) source.Span {
			return source.Span{Path: "family.otm", Start: source.Position{Offset: offset}, End: source.Position{Offset: offset + 1}}
		}
		primary := &ast.InterfaceDecl{
			Name: "Primary", NameSpan: span(1),
			Methods: []ast.InterfaceMethod{{Name: primaryName, NameSpan: span(2)}},
		}
		secondary := &ast.InterfaceDecl{
			Name: "Secondary", NameSpan: span(3),
			Methods: []ast.InterfaceMethod{{Name: secondaryName, NameSpan: span(4)}},
		}
		implemented := []ast.TypeRef{{Name: "Primary", ResolvedDeclaration: primary.NameSpan}}
		if bridgeSecondary {
			implemented = append(implemented, ast.TypeRef{Name: "Secondary", ResolvedDeclaration: secondary.NameSpan})
		}
		firstMethod := &ast.MethodDecl{Name: primaryName, NameSpan: span(5)}
		declarations := []ast.Declaration{
			primary,
			secondary,
			&ast.ClassDecl{Name: "First", NameSpan: span(6), Implements: implemented, Methods: []*ast.MethodDecl{firstMethod}},
		}
		var secondMethod *ast.MethodDecl
		if includeSecondClass {
			secondMethod = &ast.MethodDecl{Name: primaryName, NameSpan: span(7)}
			declarations = append(declarations, &ast.ClassDecl{
				Name: "Second", NameSpan: span(8),
				Implements: []ast.TypeRef{{Name: "Primary", ResolvedDeclaration: primary.NameSpan}},
				Methods:    []*ast.MethodDecl{secondMethod},
			})
		}
		family := relatedDeclarations(&ast.Program{Declarations: declarations}, primary.Methods[0].NameSpan)
		if !family.contains(primary.Methods[0].NameSpan) || !family.contains(firstMethod.NameSpan) {
			t.Fatalf("primary implementation is missing from family: %#v", family)
		}
		if bridgeSecondary && secondaryName == primaryName && !family.contains(secondary.Methods[0].NameSpan) {
			t.Fatalf("bridged interface is missing from family: %#v", family)
		}
		if includeSecondClass && !family.contains(secondMethod.NameSpan) {
			t.Fatalf("second implementation is missing from family: %#v", family)
		}
	})
}
