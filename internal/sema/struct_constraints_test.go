package sema

import (
	"strings"
	"testing"
)

func TestSourceStructConstraints(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"exact", `struct Row{public value:int;}constraint Rows=Row;function keep<T extends Rows>(v:T):T{return v;}const n=keep(Row{value:7}).value;`, ""},
		{"forward", `constraint Rows=Row;function keep<T extends Rows>(v:T):T{return v;}struct Row{public value:int;}const n=keep(Row{value:7}).value;`, ""},
		{"generic inference", `struct Box<E>{public value:E;}constraint Boxes<E>=Box<E>;function keep<E,T extends Boxes<E>>(v:T):T{return v;}const n=keep(Box<int>{value:7}).value;`, ""},
		{"alias", `alias Wrapped<E>=Box<E>;constraint Boxes<E>=Wrapped<E>;struct Box<E>{public value:E;}function keep<E,T extends Boxes<E>>(v:T):T{return v;}const n=keep(Box<int>{value:7}).value;`, ""},
		{"distinct", `struct Row{public value:int;}type Copy=distinct Row;constraint Rows=Copy;function keep<T extends Rows>(v:T):T{return v;}function use(v:Copy):Copy{return keep(v);}`, ""},
		{"comparable cycle", `constraint Rows=comparable&A;struct A{public next:A;}`, "unresolved or cyclic"},
		{"expanding value cycle", `constraint Rows=Node<int>;struct Node<E>{public next:Node<E[]>;}`, "unresolved or cyclic"},
		{"union", `struct A{public n:int;}struct B{public n:int;}constraint Rows=A|B;function keep<T extends Rows>(v:T):T{return v;}const a=keep(A{n:7});const b=keep(B{n:8});`, ""},
		{"array term", `constraint Rows<E>=~[2]Box<E>;struct Box<E>{public value:E;}function first<E,T extends Rows<E>>(v:T):Box<E>{return v[0];}function use(v:[2]Box<int>):Box<int>{return first(v);}`, ""},
		{"slice term", `constraint Rows<E>=~Box<E>[];struct Box<E>{public value:E;}function first<E,T extends Rows<E>>(v:T):Box<E>{return v[0];}function use(v:Box<int>[]):Box<int>{return first(v);}`, ""},
		{"bound dependency", `constraint Outer<E extends Rows>=~E[];constraint Rows=Row;struct Row{public n:int;}function keep<E extends Rows,T extends Outer<E>>(v:T):T{return v;}function use(v:Row[]):Row[]{return keep(v);}`, ""},
		{"method bounds", `struct Host<E>{public function keep<T extends Rows<E>>(v:T):T{return v;}}constraint Rows<E>=Row<E>;struct Row<E>{public n:E;}function use(h:Host<int>,v:Row<int>):Row<int>{return h.keep(v);}`, ""},
		{"method bound rejected", `struct Host<E>{public function keep<T extends Rows<E>>(v:T):T{return v;}}constraint Rows<E>=Row<E>;struct Row<E>{public n:E;}function use(h:Host<int>,v:int):int{return h.keep(v);}`, "does not satisfy"},
		{"comparable", `struct Row{public n:int;}constraint Rows=comparable&Row;function eq<T extends Rows>(a:T,b:T):boolean{return a===b;}`, ""},
		{"non comparable", `struct Row{public values:int[];}constraint Rows=comparable&Row;`, "no common types"},
		{"not a structural wildcard", `struct A{public n:int;}struct B{public n:int;}constraint Rows=A;function keep<T extends Rows>(v:T):T{return v;}const n=keep(B{n:7});`, "does not satisfy"},
		{"tilde named", `struct Row{public n:int;}constraint Rows=~Row;`, "must name its own underlying type"},
		{"overlap alias", `struct Row{public n:int;}alias Same=Row;constraint Rows=Row|Same;`, "overlaps an earlier term"},
		{"constraint cycle", `constraint A=B|Row;constraint B=A;struct Row{public n:int;}`, "constraint declaration cycle"},
		{"value cycle", `constraint Rows=A;struct A{public next:A;}`, "recursively contains itself by value"},
		{"pointer cycle", `constraint Rows=Node;struct Node{public next:*Node;public n:int;}function keep<T extends Rows>(v:T):T{return v;}`, ""},
		{"nullable mismatch", `class User{}struct Row<E>{public value:E;}constraint Rows=Row<User>;function keep<T extends Rows>(v:T):T{return v;}function bad(v:Row<User|null>):void{_=keep(v);}`, "nullable type information does not match"},
		{"nullable inference", `class User{}struct Row<E>{public value:E;}constraint Rows<E>=Row<E>;function keep<E,T extends Rows<E>>(v:T):T{return v;}function good(v:Row<User|null>):Row<User|null>{return keep(v);}`, ""},
		{"raw text inference", `struct Row<E>{public value:E;}constraint Rows<E>=Row<E>;function keep<E,T extends Rows<E>>(v:T):T{return v;}function good(v:Row<bstring>):Row<bstring>{return keep(v);}`, ""},
		{"verified text inference", `struct Row<E>{public value:E;}constraint Rows<E>=Row<E>;function keep<E,T extends Rows<E>>(v:T):T{return v;}function good(v:Row<string>):Row<string>{return keep(v);}`, ""},
		{"method nullable mismatch", `class User{}struct Row<E>{public value:E;}constraint Rows<E>=Row<E>;struct Host<E>{public function keep<T extends Rows<E>>(v:T):T{return v;}}function bad(h:Host<User>,v:Row<User|null>):void{_=h.keep(v);}`, "nullable type information does not match"},
		{"method nullable match", `class User{}struct Row<E>{public value:E;}constraint Rows<E>=Row<E>;struct Host<E>{public function keep<T extends Rows<E>>(v:T):T{return v;}}function good(h:Host<User|null>,v:Row<User|null>):Row<User|null>{return h.keep(v);}`, ""},
		{"structural nullable mismatch", `class User{}struct Row{public value:User|null;}constraint Rows=~{value:User};function keep<T extends Rows>(v:T):T{return v;}function bad(v:Row):void{_=keep(v);}`, "nullable type information does not match"},
		{"structural nullable match", `class User{}struct Row{public value:User|null;}constraint Rows=~{value:User|null};function keep<T extends Rows>(v:T):T{return v;}function good(v:Row):Row{return keep(v);}`, ""},
		{"structural text mismatch", `struct Row{public value:bstring;}constraint Rows=~{value:string};function keep<T extends Rows>(v:T):T{return v;}function bad(v:Row):void{_=keep(v);}`, "does not match"},
		{"structural text match", `struct Row{public value:string;}constraint Rows=~{value:string};function keep<T extends Rows>(v:T):T{return v;}function good(v:Row):Row{return keep(v);}`, ""},
		{"method composition", `import go fmt from "fmt";struct Row{public value:int;public function String():bstring{return b"row";}}constraint Rows=Row&fmt.Stringer;function show<T extends Rows>(v:T):bstring{return v.String();}function use(v:Row):bstring{return show(v);}`, ""},
		{"unsafe zero", `class User{}struct Row<E>{public value:E;}constraint Rows=Row<User>;function bad<T extends Rows>():void{_=makeSlice<T>(1);}`, "cannot initialize elements"},
		{"safe zero", `class User{}struct Row<E>{public value:E;}constraint Rows=Row<User|null>;function good<T extends Rows>():void{_=makeSlice<T>(1);}`, ""},
		{"shadow", `struct Row{public n:int;}constraint X<Row>=~Row[];function keep<T extends X<int>>(v:T):T{return v;}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
