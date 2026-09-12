package sema

import (
	gotypes "go/types"
	"testing"
)

func TestCallbackContextTracksNestedDependencies(t *testing.T) {
	t.Parallel()
	parameter := gotypes.NewTypeParam(gotypes.NewTypeName(0, nil, "T", nil), gotypes.NewInterfaceType(nil, nil).Complete())
	unknown := Type{Kind: TypeParameter, Name: "T", GoType: parameter}
	for _, value := range []Type{
		unknown,
		{Kind: Array, Element: &unknown},
		{Kind: Map, Key: &unknown, Element: &Type{Kind: Int}},
		{Kind: Function, Parameters: []Type{unknown}},
		{Kind: Function, Result: &unknown},
		{Kind: MultiValue, Results: []Type{unknown}},
		{Kind: GoNamed, TypeArguments: []Type{unknown}},
		{Kind: Object, Fields: map[string]Type{"value": unknown}},
		{Kind: GoInterface, GoMethods: []GoInterfaceMethod{{Name: "Get", Type: Type{Kind: Function, Result: &unknown}}}},
	} {
		if !containsUnboundCallbackType(value, nativeTypeBindings{parameter: {}}) {
			t.Fatalf("missed dependency in kind %v", value.Kind)
		}
		if containsUnboundCallbackType(value, nativeTypeBindings{parameter: builtins["int"]}) || containsUnboundCallbackType(value, nil) {
			t.Fatalf("treated a bound or enclosing parameter as unresolved in kind %v", value.Kind)
		}
	}
}
