package compiler

import (
	"strings"
	"testing"
)

func TestIncomingCFFIRetainedArrayValidation(t *testing.T) {
	t.Parallel()
	const prefix = `{"schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"threadSafe",`
	const types = `"enums":[{"name":"Mode","cType":"fixture_mode","underlying":"uint16"}],"structs":[{"name":"Point","cType":"fixture_point","fields":[{"name":"X","cName":"x","type":"float32"}]}],"handles":[{"name":"Image","cType":"fixture_image","release":"image_free"}],`
	const callback = `"callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"void"}],`
	registration := func(parameter string) string {
		return `"callbackRegistrations":[{"name":"Watch","callback":"Visit","register":"watch_add","unregister":"watch_remove","parameters":[` + parameter + `]}]`
	}
	for _, element := range []string{"int8", "int16", "int32", "int64", "byte", "uint16", "uint32", "uint64", "float32", "float64", "cInt32", "cUint32", "boolean", "Mode", "Point"} {
		t.Run("accept/"+element, func(t *testing.T) {
			manifest := prefix + types + callback + registration(`{"name":"values","type":"retainedArray","element":"`+element+`"}`) + `}`
			if _, err := GenerateCFFI([]byte(manifest)); err != nil {
				t.Fatalf("retained %s array: %v", element, err)
			}
		})
	}
	for _, element := range []string{"", "Unknown", "cstring", "void", "Image", "Visit", "borrowedArray", "retainedArray", "ownedBytes", "int32[]"} {
		t.Run("reject/"+element, func(t *testing.T) {
			manifest := prefix + types + callback + registration(`{"name":"values","type":"retainedArray","element":"`+element+`"}`) + `}`
			if _, err := GenerateCFFI([]byte(manifest)); err == nil || !strings.Contains(err.Error(), "requires a supported element") {
				t.Fatalf("retained %s array error = %v", element, err)
			}
		})
	}
	for _, test := range []struct{ name, body, want string }{
		{"unrelated element", callback + registration(`{"name":"data","type":"retainedBytes","element":"int32"}`), "may not declare element"},
		{"ordinary parameter", `"functions":[{"name":"Value","symbol":"value","parameters":[{"name":"values","type":"retainedArray","element":"int32"}],"result":"void","convention":"direct"}]`, "only for borrowedArray"},
		{"ordinary result", `"functions":[{"name":"Value","symbol":"value","parameters":[],"result":"retainedArray","convention":"direct"}]`, "unsupported result type"},
		{"callback parameter", `"callbacks":[{"name":"Visit","lifetime":"registered","parameters":[{"name":"values","type":"retainedArray","element":"int32"}],"result":"void"}],` + registration(``), "may not declare element"},
		{"callback result", `"callbacks":[{"name":"Visit","lifetime":"registered","parameters":[],"result":"retainedArray","resultElement":"int32"}],` + registration(``), "only for ownedArray"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := GenerateCFFI([]byte(prefix + types + test.body + `}`)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
