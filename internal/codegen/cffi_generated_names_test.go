package codegen

import (
	"encoding/json"
	"strings"
	"testing"
)

func cffiNamesFixture() cffiManifest {
	return cffiManifest{SchemaVersion: 1, Package: "fixture", Header: "fixture.h", ThreadPolicy: "threadSafe", Functions: []cffiFunction{{Name: "Run", Symbol: "fixture_run", Result: "void", Convention: "direct"}}}
}

func cffiReleaseNameFixture(result string) cffiManifest {
	m := cffiNamesFixture()
	owned := cffiCallback{Name: "Data", Lifetime: "registered", Result: result}
	if result == "ownedArray" {
		owned.ResultElement = "int32"
	}
	m.Callbacks = []cffiCallback{owned, {Name: "Data_release", Lifetime: "callScoped", Result: "void"}}
	m.CallbackRegistrations = []cffiCallbackRegistration{{Name: "Watch", Callback: "Data", Register: "fixture_register", Unregister: "fixture_unregister"}}
	m.Functions[0].Parameters = []cffiParameter{{Name: "callback", Type: "Data_release"}}
	return m
}

func TestCFFIRejectsGeneratedNameCollisions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                      string
		manifest                  cffiManifest
		identifier, first, second string
	}{}
	for _, result := range []string{"ownedBytes", "ownedCString", "ownedArray"} {
		tests = append(tests, struct {
			name                      string
			manifest                  cffiManifest
			identifier, first, second string
		}{
			result + " release", cffiReleaseNameFixture(result), "kinmokusei_cffi_callback_Data_release_fn", `callback "Data"`, `callback "Data_release"`,
		})
	}
	for _, prefix := range []string{"From", "To"} {
		m := cffiNamesFixture()
		m.Structs = []cffiStruct{{Name: "PointState", CType: "fixture_point", Fields: []cffiField{{Name: "X", CName: "x", Type: "int32"}}}}
		m.Callbacks = []cffiCallback{{Name: prefix + "Point", Lifetime: "callScoped", Result: "void"}}
		m.Functions[0].Parameters = []cffiParameter{{Name: "callback", Type: prefix + "Point"}}
		tests = append(tests, struct {
			name                      string
			manifest                  cffiManifest
			identifier, first, second string
		}{
			prefix + " struct state", m, "kinmokuseiCFFI" + prefix + "PointState", `struct "PointState"`, "callback " + `"` + prefix + `Point"`,
		})
	}
	union := func(name, ctype, variant string) cffiTaggedUnion {
		return cffiTaggedUnion{Name: name, CType: ctype, Tag: cffiField{Name: "Kind", CName: "kind", Type: "int32"}, Variants: []cffiUnionVariant{{Name: variant, CName: "item", Type: "int32", Tags: []string{"FIXTURE_NUMBER"}}}}
	}
	m := cffiNamesFixture()
	m.TaggedUnions = []cffiTaggedUnion{union("PointState", "fixture_point", "Value")}
	m.Callbacks = []cffiCallback{{Name: "FromPoint", Lifetime: "callScoped", Result: "void"}}
	m.Functions[0].Parameters = []cffiParameter{{Name: "callback", Type: "FromPoint"}}
	tests = append(tests, struct {
		name                      string
		manifest                  cffiManifest
		identifier, first, second string
	}{"union state", m, "kinmokuseiCFFIFromPointState", `tagged union "PointState"`, `callback "FromPoint"`})
	m = cffiNamesFixture()
	m.ThreadPolicy = "threadAffine"
	m.Callbacks = []cffiCallback{{Name: "RawRun", Lifetime: "callScoped", Result: "void"}}
	m.Functions[0].Name = "RunState"
	m.Functions[0].Parameters = []cffiParameter{{Name: "callback", Type: "RawRun"}}
	tests = append(tests, struct {
		name                      string
		manifest                  cffiManifest
		identifier, first, second string
	}{"affine raw function", m, "kinmokuseiCFFIRawRunState", `function "RunState"`, `callback "RawRun"`})
	m = cffiNamesFixture()
	m.Callbacks = []cffiCallback{{Name: "Data", Lifetime: "registered", Result: "void"}, {Name: "RawRegisterWatch", Lifetime: "callScoped", Result: "void"}}
	m.CallbackRegistrations = []cffiCallbackRegistration{{Name: "WatchState", Callback: "Data", Register: "fixture_register", Unregister: "fixture_unregister"}}
	tests = append(tests, struct {
		name                      string
		manifest                  cffiManifest
		identifier, first, second string
	}{"raw registration", m, "kinmokuseiCFFIRawRegisterWatchState", `registration "WatchState"`, `callback "RawRegisterWatch"`})
	m = cffiNamesFixture()
	m.TaggedUnions = []cffiTaggedUnion{union("Value", "fixture_value", "Item_get_X"), union("Value_get_Item", "fixture_item", "X")}
	tests = append(tests, struct {
		name                      string
		manifest                  cffiManifest
		identifier, first, second string
	}{"union member boundary", m, "kinmokusei_cffi_Value_get_Item_get_X", `tagged union "Value"`, `tagged union "Value_get_Item"`})
	m = cffiNamesFixture()
	m.Structs = []cffiStruct{{Name: "Point", CType: "kinmokusei_cffi_callback_Read_fn", Fields: []cffiField{{Name: "X", CName: "x", Type: "int32"}}}}
	m.Callbacks = []cffiCallback{{Name: "Read", Lifetime: "callScoped", Result: "void"}}
	m.Functions[0].Parameters = []cffiParameter{{Name: "callback", Type: "Read"}}
	tests = append(tests, struct {
		name                      string
		manifest                  cffiManifest
		identifier, first, second string
	}{"native type and generated callback", m, "kinmokusei_cffi_callback_Read_fn", `struct "Point"`, `callback "Read"`})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.manifest)
			if err != nil {
				t.Fatal(err)
			}
			output, err := GenerateCFFI(data)
			if err == nil {
				t.Fatal("colliding manifest was accepted")
			}
			message := err.Error()
			for _, fragment := range []string{"generated", test.identifier, test.first, test.second} {
				if !strings.Contains(message, fragment) {
					t.Fatalf("error %q does not identify %q", message, fragment)
				}
			}
			if len(output.Source) != 0 || output.Package != "" {
				t.Fatal("invalid manifest emitted output")
			}
			_, again := GenerateCFFI(data)
			if again == nil || again.Error() != message {
				t.Fatalf("nondeterministic diagnostic: %v / %v", err, again)
			}
		})
	}
}

