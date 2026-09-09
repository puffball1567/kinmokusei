package sema

import (
	"fmt"
	"strings"
	"testing"
)

func TestSourceGenericConstraintSemanticMatrix(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"constraint reuse forward", `constraint Number=Signed|Unsigned; constraint Signed=~int|~int32; constraint Unsigned=~uint; function add<T extends Number>(a:T,b:T):T{return a+b;}`, ""},
		{"generic reuse inference", `constraint Slice<E> =Storage<E>; constraint Storage<T> =~T[]; function elements<S extends Slice<E>,E>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;} function use(values:int[]):int[]{return elements(values);}`, ""},
		{"generic reuse nullable", `constraint Slice<E> =Storage<E>; constraint Storage<T> =~T[]; class Leaf{public value:int=1;} alias Maybe=Leaf|null; function elements<S extends Slice<E>,E>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;} function bad(values:Maybe[]):int{return elements(values)[0].value;}`, "nullable"},
		{"reuse nullable mismatch", `class Leaf{} alias Maybe=Leaf|null; constraint Base<E> =~E[]; constraint Slice<E> =Base<E>; function use<S extends Slice<Leaf>>(values:S):void{} function bad(values:Maybe[]):void{use(values);}`, "nullable type information does not match"},
		{"reuse direct cycle", `constraint A=A;`, "constraint declaration cycle"},
		{"reuse generic cycle", `constraint A<E> =B<E>; constraint B<T> =A<T>;`, "constraint declaration cycle"},
		{"reuse tilde", `constraint A=~int; constraint B=~A;`, "cannot name a constraint"},
		{"reuse overlap", `constraint A=~int; constraint B=A|int;`, "overlaps an earlier term"},
		{"reuse generic overlap", `constraint A<E> =~E[]; constraint B=A<int>|~int[];`, "overlaps an earlier term"},
		{"reuse bound mismatch", `constraint A<E extends comparable> =~E[]; constraint B=A<int[]>;`, "does not satisfy E type parameter constraint"},
		{"reuse missing argument", `constraint A<E> =~E[]; constraint B=A;`, "expects 1 type arguments, got 0"},
		{"reuse parameter shadows constraint", `constraint A=~int; constraint B<A> =A;`, "must be a concrete type"},
		{"slice inference", `constraint Slice<E> = ~E[]; function elements<S extends Slice<E>,E>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;} function use(values:int[]):int[]{return elements(values);}`, ""},
		{"map", `constraint Lookup<K extends comparable,V> = ~Map<K,V>; function values<M extends Lookup<K,V>,K extends comparable,V>(items:M):V[]{let result:V[]=[];for(const [key,value] of items){result=append(result,value);}return result;} function use(items:Map<string,int>):int[]{return values(items);}`, ""},
		{"class forward", `class Box<E,S extends Slice<E>>{constructor(private values:S){} public function elements():E[]{let result:E[]=[];for(const value of this.values){result=append(result,value);}return result;}} constraint Slice<E> = ~E[]; function use(values:int[]):int[]{return new Box<int,int[]>(values).elements();}`, ""},
		{"class element", `constraint Slice<E> = ~E[]; class Leaf{public value:int=1;} function elements<S extends Slice<E>,E>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;} function use(values:Leaf[]):int{return elements(values)[0].value;}`, ""},
		{"nullable elements remain nullable", `constraint Slice<E> = ~E[]; class Leaf{public value:int=1;} alias MaybeLeaf=Leaf|null; function elements<S extends Slice<E>,E>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;} function bad(values:MaybeLeaf[]):int{return elements(values)[0].value;}`, "nullable"},
		{"nullable elements accepted", `constraint Slice<E> = ~E[]; class Leaf{} alias MaybeLeaf=Leaf|null; function elements<S extends Slice<E>,E>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;} function use(values:MaybeLeaf[]):MaybeLeaf[]{return elements(values);}`, ""},
		{"nullable explicit element mismatch", `constraint Slice<E> = ~E[]; class Leaf{} alias MaybeLeaf=Leaf|null; function elements<E,S extends Slice<E>>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;} function bad(values:MaybeLeaf[]):Leaf[]{return elements<Leaf,MaybeLeaf[]>(values);}`, "nullable type information does not match"},
		{"nullable widening would break invariance", `constraint Slice<E> = ~E[]; class Leaf{} alias MaybeLeaf=Leaf|null; function elements<E,S extends Slice<E>>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;} function bad(values:Leaf[]):MaybeLeaf[]{return elements<MaybeLeaf,Leaf[]>(values);}`, "nullable type information does not match"},
		{"concrete nullable constraint", `class Leaf{public value:int=1;} alias MaybeLeaf=Leaf|null; constraint Leaves=~MaybeLeaf[]; function bad<S extends Leaves>(values:S):int{for(const leaf of values){return leaf.value;}return 0;}`, "nullable"},
		{"nullable method owner", `constraint Slice<E> = ~E[]; class Leaf{public value:int=1;} alias MaybeLeaf=Leaf|null; class Collector<E>{public function collect<S extends Slice<E>>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;}} function bad(values:Leaf[]):MaybeLeaf[]{return new Collector<MaybeLeaf>().collect(values);}`, "nullable type information does not match"},
		{"method owner", `constraint Slice<E> = ~E[]; class Box<E>{public function elements<S extends Slice<E>>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;}} function use(values:int[]):int[]{return new Box<int>().elements(values);}`, ""},
		{"nested bound", `constraint Rows<S extends Slice<E>,E> = ~S[]; constraint Slice<E> = ~E[]; function flatten<E,S extends Slice<E>,R extends Rows<S,E>>(rows:R):E[]{let result:E[]=[];for(const row of rows){for(const value of row){result=append(result,value);}}return result;} function use(rows:int[][]):int[]{return flatten(rows);}`, ""},
		{"bounded forward", `constraint Slice<E extends Integer> = ~E[]; constraint Integer=~int|~int32; function count<S extends Slice<int>>(values:S):int{let total=0;for(const value of values){total+=value;}return total;}`, ""},
		{"own scope", `constraint Slice<E extends Integer> = ~E[]; class Box<Integer,S extends Slice<int>>{} constraint Integer=~int;`, ""},
		{"alias", `alias Values<E,S extends Slice<E>> = S; constraint Slice<E> = ~E[]; function use(values:Values<int,int[]>):int{return len(values);}`, ""},
		{"struct interface", `constraint Slice<E> = ~E[]; struct Box<E,S extends Slice<E>>{public values:S;} interface Reader<E,S extends Slice<E>>{function read():S;} function use(value:Box<int,int[]>):int[]{return value.values;}`, ""},
		{"channel", `constraint Receive<E> = GoChannel<E>|GoReceiveChannel<E>; function collect<E,C extends Receive<E>>(channel:C):E[]{let result:E[]=[];for(const value of channel){result=append(result,value);}return result;}`, ""},
		{"iterator", `constraint Sequence<E> = (yield:(value:E)=>boolean)=>void; function collect<E,S extends Sequence<E>>(sequence:S):E[]{let result:E[]=[];for(const value of sequence){result=append(result,value);}return result;}`, ""},
		{"array pointer", `constraint Array<E> = ~[2]E; constraint Pointer<E> = *[2]E; function use<A extends Array<int>>(values:A):int{let total=0;for(const value of values){total+=value;}return total;}`, ""},
		{"missing arguments", `constraint Slice<E> = ~E[]; function use<T extends Slice>(value:T):void{}`, "expects 1 type arguments, got 0"},
		{"extra arguments", `constraint Slice<E> = ~E[]; function use<T extends Slice<int,string>>(value:T):void{}`, "expects 1 type arguments, got 2"},
		{"non generic arguments", `constraint Integer=~int; function use<T extends Integer<int>>(value:T):void{}`, "expects 0 type arguments, got 1"},
		{"wrong bound", `constraint Slice<E extends comparable> = ~E[]; function use<T extends Slice<int[]>>(value:T):void{}`, "does not satisfy comparable"},
		{"wrong inferred argument", `constraint Slice<E> = ~E[]; function keep<E,S extends Slice<E>>(value:S):S{return value;} function use(value:string[]):string[]{return keep<int,string[]>(value);}`, "does not satisfy S type parameter constraint"},
		{"void argument", `constraint Slice<E> = ~E[]; function use<T extends Slice<void>>(value:T):void{}`, "cannot be used as a constraint type argument"},
		{"runtime misuse", `constraint Slice<E> = ~E[]; function use(values:Slice<int>):void{}`, "can only be used after 'extends'"},
		{"implements misuse", `constraint Slice<E> = ~E[]; class Bad implements Slice<int>{}`, "can only be used after 'extends'"},
		{"bare parameter term", `constraint Bad<E> = E;`, "must be a concrete type"},
		{"tilde parameter term", `constraint Bad<E> = ~E;`, "must be a concrete type"},
		{"map requires comparable", `constraint Bad<K,V> = ~Map<K,V>;`, "cannot be used as a Map key"},
		{"duplicate parameters", `constraint Bad<E,E> = ~E[];`, "duplicate generic constraint type parameter"},
		{"duplicate terms", `constraint Bad<E> = ~E[]|~E[];`, "overlaps an earlier term"},
		{"direct cycle", `constraint Bad<E extends Bad<E>> = ~E[];`, "constraint declaration cycle"},
		{"indirect cycle", `constraint A<E extends B<E>> = ~E[]; constraint B<E extends A<E>> = ~E[];`, "constraint declaration cycle"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}

func TestSourceGenericConstraintExpansionLimit(t *testing.T) {
	terms := make([]string, 100)
	for i := range terms {
		terms[i] = fmt.Sprintf("~[%d]int", i)
	}
	input := "constraint Base=" + strings.Join(terms, "|") + "; constraint Copy=Base;"
	if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
		t.Fatalf("100 expanded terms: %v", diagnostics)
	}
	if diagnostics := checkSource(t, input+"constraint Large=Copy|~[100]int;"); !strings.Contains(strings.Join(diagnostics, "\n"), "expanded constraint declarations cannot contain more than 100 terms") {
		t.Fatalf("101 expanded terms: %v", diagnostics)
	}
}
