package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func signatureHelpAt(t *testing.T, path, text string, at position) map[string]any {
	t.Helper()
	uri := fileURI(path)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/signatureHelp", 2, uri, at, ""),
	)
	return messages[2]
}

func signatureResult(t *testing.T, message map[string]any) (string, float64, []any) {
	t.Helper()
	result, ok := message["result"].(map[string]any)
	if !ok {
		t.Fatalf("signatureHelp result = %#v", message)
	}
	signatures := result["signatures"].([]any)
	if len(signatures) != 1 {
		t.Fatalf("signatures = %#v", signatures)
	}
	signature := signatures[0].(map[string]any)
	parameters, _ := signature["parameters"].([]any)
	return signature["label"].(string), result["activeParameter"].(float64), parameters
}

func TestSignatureHelpOnsenFunctionAndActiveParameter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "function.otm")
	text := `function combine(left: int, right: string): boolean { return left > 0 && right !== ""; }
function main(): boolean { return combine(1, "value"); }`
	at := positionOf(text, `"value"`, 0)
	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, text, at))
	if label != "combine(left: int, right: string): boolean" || active != 1 || len(parameters) != 2 {
		t.Fatalf("signature = %q active=%v parameters=%#v", label, active, parameters)
	}
	if parameters[1].(map[string]any)["label"] != "right: string" {
		t.Fatalf("second parameter = %#v", parameters[1])
	}
}

func TestSignatureHelpNativeVariadicDeclarations(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		needle     string
		wantLabel  string
		wantActive float64
	}{
		{
			"function",
			`function sum(prefix: int, ...values: int[]): int { return prefix; } function use(): int { return sum(1, 2, 3); }`,
			"3);", "sum(prefix: int, ...values: int[]): int", 1,
		},
		{
			"constructor",
			`class Batch { constructor(public ...values: int[]) {} } function use(): Batch { return new Batch(1, 2); }`,
			"2);", "new Batch(...values: int[]): Batch", 0,
		},
		{
			"arrow",
			`function use(): int { const sum = (...values: int[]): int => len(values); return sum(1, 2); }`,
			"2);", "sum(...values: int[]): int", 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), test.name+".otm")
			label, active, _ := signatureResult(t, signatureHelpAt(t, path, test.text, positionOf(test.text, test.needle, 0)))
			if label != test.wantLabel || active != test.wantActive {
				t.Fatalf("signature = %q active=%v, want %q active=%v", label, active, test.wantLabel, test.wantActive)
			}
		})
	}
}

func TestSignatureHelpInsideValueSwitch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "switch.otm")
	text := `function combine(left: int, right: int): int { return left + right; }
function classify(value: int): int { switch (combine(value, 1)) { case combine(value, 2) { return 1; } default { return 0; } } }`
	label, active, _ := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, "2)", 0)))
	if label != "combine(left: int, right: int): int" || active != 1 {
		t.Fatalf("signature = %q active=%v", label, active)
	}
}

func TestSignatureHelpInsidePropagationExpression(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.otm")
	text := `function parse(text: string, radix: int): Result<int> { return ok(radix); }
function use(): Result<int> { const value = parse("21", 10)?; return ok(value); }`
	label, active, _ := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, "10", 0)))
	if label != "parse(text: string, radix: int): Result<int>" || active != 1 {
		t.Fatalf("signature = %q active=%v", label, active)
	}
}

func TestSignatureHelpResultConstructors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "constructors.otm")
	text := `function value(): Result<int> { return ok(42); }`
	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, "42", 0)))
	if label != "ok(value: int): Result<int>" || active != 0 || len(parameters) != 1 {
		t.Fatalf("signature = %q active=%v parameters=%#v", label, active, parameters)
	}
}

