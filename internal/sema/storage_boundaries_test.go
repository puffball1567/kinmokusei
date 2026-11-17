package sema

import (
	"go/importer"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestStorageBoundaryRecheckingAndCallOrder(t *testing.T) {
	t.Parallel()
	input := `import go context from "context";
alias Callback=()=>void;
function rawValue():context.CancelFunc{return nil;}
const top:Callback=rawValue();
const arrow=():Callback=>rawValue();
function entry(raw:context.CancelFunc):Callback{let value:Callback=raw;while(false){value=raw;}return raw;}
function accept(value:Callback,next:int):void{}
function invoke(raw:context.CancelFunc):void{accept(raw,7);}
function optional(raw:context.CancelFunc):Callback|null{return raw;}
function discard(raw:context.CancelFunc):void{const _:Callback=raw;}
`
	tokens, diagnostics := lexer.Lex("storage.km", input)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	program, diagnostics := parser.Parse(tokens)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	checked := func(expression ast.Expression) {
		t.Helper()
		wrapper, ok := expression.(*ast.UnaryExpr)
		if !ok || !wrapper.ImplicitNonNull || wrapper.Operator != ast.NonNullOperator {
			t.Fatalf("missing storage check: %#v", expression)
		}
		if inner, ok := wrapper.Operand.(*ast.UnaryExpr); ok && inner.ImplicitNonNull {
			t.Fatal("rechecking nested another storage check")
		}
		if !wrapper.AssertionType.IsFunction() || wrapper.AssertionType.Return == nil || wrapper.AssertionType.Return.Name != "void" || wrapper.AssertionSpan.Path != "storage.km" {
			t.Fatalf("lost target/location: %#v", wrapper)
		}
	}
	for pass := 0; pass < 2; pass++ {
		if diagnostics := CheckScopedWithGoImporterAndPolicy(program, nil, importer.Default(), GoInteropPolicy{}); len(diagnostics) != 0 {
			t.Fatal(diagnostics)
		}
		checked(program.Declarations[2].(*ast.VariableDecl).Value)
		checked(program.Declarations[3].(*ast.VariableDecl).Value.(*ast.ArrowExpr).ExpressionBody)
		body := program.Declarations[4].(*ast.FunctionDecl).Body
		checked(body.Statements[0].(*ast.VariableDecl).Value)
		checked(body.Statements[1].(*ast.WhileStmt).Body.Statements[0].(*ast.AssignmentStmt).Value)
		checked(body.Statements[2].(*ast.ReturnStmt).Value)
		call := program.Declarations[6].(*ast.FunctionDecl).Body.Statements[0].(*ast.ExpressionStmt).Value.(*ast.CallExpr)
		if _, ok := call.Arguments[0].(*ast.IdentifierExpr); !ok {
			t.Fatal("argument was checked before remaining arguments were evaluated")
		}
		for _, index := range []int{7, 8} {
			var expression ast.Expression
			body := program.Declarations[index].(*ast.FunctionDecl).Body
			if index == 7 {
				expression = body.Statements[0].(*ast.ReturnStmt).Value
			} else {
				expression = body.Statements[0].(*ast.VariableDecl).Value
			}
			if _, ok := expression.(*ast.IdentifierExpr); !ok {
				t.Fatal("nullable or explicit discard introduced a storage check")
			}
		}
	}
}

func TestStorageBoundariesStillRejectInvalidContracts(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ input, want string }{
		"nullable operand":     {`alias F=()=>int;function f(raw:F|null):F{return raw;}`, "cannot use"},
		"nil remains an error": {`alias F=()=>int;function f():void{const value:F=nil;}`, "cannot use nil"},
		"signature mismatch":   {`import go context from "context";function f(raw:context.CancelFunc):()=>int{return raw;}`, "cannot use"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := strings.Join(checkSource(t, test.input), "\n")
			if !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%q want=%q", got, test.want)
			}
		})
	}
}
