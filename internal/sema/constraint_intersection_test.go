package sema

import (
	gotoken "go/token"
	gotypes "go/types"
	"strings"
	"testing"
)

func TestConstraintIntersectionSemanticMatrix(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"union intersection", `constraint Both=A&B; constraint A=~int|~string; constraint B=~int|~int8; function twice<T extends Both>(x:T):T{return x*2;} function use():int{return twice(3);}`, ""},
		{"reject excluded type", `constraint A=~int|~string; constraint B=~int|~int8; constraint Both=A&B; function keep<T extends Both>(x:T):T{return x;} function bad(x:int8):int8{return keep(x);}`, "does not satisfy"},
		{"exact left", `type Score=distinct int; constraint Only=Score&~int; function keep<T extends Only>(x:T):T{return x;} function use():Score{return keep(Score(3));}`, ""},
		{"exact right", `type Score=distinct int; constraint Only=~int&Score; function keep<T extends Only>(x:T):T{return x;} function bad(x:int):int{return keep(x);}`, "does not satisfy"},
		{"repeat and reuse", `constraint A=~int|~string; constraint Both=A&A&A; constraint More=Both|~boolean; function keep<T extends More>(x:T):T{return x;}`, ""},
		{"repeat generic union", `constraint A<E>=~E[]|~[2]E; constraint Both<E>=A<E>&A<E>;`, ""},
		{"reordered generic union", `constraint A<E>=~E[]|~[2]E; constraint B<E>=~[2]E|~E[]; constraint Both<E>=A<E>&B<E>;`, ""},
		{"overlap in containing union", `constraint Both=~int&int; constraint Bad=Both|int;`, "overlaps an earlier term"},
		{"empty", `constraint Bad=~int&string;`, "no common types"},
		{"empty stays empty", `constraint Bad=int&string&int;`, "no common types"},
		{"cycle", `constraint A=B&int; constraint B=A&int;`, "constraint declaration cycle"},
		{"invalid reference", `constraint A=Missing&int;`, "unknown type"},
		{"invalid underlying", `constraint A=~int; constraint B=~A&A;`, "cannot name a constraint"},
		{"interface operand", `import go io from "io"; constraint A=io.Reader&int;`, "must be a concrete type"},
		{"generic inference", `constraint Base<E>=~E[]; constraint Slice<E>=Base<E>&~E[]; function copy<S extends Slice<E>,E>(xs:S):E[]{let result:E[]=[];for(const x of xs){result=append(result,x);}return result;} function use(xs:int[]):int[]{return copy(xs);}`, ""},
		{"concrete generic intersection", `constraint Base<E>=~E[]; constraint Slice=Base<int>&~int[]; function use<S extends Slice>(xs:S):int{let total=0;for(const x of xs){total+=x;}return total;}`, ""},
		{"dependent match", `constraint Bad<E>=~E[]&~int[];`, "unmatched parameter-dependent terms"},
		{"dependent match alongside concrete", `constraint A<E>=~E[]|int; constraint B=~int[]|int; constraint Bad<E>=A<E>&B;`, "unmatched parameter-dependent terms"},
		{"nullable preserved", `class Leaf{public value:int=1;} alias Maybe=Leaf|null; constraint A=~Maybe[]; constraint B=A&A; function bad<S extends B>(xs:S):int{for(const x of xs){return x.value;}return 0;}`, "nullable"},
		{"nullable shape conflict", `class Leaf{} alias Maybe=Leaf|null; constraint A=~Leaf[]; constraint B=~Maybe[]; constraint Bad=A&B;`, "incompatible nullable source types"},
		{"nullable exact conflict", `class Leaf{} alias Maybe=Leaf|null; constraint A=Leaf[]&~Maybe[];`, "incompatible nullable source types"},
		{"generic nullable conflict", `class Leaf{} alias Maybe=Leaf|null; constraint A<E>=~E[]; constraint Bad=A<Leaf>&A<Maybe>;`, "incompatible nullable source types"},
		{"runtime misuse", `constraint A=int&~int; function bad(x:A):void{}`, "can only be used after 'extends'"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}

func TestConstraintIntersectionOperandLimit(t *testing.T) {
	t.Parallel()
	source := "constraint A=" + strings.Repeat("~int&", 100) + "~int;"
	if diagnostics := checkSource(t, source); len(diagnostics) != 0 {
		t.Fatalf("intersection operands are not union terms: %v", diagnostics)
	}
}

func TestConstraintIntersectionParameterDetection(t *testing.T) {
	t.Parallel()
	any := gotypes.NewInterfaceType(nil, nil).Complete()
	parameter := gotypes.NewTypeParam(gotypes.NewTypeName(gotoken.NoPos, nil, "E", nil), any)
	variable := func(value gotypes.Type) *gotypes.Var { return gotypes.NewVar(gotoken.NoPos, nil, "value", value) }
	signature := func(value gotypes.Type) *gotypes.Signature {
		return gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(variable(value)), gotypes.NewTuple(), false)
	}
	named := gotypes.NewNamed(gotypes.NewTypeName(gotoken.NoPos, nil, "Box", nil), gotypes.NewSlice(parameter), nil)
	named.SetTypeParams([]*gotypes.TypeParam{parameter})
	instance, err := gotypes.Instantiate(nil, named, []gotypes.Type{parameter}, false)
	if err != nil {
		t.Fatal(err)
	}
	methodInterface := gotypes.NewInterfaceType([]*gotypes.Func{gotypes.NewFunc(gotoken.NoPos, nil, "Read", signature(parameter))}, nil).Complete()
	for _, value := range []gotypes.Type{
		parameter, instance, gotypes.NewSlice(parameter), gotypes.NewArray(parameter, 2),
		gotypes.NewPointer(parameter), gotypes.NewChan(gotypes.RecvOnly, parameter),
		gotypes.NewMap(gotypes.Typ[gotypes.String], parameter), signature(parameter),
		gotypes.NewStruct([]*gotypes.Var{variable(parameter)}, nil), methodInterface,
		gotypes.NewInterfaceType(nil, []gotypes.Type{methodInterface}).Complete(),
	} {
		if !constraintTypeHasParameters(value) {
			t.Errorf("missed parameter in %s", value)
		}
	}
	recursive := gotypes.NewNamed(gotypes.NewTypeName(gotoken.NoPos, nil, "Node", nil), nil, nil)
	recursive.SetUnderlying(gotypes.NewStruct([]*gotypes.Var{variable(gotypes.NewPointer(recursive))}, nil))
	concrete, err := gotypes.Instantiate(nil, named, []gotypes.Type{gotypes.Typ[gotypes.Int]}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []gotypes.Type{recursive, concrete, any, signature(gotypes.Typ[gotypes.Int]), gotypes.NewStruct(nil, nil)} {
		if constraintTypeHasParameters(value) {
			t.Errorf("spurious parameter in %s", value)
		}
	}
}
