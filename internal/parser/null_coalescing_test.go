package parser

import (
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
)

func TestNullCoalescingPrecedenceAndGrouping(t *testing.T) {
	for _, input := range []string{`const x=a??b??c;`, `const x=(a??b)||c;`, `const x=a??(b||c);`, `const x=(a&&b)??c;`, `const x=a??(b&&c);`, `const f=id<int>??fallback;`, `const x=a!??b;`} {
		tokens, diags := lexer.Lex("test.km", input)
		if len(diags) != 0 {
			t.Fatal(diags)
		}
		program, diags := Parse(tokens)
		if len(diags) != 0 {
			t.Fatalf("%s: %v", input, diags)
		}
		if strings.Contains(input, "a??b??c") {
			outer := program.Declarations[0].(*ast.VariableDecl).Value.(*ast.BinaryExpr)
			if left, ok := outer.Left.(*ast.BinaryExpr); !ok || left.Operator != "??" || outer.Operator != "??" {
				t.Fatalf("not left associative: %#v", outer)
			}
		}
	}
	for _, input := range []string{`const x=a??b||c;`, `const x=a&&b??c;`, `const x=a||b??c;`, `const x=a??b&&c;`} {
		tokens, _ := lexer.Lex("test.km", input)
		_, diags := Parse(tokens)
		found := false
		for _, d := range diags {
			found = found || strings.Contains(d.Message, "parenthesize")
		}
		if !found {
			t.Fatalf("ungrouped mix accepted: %s %v", input, diags)
		}
	}
}
