package sema

import (
	gotoken "go/token"
	gotypes "go/types"
	"strings"
	"testing"
)

func TestComparableDependencyThroughInstantiatedStruct(t *testing.T) {
	t.Parallel()
	any := gotypes.NewInterfaceType(nil, nil).Complete()
	pending := gotypes.NewTypeParam(gotypes.NewTypeName(gotoken.NoPos, nil, "T", nil), any)
	for _, pointer := range []bool{false, true} {
		element := gotypes.NewTypeParam(gotypes.NewTypeName(gotoken.NoPos, nil, "E", nil), any)
		var field gotypes.Type = element
		if pointer {
			field = gotypes.NewPointer(field)
		}
		named := gotypes.NewNamed(gotypes.NewTypeName(gotoken.NoPos, nil, "Holder", nil), gotypes.NewStruct([]*gotypes.Var{
			gotypes.NewField(gotoken.NoPos, nil, "value", field, false),
		}, nil), nil)
		named.SetTypeParams([]*gotypes.TypeParam{element})
		instance, err := gotypes.Instantiate(nil, named, []gotypes.Type{pending}, false)
		if err != nil {
			t.Fatal(err)
		}
		depends := comparableDependsOnPending(instance, map[gotypes.Type]gotypes.Type{}, map[gotypes.Type]bool{pending: true}, map[gotypes.Type]bool{})
		if depends == pointer {
			t.Fatalf("pointer=%v depends=%v", pointer, depends)
		}
	}
}

func TestRecursiveComparableConstraintBoundary(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		source string
		reject bool
	}{
		{`constraint Pair<E>=comparable&~[0]E;function use<A extends Pair<A>>():void{}`, true},
		{`constraint Pair<E>=comparable&~[1]E;constraint Alias<E>=Pair<E>;function use<A extends Alias<A>>():void{}`, true},
		{`constraint Pair<E>=comparable&~[1]E;function use<A extends Pair<B>,B extends Pair<A>>():void{}`, true},
		{`constraint Pointer<E>=comparable&~*E;function use<A extends Pointer<A>>():void{}`, false},
		{`constraint Pair<E>=comparable&~[1]E;function use<E extends comparable,A extends Pair<E>>():void{}`, false},
	} {
		t.Run(test.source, func(t *testing.T) {
			diagnostics := strings.Join(checkSource(t, test.source), "\n")
			if test.reject && !strings.Contains(diagnostics, "recursive comparable type-set bound cannot be resolved") || !test.reject && diagnostics != "" {
				t.Fatalf("diagnostics=%s", diagnostics)
			}
		})
	}
}

func TestUnresolvedSourceConstraintStorage(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		`struct Holder<E>{value:E;}constraint Box<E>=comparable&Holder<E>;function use<A extends Box<A>>():void{}`,
		`struct Holder<E>{value:E;}constraint Box<E>=Holder<E>;`,
		`struct Holder{value:int;}constraint Box=comparable&Holder;`,
		`struct Holder<E>{value:E;}constraint Box<E>=comparable&~[1]Holder<E>;`,
	} {
		t.Run(source, func(t *testing.T) {
			diagnostics := strings.Join(checkSource(t, source), "\n")
			if !strings.Contains(diagnostics, "storage is not yet resolved") {
				t.Fatalf("diagnostics=%s", diagnostics)
			}
		})
	}
}
