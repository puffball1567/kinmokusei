package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/project"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func framed(messages ...string) string {
	var result strings.Builder
	for _, message := range messages {
		fmt.Fprintf(&result, "Content-Length: %d\r\n\r\n%s", len(message), message)
	}
	return result.String()
}

func decodeMessages(t *testing.T, output string) []map[string]any {
	t.Helper()
	reader := bufio.NewReader(strings.NewReader(output))
	var messages []map[string]any
	for {
		payload, err := readMessage(reader)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var message map[string]any
		if err = json.Unmarshal(payload, &message); err != nil {
			t.Fatal(err)
		}
		messages = append(messages, message)
	}
	return messages
}

func fileURI(path string) string {
	return pathURI(path)
}

func TestFileURIPathRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "温泉 space.km")
	got, err := filePath(fileURI(path))
	if err != nil || got != filepath.Clean(path) {
		t.Fatalf("filePath(fileURI(%q)) = %q, %v", path, got, err)
	}
}

func completionItemsAt(t *testing.T, path, text string, line, character int) []map[string]any {
	t.Helper()
	uri := fileURI(path)
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":%q}}}`, uri, text)
	completion := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/completion","params":{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}}}`, uri, line, character)
	shutdown := `{"jsonrpc":"2.0","id":3,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, open, completion, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	for _, message := range decodeMessages(t, output.String()) {
		if id, ok := message["id"].(float64); !ok || id != 2 {
			continue
		}
		values := message["result"].([]any)
		items := make([]map[string]any, len(values))
		for index := range values {
			items[index] = values[index].(map[string]any)
		}
		return items
	}
	t.Fatalf("completion response missing: %s", output.String())
	return nil
}

func completionLabels(items []map[string]any) map[string]map[string]any {
	result := map[string]map[string]any{}
	for _, item := range items {
		result[item["label"].(string)] = item
	}
	return result
}

func TestCompletionLexicalScopeAndPrefixMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "completion.km")
	text := `import go strings from "strings";
function helper(): int { return 1; }
const global: int = 2;
function value(parameter: int): int {
  const before: int = 3;
  if (true) { const closed: int = 4; }
  return before;
  const after: int = 5;
}`
	line := 6
	character := strings.Index(strings.Split(text, "\n")[line], "before")
	items := completionLabels(completionItemsAt(t, path, text, line, character))
	for _, want := range []string{"before", "parameter", "helper", "global", "strings", "len", "clear", "min", "max", "int", "int8", "int16", "uint", "uint8", "return", "Result", "Task", "Exception", "try", "catch", "finally", "throw", "await", "detach", "goto", "fallthrough", "ok", "fail", "null"} {
		if items[want] == nil {
			t.Fatalf("missing %q in completion labels: %v", want, items)
		}
	}
	for _, unwanted := range []string{"closed", "after"} {
		if items[unwanted] != nil {
			t.Fatalf("out-of-scope %q was completed: %#v", unwanted, items[unwanted])
		}
	}
	if detail := items["parameter"]["detail"]; detail != "parameter: int" {
		t.Fatalf("parameter detail=%v", detail)
	}

	prefixCharacter := character + len("be")
	prefixed := completionLabels(completionItemsAt(t, path, text, line, prefixCharacter))
	if len(prefixed) != 1 || prefixed["before"] == nil {
		t.Fatalf("prefixed completion=%#v", prefixed)
	}
}

func TestCompletionNestedScopeBindingsMatrix(t *testing.T) {
	tests := []struct {
		name string
		text string
		line int
		want string
	}{
		{"nested block", "function value(): int {\n  const outer: int = 1;\n  if (true) { const inner: int = 2; return inner; }\n}", 2, "inner"},
		{"for initializer", "function value(): int {\n  for (let index: int = 0; index < 1; index = index + 1) { return index; }\n  return 0;\n}", 1, "index"},
		{"channel range", "function value(channel: GoReceiveChannel<int>): int {\n  for (const item: int of channel) { return item; }\n  return 0;\n}", 1, "item"},
		{"collection range index", "function value(items: int[]): int {\n  for (const [index: int, item: int] of items) { return index + item; }\n  return 0;\n}", 1, "index"},
		{"collection range value", "function value(items: int[]): int {\n  for (const [index: int, item: int] of items) { return index + item; }\n  return 0;\n}", 1, "item"},
		{"type switch", "import go io from \"io\"; import go strings from \"strings\";\nfunction value(input: io.Reader): int { switch (input) { case const reader as *strings.Reader { return reader.Len(); } default { return 0; } } }", 1, "reader"},
		{"catch binding", "function value(input: error): string { let result = \"\";\n  try { throw input; } catch (caught: error) { result = caught.Error(); } return result;\n}", 1, "caught"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "scope.km")
			lineText := strings.Split(test.text, "\n")[test.line]
			character := strings.LastIndex(lineText, test.want)
			items := completionLabels(completionItemsAt(t, path, test.text, test.line, character))
			if items[test.want] == nil {
				t.Fatalf("missing %q: %#v", test.want, items)
			}
		})
	}
}

func TestCompletionGoExportedMemberMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go_completion.km")
	text := `import go words from "strings"; function clean(value: string): string { return words.TrimSpace(value); }`
	memberStart := strings.Index(text, "TrimSpace")
	items := completionLabels(completionItemsAt(t, path, text, 0, memberStart+len("Tr")))
	if items["TrimSpace"] == nil || !strings.Contains(items["TrimSpace"]["detail"].(string), "func strings.TrimSpace") {
		t.Fatalf("TrimSpace completion=%#v", items["TrimSpace"])
	}
	if items["ToUpper"] != nil || items["trimSpace"] != nil {
		t.Fatalf("prefix/unexported filtering failed: %#v", items)
	}

	incomplete := `import go words from "strings"; function clean(): string { return words.; }`
	dot := strings.Index(incomplete, "words.") + len("words.")
	all := completionLabels(completionItemsAt(t, path, incomplete, 0, dot))
	for _, want := range []string{"Clone", "Contains", "TrimSpace"} {
		if all[want] == nil {
			t.Fatalf("trailing-dot completion missing %q", want)
		}
	}

	kindTests := []struct {
		name       string
		packageID  string
		path       string
		member     string
		wantKind   float64
		wantDetail string
	}{
		{"constant", "math", "math", "Pi", 21, "const math.Pi"},
		{"variable", "os", "os", "Stdout", 6, "var os.Stdout"},
		{"type", "time", "time", "Duration", 7, "type time.Duration"},
	}
	for _, test := range kindTests {
		t.Run(test.name, func(t *testing.T) {
			input := fmt.Sprintf(`import go %s from %q; function value(): int { return 1; } %s.%s`, test.packageID, test.path, test.packageID, test.member)
			position := strings.LastIndex(input, test.member)
			found := completionLabels(completionItemsAt(t, path, input, 0, position))[test.member]
			if found == nil || found["kind"] != test.wantKind || !strings.Contains(found["detail"].(string), test.wantDetail) {
				t.Fatalf("completion=%#v", found)
			}
		})
	}
}

func TestCompletionGoValueMemberMatrix(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		needle   string
		want     []string
		unwanted []string
	}{
		{
			name:   "explicit pointer parameter",
			text:   `import go web from "net/http"; function use(request: *web.Request): string { return request.; }`,
			needle: "request.", want: []string{"Method", "URL", "Clone", "Context"}, unwanted: []string{"ctx"},
		},
		{
			name:   "addressable value includes pointer methods",
			text:   `import go web from "net/http"; function use(request: web.Request): string { return request.; }`,
			needle: "request.", want: []string{"Method", "Clone"}, unwanted: []string{"ctx"},
		},
		{
			name:   "inferred package variable",
			text:   `import go web from "net/http"; function use(): string { const client = web.DefaultClient; return client.; }`,
			needle: "client.", want: []string{"Do", "Get", "Timeout"},
		},
		{
			name:   "inferred composite literal",
			text:   `import go web from "net/http"; function use(): string { const request = web.Request{ Method: "GET" }; return request.; }`,
			needle: "request.", want: []string{"Method", "Clone"},
		},
		{
			name:   "inferred multiple result",
			text:   `import go web from "net/http"; function use(): string { const [request, err] = web.NewRequest(web.MethodGet, "https://example.com", nil); if (err != nil) { return ""; } return request.; }`,
			needle: "request.", want: []string{"Method", "Clone", "Context"},
		},
		{
			name:   "promoted fields and methods",
			text:   `import go buffered from "bufio"; function use(value: *buffered.ReadWriter): string { return value.; }`,
			needle: "value.", want: []string{"Reader", "Writer", "ReadString", "WriteString"},
		},
		{
			name:   "global inferred package variable",
			text:   `import go web from "net/http"; const client = web.DefaultClient; function use(): string { return client.; }`,
			needle: "client.", want: []string{"Do", "Get", "Timeout"},
		},
		{
			name:   "inferred range binding",
			text:   `import go web from "net/http"; function use(requests: [1]*web.Request): string { for (const request of requests) { return request.; } return ""; }`,
			needle: "request.", want: []string{"Method", "Clone", "Context"},
		},
		{
			name:   "inferred select binding",
			text:   `import go web from "net/http"; function use(input: GoReceiveChannel<*web.Request>): string { select { case const request = <-input { return request.; } default { return ""; } } }`,
			needle: "request.", want: []string{"Method", "Clone", "Context"},
		},
		{
			name:   "named map methods",
			text:   `import go web from "net/http"; function use(headers: web.Header): string { return headers.; }`,
			needle: "headers.", want: []string{"Add", "Get", "Set", "Values"},
		},
		{
			name:   "alias type methods",
			text:   `import go operating from "os"; function use(mode: operating.FileMode): string { return mode.; }`,
			needle: "mode.", want: []string{"IsDir", "Perm", "String"},
		},
		{
			name:   "prefix",
			text:   `import go web from "net/http"; function use(request: *web.Request): string { return request.Cl; }`,
			needle: "request.Cl", want: []string{"Clone", "Close"}, unwanted: []string{"Context", "Method"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "go_value_members.km")
			position := strings.LastIndex(test.text, test.needle) + len(test.needle)
			items := completionLabels(completionItemsAt(t, path, test.text, 0, position))
			for _, want := range test.want {
				if items[want] == nil {
					t.Fatalf("missing %q: %#v", want, items)
				}
			}
			for _, unwanted := range test.unwanted {
				if items[unwanted] != nil {
					t.Fatalf("unexpected %q: %#v", unwanted, items[unwanted])
				}
			}
			for _, field := range []string{"Method", "URL", "Timeout", "Reader", "Writer", "Close"} {
				if item := items[field]; item != nil && item["kind"] != float64(5) {
					t.Fatalf("field %q kind = %#v", field, item)
				}
			}
			for _, method := range []string{"Clone", "Context", "Do", "Get", "ReadString", "WriteString"} {
				if item := items[method]; item != nil && item["kind"] != float64(2) {
					t.Fatalf("method %q kind = %#v", method, item)
				}
			}
		})
	}
}

func TestCompletionBuiltinExceptionMembers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exception_completion.km")
	text := `function value(): string { try { throw new Exception("boom"); } catch (err: Exception) { return err.; } }`
	dot := strings.Index(text, "err.") + len("err.")
	items := completionLabels(completionItemsAt(t, path, text, 0, dot))
	if items["message"] == nil || items["error"] == nil || len(items) != 2 {
		t.Fatalf("Exception members = %#v", items)
	}
	if items["message"]["kind"] != float64(5) || items["error"]["kind"] != float64(2) {
		t.Fatalf("Exception member kinds = %#v", items)
	}

	prefixed := strings.Replace(text, "err.", "err.me", 1)
	at := strings.Index(prefixed, "err.me") + len("err.me")
	prefixItems := completionLabels(completionItemsAt(t, path, prefixed, 0, at))
	if len(prefixItems) != 1 || prefixItems["message"] == nil {
		t.Fatalf("Exception prefix members = %#v", prefixItems)
	}

	goError := `function value(err: error): string { return err.; }`
	at = strings.Index(goError, "err.") + len("err.")
	errorItems := completionLabels(completionItemsAt(t, path, goError, 0, at))
	if len(errorItems) != 1 || errorItems["Error"] == nil {
		t.Fatalf("error members = %#v", errorItems)
	}
}

func TestCompletionClassMembersVisibilityInheritanceAndStatic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "class_completion.km")
	text := `class Base {
  constructor(public base: string, protected secret: string, private hidden: string) {}
  public function read(): string { return this.base; }
  public static function create(): Base { return new Base("", "", ""); }
}
class Child extends Base {
  constructor() { super("", "", ""); }
  private function own(): string { return this.secret; }
  public function inside(): string { return this.; }
}
function outside(value: Child): string { return value.; }
function staticValue(): Base { return Base.; }`
	tests := []struct {
		name       string
		needle     string
		want       []string
		unwanted   []string
		wantDetail string
	}{
		{
			name: "outside instance", needle: "value.",
			want: []string{"base", "read"}, unwanted: []string{"secret", "hidden", "own", "create"},
		},
		{
			name: "derived this", needle: "this.;",
			want: []string{"base", "secret", "read", "own", "inside"}, unwanted: []string{"hidden", "create"},
		},
		{
			name: "static", needle: "Base.;",
			want: []string{"create"}, unwanted: []string{"base", "secret", "hidden", "read"}, wantDetail: "public function create(): Base",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			position := strings.LastIndex(text, test.needle) + len(strings.TrimSuffix(test.needle, ";"))
			line := strings.Count(text[:position], "\n")
			lineStart := strings.LastIndex(text[:position], "\n") + 1
			items := completionLabels(completionItemsAt(t, path, text, line, position-lineStart))
			for _, want := range test.want {
				if items[want] == nil {
					t.Fatalf("missing %q: %#v", want, items)
				}
			}
			for _, unwanted := range test.unwanted {
				if items[unwanted] != nil {
					t.Fatalf("inaccessible/static %q completed: %#v", unwanted, items[unwanted])
				}
			}
			if test.wantDetail != "" && !strings.Contains(items[test.want[0]]["detail"].(string), test.wantDetail) {
				t.Fatalf("detail = %#v", items[test.want[0]])
			}
		})
	}
}

func TestCompletionSourceMemberReceiverMatrix(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		needle   string
		want     []string
		unwanted []string
	}{
		{
			name:   "inferred constructor local",
			text:   `class Box { constructor(public value: int) {} public function read(): int { return this.value; } } function use(): int { const box = new Box(1); return box.; }`,
			needle: "box.", want: []string{"value", "read"},
		},
		{
			name:   "inferred function result",
			text:   `class Box { constructor(public value: int) {} public function read(): int { return this.value; } } function build(): Box { return new Box(1); } function use(): int { const box = build(); return box.; }`,
			needle: "box.", want: []string{"value", "read"},
		},
		{
			name:   "inferred method result",
			text:   `class Box { constructor(public value: int) {} public function read(): int { return this.value; } } class Factory { public function build(): Box { return new Box(1); } } function use(factory: Factory): int { const box = factory.build(); return box.; }`,
			needle: "box.", want: []string{"value", "read"},
		},
		{
			name:   "checked map lookup binding",
			text:   `class User { constructor(public name: string) {} public function label(): string { return this.name; } } function use(values: Map<string, User>): string { const [user, present] = values["key"]; if (!present) { return ""; } return user.; }`,
			needle: "user.", want: []string{"name", "label"},
		},
		{
			name:   "pointer struct",
			text:   `struct Point { public x: int; private y: int; public function length(): int { return this.x; } } function use(point: *Point): int { return point.; }`,
			needle: "point.", want: []string{"x", "length"}, unwanted: []string{"y"},
		},
		{
			name:   "interface",
			text:   `interface Reader { function read(): int; function close(): void; } function use(reader: Reader): int { return reader.; }`,
			needle: "reader.", want: []string{"read", "close"},
		},
		{
			name:   "external receiver method",
			text:   `struct Point { public x: int; } public function moved(this: Point, dx: int): Point { this.x += dx; return this; } function use(point: Point): Point { return point.; }`,
			needle: "point.", want: []string{"x", "moved"},
		},
		{
			name:   "struct this private access",
			text:   `struct Point { public x: int; private y: int; public function inspect(): int { return this.; } }`,
			needle: "this.", want: []string{"x", "y", "inspect"},
		},
		{
			name:   "structural object",
			text:   `function use(value: { name: string, count: int }): string { return value.na; }`,
			needle: "value.na", want: []string{"name"}, unwanted: []string{"count"},
		},
		{
			name:   "nearest shadow",
			text:   `class First { public function first(): int { return 1; } } class Second { public function second(): int { return 2; } } function use(value: First): int { { const value = new Second(); return value.; } }`,
			needle: "value.", want: []string{"second"}, unwanted: []string{"first"},
		},
		{
			name:     "unknown inferred shadow does not leak outer type",
			text:     `class First { public function first(): int { return 1; } } function use(value: First): int { { const value = 1; value.; return 0; } }`,
			needle:   "value.",
			unwanted: []string{"first"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "members.km")
			position := strings.LastIndex(test.text, test.needle) + len(test.needle)
			items := completionLabels(completionItemsAt(t, path, test.text, 0, position))
			for _, want := range test.want {
				if items[want] == nil {
					t.Fatalf("missing %q: %#v", want, items)
				}
			}
			for _, unwanted := range test.unwanted {
				if items[unwanted] != nil {
					t.Fatalf("shadowed/inaccessible %q completed: %#v", unwanted, items[unwanted])
				}
			}
		})
	}
}

func TestCompletionStaticThisIsUnavailable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "static_this.km")
	text := `class Tools { public static function invalid(): int { return this.; } }`
	position := strings.Index(text, "this.") + len("this.")
	if items := completionItemsAt(t, path, text, 0, position); len(items) != 0 {
		t.Fatalf("static this members = %#v", items)
	}
}

func TestCompletionRelativeImportedTypeMembersAndSelection(t *testing.T) {
	directory := t.TempDir()
	dependency := filepath.Join(directory, "models.km")
	if err := os.WriteFile(dependency, []byte(`
class Visible {
  constructor(public name: string) {}
  public function label(): string { return this.name; }
}
class Hidden { public static function create(): Hidden { return new Hidden(); } }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "main.km")
	text := `import { Visible } from "./models"; function use(value: Visible): string { return value.; } function hidden(): int { Hidden.; return 0; }`
	valuePosition := strings.Index(text, "value.") + len("value.")
	valueItems := completionLabels(completionItemsAt(t, path, text, 0, valuePosition))
	for _, want := range []string{"name", "label"} {
		if valueItems[want] == nil {
			t.Fatalf("imported member %q missing: %#v", want, valueItems)
		}
	}
	hiddenPosition := strings.Index(text, "Hidden.") + len("Hidden.")
	if hiddenItems := completionItemsAt(t, path, text, 0, hiddenPosition); len(hiddenItems) != 0 {
		t.Fatalf("unselected type members leaked: %#v", hiddenItems)
	}
}

