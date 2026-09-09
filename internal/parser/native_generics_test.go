package parser

import (
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
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

func TestParsesQualifiedGoTypeParameterConstraint(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
import go cmp from "cmp";
function minimum<T extends cmp.Ordered>(left: T, right: T): T {
  if (left < right) { return left; }
  return right;
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	constraint := function.TypeParameters[0].Constraint
	if constraint == nil || constraint.Qualifier != "cmp" || constraint.Name != "Ordered" || !constraint.Go {
		t.Fatalf("constraint = %#v", constraint)
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("generic_failure.km", test.source)
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

func TestParsesNativeGenericClassAndStructMethods(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
class Box<T> {
  public function pair<U>(value: U): {left: T, right: U} {
    return {left: this.value(), right: value};
  }
  private function value(): T { return nil; }
}
struct Holder<T> {
  public value: T;
  public function replace<U>(value: U): U { return value; }
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	classMethod := program.Declarations[0].(*ast.ClassDecl).Methods[0]
	if len(classMethod.TypeParameters) != 1 || classMethod.TypeParameters[0].Name != "U" {
		t.Fatalf("class method type parameters = %#v", classMethod.TypeParameters)
	}
	structMethod := program.Declarations[1].(*ast.StructDecl).Methods[0]
	if len(structMethod.TypeParameters) != 1 || structMethod.TypeParameters[0].Name != "U" {
		t.Fatalf("struct method type parameters = %#v", structMethod.TypeParameters)
	}
}

func TestParsesExplicitGenericCallsOnExpressionReceivers(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
class Box {
  public function echo<T>(value: T): T { return value; }
}
function makeBox(): Box { return new Box(); }
function direct(): string { return new Box().echo<string>("direct"); }
function returned(): int { return makeBox().echo<int>(1); }
function indexed(boxes: Box[]): int8 { return boxes[0].echo[int8](int8(2)); }
function compared(boxes: Box[], limit: int): boolean { return boxes[0].echo(1) < limit; }
function ordinaryIndex(values: {items: int[]}[]): int { return values[0].items[1]; }
function ambiguousComparison(): boolean { return makeBox().echo < int > {value: 1}; }
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	for index, name := range []string{"direct", "returned", "indexed"} {
		function := program.Declarations[index+2].(*ast.FunctionDecl)
		returned := function.Body.Statements[0].(*ast.ReturnStmt)
		call, ok := returned.Value.(*ast.CallExpr)
		if !ok || len(call.TypeArguments) != 1 {
			t.Fatalf("%s return = %#v, want explicitly typed call", name, returned.Value)
		}
		if _, ok = call.Callee.(*ast.MemberExpr); !ok {
			t.Fatalf("%s callee = %#v, want member expression", name, call.Callee)
		}
	}
	compared := program.Declarations[5].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt).Value
	if binary, ok := compared.(*ast.BinaryExpr); !ok || binary.Operator != "<" {
		t.Fatalf("comparison = %#v, want '<' binary expression", compared)
	}
	ordinaryIndex := program.Declarations[6].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt).Value
	if indexed, ok := ordinaryIndex.(*ast.IndexExpr); !ok {
		t.Fatalf("ordinary nested member index = %#v, want index expression", ordinaryIndex)
	} else if member, memberOK := indexed.Object.(*ast.MemberExpr); !memberOK {
		t.Fatalf("ordinary nested member index object = %#v, want member expression", indexed.Object)
	} else if _, receiverOK := member.Object.(*ast.IndexExpr); !receiverOK {
		t.Fatalf("ordinary nested member receiver = %#v, want index expression", member.Object)
	}
	ambiguous := program.Declarations[7].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt).Value
	if greater, ok := ambiguous.(*ast.BinaryExpr); !ok || greater.Operator != ">" {
		t.Fatalf("ambiguous comparison = %#v, want outer '>' binary expression", ambiguous)
	} else if less, lessOK := greater.Left.(*ast.BinaryExpr); !lessOK || less.Operator != "<" {
		t.Fatalf("ambiguous comparison left = %#v, want inner '<' binary expression", greater.Left)
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
