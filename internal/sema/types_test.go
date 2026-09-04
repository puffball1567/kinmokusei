package sema

import (
	gotypes "go/types"
	"testing"
)

func TestBuiltinTypeMatrix(t *testing.T) {
	tests := []struct {
		name       string
		kind       TypeKind
		numeric    bool
		comparable bool
	}{
		{"void", Void, false, false},
		{"boolean", Boolean, false, true},
		{"string", String, false, true},
		{"int", Int, true, true},
		{"int8", Int8, true, true},
		{"int16", Int16, true, true},
		{"int32", Int32, true, true},
		{"int64", Int64, true, true},
		{"uint", Uint, true, true},
		{"uint16", Uint16, true, true},
		{"uint32", Uint32, true, true},
		{"uint64", Uint64, true, true},
		{"float32", Float32, true, true},
		{"float64", Float64, true, true},
		{"byte", Byte, true, true},
		{"error", GoNamed, false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, ok := LookupType(test.name)
			if !ok || value.Kind != test.kind || value.IsNumeric() != test.numeric || value.IsComparable() != test.comparable {
				t.Fatalf("type = %#v, ok=%v", value, ok)
			}
		})
	}
	for _, alias := range []string{"float", "number"} {
		value, ok := LookupType(alias)
		if !ok || value.Kind != Float64 || value.Name != "float" {
			t.Errorf("alias %q = %#v, ok=%v", alias, value, ok)
		}
	}
	for alias, canonical := range map[string]string{"uint8": "byte"} {
		value, ok := LookupType(alias)
		want, _ := LookupType(canonical)
		if !ok || !sameType(value, want) || value.Name != want.Name {
			t.Errorf("alias %q = %#v, want %#v, ok=%v", alias, value, want, ok)
		}
	}
	if _, ok := LookupType("missing"); ok {
		t.Fatal("unknown built-in type was accepted")
	}
}

func TestGoFunctionMultipleResultType(t *testing.T) {
	results := gotypes.NewTuple(
		gotypes.NewVar(0, nil, "value", gotypes.Typ[gotypes.Int]),
		gotypes.NewVar(0, nil, "err", gotypes.Universe.Lookup("error").Type()),
	)
	signature := gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(), results, false)
	converted, err := kinmokuseiFunctionFromGo(signature)
	if err != nil {
		t.Fatal(err)
	}
	if converted.Kind != Function || converted.Result == nil || converted.Result.Kind != MultiValue || len(converted.Result.Results) != 2 {
		t.Fatalf("converted signature = %#v", converted)
	}
	if converted.Result.Results[0].Kind != Int || converted.Result.Results[1].Name != "error" || converted.Result.String() != "(int, error)" {
		t.Fatalf("multiple results = %#v (%s)", converted.Result.Results, converted.Result.String())
	}
	if assignable(builtins["int"], *converted.Result) || assignable(*converted.Result, *converted.Result) {
		t.Fatal("multiple results unexpectedly became an assignable first-class value")
	}
}

