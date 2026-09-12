package parser

import (
	"reflect"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/token"
)

func TestGenericClosersAdjacentToAssignment(t *testing.T) {
	for _, input := range []string{
		`alias Items<T>=T[];`,
		`constraint Slice<T>=~T[];`,
		`constraint Rows<S extends Slice<E>,E>=~S[];`,
		`alias Items<T extends Slice<int>>=T;`,
		`function use():void{let items:Map<string,int>=create();}`,
		`function use():void{let items:Map<string,Map<string,int>>=create();}`,
		`class Store{public items:Box<Box<Box<int>>>=new Box<Box<Box<int>>>();}`,
		`function use():void{for(let items:Box<Box<int>>=new Box<Box<int>>();false;) {}}`,
	} {
		if _, count := parseSource(t, input); count != 0 {
			t.Fatalf("diagnostics=%d input=%s", count, input)
		}
	}
}

func TestGenericClosersAdjacentToArrow(t *testing.T) {
	t.Parallel()
	for _, result := range []string{"Box<int>", "Box<Box<int>>", "Box<Box<Box<int>>>", "Result<int>"} {
		input := "const f=():" + result + "=>value;"
		tokens, _ := lexer.Lex("arrow.km", input)
		original := append([]token.Token(nil), tokens...)
		program, diagnostics := Parse(tokens)
		if len(diagnostics) != 0 {
			t.Fatalf("%s: %v", input, diagnostics)
		}
		arrow := program.Declarations[0].(*ast.VariableDecl).Value.(*ast.ArrowExpr)
		span := arrow.ReturnType.Span
		if got := input[span.Start.Offset:span.End.Offset]; got != result {
			t.Fatalf("return type span=%q, want %q", got, result)
		}
		repeated, errors := Parse(tokens)
		if !reflect.DeepEqual(tokens, original) || len(errors) != 0 || !reflect.DeepEqual(program, repeated) {
			t.Fatal("generic arrow parsing changed caller-owned tokens or repeated results")
		}
	}
	for _, input := range []string{`const f=():Box<int>= >value;`, "const f=():Box<int>=\n>value;", `const f=():Box<int>=/*gap*/>value;`} {
		if _, count := parseSource(t, input); count == 0 {
			t.Fatalf("accepted separated arrow: %s", input)
		}
	}
}

func TestGenericSpeculationRestoresOperators(t *testing.T) {
	for _, test := range []struct{ expression, operator string }{
		{"a<b>>c", "<"},
		{"a<b>=c", ">="},
		{"a<b<c>>d", "<"},
	} {
		program, count := parseSource(t, "function test():void{return "+test.expression+";}")
		if count != 0 {
			t.Fatalf("diagnostics=%d expression=%s", count, test.expression)
		}
		returned := program.Declarations[0].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt)
		root := requireBinaryOperator(t, returned.Value, test.operator)
		if test.operator == "<" {
			requireBinaryOperator(t, root.Right, ">>")
		}
	}
	program, count := parseSource(t, `function test():void{return flags[a<b>>c];}`)
	if count != 0 {
		t.Fatalf("subscript diagnostics=%d", count)
	}
	returned := program.Declarations[0].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt)
	index := returned.Value.(*ast.IndexExpr)
	comparison := requireBinaryOperator(t, index.Index, "<")
	requireBinaryOperator(t, comparison.Right, ">>")
}

func TestGenericCloserSpansAndTokenOwnership(t *testing.T) {
	input := `function use():void{let value:Box<Box<int>>=make();}`
	tokens, diagnostics := lexer.Lex("closers.km", input)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	original := append([]token.Token(nil), tokens...)
	first, errors := Parse(tokens)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	if !reflect.DeepEqual(original, tokens) {
		t.Fatal("Parse modified caller-owned tokens")
	}
	second, errors := Parse(tokens)
	if len(errors) != 0 || !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated Parse differs: %v", errors)
	}
	variable := first.Declarations[0].(*ast.FunctionDecl).Body.Statements[0].(*ast.VariableDecl)
	outer := variable.Type.Span
	inner := variable.Type.GenericArguments[0].Span
	if got := input[outer.Start.Offset:outer.End.Offset]; got != "Box<Box<int>>" {
		t.Fatalf("outer span=%q", got)
	}
	if got := input[inner.Start.Offset:inner.End.Offset]; got != "Box<int>" {
		t.Fatalf("inner span=%q", got)
	}
}

func TestGenericCloserCheckpointRestoresEverySplit(t *testing.T) {
	tokens, _ := lexer.Lex("closers.km", ">>=")
	p := &Parser{tokens: tokens}
	checkpoint := p.checkpoint()
	for i := 0; i < 2; i++ {
		closer, ok := p.expectTypeGreater("closer")
		if !ok || closer.Span.Start.Offset != i || closer.Span.End.Offset != i+1 || p.previous() != closer {
			t.Fatalf("closer %d=%#v", i, closer)
		}
	}
	if p.peek().Kind != token.Assign || p.peek().Span.Start.Offset != 2 {
		t.Fatalf("remainder=%#v", p.peek())
	}
	p.restore(checkpoint)
	if p.peek().Kind != token.ShrAssign || p.peek().Lexeme != ">>=" {
		t.Fatalf("restored=%#v", p.peek())
	}
	for _, input := range []string{`alias Items<>=int;`, `constraint Bad<T>>=~int;`, `function use():void{let x:Box<int>=;}`} {
		if _, count := parseSource(t, input); count == 0 {
			t.Fatalf("accepted malformed input: %s", strings.TrimSpace(input))
		}
	}
}