func TestCompletionUnicodeAndShadowing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unicode.km")
	text := "const item: string = \"global\";\nfunction value(item: int, 日本語: int): int { return 日本語 + item; }"
	line := 1
	lineText := strings.Split(text, "\n")[line]
	unicodeUse := strings.LastIndex(lineText, "日本語")
	unicodeCharacter := len(utf16.Encode([]rune(lineText[:unicodeUse] + "日")))
	unicodeItems := completionLabels(completionItemsAt(t, path, text, line, unicodeCharacter))
	if len(unicodeItems) != 1 || unicodeItems["日本語"] == nil {
		t.Fatalf("unicode completion=%#v", unicodeItems)
	}
	itemUse := strings.LastIndex(lineText, "item")
	itemCharacter := len(utf16.Encode([]rune(lineText[:itemUse])))
	items := completionLabels(completionItemsAt(t, path, text, line, itemCharacter))
	if detail := items["item"]["detail"]; detail != "item: int" {
		t.Fatalf("shadowed item detail=%v", detail)
	}
}

func TestCompletionInvalidRequestPositionMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid_position.km")
	uri := fileURI(path)
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	unopened := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/completion","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":0}}}`, uri)
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":"function value(): int { return 1; }"}}}`, uri)
	outside := fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"textDocument/completion","params":{"textDocument":{"uri":%q},"position":{"line":99,"character":0}}}`, uri)
	negative := fmt.Sprintf(`{"jsonrpc":"2.0","id":4,"method":"textDocument/completion","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":-1}}}`, uri)
	shutdown := `{"jsonrpc":"2.0","id":5,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	unopenedMessages := serveMessages(t, unopened)
	if result := unopenedMessages[2]["result"].([]any); len(result) != 0 {
		t.Fatalf("unopened completion = %#v", unopenedMessages[2])
	}
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, open, outside, negative, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	seen := map[float64]bool{}
	for _, message := range decodeMessages(t, output.String()) {
		id, ok := message["id"].(float64)
		if !ok || (id != 3 && id != 4) {
			continue
		}
		seen[id] = true
		if result := message["result"].([]any); len(result) != 0 {
			t.Fatalf("id=%v result=%#v", id, result)
		}
	}
	if len(seen) != 2 {
		t.Fatalf("responses=%#v", seen)
	}
}

