package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// These are compiler-owned scalar observations, not a permission to erase the
// argument's contracts at arbitrary Go boundaries. Nested expressions retain
// their ordinary checks, and no reflective capability is returned.
func (c *Checker) checkObservationBuiltin(expr *ast.CallExpr, definition ast.ObservationBuiltin) Type {
	c.checkObservationShape(expr, definition.Name, definition.Arity)
	parameters := make([]string, definition.Arity)
	for i, argument := range expr.Arguments {
		value := c.checkObservationOperand(argument)
		if i < len(parameters) {
			parameters[i] = value.String()
		}
	}
	c.usesObservation = true
	expr.Builtin = definition.Kind
	names := []string{"value"}
	if definition.Arity == 2 {
		names = []string{"a", "b"}
	}
	expr.Signature = &ast.CallableSignature{ParameterNames: names, ParameterTypes: parameters, Result: definition.Result}
	return builtins[definition.Result]
}

func (c *Checker) checkObservationShape(expr *ast.CallExpr, name string, arity int) {
	if expr.Expanded {
		c.report(expr.Span, name+" does not accept spread arguments")
	}
	if len(expr.TypeArguments) != 0 {
		c.report(expr.Span, name+" does not accept explicit type arguments")
	}
	if len(expr.Arguments) != arity {
		c.report(expr.Span, fmt.Sprintf("%s expects %d arguments, got %d", name, arity, len(expr.Arguments)))
	}
}

func (c *Checker) checkObservationOperand(argument ast.Expression) Type {
	value := defaultLiteralType(c.singleValue(c.checkExpression(argument), argument.GetSpan()))
	c.checkNumericMaterialization(argument, value)
	if containsTaskType(value) {
		c.report(argument.GetSpan(), "an observation cannot consume or conceal a Task; await it first")
		return value
	}
	switch value.Kind {
	case Invalid:
		return value
	case Nil, Null:
		return value // A nil interface has well-defined scalar observations.
	case Void, Result, MultiValue, GoPackage, GoTypeName:
		c.report(argument.GetSpan(), fmt.Sprintf("cannot inspect %s as a value", value.String()))
		return value
	}
	storage, ok := c.goTypeForNativeStorage(value)
	if !ok || !nativeStorageGraphReady(storage, map[gotypes.Type]bool{}) {
		c.report(argument.GetSpan(), fmt.Sprintf("cannot inspect unresolved storage for %s", value.String()))
	}
	return value
}

func (c *Checker) checkJSONSnapshotBuiltin(expr *ast.CallExpr) Type {
	// Indentation is explicit when supplied, matching MarshalIndent. The error
	// bridge removes reflect.Value/Type and custom-error aliases before return.
	arity := 1
	if len(expr.Arguments) > 1 {
		arity = 3
	}
	c.checkObservationShape(expr, "jsonSnapshot", arity)
	parameters := make([]string, len(expr.Arguments))
	for i, argument := range expr.Arguments {
		if i == 0 {
			parameters[i] = c.checkObservationOperand(argument).String()
		} else {
			actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], builtins["bstring"])
			c.requireAssignable(builtins["bstring"], actual, argument.GetSpan())
			parameters[i] = "bstring"
		}
	}
	c.usesJSONSnapshot = true
	expr.Builtin = ast.JSONSnapshotCall
	element := builtins["byte"]
	bytes := Type{Kind: Array, Name: "array", Element: &element}
	result := Type{Kind: Result, Name: "Result", Element: &bytes}
	expr.Signature = &ast.CallableSignature{ParameterNames: []string{"value", "prefix", "indent"}, ParameterTypes: parameters, Result: result.String()}
	// Marshaler/MarshalText/IsZero and Error callbacks may mutate checked
	// application state. Unlike scalar observations, this call is not pure.
	c.recordMemberWrite(expr.Span)
	c.invalidateAllMemberFacts(expr.Span, "JSON snapshot callbacks may mutate storage")
	return result
}
