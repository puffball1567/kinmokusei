package codegen

import (
	"reflect"
	"testing"
)

func TestCFFIGoParametersAvoidShadowingWithoutChangingManifest(t *testing.T) {
	bindings := cffiGoBindingNames(cffiManifest{
		Structs:   []cffiStruct{{Name: "Point"}},
		Functions: []cffiFunction{{Name: "Later"}},
	})
	parameters := []cffiParameter{
		{Name: "elementSize", Type: "borrowedArray", Element: "Point"},
		{Name: "kinmokuseiParameter0", Type: "int32"},
		{Name: "int32", Type: "int32"},
		{Name: "Point", Type: "Point"},
		{Name: "Later", Type: "int32"},
		{Name: "_", Type: "int32"},
		{Name: "ordinary", Type: "int32"},
	}
	original := append([]cffiParameter(nil), parameters...)
	want := []string{"kinmokuseiParameter0_", "kinmokuseiParameter1", "kinmokuseiParameter2", "kinmokuseiParameter3", "kinmokuseiParameter4", "kinmokuseiParameter5", "ordinary"}
	for i := 0; i < 2; i++ {
		got := cffiGoParameters(parameters, bindings, cffiFunctionLocals)
		for index, parameter := range got {
			if parameter.Name != want[index] || parameter.Type != original[index].Type || parameter.Element != original[index].Element || parameter.manifestName != original[index].Name {
				t.Fatalf("parameter %d: got %+v; want name %q with original type/element", index, parameter, want[index])
			}
		}
		if !reflect.DeepEqual(parameters, original) {
			t.Fatalf("manifest mutated: %+v", parameters)
		}
	}
}