func TestCompletionExternalLockedGoModule(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "completionapi")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "completion"
version = "0.1.0"
go-module = "example.com/completion"
go-version = "1.23"
`
	for path, contents := range map[string]string{
		filepath.Join(root, product.ProjectFileName): manifest,
		filepath.Join(library, "go.mod"):             "module example.com/completionapi\n\ngo 1.23\n",
		filepath.Join(library, "api.go"):             "package completionapi\nfunc Visible() int { return 42 }\nfunc hidden() int { return 0 }\ntype Record struct{}\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := project.AddDependency(root, "example.com/completionapi", "v0.0.0", "./completionapi", true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "main.km")
	text := `import go api from "example.com/completionapi"; function value(): int { return api.Visible(); }`
	memberStart := strings.Index(text, "Visible")
	items := completionLabels(completionItemsAt(t, path, text, 0, memberStart))
	if items["Visible"] == nil || items["Record"] == nil || items["hidden"] != nil {
		t.Fatalf("external completions=%#v", items)
	}
}

func TestCompletionSelectedRelativeImport(t *testing.T) {
	directory := t.TempDir()
	dependency := filepath.Join(directory, "dependency.km")
	if err := os.WriteFile(dependency, []byte(`function helper(value: int): int { return value; } function hidden(): int { return 0; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "main.km")
	text := `import { helper } from "./dependency"; function value(): int { return helper(1); }`
	position := strings.LastIndex(text, "helper")
	items := completionLabels(completionItemsAt(t, path, text, 0, position))
	if items["helper"] == nil || items["hidden"] != nil {
		t.Fatalf("relative import completions=%#v", items)
	}
}