func TestSignatureHelpNestedDelimiterAndStringCommaMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested.otm")
	text := `function inner(first: int, second: int): int { return first + second; }
function outer(value: int, text: string, items: int[]): int { return value + len(text) + len(items); }
function main(): int { return outer(inner(1, 2), "a,b", [1, 2]); }`
	tests := []struct {
		name       string
		at         position
		wantPrefix string
		active     float64
	}{
		{"nested call", positionOf(text, "2),", 0), "inner(", 1},
		{"string comma", positionOf(text, "a,b", 0), "outer(", 1},
		{"array comma", positionOf(text, "2]);", 0), "outer(", 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			label, active, _ := signatureResult(t, signatureHelpAt(t, path, text, test.at))
			if !strings.HasPrefix(label, test.wantPrefix) || active != test.active {
				t.Fatalf("signature = %q active=%v", label, active)
			}
		})
	}
}

func TestSignatureHelpFunctionValuesAndClassMethod(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		needle     string
		wantLabel  string
		wantActive float64
	}{
		{
			"function typed parameter",
			`function apply(operation: (left: int, right: string) => boolean): boolean { return operation(1, "x"); }`,
			`"x"`, "operation(arg1: int, arg2: string): boolean", 1,
		},
		{
			"inferred arrow local",
			`function apply(): int { const operation = (left: int, right: int) => left + right; return operation(1, 2); }`,
			"2);", "operation(left: int, right: int): int", 1,
		},
		{
			"class method",
			`class Box { public function mix(left: int, right: string): int { return left + len(right); } } function apply(box: Box): int { return box.mix(1, "x"); }`,
			`"x"`, "box.mix(left: int, right: string): int", 1,
		},
		{
			"external struct receiver method",
			`struct Box { public value: int; } public function mix(this: *Box, left: int, right: string): int { return this.value + left + len(right); } function apply(box: *Box): int { return box.mix(1, "x"); }`,
			`"x"`, "box.mix(left: int, right: string): int", 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "callable.otm")
			label, active, _ := signatureResult(t, signatureHelpAt(t, path, test.text, positionOf(test.text, test.needle, 0)))
			if label != test.wantLabel || active != test.wantActive {
				t.Fatalf("signature = %q active=%v", label, active)
			}
		})
	}
}

func TestSignatureHelpConstructorMatrix(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		at         position
		wantLabel  string
		wantActive float64
	}{
		{
			name: "complete constructor",
			text: `class Box { constructor(value: int, label: string) {} }
function make(): Box { return new Box(1, "x"); }`,
			wantLabel: "new Box(value: int, label: string): Box", wantActive: 1,
		},
		{
			name: "incomplete constructor",
			text: `class Box { constructor(value: int, label: string) {} }
function make(): Box { return new Box(1, `,
			wantLabel: "new Box(value: int, label: string): Box", wantActive: 1,
		},
		{
			name: "implicit zero argument constructor",
			text: `class Empty {}
function make(): Empty { return new Empty(); }`,
			wantLabel: "new Empty(): Empty", wantActive: 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "constructor.otm")
			at := test.at
			if at == (position{}) {
				if strings.Contains(test.text, `"x"`) {
					at = positionOf(test.text, `"x"`, 0)
				} else if strings.HasSuffix(test.text, "new Box(1, ") {
					lines := strings.Split(test.text, "\n")
					at = position{Line: len(lines) - 1, Character: utf16Length(lines[len(lines)-1])}
				} else {
					at = positionOf(test.text, "Empty()", 0)
					at.Character += utf16Length("Empty(")
				}
			}
			label, active, _ := signatureResult(t, signatureHelpAt(t, path, test.text, at))
			if label != test.wantLabel || active != test.wantActive {
				t.Fatalf("signature = %q active=%v", label, active)
			}
		})
	}
}

func TestSignatureHelpBuiltinExceptionConstructor(t *testing.T) {
	tests := []struct {
		name string
		text string
		at   position
	}{
		{
			name: "complete",
			text: `function make(): Exception { return new Exception("boom"); }`,
		},
		{
			name: "incomplete",
			text: `function make(): Exception { return new Exception(`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "exception.otm")
			at := test.at
			if at == (position{}) {
				if strings.HasSuffix(test.text, "(") {
					at = position{Character: utf16Length(test.text)}
				} else {
					at = positionOf(test.text, `"boom"`, 0)
				}
			}
			label, active, parameters := signatureResult(t, signatureHelpAt(t, path, test.text, at))
			if label != "new Exception(message: string): Exception" || active != 0 || len(parameters) != 1 {
				t.Fatalf("signature = %q active=%v parameters=%#v", label, active, parameters)
			}
		})
	}
}

