package sema

import (
	"go/importer"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestMultipleStorageBoundaryRechecking(t *testing.T) {
	t.Parallel()
	input := `import go context from "context";
alias F=()=>void;
function raw():(context.CancelFunc,int){return nil,9;}
function forward():(F,int){return raw();}
function assign():void{let cb:F=()=>{};let n=0;[cb,n]=raw();}
function checked(m:Map<int,F>):void{let cb:F=()=>{};let present=false;[cb,present]=m[0];if(present){cb();}}
function rawForward():(context.CancelFunc,int){return raw();}
function optional():(F|null,int){return raw();}
`
	tokens, ds := lexer.Lex("multi.km", input)
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	program, ds := parser.Parse(tokens)
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	for pass := 0; pass < 2; pass++ {
		if ds := CheckScopedWithGoImporterAndPolicy(program, nil, importer.Default(), GoInteropPolicy{}); len(ds) != 0 {
			t.Fatal(ds)
		}
		expression := program.Declarations[2].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt).Value
		wrapper, ok := expression.(*ast.UnaryExpr)
		if !ok || !wrapper.ImplicitNonNull || len(wrapper.NonNullResults) != 2 || !wrapper.NonNullResults[0] || wrapper.NonNullResults[1] {
			t.Fatalf("missing per-result check: %#v", expression)
		}
		if _, ok := wrapper.Operand.(*ast.CallExpr); !ok {
			t.Fatal("rechecking nested another result boundary")
		}
		if len(wrapper.AssertionType.GoResults) != 2 || !wrapper.AssertionType.GoResults[0].IsFunction() || wrapper.AssertionType.GoResults[1].Name != "int" {
			t.Fatal("lost target result contracts")
		}
		assignment := program.Declarations[3].(*ast.FunctionDecl).Body.Statements[2].(*ast.MultiAssignmentStmt)
		if len(assignment.NonNullResults) != 2 || assignment.NonNullResults[0] == nil || assignment.NonNullResults[1] != nil || !assignment.NonNullResults[0].IsFunction() {
			t.Fatalf("missing assignment checks: %#v", assignment.NonNullResults)
		}
		checked := program.Declarations[4].(*ast.FunctionDecl).Body.Statements[2].(*ast.MultiAssignmentStmt)
		if checked.NonNullCondition != ast.NonNullIfPresent || checked.NonNullResults[0] == nil || checked.NonNullResults[1] != nil {
			t.Fatal("checked transfer lost its success condition")
		}

		for _, index := range []int{5, 6} {
			value := program.Declarations[index].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt).Value
			if _, ok := value.(*ast.CallExpr); !ok {
				t.Fatal("raw/nullable result acquired a non-null boundary")
			}
		}
	}
}

func TestMultipleStorageBoundariesRejectOtherInvalidSlots(t *testing.T) {
	t.Parallel()
	prefix := `import go context from "context";alias F=()=>void;`
	for name, test := range map[string]struct{ input, want string }{
		"scalar mismatch":           {prefix + `function raw():(context.CancelFunc,boolean){return nil,false;}function f():(F,int){return raw();}`, "cannot use result 2"},
		"class storage mismatch":    {prefix + `class Base{}class Child extends Base{}function raw():(context.CancelFunc,Child){return nil,new Child();}function f():(F,Base){return raw();}`, "cannot use result 2"},
		"nullable mismatch":         {prefix + `class C{}function raw():(context.CancelFunc,C|null){return nil,null;}function f():(F,C){return raw();}`, "cannot use result 2"},
		"text mismatch":             {prefix + `function raw():(context.CancelFunc,bstring){return nil,b"raw";}function f():(F,string){return raw();}`, "cannot use result 2"},
		"wrong arity":               {prefix + `function raw():(context.CancelFunc,int){return nil,1;}function f():(F,int,int){return raw();}`, "multiple result count mismatch"},
		"unchecked failure payload": {prefix + `function f(m:Map<int,F>):void{let cb:F=()=>{};let present=false;[cb,present]=m[0];cb();}`, "check its corresponding ok"},
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
