package codegen

import "testing"

func TestCFFICTypeUsesStandardCSpellings(t *testing.T) {
	for _, test := range []struct {
		name string
		info cffiScalar
		want string
	}{
		{"C unsigned int", cffiScalars["cUint32"], "unsigned int"},
		{"C signed int", cffiScalars["cInt32"], "int"},
		{"fixed width", cffiScalars["uint32"], "uint32_t"},
		{"pointer", cffiScalar{cgoType: "*C.char"}, "char *"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := cffiCType(test.info); got != test.want {
				t.Fatalf("cffiCType(%q) = %q, want %q", test.info.cgoType, got, test.want)
			}
		})
	}
}