func TestGoInteropTypeConversionMatrix(t *testing.T) {
	stringType := gotypes.Typ[gotypes.String]
	intType := gotypes.Typ[gotypes.Int]
	anonymousStruct := gotypes.NewStruct([]*gotypes.Var{gotypes.NewField(0, nil, "Value", intType, false)}, []string{`json:"value"`})
	anonymousInterface := gotypes.NewInterfaceType(nil, nil).Complete()
	tests := []struct {
		name   string
		input  gotypes.Type
		kind   TypeKind
		failed bool
	}{
		{"boolean", gotypes.Typ[gotypes.Bool], Boolean, false},
		{"string", stringType, String, false},
		{"int", intType, Int, false},
		{"int8", gotypes.Typ[gotypes.Int8], Int8, false},
		{"int16", gotypes.Typ[gotypes.Int16], Int16, false},
		{"int32", gotypes.Typ[gotypes.Int32], Int32, false},
		{"int64", gotypes.Typ[gotypes.Int64], Int64, false},
		{"byte", gotypes.Typ[gotypes.Uint8], Byte, false},
		{"float32", gotypes.Typ[gotypes.Float32], Float32, false},
		{"float64", gotypes.Typ[gotypes.Float64], Float64, false},
		{"slice", gotypes.NewSlice(stringType), Array, false},
		{"fixed array", gotypes.NewArray(stringType, 3), FixedArray, false},
		{"map", gotypes.NewMap(stringType, intType), Map, false},
		{"unsigned integer", gotypes.Typ[gotypes.Uint], Uint, false},
		{"uint16", gotypes.Typ[gotypes.Uint16], Uint16, false},
		{"uint32", gotypes.Typ[gotypes.Uint32], Uint32, false},
		{"uint64", gotypes.Typ[gotypes.Uint64], Uint64, false},
		{"uintptr", gotypes.Typ[gotypes.Uintptr], GoBasic, false},
		{"complex64", gotypes.Typ[gotypes.Complex64], GoBasic, false},
		{"complex128", gotypes.Typ[gotypes.Complex128], GoBasic, false},
		{"pointer", gotypes.NewPointer(stringType), GoPointer, false},
		{"bidirectional channel", gotypes.NewChan(gotypes.SendRecv, intType), GoChannel, false},
		{"send-only channel", gotypes.NewChan(gotypes.SendOnly, intType), GoChannel, false},
		{"receive-only channel", gotypes.NewChan(gotypes.RecvOnly, intType), GoChannel, false},
		{"anonymous struct", anonymousStruct, GoStruct, false},
		{"anonymous interface", anonymousInterface, Invalid, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			converted, err := kinmokuseiTypeFromGo(test.input)
			if test.failed {
				if err == nil {
					t.Fatalf("conversion unexpectedly succeeded: %#v", converted)
				}
				return
			}
			if err != nil || converted.Kind != test.kind {
				t.Fatalf("converted=%#v err=%v, want kind %v", converted, err, test.kind)
			}
		})
	}
	converted, err := kinmokuseiTypeFromGo(anonymousStruct)
	if err != nil || len(converted.GoFields) != 1 || converted.GoFields[0].Name != "Value" || converted.GoFields[0].Tag != `json:"value"` || converted.GoFields[0].Type.Kind != Int {
		t.Fatalf("anonymous struct fields = %#v, err=%v", converted.GoFields, err)
	}
}

func TestGoNamedPointerAndNilAssignabilityMatrix(t *testing.T) {
	pkg := gotypes.NewPackage("example.com/types", "sample")
	durationName := gotypes.NewTypeName(0, pkg, "Duration", nil)
	durationGo := gotypes.NewNamed(durationName, gotypes.Typ[gotypes.Int64], nil)
	monthName := gotypes.NewTypeName(0, pkg, "Month", nil)
	monthGo := gotypes.NewNamed(monthName, gotypes.Typ[gotypes.Int64], nil)
	duration, err := kinmokuseiTypeFromGo(durationGo)
	if err != nil {
		t.Fatal(err)
	}
	month, err := kinmokuseiTypeFromGo(monthGo)
	if err != nil {
		t.Fatal(err)
	}
	pointer, err := kinmokuseiTypeFromGo(gotypes.NewPointer(durationGo))
	if err != nil {
		t.Fatal(err)
	}
	untyped := Type{Kind: UntypedInt, Name: "integer literal"}
	nilType := Type{Kind: Nil, Name: "nil"}
	tests := []struct {
		name          string
		target, value Type
		want          bool
	}{
		{"same named", duration, duration, true},
		{"different named", duration, month, false},
		{"typed integer to named", duration, builtins["int64"], false},
		{"untyped integer to named", duration, untyped, true},
		{"nil to pointer", pointer, nilType, true},
		{"nil to named integer", duration, nilType, false},
		{"pointer to named", pointer, duration, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := assignable(test.target, test.value); got != test.want {
				t.Fatalf("assignable(%s, %s) = %v, want %v", test.target, test.value, got, test.want)
			}
		})
	}
}

func TestNullableAssignabilityAndGoRepresentation(t *testing.T) {
	integer := builtins["int"]
	base := Type{Kind: GoPointer, Name: "*int", Element: &integer, GoType: gotypes.NewPointer(gotypes.Typ[gotypes.Int])}
	nullable := Type{Kind: Nullable, Name: "nullable", Element: &base}
	nullValue := Type{Kind: Null, Name: "null"}
	nilValue := Type{Kind: Nil, Name: "nil"}
	if !assignable(nullable, base) || !assignable(nullable, nullValue) {
		t.Fatal("nullable type did not accept its base or null")
	}
	if assignable(base, nullable) || assignable(nullable, nilValue) {
		t.Fatal("nullable type escaped into its base or accepted raw nil")
	}
	if got := nullable.String(); got != "*int | null" {
		t.Fatalf("nullable.String() = %q", got)
	}
	generated, ok := goTypeOf(nullable)
	if !ok || !gotypes.Identical(generated, base.GoType) {
		t.Fatalf("nullable Go type = %v, ok=%v", generated, ok)
	}
}