func TestSignatureHelpGoFunctionsGenericAndVariadic(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		needle     string
		wantParts  []string
		wantActive float64
	}{
		{
			"ordinary", `import go strings from "strings"; function value(): string { return strings.ReplaceAll("a", "a", "b"); }`,
			`"b"`, []string{"strings.ReplaceAll(", "s: string", "old: string", "new: string", "): string"}, 2,
		},
		{
			"variadic clamps to final parameter", `import go path from "path"; function value(): string { return path.Join("a", "b", "c"); }`,
			`"c"`, []string{"path.Join(", "...elem: string", "): string"}, 0,
		},
		{
			"inferred generic", `import go slices from "slices"; function value(items: int[]): int[] { return slices.Clone(items); }`,
			"items);", []string{"slices.Clone(", "int[]", "): int[]"}, 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "go_call.otm")
			label, active, _ := signatureResult(t, signatureHelpAt(t, path, test.text, positionOf(test.text, test.needle, 0)))
			for _, part := range test.wantParts {
				if !strings.Contains(label, part) {
					t.Fatalf("signature %q does not contain %q", label, part)
				}
			}
			if test.name == "variadic clamps to final parameter" {
				if active != 0 {
					t.Fatalf("variadic active parameter = %v, want final parameter index 0", active)
				}
			} else if active != test.wantActive {
				t.Fatalf("active parameter = %v, want %v", active, test.wantActive)
			}
		})
	}
}

func TestSignatureHelpGoValueMethodCompleteAndIncomplete(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		at         position
		wantMethod string
		wantParam  string
		wantResult string
	}{
		{
			name:       "complete",
			text:       `import go context from "context"; import go web from "net/http"; function clone(request: *web.Request, ctx: context.Context): *web.Request { return request.Clone(ctx); }`,
			wantMethod: "request.Clone(", wantParam: "ctx: context.Context", wantResult: "*http.Request",
		},
		{
			name:       "incomplete",
			text:       `import go context from "context"; import go web from "net/http"; function clone(request: *web.Request, ctx: context.Context): *web.Request { return request.Clone(`,
			wantMethod: "request.Clone(", wantParam: "ctx: context.Context", wantResult: "*http.Request",
		},
		{
			name:       "inferred package variable incomplete",
			text:       `import go web from "net/http"; function send(request: *web.Request): Result<*web.Response> { const client = web.DefaultClient; return client.Do(`,
			wantMethod: "client.Do(", wantParam: "req: *http.Request", wantResult: "(*http.Response, error)",
		},
		{
			name:       "multiple result binding incomplete",
			text:       `import go web from "net/http"; function clone(): Result<*web.Request> { const [request, err] = web.NewRequest(web.MethodGet, "https://example.com", nil); if (err != nil) { return fail(err); } return request.Clone(`,
			wantMethod: "request.Clone(", wantParam: "ctx: context.Context", wantResult: "*http.Request",
		},
		{
			name:       "nested incomplete braces",
			text:       `import go web from "net/http"; class Sender { public function send(client: *web.Client, request: *web.Request): Result<*web.Response> { if (true) { return client.Do(`,
			wantMethod: "client.Do(", wantParam: "req: *http.Request", wantResult: "(*http.Response, error)",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "go_method.otm")
			at := test.at
			if at == (position{}) {
				if strings.HasSuffix(test.text, "(") {
					at = position{Character: utf16Length(test.text)}
				} else {
					at = positionOf(test.text, "ctx);", 0)
				}
			}
			label, active, parameters := signatureResult(t, signatureHelpAt(t, path, test.text, at))
			if !strings.Contains(label, test.wantMethod) || !strings.Contains(label, test.wantParam) || !strings.HasSuffix(label, "): "+test.wantResult) || active != 0 || len(parameters) != 1 {
				t.Fatalf("signature = %q active=%v parameters=%#v", label, active, parameters)
			}
		})
	}
}

