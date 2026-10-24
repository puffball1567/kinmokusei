package ast

// ObservationBuiltins expose scalar observations, never reflect.Value/Type or
// aliases into the inspected value. Shared by checking and editor tooling.
type ObservationBuiltin struct {
	Name   string
	Kind   BuiltinCallKind
	Arity  int
	Result string
}

func ObservationBuiltins() []ObservationBuiltin {
	return []ObservationBuiltin{
		{"inspectTypeName", InspectTypeNameCall, 1, "bstring"},
		{"inspectSameType", InspectSameTypeCall, 2, "boolean"},
		{"inspectKind", InspectKindCall, 1, "bstring"},
		{"inspectNil", InspectNilCall, 1, "boolean"},
		{"inspectComparable", InspectComparableCall, 1, "boolean"},
		{"inspectEqual", InspectEqualCall, 2, "boolean"},
		{"inspectFloat", InspectFloatCall, 1, "float"},
		{"inspectString", InspectStringCall, 1, "bstring"},
	}
}
