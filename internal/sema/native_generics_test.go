package sema

import (
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestNativeGenericFunctionSemanticMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"inferred scalar", `function identity<T>(value: T): T { return value; } function use(): string { return identity("onsen"); }`},
		{"explicit angle", `function identity<T>(value: T): T { return value; } function use(): string { return identity<string>("onsen"); }`},
		{"explicit bracket", `function identity<T>(value: T): T { return value; } function use(): string { return identity[string]("onsen"); }`},
		{"multiple parameters", `function second<T, U>(left: T, right: U): U { return right; } function use(): string { return second(1, "onsen"); }`},
		{"partial explicit", `function second<T, U>(left: T, right: U): U { return right; } function use(): string { return second<int>(1, "onsen"); }`},
		{"repeated parameter", `function choose<T>(left: T, right: T): T { return left; } function use(): int { return choose(1, 2); }`},
		{"slice inference", `function first<T>(items: T[]): T { return items[0]; } function use(): string { return first(["onsen", "tamago"]); }`},
		{"fixed array inference", `function first<T>(items: [2]T): T { return items[0]; } function use(items: [2]int): int { return first(items); }`},
		{"nested collection inference", `function first<T>(values: Map<string, T[]>): T { return values["value"][0]; } function use(values: Map<string, string[]>): string { return first(values); }`},
		{"pointer round trip", `struct Point { public x: int; } function identity<T>(value: T): T { return value; } function use(value: *Point): *Point { return identity(value); }`},
		{"pointer element inference", `struct Point { public x: int; } function dereference<T>(value: *T): T { return *value; } function use(value: *Point): Point { return dereference(value); }`},
		{"class round trip", `class Box { constructor(public value: int) {} } function identity<T>(value: T): T { return value; } function use(value: Box): Box { return identity(value); }`},
		{"nullable inference", `class Box {} function identity<T>(value: T): T { return value; } function use(value: Box | null): Box | null { return identity(value); }`},
		{"map value inference", `function lookup<T>(values: Map<string, T>): T { return values["value"]; } function use(values: Map<string, int>): int { return lookup(values); }`},
		{"object field inference", `function unwrap<T>(holder: { value: T }): T { return holder.value; } function use(holder: { value: string }): string { return unwrap(holder); }`},
		{"callback inference", `function apply<T>(value: T, transform: (item: T) => T): T { return transform(value); } function use(): int { return apply(1, (value: int): int => value); }`},
		{"generic forwarding", `function identity<T>(value: T): T { return value; } function relay<T>(value: T): T { return identity(value); } function use(): string { return relay("onsen"); }`},
		{"channel element inference", `function sameChannel<T>(value: GoChannel<T>): GoChannel<T> { return value; } function use(value: GoChannel<int>): GoChannel<int> { return sameChannel(value); }`},
		{"Result propagation", `function present<T>(value: T): Result<T> { return ok(value); } function use(): Result<string> { const value = present("onsen")?; return ok(value); }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestNativeGenericFunctionResolvedMetadata(t *testing.T) {
	input := `function identity<T>(value: T): T { return value; }
function use(value: string): string { return identity<string>(value); }`
	tokens, lexDiagnostics := lexer.Lex("generic_metadata.km", input)
	if len(lexDiagnostics) != 0 {
		t.Fatalf("lexer diagnostics = %v", lexDiagnostics)
	}
	program, parseDiagnostics := parser.Parse(tokens)
	if len(parseDiagnostics) != 0 {
		t.Fatalf("parser diagnostics = %v", parseDiagnostics)
	}
	if diagnostics := Check(program); len(diagnostics) != 0 {
		t.Fatalf("semantic diagnostics = %v", diagnostics)
	}
	identity := program.Declarations[0].(*ast.FunctionDecl)
	for name, ref := range map[string]ast.TypeRef{"parameter": identity.Parameters[0].Type, "result": identity.ReturnType} {
		if !ref.TypeParameter || ref.ResolvedDeclaration.Start.Offset != identity.TypeParameters[0].NameSpan.Start.Offset {
			t.Errorf("%s type metadata = %#v, declaration = %#v", name, ref, identity.TypeParameters[0])
		}
	}
	use := program.Declarations[1].(*ast.FunctionDecl)
	call := use.Body.Statements[0].(*ast.ReturnStmt).Value.(*ast.CallExpr)
	if call.Signature == nil || len(call.Signature.ParameterTypes) != 1 || call.Signature.ParameterTypes[0] != "string" || call.Signature.Result != "string" {
		t.Fatalf("instantiated call signature = %#v", call.Signature)
	}
}

func TestNativeGenericFunctionFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"duplicate type parameter", `function bad<T, T>(value: T): T { return value; }`, `duplicate generic function type parameter "T"`},
		{"built-in type parameter", `function bad<int>(value: int): int { return value; }`, `conflicts with a built-in type`},
		{"blank type parameter", `function bad<_>(value: int): int { return value; }`, `cannot be '_'`},
		{"Go constraint name", `function bad<any>(value: any): any { return value; }`, `cannot be 'any'`},
		{"unknown outside scope", `function identity<T>(value: T): T { return value; } function bad(value: T): T { return value; }`, `unknown type "T"`},
		{"inconsistent inference", `function choose<T>(left: T, right: T): T { return left; } function bad(): int { return choose(1, "x"); }`, `was already inferred as int, not string`},
		{"explicit argument mismatch", `function identity<T>(value: T): T { return value; } function bad(): int { return identity<int>("x"); }`, `was already inferred as int, not string`},
		{"too many explicit", `function identity<T>(value: T): T { return value; } function bad(): int { return identity<int, string>(1); }`, `has 1 type parameters, got 2 explicit type arguments`},
		{"missing inference", `function choose<T, U>(value: T): T { return value; } function bad(): int { return choose(1); }`, `cannot infer type argument U`},
		{"invalid explicit type", `function identity<T>(value: T): T { return value; } function bad(): int { return identity<void>(1); }`, `type void cannot be used as a generic function type argument`},
		{"uninstantiated value", `function identity<T>(value: T): T { return value; } function bad(): void { const callback = identity; }`, `must be called before it can be used as a value`},
		{"unsupported operator", `function add<T>(left: T, right: T): T { return left + right; }`, `operator + requires numeric operands`},
		{"generic main", `function main<T>(value: T): T { return value; }`, `function "main" cannot declare type parameters`},
		{"generic C ABI", `export c("identity") function identity<T>(value: T): T { return value; }`, `generic functions cannot be exported through the C ABI`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