func TestCompletionSuppressesInvalidContexts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contexts.km")
	tests := []struct {
		name      string
		text      string
		character int
	}{
		{"string", `function value(): string { return "words.Trim"; }`, strings.Index(`function value(): string { return "words.Trim"; }`, "Trim") + 2},
		{"line comment", "function value(): int { return 1; } // words.Trim", strings.Index("function value(): int { return 1; } // words.Trim", "Trim") + 2},
		{"block comment", "function value(): int { /* words.Trim */ return 1; }", strings.Index("function value(): int { /* words.Trim */ return 1; }", "Trim") + 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if items := completionItemsAt(t, path, test.text, 0, test.character); len(items) != 0 {
				t.Fatalf("items=%#v", items)
			}
		})
	}
}

func TestFormatTypeRefMatrix(t *testing.T) {
	integer := ast.TypeRef{Name: "int"}
	stringType := ast.TypeRef{Name: "string"}
	innerLength := int64(3)
	inner := ast.TypeRef{Element: &integer, FixedLength: &innerLength}
	outerLength := int64(2)
	outer := ast.TypeRef{Element: &inner, FixedLength: &outerLength}
	zero := int64(0)
	tests := []struct {
		name string
		ref  ast.TypeRef
		want string
	}{
		{"fixed", inner, "[3]int"},
		{"nested", outer, "[2][3]int"},
		{"zero", ast.TypeRef{Element: &integer, FixedLength: &zero}, "[0]int"},
		{"slice", ast.TypeRef{Element: &integer}, "int[]"},
		{"nullable slice", ast.TypeRef{Element: &integer, Nullable: true}, "int[] | null"},
		{
			"object uses source names",
			ast.TypeRef{Object: true, ObjectFields: []ast.ObjectTypeField{
				{Name: "Message", JSONName: "message", Type: stringType},
				{Name: "Count", JSONName: "count", Type: integer},
			}},
			"{ message: string, count: int }",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatTypeRef(test.ref); got != test.want {
				t.Fatalf("formatTypeRef() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSliceBoundHoverDefinitionAndDiagnostics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slice.km")
	uri := fileURI(path)
	sourceText := `function take(values: int[], high: int): int[] { return values[:high]; }`
	character := strings.LastIndex(sourceText, "high") + 1
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":%q}}}`, uri, sourceText)
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":%d}}}`, uri, character)
	definition := fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"textDocument/definition","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":%d}}}`, uri, character)
	shutdown := `{"jsonrpc":"2.0","id":4,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, open, hover, definition, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	messages := decodeMessages(t, output.String())
	byID := map[float64]map[string]any{}
	for _, message := range messages {
		if id, ok := message["id"].(float64); ok {
			byID[id] = message
		}
		if method, _ := message["method"].(string); method == "textDocument/publishDiagnostics" {
			diagnostics := message["params"].(map[string]any)["diagnostics"].([]any)
			if len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %#v", diagnostics)
			}
		}
	}
	hoverText := byID[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hoverText, "high: int") {
		t.Fatalf("hover = %q", hoverText)
	}
	definitionResult := byID[3]["result"].(map[string]any)
	start := definitionResult["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(0) || start["character"] != float64(strings.Index(sourceText, "high")) {
		t.Fatalf("definition = %#v", definitionResult)
	}
}