func TestSignatureHelpRejectsGoFieldAsMethod(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go_field.otm")
	text := `import go web from "net/http"; function invalid(request: *web.Request): string { return request.Method(`
	at := position{Character: utf16Length(text)}
	message := signatureHelpAt(t, path, text, at)
	if result, exists := message["result"]; !exists || result != nil {
		t.Fatalf("field signature help = %#v", message)
	}
}

func TestSignatureHelpBuiltinMatrix(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		needle     string
		wantLabel  string
		wantActive float64
	}{
		{
			"fixed arity", `function size(values: int[]): int { return len(values); }`,
			"values);", "len(value: collection): int", 0,
		},
		{
			"variadic", `function grow(values: int[]): int[] { return append(values, 1, 2); }`,
			"2);", "append(destination: T[], ...values: T): T[]", 1,
		},
		{
			"ordered variadic", `function lower(left: int, right: int): int { return min(left, right); }`,
			"right);", "min(...values: ordered): T", 0,
		},
		{
			"clear", `function reset(values: int[]): void { clear(values); }`,
			"values);", "clear(collection: T[] | Map<K, V>): void", 0,
		},
		{
			"optional capacity", `function allocate(): int[] { return makeSlice[int](1, 2); }`,
			"2);", "makeSlice(length: int, capacity?: int): T[]", 1,
		},
		{
			"incomplete generic builtin", `function allocate(): int[] { return makeSlice[int](1, `,
			"", "makeSlice(length: int, capacity?: int): T[]", 1,
		},
		{
			"user shadow wins", `function len(value: int, radix: string): int { return value; }
function size(): int { return len(1, "decimal"); }`,
			`"decimal"`, "len(value: int, radix: string): int", 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "builtin.otm")
			var at position
			if test.needle == "" {
				lines := strings.Split(test.text, "\n")
				at = position{Line: len(lines) - 1, Character: utf16Length(lines[len(lines)-1])}
			} else {
				at = positionOf(test.text, test.needle, 0)
			}
			label, active, _ := signatureResult(t, signatureHelpAt(t, path, test.text, at))
			if label != test.wantLabel || active != test.wantActive {
				t.Fatalf("signature = %q active=%v", label, active)
			}
		})
	}
}

func TestSignatureHelpIncompleteOnsenAndGoCalls(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		wantPrefix string
		active     float64
	}{
		{
			"OnsenTamago", `function add(left: int, right: int): int { return left + right; }
function main(): int { return add(1, `,
			"add(left: int, right: int): int", 1,
		},
		{
			"Go package", `import go strings from "strings";
function main(): string { return strings.ReplaceAll("a", `,
			"strings.ReplaceAll(", 1,
		},
		{
			"Go collection spelling", `import go bytes from "bytes";
function clone(value: byte[]): byte[] { return bytes.Clone(`,
			"bytes.Clone(b: byte[]): byte[]", 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "incomplete.otm")
			lines := strings.Split(test.text, "\n")
			at := position{Line: len(lines) - 1, Character: utf16Length(lines[len(lines)-1])}
			label, active, _ := signatureResult(t, signatureHelpAt(t, path, test.text, at))
			if !strings.HasPrefix(label, test.wantPrefix) || active != test.active {
				t.Fatalf("signature = %q active=%v", label, active)
			}
		})
	}
}

