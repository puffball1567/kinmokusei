package sema

import (
	"go/importer"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestReferenceReadMetadataRechecking(t *testing.T) {
	t.Parallel()
	input := `class Item{public n:int=7;}class Node{constructor(public item:Item){}}function read(node:Node):int{return node.item.n;}`
	tokens, ds := lexer.Lex("reads.km", input)
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	program, ds := parser.Parse(tokens)
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	read := program.Declarations[2].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt).Value.(*ast.MemberExpr)
	item := read.Object.(*ast.MemberExpr)
	for pass := 0; pass < 2; pass++ {
		read.NonNullRead = &ast.TypeRef{Name: "Item"}
		item.NonNullRead = &ast.TypeRef{Name: "int"}
		if ds := Check(program); len(ds) != 0 {
			t.Fatal(ds)
		}
		if read.NonNullRead != nil || item.NonNullRead == nil || item.NonNullRead.Name != "Item" || item.NonNullRead.NonNullKinds != 1 {
			t.Fatalf("stale metadata: %v %v", read.NonNullRead, item.NonNullRead)
		}
	}
}

func TestReferenceReadLayoutIsUnevaluated(t *testing.T) {
	t.Parallel()
	input := `import go unsafe from "unsafe";class Item{}class Node{constructor(public item:Item){}}function size(node:Node):int{return int(unsafe.Sizeof(node.item));}`
	tokens, ds := lexer.Lex("layout.km", input)
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	program, ds := parser.Parse(tokens)
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	if ds := CheckScopedWithGoImporterAndPolicy(program, nil, importer.Default(), GoInteropPolicy{AllowUnsafe: true}); len(ds) != 0 {
		t.Fatal(ds)
	}
	outer := program.Declarations[2].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt).Value.(*ast.CallExpr)
	size := outer.Arguments[0].(*ast.CallExpr)
	member := size.Arguments[0].(*ast.MemberExpr)
	if member.NonNullRead != nil {
		t.Fatal("layout added a runtime read")
	}
}

func TestReferenceReadsKeepAddressabilityAndNullableFacts(t *testing.T) {
	t.Parallel()
	input := `class Item{public n:int=7;}class Node{constructor(public item:Item){}public next:Node|null=null;}
function address(node:Node):boolean{const p=&node.item;return *p===node.item;}
function narrow(node:Node):int{if(node.next===null){return 0;}return node.next.item.n;}
function scalar(a:[2]int):int{return len(a);}
function methods(node:Node):void{node.item.n=9;}
`
	if ds := checkSource(t, input); len(ds) != 0 {
		t.Fatal(ds)
	}
}

func TestPresenceJoinDoesNotNarrowReplacedNullableCallback(t *testing.T) {
	t.Parallel()
	prefix := `import go context from "context";alias F=()=>void;function f(m:Map<int,F>,replace:boolean,raw:context.CancelFunc):void{let value:F|null=null;let ok=false;[value,ok]=m[0];if(replace){value=raw;}`
	if ds := strings.Join(checkSource(t, prefix+`if(ok){value();}}`), "\n"); !strings.Contains(ds, "nullable callable") {
		t.Fatal(ds)
	}
	if ds := checkSource(t, prefix+`if(ok&&value!==null){value();}}`); len(ds) != 0 {
		t.Fatal(ds)
	}
}