func TestCollectionBuiltinDiagnosticsAndNavigation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collections.km")
	uri := fileURI(path)
	valid := `function grow(length: int, capacity: int): int[] { return makeSlice[int](length, capacity); }`
	invalid := `function bad(value: [2]int): void { append(value, 3); }`
	character := strings.LastIndex(valid, "length") + 1
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":%q}}}`, uri, valid)
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":%d}}}`, uri, character)
	definition := fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"textDocument/definition","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":%d}}}`, uri, character)
	navigation := serveMessages(t, openDocument(uri, valid), hover, definition)
	hoverText := navigation[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hoverText, "length: int") {
		t.Fatalf("hover = %q", hoverText)
	}
	definitionResult := navigation[3]["result"].(map[string]any)
	start := definitionResult["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(0) || start["character"] != float64(strings.Index(valid, "length")) {
		t.Fatalf("definition = %#v", definitionResult)
	}
	change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`, uri, invalid)
	shutdown := `{"jsonrpc":"2.0","id":4,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, open, change, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	messages := decodeMessages(t, output.String())
	var publications [][]any
	for _, message := range messages {
		if method, _ := message["method"].(string); method == "textDocument/publishDiagnostics" {
			publications = append(publications, message["params"].(map[string]any)["diagnostics"].([]any))
		}
	}
	if len(publications) != 2 || len(publications[0]) != 0 || len(publications[1]) != 1 {
		t.Fatalf("diagnostic publications = %#v", publications)
	}
	diagnostic := publications[1][0].(map[string]any)
	if !strings.Contains(diagnostic["message"].(string), "append requires a slice") {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}

func TestSliceToArrayDiagnosticsAndNavigation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "array_conversion.km")
	uri := fileURI(path)
	valid := `function convert(values: int[]): [2]int { return copyArray[[2]int](values); }`
	invalid := `function bad(value: string): [2]int { return viewArray[[2]int](value); }`
	character := strings.LastIndex(valid, "values") + 1
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":%q}}}`, uri, valid)
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":%d}}}`, uri, character)
	definition := fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"textDocument/definition","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":%d}}}`, uri, character)
	navigation := serveMessages(t, openDocument(uri, valid), hover, definition)
	hoverText := navigation[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hoverText, "values: int[]") {
		t.Fatalf("hover = %q", hoverText)
	}
	definitionResult := navigation[3]["result"].(map[string]any)
	start := definitionResult["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(0) || start["character"] != float64(strings.Index(valid, "values")) {
		t.Fatalf("definition = %#v", definitionResult)
	}
	change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`, uri, invalid)
	shutdown := `{"jsonrpc":"2.0","id":4,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, open, change, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	messages := decodeMessages(t, output.String())
	var publications [][]any
	for _, message := range messages {
		if method, _ := message["method"].(string); method == "textDocument/publishDiagnostics" {
			publications = append(publications, message["params"].(map[string]any)["diagnostics"].([]any))
		}
	}
	if len(publications) != 2 || len(publications[0]) != 0 || len(publications[1]) == 0 {
		t.Fatalf("diagnostic publications = %#v", publications)
	}
	diagnostic := publications[1][0].(map[string]any)
	if !strings.Contains(diagnostic["message"].(string), "viewArray requires a slice source") {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}

func TestManifestMissingLockIsPublishedAsDiagnostic(t *testing.T) {
	root := t.TempDir()
	manifest := `[project]
