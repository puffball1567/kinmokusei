package sema

import (
	"go/constant"
	gotypes "go/types"
	"strings"
	"testing"
)

func TestAssessGoInteropObjectMatrix(t *testing.T) {
	packageInfo := gotypes.NewPackage("example.com/audit", "audit")
	integer := gotypes.Typ[gotypes.Int]
	unsafePointer := gotypes.Typ[gotypes.UnsafePointer]
	anonymousStruct := gotypes.NewStruct([]*gotypes.Var{gotypes.NewField(0, packageInfo, "Value", integer, false)}, nil)
	method := gotypes.NewFunc(0, packageInfo, "Read", gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(), gotypes.NewTuple(), false))
	anonymousInterface := gotypes.NewInterfaceType([]*gotypes.Func{method}, nil).Complete()
	structWithInterface := gotypes.NewStruct([]*gotypes.Var{gotypes.NewField(0, packageInfo, "Contract", anonymousInterface, false)}, nil)
	genericParameter := gotypes.NewTypeParam(gotypes.NewTypeName(0, packageInfo, "T", nil), gotypes.Universe.Lookup("comparable").Type())

	makeFunction := func(name string, parameters []gotypes.Type, results []gotypes.Type, typeParameters []*gotypes.TypeParam) *gotypes.Func {
		params := make([]*gotypes.Var, len(parameters))
		for index, parameter := range parameters {
			params[index] = gotypes.NewVar(0, packageInfo, "", parameter)
		}
		returns := make([]*gotypes.Var, len(results))
		for index, result := range results {
			returns[index] = gotypes.NewVar(0, packageInfo, "", result)
		}
		signature := gotypes.NewSignatureType(nil, nil, typeParameters, gotypes.NewTuple(params...), gotypes.NewTuple(returns...), false)
		return gotypes.NewFunc(0, packageInfo, name, signature)
	}

	tests := []struct {
		name       string
		object     gotypes.Object
		support    GoInteropSupport
		reasonText string
	}{
		{"basic function", makeFunction("Value", []gotypes.Type{integer}, []gotypes.Type{integer}, nil), GoInteropSupported, ""},
		{"multiple result", makeFunction("Pair", nil, []gotypes.Type{integer, gotypes.Typ[gotypes.String]}, nil), GoInteropSupported, ""},
		{"generic", makeFunction("Generic", []gotypes.Type{genericParameter}, []gotypes.Type{genericParameter}, []*gotypes.TypeParam{genericParameter}), GoInteropSupported, ""},
		{"unsafe", makeFunction("Unsafe", []gotypes.Type{unsafePointer}, nil, nil), GoInteropRequiresUnsafe, "unsafe.Pointer"},
		{"anonymous struct", makeFunction("Struct", []gotypes.Type{anonymousStruct}, nil, nil), GoInteropSupported, ""},
		{"anonymous struct with unsupported field", makeFunction("StructInterface", []gotypes.Type{structWithInterface}, nil, nil), GoInteropUnsupported, "anonymous Go interface"},
		{"anonymous interface", makeFunction("Interface", nil, []gotypes.Type{anonymousInterface}, nil), GoInteropUnsupported, "anonymous Go interface"},
		{"variable slice", gotypes.NewVar(0, packageInfo, "Items", gotypes.NewSlice(integer)), GoInteropSupported, ""},
		{"constant", gotypes.NewConst(0, packageInfo, "Answer", integer, constant.MakeInt64(42)), GoInteropSupported, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assessment := AssessGoInteropObject(test.object)
			if assessment.Support != test.support || !strings.Contains(assessment.Reason, test.reasonText) {
				t.Fatalf("assessment=%#v want support=%q reason containing %q", assessment, test.support, test.reasonText)
			}
		})
	}
}