func TestGoTypeContainsUnsafePointerMatrix(t *testing.T) {
	unsafePointer := gotypes.Typ[gotypes.UnsafePointer]
	parameter := gotypes.NewParam(0, nil, "value", unsafePointer)
	result := gotypes.NewParam(0, nil, "result", gotypes.NewSlice(unsafePointer))
	signature := gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(parameter), gotypes.NewTuple(result), false)
	pkg := gotypes.NewPackage("example.com/unsafeapi", "unsafeapi")
	namedUnsafe := gotypes.NewNamed(gotypes.NewTypeName(0, pkg, "UnsafeAlias", nil), unsafePointer, nil)
	hiddenStruct := gotypes.NewNamed(gotypes.NewTypeName(0, pkg, "Hidden", nil), gotypes.NewStruct([]*gotypes.Var{gotypes.NewField(0, pkg, "hidden", unsafePointer, false)}, nil), nil)
	method := gotypes.NewFunc(0, pkg, "Use", signature)
	contract := gotypes.NewInterfaceType([]*gotypes.Func{method}, nil)
	contract.Complete()
	for _, test := range []struct {
		name     string
		typeInfo gotypes.Type
		want     bool
	}{
		{"safe basic", gotypes.Typ[gotypes.Int], false},
		{"unsafe basic", unsafePointer, true},
		{"pointer", gotypes.NewPointer(unsafePointer), true},
		{"slice", gotypes.NewSlice(unsafePointer), true},
		{"array", gotypes.NewArray(unsafePointer, 2), true},
		{"map key", gotypes.NewMap(unsafePointer, gotypes.Typ[gotypes.Int]), true},
		{"channel", gotypes.NewChan(gotypes.SendRecv, unsafePointer), true},
		{"signature", signature, true},
		{"named unsafe", namedUnsafe, true},
		{"named struct internals remain encapsulated", hiddenStruct, false},
		{"interface method", contract, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := goTypeContainsUnsafePointer(test.typeInfo, nil); got != test.want {
				t.Fatalf("containsUnsafe(%s)=%v, want %v", test.typeInfo, got, test.want)
			}
		})
	}
}

func TestAssignableCompositeTypeMatrix(t *testing.T) {
	integer := builtins["int"]
	text := builtins["string"]
	arrayOfInt := Type{Kind: Array, Name: "array", Element: &integer}
	arrayOfString := Type{Kind: Array, Name: "array", Element: &text}
	fixedTwoInts := Type{Kind: FixedArray, Name: "fixed array", Element: &integer, Length: 2}
	fixedThreeInts := Type{Kind: FixedArray, Name: "fixed array", Element: &integer, Length: 3}
	mapOfInt := Type{Kind: Map, Name: "Map", Key: &text, Element: &integer}
	result := integer
	intFunction := Type{Kind: Function, Parameters: []Type{integer}, Result: &result}
	variadicIntFunction := Type{Kind: Function, Parameters: []Type{integer}, Variadic: true, Result: &result}
	stringResult := text
	stringFunction := Type{Kind: Function, Parameters: []Type{integer}, Result: &stringResult}
	tests := []struct {
		name          string
		target, value Type
		want          bool
	}{
		{"same primitive", integer, integer, true},
		{"untyped integer", integer, Type{Kind: UntypedInt, Name: "integer literal"}, true},
		{"different primitive", integer, text, false},
		{"same array", arrayOfInt, arrayOfInt, true},
		{"different array", arrayOfInt, arrayOfString, false},
		{"same fixed array", fixedTwoInts, fixedTwoInts, true},
		{"different fixed array length", fixedTwoInts, fixedThreeInts, false},
		{"fixed array differs from slice", fixedTwoInts, arrayOfInt, false},
		{"same map", mapOfInt, mapOfInt, true},
		{"different map", mapOfInt, Type{Kind: Map, Key: &integer, Element: &integer}, false},
		{"same function", intFunction, intFunction, true},
		{"variadic function identity", variadicIntFunction, variadicIntFunction, true},
		{"variadic differs from fixed function", variadicIntFunction, intFunction, false},
		{"different function result", intFunction, stringFunction, false},
		{"same class", Type{Kind: Class, Name: "A"}, Type{Kind: Class, Name: "A"}, true},
		{"different class", Type{Kind: Class, Name: "A"}, Type{Kind: Class, Name: "B"}, false},
		{"same interface", Type{Kind: Interface, Name: "A"}, Type{Kind: Interface, Name: "A"}, true},
		{"different interface", Type{Kind: Interface, Name: "A"}, Type{Kind: Interface, Name: "B"}, false},
		{"invalid recovery", integer, Type{Kind: Invalid}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := assignable(test.target, test.value); got != test.want {
				t.Fatalf("assignable(%s, %s) = %v, want %v", test.target.String(), test.value.String(), got, test.want)
			}
		})
	}
}

