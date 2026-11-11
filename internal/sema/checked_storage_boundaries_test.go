package sema

import (
	"go/importer"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestCheckedStorageBoundaryConditionsAndRechecking(t *testing.T) {
	t.Parallel()
	input := `alias F=()=>int;
function source():Result<F>{return ok(()=>3);}
function checked(m:Map<int,F>):void{let cb:F=()=>1;let ok=false;[cb,ok]=m[0];if(ok){cb();}}
function selected(ch:GoChannel<F>):void{let cb:F=()=>1;let open=false;select{case [cb,open]=<-ch{if(open){cb();}}}}
function result():void{let cb:F=()=>1;let err:error=nil;[cb,err]=source();if(err!==nil){return;}cb();}
function discarded(m:Map<int,F>):void{let cb:F=()=>1;[cb,_]=m[0];}
function raw(m:Map<int,int>):void{let value=0;let ok=false;[value,ok]=m[0];}
`
	tokens, ds := lexer.Lex("checked.km", input)
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
		for _, test := range []struct {
			index     int
			condition ast.NonNullCondition
			statement int
		}{{2, ast.NonNullIfPresent, 2}, {4, ast.NonNullIfNoError, 2}, {5, ast.NonNullIfPresent, 1}} {
			assignment := program.Declarations[test.index].(*ast.FunctionDecl).Body.Statements[test.statement].(*ast.MultiAssignmentStmt)
			if assignment.NonNullCondition != test.condition || len(assignment.NonNullResults) != 2 || assignment.NonNullResults[0] == nil || assignment.NonNullResults[1] != nil || !assignment.NonNullResults[0].IsFunction() {
				t.Fatalf("lost conditional contract: %#v", assignment)
			}
		}
		selected := program.Declarations[3].(*ast.FunctionDecl).Body.Statements[2].(*ast.SelectStmt).Cases[0]
		if selected.NonNullCondition != ast.NonNullIfPresent || len(selected.NonNullResults) != 2 || selected.NonNullResults[0] == nil || selected.NonNullResults[1] != nil {
			t.Fatalf("lost selected contract: %#v", selected)
		}
		raw := program.Declarations[6].(*ast.FunctionDecl).Body.Statements[2].(*ast.MultiAssignmentStmt)
		for _, boundary := range raw.NonNullResults {
			if boundary != nil {
				t.Fatal("scalar received a reference check")
			}
		}
	}
}

func TestCheckedStorageStillRequiresPresenceProof(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"map":            `alias F=()=>int;function f(m:Map<int,F>):int{let cb:F=()=>1;let ok=false;[cb,ok]=m[0];return cb();}`,
		"receive":        `alias F=()=>int;function f(ch:GoChannel<F>):int{let cb:F=()=>1;let ok=false;[cb,ok]=<-ch;return cb();}`,
		"selected":       `alias F=()=>int;function f(ch:GoChannel<F>):int{let cb:F=()=>1;let ok=false;select{case [cb,ok]=<-ch{return cb();}}}`,
		"discarded flag": `alias F=()=>int;function f(m:Map<int,F>):int{let cb:F=()=>1;[cb,_]=m[0];return cb();}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := strings.Join(checkSource(t, input), "\n")
			if !strings.Contains(got, "check its corresponding ok") {
				t.Fatalf("missing presence diagnostic: %s", got)
			}
		})
	}
}
