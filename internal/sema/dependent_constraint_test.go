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

type dependentConstraintImporter struct {
	gotypes.Importer
	fixture *gotypes.Package
}

func (i dependentConstraintImporter) Import(path string) (*gotypes.Package, error) {
	if path == "bounds.test/constraints" {
		return i.fixture, nil
	}
	return i.Importer.Import(path)
}

func TestDependentConstraintSemanticMatrix(t *testing.T) {
	set := gotoken.NewFileSet()
	file, err := goparser.ParseFile(set, "constraints.go", `package constraints
type Slice[E any] interface { ~[]E }
type Map[K comparable, V any] interface { ~map[K]V }
type Pointer[E any] interface { ~*E }
type Pair[E any] interface { ~[2]E }
type Seq[E any] interface { ~func(func(E) bool) }
type Reader[E any] interface { Read() E }
type Nested[E any] interface { ~[]*E }
type Equal[E comparable] interface { comparable; Equal(E) bool }
type Count int
func (value Count) Read() int { return int(value) }
func (value Count) Equal(other Count) bool { return value == other }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := (&gotypes.Config{GoVersion: "go1.23"}).Check("bounds.test/constraints", set, []*goast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, source, want string }{
		{"explicit", `function first<E, S extends c.Slice<E>>(values:S, fallback:E):E { for(const value of values){return value;} return fallback; } function use(values:int[]):int { return first<int,int[]>(values,0); }`, ""},
		{"inferred", `function elements<E, S extends c.Slice<E>>(values:S):E[] { let result:E[]=[]; for(const value of values){result=append(result,value);} return result; } function use(values:string[]):string[] { return elements(values); }`, ""},
		{"forward bound", `function elements<S extends c.Slice<E>, E>(values:S):E[] { let result:E[]=[]; for(const value of values){result=append(result,value);} return result; } function use(values:int[]):int[] { return elements(values); }`, ""},
		{"partial explicit", `function elements<E, S extends c.Slice<E>>(values:S):E[] { let result:E[]=[]; for(const value of values){result=append(result,value);} return result; } function use(values:int[]):int[] { return elements<int>(values); }`, ""},
		{"dependent comparable", `function values<M extends c.Map<K,V>, K extends comparable, V>(values:M):V[] { let result:V[]=[]; for(const [key,value] of values){result=append(result,value);} return result; } function use(input:Map<string,int>):int[] {return values(input);}`, ""},
		{"named slice", `type Numbers=distinct int[]; function elements<E,S extends c.Slice<E>>(values:S):E[] {let result:E[]=[];for(const value of values){result=append(result,value);}return result;} function use(values:Numbers):int[]{return elements(values);}`, ""},
		{"constraint inference chain", `function flatten<E,S extends c.Slice<E>,SS extends c.Slice<S>>(values:SS):E[] {let result:E[]=[];for(const items of values){for(const item of items){result=append(result,item);}}return result;} function use(values:int[][]):int[]{return flatten(values);}`, ""},
		{"class", `class Box<E,S extends c.Slice<E>> {constructor(private values:S){} public function elements():E[]{let result:E[]=[];for(const value of this.values){result=append(result,value);}return result;}} function use(values:int[]):int[]{return new Box<int,int[]>(values).elements();}`, ""},
		{"method owner substitution", `class Box<E>{public function elements<S extends c.Slice<E>>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;}} function use(values:int[]):int[]{const box=new Box<int>();return box.elements(values);}`, ""},
		{"inherited method", `class Box<E>{public function elements<S extends c.Slice<E>>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;}} class IntBox extends Box<int>{} function use(values:int[]):int[]{return new IntBox().elements(values);}`, ""},
		{"method shadows owner", `class Box<E>{public function elements<E,S extends c.Slice<E>>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;}} function use(values:int[]):int[]{return new Box<string>().elements(values);}`, "conflicts with a class type parameter"},
		{"struct", `struct Box<E,S extends c.Slice<E>>{public values:S;} function use(values:int[]):Box<int,int[]>{return Box<int,int[]>{values:values};}`, ""},
		{"interface", `interface Box<E,S extends c.Slice<E>>{function get():S;} function use(box:Box<int,int[]>):int[]{return box.get();}`, ""},
		{"alias", `alias Box<E,S extends c.Slice<E>> = S; function use(values:Box<int,int[]>):int[]{return values;}`, ""},
		{"defined type", `type Box<E,S extends c.Slice<E>> = distinct S[]; function use(values:Box<int,int[]>):int{return len(values);}`, ""},
		{"forward source dependency", `class Box<S extends c.Slice<E>,E extends Integer>{constructor(private values:S){} public function sum():E{let total=E(0);for(const value of this.values){total+=value;}return total;}} constraint Integer=~int|~int32; function use(values:int[]):int{return new Box<int[],int>(values).sum();}`, ""},
		{"recursive bound", `function use<T extends c.Pointer<T>>(value:T):T{return value;}`, ""},
		{"recursive comparable bound", `function use<T extends c.Equal<T>>(value:T):T{return value;}`, ""},
		{"recursive comparable argument", `function identity<T extends c.Equal<T>>(value:T):T{return value;} function use(value:c.Count):c.Count{return identity(value);}`, ""},
		{"method constraint inference", `function extract<E,R extends c.Reader<E>>(value:R):E{return value.Read();} function use(value:c.Count):int{return extract(value);}`, ""},
		{"mutual bound", `function use<T extends c.Pointer<U>,U extends c.Pointer<T>>(value:T):T{return value;}`, ""},
		{"caller parameters", `function elements<E,S extends c.Slice<E>>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;} function use<E,S extends c.Slice<E>>(values:S):E[]{return elements(values);}`, ""},
		{"wrong dependent argument", `function use<E,S extends c.Slice<E>>(values:S):void{} function bad(values:string[]):void{use<int,string[]>(values);}`, "does not satisfy S type parameter constraint"},
		{"wrong class argument", `class Box<E,S extends c.Slice<E>>{} function bad():void{const box=new Box<int,string[]>();}`, "does not satisfy S type parameter constraint"},
		{"wrong inferred element", `function use<E,S extends c.Slice<E>>(values:S, element:E):void{} function bad(values:string[]):void{use(values,1);}`, "cannot use integer literal as string"},
		{"missing comparable", `function use<K,V,M extends c.Map<K,V>>(values:M):void{}`, "does not satisfy comparable"},
		{"bare parameter", `function use<E,S extends E>(value:S):void{}`, "must be a Go interface constraint"},
		{"bare cycle", `function use<E extends S,S extends E>(value:S):void{}`, "must be a Go interface constraint"},
		{"shadow source constraint", `constraint E=~int; function use<E,S extends E>(value:S):void{}`, "must be a Go interface constraint"},
		{"unknown bound argument", `function use<S extends c.Slice<Missing>>(value:S):void{}`, "unknown type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("bounds.km", `import go c from "bounds.test/constraints"; `+test.source)
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
				t.Fatalf("diagnostics=%v, want %q", messages, test.want)
			}
		})
	}
}

func TestDependentConstraintSubstitutionPreservesIdentity(t *testing.T) {
	any := gotypes.NewInterfaceType(nil, nil).Complete()
	first := gotypes.NewTypeParam(gotypes.NewTypeName(gotoken.NoPos, nil, "E", nil), any)
	second := gotypes.NewTypeParam(gotypes.NewTypeName(gotoken.NoPos, nil, "E", nil), any)
	alias := gotypes.NewAlias(gotypes.NewTypeName(gotoken.NoPos, nil, "Element", nil), first)
	original := gotypes.NewMap(first, gotypes.NewSlice(second))
	bindings := map[gotypes.Type]gotypes.Type{first: gotypes.Typ[gotypes.Int]}
	got := substituteConstraintType(original, bindings).(*gotypes.Map)
	if got.Key() != gotypes.Typ[gotypes.Int] || got.Elem().(*gotypes.Slice).Elem() != second {
		t.Fatalf("substitution crossed declaration identities: %s", got)
	}
	if original.Key() != first || original.Elem().(*gotypes.Slice).Elem() != second {
		t.Fatal("substitution mutated the declaration")
	}
	if got := substituteConstraintType(alias, bindings); got != gotypes.Typ[gotypes.Int] {
		t.Fatalf("alias substitution = %s", got)
	}
}
