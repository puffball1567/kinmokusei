package sema

import (
	"reflect"
	"testing"
)

func TestNativeContractPreservesErasedQualifiers(t *testing.T) {
	t.Parallel()
	verified, raw := builtins["string"], builtins["bstring"]
	item := Type{Kind: Class, Name: "Item"}
	cases := []struct {
		name        string
		left, right Type
	}{
		{"text", verified, raw},
		{"slice", Type{Kind: Array, Element: &verified}, Type{Kind: Array, Element: &raw}},
		{"pointer", Type{Kind: GoPointer, Element: &verified}, Type{Kind: GoPointer, Element: &raw}},
		{"nullable", item, Type{Kind: Nullable, Element: &item}},
		{"nominal arguments", Type{Kind: Struct, Name: "Cell", TypeArguments: []Type{verified}}, Type{Kind: Struct, Name: "Cell", TypeArguments: []Type{raw}}},
		{"nominal identity", Type{Kind: Struct, Name: "First"}, Type{Kind: Struct, Name: "Second"}},
		{"function parameters", Type{Kind: Function, Parameters: []Type{verified}, Result: &raw}, Type{Kind: Function, Parameters: []Type{raw}, Result: &raw}},
		{"function results", Type{Kind: Function, Parameters: []Type{raw}, Result: &verified}, Type{Kind: Function, Parameters: []Type{raw}, Result: &raw}},
		{"function slots", Type{Kind: Function, Parameters: []Type{verified}, Results: []Type{raw}}, Type{Kind: Function, Parameters: []Type{verified, raw}}},
		{"array length", Type{Kind: FixedArray, Length: 1, Element: &raw}, Type{Kind: FixedArray, Length: 2, Element: &raw}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if reflect.DeepEqual(nativeContract(test.left), nativeContract(test.right)) {
				t.Fatal("distinct source contracts collapsed")
			}
		})
	}
}

func TestNativeContractIsFiniteAndDeterministic(t *testing.T) {
	t.Parallel()
	left := Type{Kind: Object, Fields: map[string]Type{"a": builtins["string"], "b": builtins["bstring"]}}
	right := Type{Kind: Object, Fields: map[string]Type{"b": builtins["bstring"], "a": builtins["string"]}}
	if !reflect.DeepEqual(nativeContract(left), nativeContract(right)) {
		t.Fatal("map iteration changed contract identity")
	}
	recursive := Type{Kind: Struct, Name: "Node", TypeArguments: []Type{builtins["string"]}}
	recursive.Fields = map[string]Type{"next": {Kind: GoPointer, Element: &recursive}}
	contract := nativeContract(recursive)
	if len(contract.Parts) != 1 {
		t.Fatalf("nominal fields must not recursively expand: %#v", contract)
	}
}
