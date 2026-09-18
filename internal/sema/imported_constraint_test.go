package sema

import (
	goast "go/ast"
	"go/importer"
	goparser "go/parser"
	gotoken "go/token"
	gotypes "go/types"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestImportedConstraintComposition(t *testing.T) {
	t.Parallel()
	set := gotoken.NewFileSet()
	file, err := goparser.ParseFile(set, "constraints.go", `package constraints
type Integer interface{~int|~int64}
type Ints interface{~int}
type Exact interface{int}
type Nested interface{Integer;Ints}
type Overlap interface{Exact|Ints}
type ReverseOverlap interface{Ints|Exact}
type AnyUnion interface{int|any}
type Nothing interface{int;string}
type Slice[E any] interface{~[]E}
type Sequence[E any] interface{~[]E|~[2]E|~*[3]E}
type Text interface{~string|~[]byte}
type Pair[E any] interface{~[2]E}
type IntSlice[E any] interface{~[]E;~[]int}
type Lookup[K comparable,V any] interface{~map[K]V}
type Checked interface{comparable}
type CheckedAlias = Checked
type Mixed interface{~int|~[]int}
type Filtered interface{Checked;Mixed}
type Empty interface{comparable;~[]int}
type UnsafeEquality interface{comparable;~[1]any}
type NamedInteger interface{Integer;String()string}
type GenericMethod[E any] interface{~int;Get()E}
type Hidden interface{~int;hidden()}
type Alias = Nested
type Any = any
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := (&gotypes.Config{GoVersion: "go1.23"}).Check("bounds.test/constraints", set, []*goast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, source, want string }{
		{"mixed imported sequence", `function f<E,T extends c.Sequence<E>>(v:T):E{return v[0];}`, ""},
		{"mixed imported text", `function f<T extends c.Text>(v:T):T{const b:byte=v[0];return v[1:];}`, ""},
		{"mixed wrapped nullable", `class C{public value:int=1;}constraint S<E>=c.Sequence<E>;function f<T extends S<C|null>>(v:T):int{return v[0].value;}`, "nullable"},
		{"mixed wrapped nullable call", `class C{}alias M=C|null;constraint S<E>=c.Sequence<E>;function f<T extends S<C>>(v:T):C{return v[0];}function bad(v:M[]):C{return f(v);}`, "nullable type information"},
		{"nested intersections", `constraint A=c.Nested&~int; function double<T extends A>(x:T):T{return x*2;} function use():int{return double(3);}`, ""},
		{"excluded argument", `constraint A=c.Nested; function keep<T extends A>(x:T):T{return x;} function bad(x:int64):int64{return keep(x);}`, "does not satisfy"},
		{"Go alias", `constraint A=c.Alias; function double<T extends A>(x:T):T{return x*2;}`, ""},
		{"imported union", `constraint A=c.Integer|~string; function keep<T extends A>(x:T):T{return x;} function use():string{return keep("yes");}`, ""},
		{"legal imported overlap", `constraint A=c.Overlap&~int; function double<T extends A>(x:T):T{return x*2;}`, ""},
		{"reversed imported overlap", `constraint A=c.ReverseOverlap&~int;`, ""},
		{"source overlap", `constraint A=c.Integer|~int;`, "overlaps an earlier term"},
		{"unrestricted union intersection", `constraint A=c.AnyUnion&~int; function double<T extends A>(x:T):T{return x*2;}`, ""},
		{"unrestricted cannot join union", `constraint A=c.AnyUnion|~string;`, "union operands must"},
		{"empty imported", `constraint A=c.Nothing;`, "no common types"},
		{"empty source", `constraint A=c.Integer&~string;`, "no common types"},
		{"slice inference", `constraint A<E>=c.Slice<E>&~E[]; function copy<S extends A<E>,E>(xs:S):E[]{let out:E[]=[];for(const x of xs){out=append(out,x);}return out;} function use(xs:int[]):int[]{return copy(xs);}`, ""},
		{"concrete imported substitution", `constraint A=c.IntSlice<int>; function total<T extends A>(xs:T):int{let sum=0;for(const x of xs){sum+=x;}return sum;}`, ""},
		{"empty imported substitution", `constraint A=c.IntSlice<string>;`, "no common types"},
		{"nullable shape preserved", `class Leaf{public value:int=1;} alias Maybe=Leaf|null; constraint A<E>=c.Slice<E>; function bad<S extends A<Maybe>>(xs:S):int{for(const x of xs){return x.value;}return 0;}`, "nullable"},
		{"nullable shape mismatch", `class Leaf{} alias Maybe=Leaf|null; constraint A<E>=c.Slice<E>; constraint Bad=A<Leaf>&~Maybe[];`, "incompatible nullable source types"},
		{"native element identity", `class Leaf{public value:int=1;} constraint A<E>=c.Slice<E>; function read<S extends A<Leaf>>(xs:S):int{for(const x of xs){return x.value;}return 0;} function use(xs:Leaf[]):int{return read(xs);}`, ""},
		{"comparable source", `constraint Key=comparable; constraint Both=Key&Key; function equal<T extends Both>(a:T,b:T):boolean{return a===b;} function use():boolean{return equal(1,1);}`, ""},
		{"comparable imported", `constraint Key=c.CheckedAlias; constraint Both=Key&c.Checked; function equal<T extends Both>(a:T,b:T):boolean{return a===b;}`, ""},
		{"comparable rejects slice", `constraint Key=c.Checked; constraint Same=Key; function keep<T extends Same>(a:T):T{return a;} function bad(xs:int[]):int[]{return keep(xs);}`, "does not satisfy"},
		{"comparable exception", `constraint Key=c.Checked; function keep<T extends Key>(a:T):T{return a;} function use(x:c.Any):c.Any{return keep<c.Any>(x);}`, ""},
		{"filter imported", `constraint A=c.Filtered; function double<T extends A>(x:T):T{return x*2;}`, ""},
		{"filter source", `constraint A=c.Checked&c.Mixed; function double<T extends A>(x:T):T{return x*2;}`, ""},
		{"empty comparable", `constraint A=c.Empty;`, "no common types"},
		{"strict comparability", `constraint A=c.UnsafeEquality;`, "no common types"},
		{"empty source comparable", `constraint A=comparable&~int[];`, "no common types"},
		{"comparable union", `constraint A=c.Checked|~int;`, "union operands must"},
		{"comparable restricted union", `constraint A=c.Filtered|~string;`, "union operands must"},
		{"comparable reuse union", `constraint A=comparable&~int; constraint Bad=A|~string;`, "union operands must"},
		{"implied comparable union", `constraint A=c.Integer|~string;`, ""},
		{"comparable generic substitution", `constraint A<E>=comparable&c.Pair<E>; function keep<T extends A<int>>(x:T):T{return x;}`, ""},
		{"comparable forward inference", `constraint A<E>=comparable&c.Pair<E>; function equal<T extends A<E>,E extends comparable>(a:T,b:T):boolean{return a===b;} function use(a:[2]int,b:[2]int):boolean{return equal(a,b);}`, ""},
		{"empty dependent inference does not panic", `constraint A<E>=comparable&c.Pair<E>; function keep<T extends A<E>,E>(x:T):T{return x;} function bad(x:[2]int):[2]int{return keep(x);}`, "cannot infer type argument"},
		{"method forward comparable parameter", `constraint A<E>=comparable&c.Pair<E>; class Box<X>{public function equal<T extends A<E>,E extends comparable>(a:T,b:T):boolean{return a===b;}} function use(a:[2]int,b:[2]int):boolean{return new Box<string>().equal(a,b);}`, ""},
		{"missing comparable parameter", `constraint A<K,V>=c.Lookup<K,V>;`, "does not satisfy comparable"},
		{"method with type set", `constraint A=c.NamedInteger&~int; function show<T extends A>(x:T):string{return (x*2).String();}`, ""},
		{"method union", `constraint A=c.NamedInteger|~string;`, "union operands must"},
		{"missing method", `constraint A=c.NamedInteger; function keep<T extends A>(x:T):T{return x;} function bad():int{return keep(1);}`, "does not satisfy"},
		{"generic method", `constraint A<E>=c.GenericMethod<E>; function read<T extends A<E>,E>(x:T):E{return x.Get();}`, ""},
		{"method nullable rejected", `class Leaf{} constraint A<E>=c.GenericMethod<E>; function bad<T extends A<Leaf|null>>(x:T):void{}`, "must preserve source type information"},
		{"hidden identity", `constraint A=c.Hidden; function keep<T extends A>(x:T):T{return x;} function bad():int{return keep(1);}`, "does not satisfy"},
		{"parameter dependent intersection", `constraint A<E>=c.Slice<E>&~int[];`, "unmatched parameter-dependent terms"},
		{"tilde imported", `constraint A=~c.Integer;`, "must name concrete types"},
		{"tilde comparable", `constraint A=~comparable;`, "must name concrete types"},
	} {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("test.km", `import go c from "bounds.test/constraints";`+test.source)
			program, parseDiagnostics := parser.Parse(tokens)
			if len(lexDiagnostics)+len(parseDiagnostics) != 0 {
				t.Fatalf("lex=%v parse=%v", lexDiagnostics, parseDiagnostics)
			}
			diagnostics := CheckScopedWithGoImporter(program, nil, dependentConstraintImporter{importer.Default(), pkg})
			var messages []string
			for _, diagnostic := range diagnostics {
				messages = append(messages, diagnostic.Message)
			}
			if test.want == "" && len(messages) != 0 || test.want != "" && !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", messages, test.want)
			}
		})
	}
}