name = "lsp"
version = "0.1.0"
go-module = "example.com/lsp"
go-version = "1.23"
`
	if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "main.km")
	uri := fileURI(path)
	sourceText := `function value(): int { return 42; }`
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":%q}}}`, uri, sourceText)
	shutdown := `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, open, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, message := range decodeMessages(t, output.String()) {
		if method, _ := message["method"].(string); method == "textDocument/publishDiagnostics" {
			diagnostics := message["params"].(map[string]any)["diagnostics"].([]any)
			if len(diagnostics) == 1 && strings.Contains(diagnostics[0].(map[string]any)["message"].(string), "deps lock") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("missing-lock diagnostic was not published: %s", output.String())
	}
}

func TestLifecycleSynchronizationAndDiagnosticMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.km")
	uri := fileURI(path)
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	initialized := `{"jsonrpc":"2.0","method":"initialized","params":{}}`
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"languageId":"kinmokusei","version":1,"text":"function value(): int { return \"wrong\"; }"}}}`, uri)
	change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":"function value(): int { return 42; }"}]}}`, uri)
	closeMessage := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didClose","params":{"textDocument":{"uri":%q}}}`, uri)
	shutdown := `{"jsonrpc":"2.0","id":2,"method":"shutdown","params":null}`
	exit := `{"jsonrpc":"2.0","method":"exit","params":null}`
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, initialized, open, change, closeMessage, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	messages := decodeMessages(t, output.String())
	if len(messages) != 5 {
		t.Fatalf("messages = %#v", messages)
	}
	initializeResult := messages[0]["result"].(map[string]any)
	if initializeResult["serverInfo"].(map[string]any)["name"] != product.DisplayName {
		t.Fatalf("initialize result = %#v", initializeResult)
	}
	capabilities := initializeResult["capabilities"].(map[string]any)
	renameProvider, renameOK := capabilities["renameProvider"].(map[string]any)
	syncProvider, syncOK := capabilities["textDocumentSync"].(map[string]any)
	if capabilities["positionEncoding"] != "utf-16" || !syncOK || syncProvider["change"] != float64(2) || capabilities["hoverProvider"] != true || capabilities["definitionProvider"] != true || capabilities["referencesProvider"] != true || !renameOK || renameProvider["prepareProvider"] != true || capabilities["documentSymbolProvider"] != true || capabilities["completionProvider"] == nil {
		t.Fatalf("capabilities = %#v", capabilities)
	}
	firstDiagnostics := messages[1]["params"].(map[string]any)["diagnostics"].([]any)
	if len(firstDiagnostics) != 1 || !strings.Contains(firstDiagnostics[0].(map[string]any)["message"].(string), "cannot use string as int") {
		t.Fatalf("diagnostics = %#v", firstDiagnostics)
	}
	for _, index := range []int{2, 3} {
		diagnostics := messages[index]["params"].(map[string]any)["diagnostics"].([]any)
		if len(diagnostics) != 0 {
			t.Fatalf("message %d diagnostics = %#v", index, diagnostics)
		}
	}
	if value, exists := messages[4]["result"]; !exists || value != nil {
		t.Fatalf("shutdown response = %#v", messages[4])
	}
}

func TestHoverDefinitionAndDocumentSymbolMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "navigation.km")
	uri := fileURI(path)
	sourceText := "function add(left: int, right: int): int { return left + right; }\n" +
		"function main(): int { const value: int = add(1, 2); return value; }\n" +
		"class Box { public item: int; public function get(): int { return this.item; } }"
	callCharacter := strings.Index(strings.Split(sourceText, "\n")[1], "add") + 1
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":%q}}}`, uri, sourceText)
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":1,"character":%d}}}`, uri, callCharacter)
	definition := fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"textDocument/definition","params":{"textDocument":{"uri":%q},"position":{"line":1,"character":%d}}}`, uri, callCharacter)
	symbols := fmt.Sprintf(`{"jsonrpc":"2.0","id":4,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":%q}}}`, uri)
	missing := fmt.Sprintf(`{"jsonrpc":"2.0","id":5,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":1,"character":0}}}`, uri)
	shutdown := `{"jsonrpc":"2.0","id":6,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, open, hover, definition, symbols, missing, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	messages := decodeMessages(t, output.String())
	byID := map[float64]map[string]any{}
	for _, message := range messages {
		if id, ok := message["id"].(float64); ok {
			byID[id] = message
		}
	}
	hoverResult := byID[2]["result"].(map[string]any)
	hoverText := hoverResult["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hoverText, "function add(left: int, right: int): int") {
		t.Fatalf("hover = %#v", hoverResult)
	}
	definitionResult := byID[3]["result"].(map[string]any)
	definitionStart := definitionResult["range"].(map[string]any)["start"].(map[string]any)
	if definitionResult["uri"] != uri || definitionStart["line"] != float64(0) || definitionStart["character"] != float64(9) {
		t.Fatalf("definition = %#v", definitionResult)
	}
	documentSymbols := byID[4]["result"].([]any)
	if len(documentSymbols) != 3 {
		t.Fatalf("symbols = %#v", documentSymbols)
	}
	box := documentSymbols[2].(map[string]any)
	if box["name"] != "Box" || len(box["children"].([]any)) != 2 {
		t.Fatalf("class symbol = %#v", box)
	}
	if result, exists := byID[5]["result"]; !exists || result != nil {
		t.Fatalf("missing hover = %#v", byID[5])
	}
}

func TestBuiltinExceptionHoverAndDefinition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exception.km")
	uri := fileURI(path)
	text := `function message(): string {
  try { throw new Exception("boom"); }
  catch (err: Exception) { return err.message + err.error(); }
}`
	line := strings.Split(text, "\n")[2]
	positions := map[float64]int{
		2: strings.Index(line, "Exception") + 1,
		3: strings.Index(line, "message") + 1,
		4: strings.Index(line, "error") + 1,
	}
	requests := []string{openDocument(uri, text)}
	for id, character := range positions {
		requests = append(requests, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":2,"character":%d}}}`, int(id), uri, character))
	}
	for id, hoverID := range map[int]float64{5: 2, 6: 3, 7: 4} {
		requests = append(requests, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"textDocument/definition","params":{"textDocument":{"uri":%q},"position":{"line":2,"character":%d}}}`, id, uri, positions[hoverID]))
	}
	messages := serveMessages(t, requests...)
	for id, want := range map[float64]string{2: "class Exception", 3: "public message: string", 4: "public function error(): string"} {
		result, ok := messages[id]["result"].(map[string]any)
		if !ok {
			t.Fatalf("hover %v = %#v", id, messages[id])
		}
		contents := result["contents"].(map[string]any)["value"].(string)
		if !strings.Contains(contents, want) {
			t.Fatalf("hover %v = %q, want %q", id, contents, want)
		}
	}
	for _, id := range []float64{5, 6, 7} {
		if result, exists := messages[id]["result"]; !exists || result != nil {
			t.Fatalf("built-in definition %v = %#v", id, messages[id])
		}
	}
}

func TestNativeStructDocumentSymbolShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "struct.km")
	uri := fileURI(path)
	text := `struct Point { public x: int; private label: string; public function read(): int { return this.x; } }`
	messages := serveMessages(t,
		openDocument(uri, text),
		fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":%q}}}`, uri),
	)
	symbols := messages[2]["result"].([]any)
	if len(symbols) != 1 {
		t.Fatalf("symbols = %#v", symbols)
	}
	structure := symbols[0].(map[string]any)
	if structure["name"] != "Point" || structure["detail"] != "struct Point" || structure["kind"] != float64(23) || len(structure["children"].([]any)) != 3 {
		t.Fatalf("struct symbol = %#v", structure)
	}
	method := structure["children"].([]any)[2].(map[string]any)
	if method["name"] != "read" || method["kind"] != float64(6) {
		t.Fatalf("struct method symbol = %#v", method)
	}
}