func TestSignatureHelpAcrossUnsavedRelativeImport(t *testing.T) {
	directory := t.TempDir()
	dependency := filepath.Join(directory, "dependency.otm")
	entry := filepath.Join(directory, "main.otm")
	dependencyText := `function helper(value: int, label: string): int { return value + len(label); }`
	entryText := `import { helper } from "./dependency"; function main(): int { return helper(1, "x"); }`
	if err := os.WriteFile(dependency, []byte(`function stale(): int { return 0; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	entryURI := fileURI(entry)
	messages := serveMessages(t,
		openDocument(fileURI(dependency), dependencyText),
		openDocument(entryURI, entryText),
		requestAt("textDocument/signatureHelp", 2, entryURI, positionOf(entryText, `"x"`, 0), ""),
	)
	label, active, _ := signatureResult(t, messages[2])
	if label != "helper(value: int, label: string): int" || active != 1 {
		t.Fatalf("signature = %q active=%v", label, active)
	}
}

func TestSignatureHelpInvalidContextMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.otm")
	uri := fileURI(path)
	text := `function helper(value: int): int { return value; }
function main(): int { const value = helper(1); return value; } // helper(`
	tests := []struct {
		name string
		at   position
	}{
		{"function declaration", positionOf(text, "value: int", 0)},
		{"after closed call", positionOf(text, "; return", 0)},
		{"comment", position{Line: 1, Character: utf16Length(strings.Split(text, "\n")[1])}},
		{"outside document", position{Line: 99, Character: 0}},
	}
	requests := []string{openDocument(uri, text)}
	for index, test := range tests {
		requests = append(requests, requestAt("textDocument/signatureHelp", index+2, uri, test.at, ""))
	}
	messages := serveMessages(t, requests...)
	for index, test := range tests {
		if messages[float64(index+2)]["result"] != nil {
			t.Errorf("%s signatureHelp = %#v, want null", test.name, messages[float64(index+2)])
		}
	}
}

func TestCallContextActiveParameterMatrix(t *testing.T) {
	tests := []struct {
		text   string
		active int
		name   string
	}{
		{"call(", 0, "call"},
		{"call(1, ", 1, "call"},
		{"outer(inner(1, 2), ", 1, "outer"},
		{`call("a,b", `, 1, "call"},
		{"call([1, 2], { value: 1, other: 2 }, ", 2, "call"},
		{"pkg.Call[int, string](1, ", 1, "Call"},
		{"new Value(1, ", 1, "Value"},
	}
	for _, test := range tests {
		context, ok := callContextAt("context.otm", test.text, len(test.text))
		if !ok || context.ActiveParameter != test.active || context.Name != test.name {
			t.Errorf("callContextAt(%q) = %#v, %v", test.text, context, ok)
		}
	}
}

func TestSignatureHelpCapability(t *testing.T) {
	messages := serveMessages(t)
	capabilities := messages[1]["result"].(map[string]any)["capabilities"].(map[string]any)
	provider, ok := capabilities["signatureHelpProvider"].(map[string]any)
	triggers, triggerOK := provider["triggerCharacters"].([]any)
	if !ok || !triggerOK || len(triggers) != 2 || triggers[0] != "(" || triggers[1] != "," {
		t.Fatalf("signatureHelpProvider = %#v", capabilities["signatureHelpProvider"])
	}
}

func FuzzCallContextAtNeverPanics(f *testing.F) {
	for _, seed := range []string{
		"call(",
		"outer(inner(1, 2), [3, 4], ",
		`pkg.Call[Map<string, int>, string]("a,b", `,
		"new Value(1, ",
		"await go service.load(1, ",
		"call(/* unfinished comment",
	} {
		f.Add(seed, uint32(len(seed)))
	}
	f.Fuzz(func(t *testing.T, text string, rawOffset uint32) {
		if len(text) > 1<<16 {
			t.Skip()
		}
		offset := int(rawOffset % uint32(len(text)+1))
		context, ok := callContextAt("fuzz.otm", text, offset)
		if !ok {
			return
		}
		_ = qualifiedCallAnalysisText(text, offset, context)
		if context.ActiveParameter < 0 || context.OpenOffset < 0 || context.OpenOffset >= offset || context.CalleeOffset > context.OpenOffset || context.DisplayName == "" {
			t.Fatalf("invalid context at %d: %#v", offset, context)
		}
	})
}
