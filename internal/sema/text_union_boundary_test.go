package sema

import (
	"strings"
	"testing"
)

func TestUTF8UnionGoInterfaceBoundary(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, declarations, bound string
		unsafe                    bool
	}{
		{"scalar union", "", "~int | ~string | ~boolean", false},
		{"single string", "", "~string", false},
		{"raw scalar union", "", "~int | ~bstring | ~boolean", false},
		{"exact string", "", "int | string | boolean", false},
		{"named text", "type Text=distinct string;", "Text | ~int", false},
		{"named union", "constraint Scalar=~string|~int;", "Scalar | ~boolean", false},
		{"intersection", "constraint Scalar=~string|~int|~boolean;", "Scalar & comparable", false},
		{"value array", "", "~[2]string | ~int", false},
		{"value record", "alias Row={text:string};", "Row | ~int", false},
		{"raw reference", "", "~bstring[] | ~string", false},
		{"open parameter", "", "", true},
		{"comparable parameter", "", "comparable", true},
		{"slice", "", "~string[] | ~int", true},
		{"pointer", "", "~*string | ~int", true},
		{"map", "", "~Map<int,string> | ~int", true},
		{"channel", "", "~GoChannel<string> | ~int", true},
		{"callback", "alias Callback=()=>string;", "Callback | ~int", true},
		{"class", "class Row{public text:string=\"\";}", "Row | ~int", true},
		{"nested shared array", "", "~[2]string[] | ~int", true},
		{"nested shared record", "alias Row={text:string[]};", "Row | ~int", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			bound := ""
			if test.bound != "" {
				bound = " extends Bound"
				test.declarations += "constraint Bound=" + test.bound + ";"
			}
			diagnostics := checkSource(t, `import go reflect from "reflect";`+test.declarations+
				`function same<T`+bound+`>(left:T,right:T):boolean{reflect.ValueOf(left);reflect.ValueOf(right);return true;}`)
			if !test.unsafe {
				if len(diagnostics) != 0 {
					t.Fatalf("safe values rejected: %v", diagnostics)
				}
				return
			}
			if len(diagnostics) != 2 {
				t.Fatalf("want two boundary errors, got %v", diagnostics)
			}
			for _, message := range diagnostics {
				if !strings.Contains(message, "cannot use T as any") {
					t.Fatalf("wrong diagnostic: %v", diagnostics)
				}
			}
		})
	}
}