func TestExternalReceiverMethodDocumentSymbolShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "external_receiver.km")
	uri := fileURI(path)
	text := `struct Point { public x: int; }
public function move(this: *Point, delta: int): void { this.x += delta; }`
	messages := serveMessages(t,
		openDocument(uri, text),
		fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":%q}}}`, uri),
	)
	symbols := messages[2]["result"].([]any)
	if len(symbols) != 2 {
		t.Fatalf("symbols = %#v", symbols)
	}
	method := symbols[1].(map[string]any)
	if method["name"] != "move" || method["kind"] != float64(6) || len(method["children"].([]any)) != 2 {
		t.Fatalf("external method symbol = %#v", method)
	}
}

func TestDefinitionAcrossRelativeImportUsesDependencyURI(t *testing.T) {
	directory := t.TempDir()
	dependencyPath := filepath.Join(directory, "dependency.km")
	if err := os.WriteFile(dependencyPath, []byte(`function helper(value: int): int { return value; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	rootPath := filepath.Join(directory, "root.km")
	rootURI := fileURI(rootPath)
	rootText := `import { helper } from "./dependency"; function value(): int { return helper(42); }`
	character := strings.LastIndex(rootText, "helper") + 1
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":%q}}}`, rootURI, rootText)
	definition := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/definition","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":%d}}}`, rootURI, character)
	shutdown := `{"jsonrpc":"2.0","id":3,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, open, definition, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	for _, message := range decodeMessages(t, output.String()) {
		if message["id"] == float64(2) {
			result := message["result"].(map[string]any)
			if result["uri"] != fileURI(dependencyPath) {
				t.Fatalf("definition = %#v", result)
			}
			return
		}
	}
	t.Fatal("definition response is missing")
}

func TestHoverAndDefinitionForCollectionRangeBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "range.km")
	uri := fileURI(path)
	sourceText := "function total(items: int[]): int {\n" +
		"  let result = 0;\n" +
		"  for (const [index: int, value: int] of items) { result = result + index + value; }\n" +
		"  return result;\n}"
	useCharacter := strings.LastIndex(strings.Split(sourceText, "\n")[2], "index") + 1
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":%q}}}`, uri, sourceText)
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":2,"character":%d}}}`, uri, useCharacter)
	definition := fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"textDocument/definition","params":{"textDocument":{"uri":%q},"position":{"line":2,"character":%d}}}`, uri, useCharacter)
	shutdown := `{"jsonrpc":"2.0","id":4,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, open, hover, definition, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	byID := map[float64]map[string]any{}
	for _, message := range decodeMessages(t, output.String()) {
		if id, ok := message["id"].(float64); ok {
			byID[id] = message
		}
	}
	hoverText := byID[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hoverText, "const index: int") {
		t.Fatalf("hover = %q", hoverText)
	}
	definitionResult := byID[3]["result"].(map[string]any)
	start := definitionResult["range"].(map[string]any)["start"].(map[string]any)
	if definitionResult["uri"] != uri || start["line"] != float64(2) || start["character"] != float64(14) {
		t.Fatalf("definition = %#v", definitionResult)
	}
}

func TestHoverAndDefinitionForSelectReceiveBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "select.km")
	uri := fileURI(path)
	sourceText := "function receive(channel: GoReceiveChannel<int>): int {\n" +
		"  select {\n" +
		"    case const value = <-channel { return value; }\n" +
		"  }\n}"
	useCharacter := strings.LastIndex(strings.Split(sourceText, "\n")[2], "value") + 1
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":%q}}}`, uri, sourceText)
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":2,"character":%d}}}`, uri, useCharacter)
	definition := fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"textDocument/definition","params":{"textDocument":{"uri":%q},"position":{"line":2,"character":%d}}}`, uri, useCharacter)
	shutdown := `{"jsonrpc":"2.0","id":4,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, open, hover, definition, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	byID := map[float64]map[string]any{}
	for _, message := range decodeMessages(t, output.String()) {
		if id, ok := message["id"].(float64); ok {
			byID[id] = message
		}
	}
	hoverText := byID[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hoverText, "const value") {
		t.Fatalf("hover = %q", hoverText)
	}
	definitionResult := byID[3]["result"].(map[string]any)
	start := definitionResult["range"].(map[string]any)["start"].(map[string]any)
	if definitionResult["uri"] != uri || start["line"] != float64(2) || start["character"] != float64(15) {
		t.Fatalf("definition = %#v", definitionResult)
	}
}