func TestFixedArrayComparabilityMatrix(t *testing.T) {
	integer := builtins["int"]
	slice := Type{Kind: Array, Name: "array", Element: &integer}
	tests := []struct {
		name       string
		element    Type
		comparable bool
	}{
		{"primitive element", integer, true},
		{"slice element", slice, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			array := Type{Kind: FixedArray, Name: "fixed array", Element: &test.element, Length: 2}
			if got := array.IsComparable(); got != test.comparable {
				t.Fatalf("IsComparable() = %v, want %v", got, test.comparable)
			}
		})
	}
}

func TestVariadicFunctionTypeDisplay(t *testing.T) {
	integer := builtins["int"]
	text := builtins["string"]
	function := Type{Kind: Function, Parameters: []Type{text, integer}, Variadic: true, Result: &text}
	if got, want := function.String(), "(string, ...int) => string"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestGoTypeParameterOperatorCapabilitiesFollowCompleteTypeSet(t *testing.T) {
	constraint := func(types ...gotypes.Type) gotypes.Type {
		terms := make([]*gotypes.Term, len(types))
		for index, item := range types {
			terms[index] = gotypes.NewTerm(true, item)
		}
		contract := gotypes.NewInterfaceType(nil, []gotypes.Type{gotypes.NewUnion(terms)})
		contract.Complete()
		return contract
	}
	intersection := func(left, right gotypes.Type) gotypes.Type {
		contract := gotypes.NewInterfaceType(nil, []gotypes.Type{left, right})
		contract.Complete()
		return contract
	}
	ordered := constraint(gotypes.Typ[gotypes.Int], gotypes.Typ[gotypes.String])
	integer := constraint(gotypes.Typ[gotypes.Int], gotypes.Typ[gotypes.Uint8])
	text := constraint(gotypes.Typ[gotypes.String])
	boolean := constraint(gotypes.Typ[gotypes.Bool])
	slice := constraint(gotypes.NewSlice(gotypes.Typ[gotypes.Int]))
	narrowed := intersection(ordered, constraint(gotypes.Typ[gotypes.Int], gotypes.Typ[gotypes.Bool]))
	any := gotypes.NewInterfaceType(nil, nil)
	any.Complete()
	tests := []struct {
		name                                     string
		constraint                               gotypes.Type
		numeric, integer, ordered, text, boolean bool
		addable                                  bool
	}{
		{"integer union", integer, true, true, true, false, false, true},
		{"mixed ordered union", ordered, false, false, true, false, false, true},
		{"string", text, false, false, true, true, false, true},
		{"boolean", boolean, false, false, false, false, true, false},
		{"slice", slice, false, false, false, false, false, false},
		{"intersection narrows to integer", narrowed, true, true, true, false, false, true},
		{"unconstrained", any, false, false, false, false, false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parameter := gotypes.NewTypeParam(gotypes.NewTypeName(0, nil, "T", nil), test.constraint)
			value := Type{Kind: TypeParameter, Name: "T", GoType: parameter}
			if value.IsNumeric() != test.numeric || value.IsInteger() != test.integer || value.IsOrdered() != test.ordered || value.IsString() != test.text || value.IsBoolean() != test.boolean || value.IsAddable() != test.addable {
				t.Fatalf("capabilities numeric=%v integer=%v ordered=%v string=%v boolean=%v addable=%v", value.IsNumeric(), value.IsInteger(), value.IsOrdered(), value.IsString(), value.IsBoolean(), value.IsAddable())
			}
		})
	}
}
