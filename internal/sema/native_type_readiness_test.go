package sema

import (
	gotypes "go/types"
	"testing"
)

func TestNativeStorageReadiness(t *testing.T) {
	t.Parallel()
	unfinished := gotypes.NewNamed(gotypes.NewTypeName(0, nil, "Pending", nil), nil, nil)
	integer := gotypes.Typ[gotypes.Int]
	tuple := gotypes.NewTuple(gotypes.NewVar(0, nil, "value", unfinished))
	signature := gotypes.NewSignatureType(nil, nil, nil, tuple, gotypes.NewTuple(), false)
	contract := gotypes.NewInterfaceType([]*gotypes.Func{gotypes.NewFunc(0, nil, "Read", signature)}, nil).Complete()
	recursive := gotypes.NewNamed(gotypes.NewTypeName(0, nil, "Recursive", nil), nil, nil)
	recursive.SetUnderlying(gotypes.NewPointer(recursive))
	cycle := gotypes.NewNamed(gotypes.NewTypeName(0, nil, "Cycle", nil), nil, nil)
	cycle.SetUnderlying(gotypes.NewStruct([]*gotypes.Var{gotypes.NewField(0, nil, "Value", cycle, false)}, nil))
	for _, test := range []struct {
		name              string
		value             gotypes.Type
		comparable, graph bool
	}{
		{"nil", nil, false, false},
		{"basic", integer, true, true},
		{"unfinished named", unfinished, false, false},
		{"pointer", gotypes.NewPointer(unfinished), true, false},
		{"slice", gotypes.NewSlice(unfinished), true, false},
		{"channel", gotypes.NewChan(gotypes.SendRecv, unfinished), true, false},
		{"map", gotypes.NewMap(integer, unfinished), true, false},
		{"array", gotypes.NewArray(unfinished, 2), false, false},
		{"struct", gotypes.NewStruct([]*gotypes.Var{gotypes.NewField(0, nil, "Value", unfinished, false)}, nil), false, false},
		{"tuple", tuple, true, false},
		{"signature", signature, true, false},
		{"method interface", contract, true, false},
		{"complete recursive pointer", recursive, true, true},
		{"invalid by-value cycle", cycle, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := comparableStorageReady(test.value, map[gotypes.Type]bool{}); got != test.comparable {
				t.Fatalf("comparability predicate readiness=%v want=%v", got, test.comparable)
			}
			if got := nativeStorageGraphReady(test.value, map[gotypes.Type]bool{}); got != test.graph {
				t.Fatalf("nested graph readiness=%v want=%v", got, test.graph)
			}
		})
	}
	for _, storage := range []gotypes.Type{unfinished, cycle} {
		if (Type{Kind: GoNamed, GoType: storage}).IsComparable() {
			t.Fatal("unfinished/cyclic storage reported comparable")
		}
	}
}

func TestNativeStorageGraphReadinessBoundsGenericExpansion(t *testing.T) {
	t.Parallel()
	parameter := gotypes.NewTypeParam(gotypes.NewTypeName(0, nil, "T", nil), gotypes.NewInterfaceType(nil, nil).Complete())
	origin := gotypes.NewNamed(gotypes.NewTypeName(0, nil, "Node", nil), nil, nil)
	origin.SetTypeParams([]*gotypes.TypeParam{parameter})
	child, err := gotypes.Instantiate(nil, origin, []gotypes.Type{gotypes.NewSlice(parameter)}, false)
	if err != nil {
		t.Fatal(err)
	}
	origin.SetUnderlying(gotypes.NewStruct([]*gotypes.Var{gotypes.NewField(0, nil, "Next", gotypes.NewPointer(child), false)}, nil))
	instance, err := gotypes.Instantiate(nil, origin, []gotypes.Type{gotypes.Typ[gotypes.Int]}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !nativeStorageGraphReady(instance, map[gotypes.Type]bool{}) {
		t.Fatal("complete generic graph reported unfinished")
	}
	// A by-value expanding declaration is invalid, unlike its pointer form.
	valueParameter := gotypes.NewTypeParam(gotypes.NewTypeName(0, nil, "T", nil), gotypes.NewInterfaceType(nil, nil).Complete())
	valueOrigin := gotypes.NewNamed(gotypes.NewTypeName(0, nil, "Value", nil), nil, nil)
	valueOrigin.SetTypeParams([]*gotypes.TypeParam{valueParameter})
	valueChild, err := gotypes.Instantiate(nil, valueOrigin, []gotypes.Type{gotypes.NewSlice(valueParameter)}, false)
	if err != nil {
		t.Fatal(err)
	}
	valueOrigin.SetUnderlying(gotypes.NewStruct([]*gotypes.Var{gotypes.NewField(0, nil, "Next", valueChild, false)}, nil))
	valueInstance, err := gotypes.Instantiate(nil, valueOrigin, []gotypes.Type{gotypes.Typ[gotypes.Int]}, false)
	if err != nil {
		t.Fatal(err)
	}
	if comparableStorageReady(valueInstance, map[gotypes.Type]bool{}) {
		t.Fatal("expanding by-value storage reported safe for comparability")
	}
}
