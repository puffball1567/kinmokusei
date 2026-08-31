package parser

import (
	"strings"
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
	"github.com/puffball1567/onsentamago/internal/lexer"
)

func TestParsesNativeGenericFunctionsAndCalls(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function identity<T>(value: T): T { return value; }
function pair<T, U>(left: T, right: U): U { return right; }
function use(): string {
  const inferred = identity("onsen");
  const explicitAngle = identity<string>(inferred);
  const explicitBracket = identity[string](explicitAngle);
  return pair<int, string>(1, explicitBracket);
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	identity := program.Declarations[0].(*ast.FunctionDecl)
	if len(identity.TypeParameters) != 1 || identity.TypeParameters[0].Name != "T" || identity.Parameters[0].Type.Name != "T" || identity.ReturnType.Name != "T" {
		t.Fatalf("identity declaration = %#v", identity)
	}
	pair := program.Declarations[1].(*ast.FunctionDecl)
	if len(pair.TypeParameters) != 2 || pair.TypeParameters[0].Name != "T" || pair.TypeParameters[1].Name != "U" {
		t.Fatalf("pair type parameters = %#v", pair.TypeParameters)
	}
	use := program.Declarations[2].(*ast.FunctionDecl)
	for index, want := range []string{"", "string", "string"} {
		call := use.Body.Statements[index].(*ast.VariableDecl).Value.(*ast.CallExpr)
		if want == "" {
			if len(call.TypeArguments) != 0 {
				t.Fatalf("inferred call type arguments = %#v", call.TypeArguments)
			}
			continue
		}
		if len(call.TypeArguments) != 1 || call.TypeArguments[0].Name != want {
			t.Fatalf("call %d type arguments = %#v", index, call.TypeArguments)
		}
	}
	returned := use.Body.Statements[3].(*ast.ReturnStmt).Value.(*ast.CallExpr)
	if len(returned.TypeArguments) != 2 || returned.TypeArguments[0].Name != "int" || returned.TypeArguments[1].Name != "string" {
		t.Fatalf("pair call type arguments = %#v", returned.TypeArguments)
	}
}

func TestNativeGenericFunctionSyntaxFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"empty parameters", `function identity<>(value: int): int { return value; }`, "type parameter list cannot be empty"},
		{"trailing comma", `function identity<T,>(value: T): T { return value; }`, "type parameter name after ','"},
		{"missing close", `function identity<T(value: T): T { return value; }`, "expected '>'"},
		{"class method", `class Box { public function identity<T>(value: T): T { return value; } }`, "generic class methods are not supported"},
		{"struct method", `struct Box { public function identity<T>(value: T): T { return value; } }`, "generic struct methods are not supported"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("generic_failure.otm", test.source)
			if len(lexDiagnostics) != 0 {
				t.Fatalf("lexer diagnostics = %v", lexDiagnostics)
			}
			_, diagnostics := Parse(tokens)
			var messages []string
			for _, diagnostic := range diagnostics {
				messages = append(messages, diagnostic.Message)
			}
			if !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", messages, test.want)
			}
		})
	}
}

func TestParsesExternalGenericReceiverMethod(t *testing.T) {
	program, diagnosticCount := parseSource(t, `struct Box<T> { public value: T; } public function get<U>(this: Box<U>): U { return this.value; }`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	method := program.Declarations[1].(*ast.MethodDecl)
	if len(method.TypeParameters) != 1 || method.TypeParameters[0].Name != "U" || len(method.ReceiverType.GenericArguments) != 1 || method.ReceiverType.GenericArguments[0].Name != "U" {
		t.Fatalf("method = %#v", method)
	}
}

func TestGenericCallSpeculationPreservesComparisonsAndGoComposites(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
import go unique from "unique";
function less(left: int, right: int): boolean { return left < right; }
function value(): unique.Handle<Map<string, int>> { return unique.Handle<Map<string, int>>{}; }
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	less := program.Declarations[0].(*ast.FunctionDecl)
	if operator := less.Body.Statements[0].(*ast.ReturnStmt).Value.(*ast.BinaryExpr).Operator; operator != "<" {
		t.Fatalf("comparison operator = %q", operator)
	}
	value := program.Declarations[1].(*ast.FunctionDecl)
	literal := value.Body.Statements[0].(*ast.ReturnStmt).Value.(*ast.GoCompositeLiteralExpr)
	if literal.Type.Qualifier != "unique" || literal.Type.Name != "Handle" || len(literal.Type.GenericArguments) != 1 {
		t.Fatalf("Go composite type = %#v", literal.Type)
	}
}
