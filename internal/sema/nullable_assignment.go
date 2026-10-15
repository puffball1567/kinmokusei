package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

// A raw Go reference type describes storage, not a non-null contract. In
// particular, *T, error, and Go function values can all contain nil even though
// their type is not Nullable. Copying one into an explicitly nullable binding
// must not silently turn that binding into a non-null value. Inspect the actual
// storage rather than the import spelling, so aliases and instantiated named
// types follow the same rule. Explicit null guards still establish flow facts.
func (c *Checker) assignmentProvesNonNull(value Type, expression ast.Expression) bool {
	switch value.Kind {
	case Invalid, Nil, Null, Nullable:
		return false
	case GoPointer, GoInterface, Array, Map, GoChannel, Function, TypeParameter:
		// These types alone do not prove that the runtime value is non-nil.
	case GoNamed:
		if !isNilable(value) {
			return true
		}
	default:
		return true
	}
	switch expression := expression.(type) {
	case *ast.ArrowExpr, *ast.ArrayLiteralExpr, *ast.ObjectLiteralExpr, *ast.GoCompositeLiteralExpr, *ast.NewExpr:
		return true
	case *ast.UnaryExpr:
		return expression.Operator == "&"
	case *ast.CallExpr:
		return expression.Builtin == ast.MakeSliceCall || expression.Builtin == ast.MakeMapCall || expression.Builtin == ast.MakeGoChannelCall
	case *ast.IdentifierExpr:
		for index := len(c.scopes) - 1; index >= 0; index-- {
			if symbol, exists := c.scopes[index][expression.Name]; exists {
				// Reuse a checked nullable snapshot, not a raw Go variable's
				// spelling. Its current fact already includes escape analysis.
				return symbol.declaredType.Kind == Nullable && symbol.typeInfo.Kind != Nullable
			}
		}
		_, function := c.functions[expression.Name]
		return function
	case *ast.MemberExpr:
		key, stable := c.stableMemberFlowKey(expression)
		return stable && c.memberFlow[key].nonNull
	}
	return false
}
