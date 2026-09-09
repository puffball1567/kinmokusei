package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Keep values separate from Type: inference needs both constant kind and exact
// representability, while ordinary runtime arguments carry only their type.
func (c *Checker) genericNumericArguments(arguments []ast.Expression, actuals []Type) []gotypes.TypeAndValue {
	values := make([]gotypes.TypeAndValue, len(arguments))
	for i, argument := range arguments {
		if !actuals[i].IsNumeric() {
			continue
		}
		if info, ok := c.checkedNumericConstant(argument, actuals[i]); ok {
			values[i] = info
			if basic, ok := info.Type.(*gotypes.Basic); ok && basic.Info()&gotypes.IsUntyped != 0 {
				actuals[i] = Type{Kind: GoBasic, Name: basic.Name(), GoType: basic}
				if basic.Kind() == gotypes.UntypedInt {
					actuals[i] = Type{Kind: UntypedInt, Name: "integer literal"}
				}
			}
		}
	}
	return values
}

func untypedNumericRank(info gotypes.TypeAndValue) int {
	basic, ok := info.Type.(*gotypes.Basic)
	if !ok || info.Value == nil || basic.Info()&gotypes.IsUntyped == 0 {
		return 0
	}
	switch {
	case basic.Info()&gotypes.IsComplex != 0:
		return 4
	case basic.Info()&gotypes.IsFloat != 0:
		return 3
	case basic.Kind() == gotypes.UntypedRune:
		return 2
	case basic.Info()&gotypes.IsInteger != 0:
		return 1
	}
	return 0
}
