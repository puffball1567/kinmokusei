package sema

import gotypes "go/types"

func scalarBinaryOperation(operator string, left, right Type) bool {
	if left.IsString() && right.IsString() {
		switch operator {
		case "+", "==", "===", "!=", "!==", "<", "<=", ">", ">=":
			return true
		}
	}
	if left.IsBoolean() && right.IsBoolean() {
		switch operator {
		case "&&", "||", "==", "===", "!=", "!==":
			return true
		}
	}
	return false
}

func isScalarConstantType(value Type) bool {
	if value.Kind == Invalid || value.Kind == Nullable || value.Kind == TypeParameter {
		return false
	}
	return value.IsNumeric() || value.IsString() || value.IsBoolean()
}

// Keep ordinary string/boolean source kinds for their language operations,
// with Go's untyped status until storage or an explicit annotation fixes a type.
func preserveUntypedScalar(value Type, goType gotypes.Type) Type {
	if basic, ok := goType.(*gotypes.Basic); ok && (basic.Kind() == gotypes.UntypedString || basic.Kind() == gotypes.UntypedBool) {
		value.GoType = basic
	}
	return value
}
