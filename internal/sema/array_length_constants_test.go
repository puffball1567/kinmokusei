package sema

import (
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestArrayLengthConstants(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"length address", `function f(a:[3]int):void{const n=len(a);const p=&n;}`, "addressable"},
		{"capacity alias address", `function f(a:*[3]int):void{const n=cap(a);const m=n;const p=&m;}`, "addressable"},
		{"mutable array length", `function f():void{let a:[3]int=[1,2,3];const n=len(a);const p=&n;}`, "addressable"},
		{"length index", `function f(a:[3]int):int{return a[len(a)];}`, "out of bounds"},
		{"capacity slice", `function f(a:*[3]int):int[]{const n=cap(a);return a[:n+1];}`, "exceeds fixed array length 3"},
		{"length stays int", `function f(a:[3]int):byte{return len(a);}`, "of type int"},
		{"capacity negative size", `function f(a:[3]int):void{const n=cap(a)-4;const xs=makeSlice<int>(n);}`, "negative"},
		{"valid sizes", `function f(a:[3]int):int{const n=len(a);const m=cap(a);const xs=makeSlice<int>(n,m);return a[n-1]+len(xs);}`, ""},
		{"named array", `type A=distinct [3]int;function f(a:A):void{const n=len(a);const p=&n;}`, "addressable"},
		{"generic element", `function f<T>(a:[3]T):void{const n=cap(a);const p=&n;}`, "addressable"},
		{"generic array remains runtime", `constraint A=~[3]int;function f<T extends A>(a:T):void{const n=len(a);const p=&n;}`, ""},
		{"generic mixed length", `constraint A=~[3]int|~int[]|~string|~Map<string,int>|~GoChannel<int>;function f<T extends A>(a:T):int{return len(a);}`, ""},
		{"generic mixed capacity", `constraint A=~[3]int|~int[]|~GoChannel<int>;function f<T extends A>(a:T):int{return cap(a);}`, ""},
		{"generic pointer capacity", `constraint A=~*[3]int;function f<T extends A>(a:T):void{const n=cap(a);const p=&n;}`, ""},
		{"generic bad length", `constraint A=~[3]int|~int;function f<T extends A>(a:T):int{return len(a);}`, "len requires"},
		{"generic bad capacity", `constraint A=~[3]int|~string;function f<T extends A>(a:T):int{return cap(a);}`, "cap requires"},
		{"unconstrained length", `function f<T>(a:T):int{return len(a);}`, "len requires"},
		{"slice remains runtime", `function f(a:int[]):void{const n=cap(a);const p=&n;}`, ""},
		{"call remains runtime", `function get():[3]int{return [1,2,3];}function f():void{const n=len(get());const p=&n;}`, ""},
		{"receive remains runtime", `function f(c:GoChannel<[3]int>):void{const n=cap(<-c);const p=&n;}`, ""},
		{"index call remains runtime", `function index():int{return 0;}function f(a:[1][3]int):void{const n=len(a[index()]);const p=&n;}`, ""},
		{"conversion ignored", `function f(a:int[]):void{const n=len(copyArray[[3]int](a));const p=&n;}`, "addressable"},
		{"pointer conversion ignored", `function f(a:int[]):void{const n=cap(viewArray[[3]int](a));const p=&n;}`, "addressable"},
		{"typed nil pointer", `function f():int{const a:*[3]int=nil;const n=len(a);return n;}`, ""},
		{"zero length", `function f(a:[0]int):int{return a[len(a)];}`, "out of bounds"},
		{"arity", `function f(a:[3]int):void{len(a,a);}`, "expects 1"},
		{"type argument", `function f(a:[3]int):void{len<int>(a);}`, "expects 0 type arguments"},
		{"spread", `function f(a:[3]int):void{cap(a...);}`, "does not accept spread"},
		{"nullable pointer length", `function f(a:*[3]int|null):void{const n=len(a);const p=&n;}`, "addressable"},
		{"nullable pointer capacity", `function f(a:*[3]int|null):void{const n=cap(a);const alias=n;const p=&alias;}`, "addressable"},
		{"nullable named array", `type A=distinct [3]int;function f(a:*A|null):void{const n=len(a);const p=&n;}`, "addressable"},
		{"nullable named pointer", `type P=distinct *[3]int;function f(a:P|null):void{const n=cap(a);const p=&n;}`, "addressable"},
		{"nullable generic element", `function f<T>(a:*[3]T|null):void{const n=len(a);const p=&n;}`, "addressable"},
		{"nullable class element", `class User{}function f(a:*[3]User|null):void{const n=cap(a);const p=&n;}`, "addressable"},
		{"nullable pointer bounds", `function f(a:*[3]int|null,b:[3]int):int{const n=len(a);return b[n];}`, "out of bounds"},
		{"nullable pointer negative size", `function f(a:*[3]int|null):void{const n=cap(a)-4;const xs=makeSlice<int>(n);}`, "negative"},
		{"nullable zero bounds", `function f(a:*[0]int|null,b:[0]int):int{return b[len(a)];}`, "out of bounds"},
		{"nullable length stays int", `function f(a:*[3]int|null):byte{return len(a);}`, "of type int"},
		{"nullable pointer call runtime", `function get():*[3]int|null{return null;}function f():void{const n=len(get());const p=&n;}`, ""},
		{"nullable pointer receive runtime", `function f(c:GoChannel<*[3]int|null>):void{const n=cap(<-c);const p=&n;}`, ""},
		{"length does not narrow pointer", `function f(a:*[3]int|null):int{const n=len(a);return a[0];}`, "must be checked against null"},
		{"capacity does not permit dereference", `function f(a:*[3]int|null):[3]int{const n=cap(a);return *a;}`, "must be checked against null"},
		{"length is not a null test", `function f(a:*[3]int|null):int{if(len(a)>0){return a[0];}return 0;}`, "must be checked against null"},
		{"nullable slice runtime", `function f(a:int[]|null):void{const n=len(a);const p=&n;}`, ""},
		{"nullable map runtime", `function f(a:Map<string,int>|null):void{const n=len(a);const p=&n;}`, ""},
		{"nullable channel runtime", `function f(a:GoChannel<int>|null):void{const n=cap(a);const p=&n;}`, ""},
		{"pointer to generic array runtime", `constraint A=~[3]int;function f<T extends A>(a:*T|null):void{const n=len(a);const p=&n;}`, "len requires"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}

func TestArrayLengthOperandClassification(t *testing.T) {
	t.Parallel()
	value := &ast.IdentifierExpr{Name: "value"}
	call := &ast.CallExpr{Callee: &ast.IdentifierExpr{Name: "next"}}
	for _, test := range []struct {
		name string
		expr ast.Expression
		want bool
	}{
		{"identifier", value, true},
		{"literal", &ast.LiteralExpr{Kind: ast.IntegerLiteral, Text: "1"}, true},
		{"constant length", &ast.CallExpr{Builtin: ast.LenCall, GoConstant: true, Arguments: []ast.Expression{call}}, true},
		{"constant layout containing call", &ast.CallExpr{Builtin: ast.UnsafeSizeofCall, GoConstant: true, Arguments: []ast.Expression{call}}, false},
		{"constant conversion containing call", &ast.CallExpr{Conversion: true, GoConstant: true, Arguments: []ast.Expression{call}}, false},
		{"runtime call", call, false},
		{"closure body", &ast.ArrowExpr{ExpressionBody: call}, true},
		{"dereference", &ast.UnaryExpr{Operator: "*", Operand: value}, true},
		{"receive", &ast.UnaryExpr{Operator: "<-", Operand: value}, false},
		{"arithmetic", &ast.BinaryExpr{Left: value, Operator: "+", Right: value}, true},
		{"arithmetic call", &ast.BinaryExpr{Left: call, Operator: "+", Right: value}, false},
		{"field", &ast.MemberExpr{Object: value}, true},
		{"field call", &ast.MemberExpr{Object: call}, false},
		{"index", &ast.IndexExpr{Object: value, Index: value}, true},
		{"index call", &ast.IndexExpr{Object: value, Index: call}, false},
		{"slice", &ast.SliceExpr{Object: value}, true},
		{"slice high call", &ast.SliceExpr{Object: value, High: call}, false},
		{"slice max call", &ast.SliceExpr{Object: value, Max: call}, false},
		{"assertion", &ast.GoTypeAssertionExpr{Value: value}, true},
		{"assertion call", &ast.GoTypeAssertionExpr{Value: call}, false},
		{"class downcast helper", &ast.GoTypeAssertionExpr{Value: value, ClassDowncast: true}, false},
		{"conversion", &ast.CallExpr{Conversion: true, Arguments: []ast.Expression{value}}, true},
		{"conversion call", &ast.CallExpr{Conversion: true, Arguments: []ast.Expression{call}}, false},
		{"array", &ast.ArrayLiteralExpr{Elements: []ast.Expression{value}}, true},
		{"array call", &ast.ArrayLiteralExpr{Elements: []ast.Expression{call}}, false},
		{"object", &ast.ObjectLiteralExpr{Fields: []ast.ObjectField{{Value: value}}}, true},
		{"object call", &ast.ObjectLiteralExpr{Fields: []ast.ObjectField{{Value: call}}}, false},
		{"struct", &ast.GoCompositeLiteralExpr{Fields: []ast.ObjectField{{Value: value}}}, true},
		{"struct call", &ast.GoCompositeLiteralExpr{Fields: []ast.ObjectField{{Value: call}}}, false},
		{"constructor", &ast.NewExpr{}, false},
		{"class upcast helper", &ast.ClassUpcastExpr{Value: value}, false},
		{"propagation", &ast.PropagateExpr{Value: call}, false},
		{"await", &ast.AwaitExpr{Value: value}, false},
		{"task start", &ast.TaskStartExpr{Call: call}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := arrayLengthOperandUnevaluated(test.expr); got != test.want {
				t.Fatalf("unevaluated=%v want=%v", got, test.want)
			}
		})
	}
}
