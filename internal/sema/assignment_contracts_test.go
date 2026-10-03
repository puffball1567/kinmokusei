package sema

import (
	"strings"
	"testing"
)

func TestAssignmentSourceContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source string }{
		{"function return", `function f(v: () => Maybe): () => Ptr { return v; }`},
		{"function parameter", `function f(v: (x: Ptr) => int): (x: Maybe) => int { return v; }`},
		{"named function", `type F = distinct () => Ptr; function f(v: () => Maybe): F { return v; }`},
		{"named slice", `type S = distinct Ptr[]; function f(v: Maybe[]): S { return v; }`},
		{"unnamed slice", `function f(v: Maybe[]): Ptr[] { return v; }`},
		{"generic slice", `type S<T> = distinct T[]; function f(v: S<Maybe>): S<Ptr> { return v; }`},
		{"named slice index", `type S<T> = distinct T[]; function f(v: S<Maybe>): Ptr { return v[0]; }`},
		{"named slice reslice", `type S<T> = distinct T[]; function f(v: S<Maybe>): Ptr { return v[:][0]; }`},
		{"named map index", `type S<T> = distinct Map<int,T>; function f(v: S<Maybe>): Ptr { return v[0]; }`},
		{"named map checked index", `type S<T> = distinct Map<int,T>; function f(v: S<Maybe>): Ptr { const [item, found] = v[0]; return item; }`},
		{"nested named slice", `type S<T> = distinct T[]; function f(v: S<Maybe>[]): S<Ptr>[] { return v; }`},
		{"pointer", `function f(v: *Maybe): *Ptr { return v; }`},
		{"map", `function f(v: Map<int, Maybe>): Map<int, Ptr> { return v; }`},
		{"object", `function f(v: {item: Maybe}): {item: Ptr} { return v; }`},
		{"call", `type S<T> = distinct T[]; function use(v: S<Ptr>): void {} function f(v: S<Maybe>): void { use(v); }`},
		{"reassignment", `type S<T> = distinct T[]; function f(v: S<Maybe>, x: S<Ptr>): void { x = v; }`},
		{"constructor", `type S<T> = distinct T[]; class C { constructor(public values: S<Ptr>) {} } function f(v: S<Maybe>): C { return new C(v); }`},
		{"struct field", `type S<T> = distinct T[]; struct C { public values: S<Ptr>; } function f(v: S<Maybe>): C { return C{values: v}; }`},
		{"channel nested named", `type S<T> = distinct T[]; function f(v: GoChannel<S<Maybe>>): GoChannel<S<Ptr>> { return v; }`},
		{"class nested argument", `type S<T> = distinct T[]; class C<T> {} function f(v: C<S<Maybe>>): C<S<Ptr>> { return v; }`},
		{"struct nested argument", `type S<T> = distinct T[]; struct C<T> { public value: T; } function f(v: C<S<Maybe>>): C<S<Ptr>> { return v; }`},
		{"interface nested argument", `type S<T> = distinct T[]; interface I<T> { function read(): T; } function f(v: I<S<Maybe>>): I<S<Ptr>> { return v; }`},
		{"interface upcast", `type S<T> = distinct T[]; interface I<T> { function read(): T; } interface J<T> extends I<T> {} function f(v: J<S<Maybe>>): I<S<Ptr>> { return v; }`},
		{"class implementation", `type S<T> = distinct T[]; interface I<T> { function read(): T; } class C<T> implements I<T> { constructor(public value:T) {} public function read(): T { return this.value; } } function f(v: C<S<Maybe>>): I<S<Ptr>> { return v; }`},
		{"class ancestor remapping", `type S<T> = distinct T[]; class Base<T> {} class Child<T> extends Base<S<T>> {} function f(v: Child<Maybe>): Base<S<Ptr>> { return v; }`},
		{"Go callback", `import go http from "net/http"; alias Request = *http.Request | null; function f(v:(w:http.ResponseWriter,r:Request)=>void):http.HandlerFunc{return v;}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := `alias Ptr = *int; alias Maybe = *int | null; ` + test.source
			if diagnostics := checkSource(t, input); !strings.Contains(strings.Join(diagnostics, "\n"), "cannot use") {
				t.Fatalf("missing contract diagnostic: %v", diagnostics)
			}
			matched := strings.NewReplacer("*int | null", "*int", "*http.Request | null", "*http.Request").Replace(input)
			if diagnostics := checkSource(t, matched); len(diagnostics) != 0 {
				t.Fatalf("matching contracts rejected: %v", diagnostics)
			}
		})
	}
}

func TestAssignmentSourceContractCompatibility(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`alias Maybe = *int | null; function f(v:*int):Maybe{return v;}`,
		`class Base{} class Child extends Base{} function f(v:Child):Base|null{return v;}`,
		`class Base<T>{} class Child<U> extends Base<U[]>{} function f(v:Child<int>):Base<int[]>{return v;}`,
		`type Link<T>=distinct *Link<T>;function f(v:Link<int>):Link<int>{return v;}`,
		`type Nodes<T>=distinct Nodes<T>[];function f(v:Nodes<int>):Nodes<int>{return v;}`,
		`struct Node<T>{public value:T;public next:*Node<T>;}function f(v:Node<int>):Node<int>{return v;}`,
		`type Link<T>=distinct *Link<T>;function f(v:Link<int>):Link<int>{return Link<int>(v);}`,
		`type Nodes<T>=distinct Nodes<T>[];function f(v:Nodes<int>[]):Nodes<int>{return Nodes<int>(v);}`,
		`import go fs from "io/fs";function f(v:(path:bstring,entry:fs.DirEntry,err:error)=>Result<void>):fs.WalkDirFunc{return v;}`,
		`import go fmt from "fmt";function f(v:int[]|null):void{if(v!==null){fmt.Println(v);}}`,
	} {
		t.Run(input, func(t *testing.T) {
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("compatible assignment rejected: %v", diagnostics)
			}
		})
	}
}