func TestCFFISimilarNamesWithoutGeneratedCollision(t *testing.T) {
	t.Parallel()
	for _, policy := range []string{"threadSafe", "serialized", "threadAffine", "mainThread"} {
		t.Run(policy, func(t *testing.T) {
			m := cffiReleaseNameFixture("void")
			m.ThreadPolicy = policy
			m.Structs = []cffiStruct{{Name: "Point", CType: "fixture_point", Fields: []cffiField{{Name: "X", CName: "x", Type: "int32"}}}}
			m.Callbacks = append(m.Callbacks, cffiCallback{Name: "FromPoint", Lifetime: "callScoped", Result: "void"})
			m.Functions = append(m.Functions, cffiFunction{Name: "RunAgain", Symbol: "fixture_run", Parameters: m.Functions[0].Parameters, Result: "void", Convention: "direct"})
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := GenerateCFFI(data); err != nil {
				t.Fatalf("valid neighboring names rejected: %v", err)
			}
		})
	}
}

func TestCFFIConditionalRuntimeNameCollisions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ symbol, parameter, element, policy string }{
		{"kinmokusei_cffi_free_string", "cstring", "", "threadSafe"},
		{"kinmokusei_cffi_free_bytes", "borrowedBytes", "", "threadSafe"},
		{"kinmokusei_cffi_alloc_array", "borrowedArray", "int32", "threadSafe"},
		{"kinmokusei_c_int_must_be_32_bits", "cInt32", "", "threadSafe"},
		{"kinmokusei_c_uint_must_be_32_bits", "cUint32", "", "threadSafe"},
		{"kinmokusei_cffi_mark_main_thread", "", "", "mainThread"},
	} {
		t.Run(test.symbol, func(t *testing.T) {
			m := cffiNamesFixture()
			m.ThreadPolicy = test.policy
			m.Functions[0].Symbol = test.symbol
			if test.parameter != "" {
				m.Functions[0].Parameters = []cffiParameter{{Name: "value", Type: test.parameter, Element: test.element}}
			}
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := GenerateCFFI(data); err == nil || !strings.Contains(err.Error(), `generated C identifier "`+test.symbol+`"`) || !strings.Contains(err.Error(), `function "Run"`) {
				t.Fatalf("missing runtime helper collision: %v", err)
			}
			// The same native name is valid when that helper is not emitted.
			m.ThreadPolicy = "threadSafe"
			m.Functions[0].Parameters = nil
			data, err = json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := GenerateCFFI(data); err != nil {
				t.Fatalf("unused conditional helper name was reserved: %v", err)
			}
		})
	}
}

func TestCFFISeparateCAndGoIdentifierScopes(t *testing.T) {
	t.Parallel()
	m := cffiNamesFixture()
	m.Structs = []cffiStruct{{Name: "Point", CType: "kinmokuseiCFFIFromPoint", Fields: []cffiField{{Name: "X", CName: "x", Type: "int32"}}}}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateCFFI(data); err != nil {
		t.Fatalf("C type and Go helper occupy separate namespaces: %v", err)
	}
}