func TestHoverAndDefinitionForTypeSwitchBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "type_switch.km")
	uri := fileURI(path)
	sourceText := "import go io from \"io\"; import go strings from \"strings\";\n" +
		"function size(value: io.Reader): int {\n" +
		"  switch (value) { case const reader as *strings.Reader { return reader.Len(); } default { return 0; } }\n}"
	useCharacter := strings.LastIndex(strings.Split(sourceText, "\n")[2], "reader") + 1
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":%q}}}`, uri, sourceText)
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":2,"character":%d}}}`, uri, useCharacter)
	definition := fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"textDocument/definition","params":{"textDocument":{"uri":%q},"position":{"line":2,"character":%d}}}`, uri, useCharacter)
	shutdown := `{"jsonrpc":"2.0","id":4,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, open, hover, definition, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	byID := map[float64]map[string]any{}
	for _, message := range decodeMessages(t, output.String()) {
		if id, ok := message["id"].(float64); ok {
			byID[id] = message
		}
	}
	hoverText := byID[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hoverText, "const reader as *strings.Reader") {
		t.Fatalf("hover = %q", hoverText)
	}
	definitionResult := byID[3]["result"].(map[string]any)
	start := definitionResult["range"].(map[string]any)["start"].(map[string]any)
	if definitionResult["uri"] != uri || start["line"] != float64(2) || start["character"] != float64(30) {
		t.Fatalf("definition = %#v", definitionResult)
	}
}

func TestMultipleOpenDocumentsKeepIndependentDiagnostics(t *testing.T) {
	directory := t.TempDir()
	firstURI := fileURI(filepath.Join(directory, "first.km"))
	secondURI := fileURI(filepath.Join(directory, "second.km"))
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	openFirst := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":"function first(): int { return \"wrong\"; }"}}}`, firstURI)
	openSecond := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":"function second(): int { return \"wrong\"; }"}}}`, secondURI)
	shutdown := `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(initialize, openFirst, openSecond, shutdown, exit)), &output); err != nil {
		t.Fatal(err)
	}
	messages := decodeMessages(t, output.String())
	if len(messages) != 4 {
		t.Fatalf("messages = %#v", messages)
	}
	lastDiagnostic := messages[2]["params"].(map[string]any)
	if lastDiagnostic["uri"] != secondURI {
		t.Fatalf("last diagnostic = %#v", lastDiagnostic)
	}
	if diagnostics := lastDiagnostic["diagnostics"].([]any); len(diagnostics) != 1 {
		t.Fatalf("second diagnostics = %#v", diagnostics)
	}
}

func TestRequestAndLifecycleFailureMatrix(t *testing.T) {
	tests := []struct {
		name     string
		messages []string
		wantCode float64
		wantErr  error
	}{
		{"request before initialize", []string{`{"jsonrpc":"2.0","id":1,"method":"shutdown"}`}, -32002, nil},
		{"unknown request", []string{`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, `{"jsonrpc":"2.0","id":2,"method":"unknown"}`}, -32601, nil},
		{"duplicate initialize", []string{`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, `{"jsonrpc":"2.0","id":2,"method":"initialize"}`}, -32600, nil},
		{"object request id", []string{`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, `{"jsonrpc":"2.0","id":{},"method":"textDocument/hover"}`}, -32600, nil},
		{"fractional request id", []string{`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, `{"jsonrpc":"2.0","id":2.5,"method":"textDocument/hover"}`}, -32600, nil},
		{"cancellation sent as request", []string{`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, `{"jsonrpc":"2.0","id":2,"method":"$/cancelRequest","params":{"id":99}}`}, -32600, nil},
		{"request after shutdown", []string{`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","id":3,"method":"unknown"}`}, -32600, nil},
		{"exit without shutdown", []string{`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, `{"jsonrpc":"2.0","method":"exit"}`}, 0, ErrExitWithoutShutdown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := Serve(strings.NewReader(framed(test.messages...)), &output)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("err = %v, want %v", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			messages := decodeMessages(t, output.String())
			last := messages[len(messages)-1]
			if code := last["error"].(map[string]any)["code"]; code != test.wantCode {
				t.Fatalf("last response = %#v", last)
			}
		})
	}
}

func TestProtocolPositionUsesZeroBasedUTF16(t *testing.T) {
	text := "a😀b\n日本"
	tests := []struct {
		name   string
		offset int
		want   position
	}{
		{"start", 0, position{}},
		{"astral rune", len("a😀"), position{Line: 0, Character: 3}},
		{"second line", len("a😀b\n日"), position{Line: 1, Character: 1}},
		{"clamped", len(text) + 10, position{Line: 1, Character: 2}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := protocolPosition(text, source.Position{Offset: test.offset}); got != test.want {
				t.Fatalf("position = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestByteOffsetAtPositionUTF16Matrix(t *testing.T) {
	text := "a😀b\n日本"
	tests := []struct {
		name   string
		input  position
		want   int
		failed bool
	}{
		{"start", position{}, 0, false},
		{"after astral rune", position{Line: 0, Character: 3}, len("a😀"), false},
		{"second line", position{Line: 1, Character: 1}, len("a😀b\n日"), false},
		{"split surrogate pair", position{Line: 0, Character: 2}, 0, true},
		{"line overflow", position{Line: 0, Character: 9}, 0, true},
		{"missing line", position{Line: 3, Character: 0}, 0, true},
		{"negative", position{Line: -1, Character: 0}, 0, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := byteOffsetAtPosition(text, test.input)
			if (err != nil) != test.failed || got != test.want {
				t.Fatalf("offset=%d err=%v, want offset=%d failed=%v", got, err, test.want, test.failed)
			}
		})
	}
}

func TestFramingAndURIErrorMatrix(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing length", "Other: 1\r\n\r\n"},
		{"invalid length", "Content-Length: nope\r\n\r\n"},
		{"negative length", "Content-Length: -1\r\n\r\n"},
		{"too large", fmt.Sprintf("Content-Length: %d\r\n\r\n", maxMessageSize+1)},
		{"truncated", "Content-Length: 5\r\n\r\n{}"},
		{"malformed header", "broken\r\n\r\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := readMessage(bufio.NewReader(strings.NewReader(test.input))); err == nil {
				t.Fatal("expected framing error")
			}
		})
	}
	for _, uri := range []string{"https://example.com/main.km", "file://remote/main.km", "://bad"} {
		if _, err := filePath(uri); err == nil {
			t.Errorf("filePath(%q) unexpectedly succeeded", uri)
		}
	}
}
