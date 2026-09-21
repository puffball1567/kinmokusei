package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

type arrowReturnValue struct {
	statement *ast.ReturnStmt
	typeInfo  Type
}

// Inference belongs to one callable. Nested arrows start a fresh control context
// so their returns cannot influence their enclosing function's result.
type arrowReturnInference struct {
	values []arrowReturnValue
	tries  []*ast.TryStmt
}

func (c *Checker) collectArrowReturn(statement *ast.ReturnStmt) {
	check := func(expression ast.Expression) Type {
		value := c.checkExpression(expression)
		// Inference validates storage after branch scopes have closed. Retain
		// constant facts now, including literals not otherwise cached.
		if info, known := c.scalarConstant(expression); known {
			if c.constantValues == nil {
				c.constantValues = map[ast.Expression]gotypes.TypeAndValue{}
			}
			c.constantValues[expression] = info
		}
		return value
	}
	value := builtins["void"]
	if statement.Value != nil {
		value = check(statement.Value)
		if len(statement.AdditionalValues) != 0 {
			values := []Type{c.singleValue(value, statement.Value.GetSpan())}
			for _, expression := range statement.AdditionalValues {
				values = append(values, c.singleValue(check(expression), expression.GetSpan()))
			}
			value = Type{Kind: MultiValue, Name: "multiple values", Results: values}
		} else if value.Kind != MultiValue {
			value = c.singleValue(value, statement.Value.GetSpan())
		}
	}
	c.arrowReturns.values = append(c.arrowReturns.values, arrowReturnValue{statement, value})
}

func (c *Checker) finishArrowReturnInference(inference *arrowReturnInference, arrow *ast.ArrowExpr) Type {
	result := builtins["void"]
	if len(inference.values) != 0 {
		result = defaultLiteralType(inference.values[0].typeInfo)
	}
	if result.Kind == MultiValue {
		result.Results = append([]Type(nil), result.Results...)
		for i, value := range result.Results {
			if value.Kind == Nil || value.Kind == Null || value.Kind == Void || value.Kind == Result || containsTaskType(value) {
				c.report(arrow.Span, fmt.Sprintf("cannot infer multiple result %d; add an explicit return annotation with an ordinary value type", i+1))
				result.Results[i] = Type{Kind: Invalid}
			} else {
				result.Results[i] = defaultLiteralType(value)
			}
		}
	}
	if result.Kind == Nil || result.Kind == Null || result.Kind == Result {
		c.report(arrow.Span, "cannot infer this arrow return type; add an explicit return annotation")
		result = Type{Kind: Invalid, Name: "<invalid>"}
	}
	for _, returned := range inference.values {
		if returned.statement.Value == nil {
			if result.Kind != Void && result.Kind != Invalid {
				c.report(returned.statement.Span, "cannot mix bare and value returns in an inferred arrow")
			}
			continue
		}
		if result.Kind == Void {
			c.report(returned.statement.Span, "cannot mix bare and value returns in an inferred arrow")
			continue
		}
		if result.Kind == MultiValue && len(returned.statement.AdditionalValues) != 0 {
			if len(result.Results) != len(returned.typeInfo.Results) {
				c.report(returned.statement.Span, "multiple result count mismatch between inferred returns")
				continue
			}
			check := func(slot *ast.Expression, i int) {
				c.requireAssignable(result.Results[i], returned.typeInfo.Results[i], (*slot).GetSpan())
				c.checkNumericMaterialization(*slot, result.Results[i])
				c.applyClassUpcast(slot, result.Results[i], returned.typeInfo.Results[i])
			}
			check(&returned.statement.Value, 0)
			for i := range returned.statement.AdditionalValues {
				check(&returned.statement.AdditionalValues[i], i+1)
			}
		} else {
			c.requireAssignable(result, returned.typeInfo, returned.statement.Value.GetSpan())
			c.checkNumericMaterialization(returned.statement.Value, result)
			c.applyClassUpcast(&returned.statement.Value, result, returned.typeInfo)
		}
	}
	c.prepareGoTypeForEmission(&result, arrow.Span)
	if result.Kind == MultiValue {
		for _, returned := range inference.values {
			if returned.statement.CrossesTry {
				returned.statement.ResultType = typeRefFromType(result, returned.statement.Span)
			}
		}
	}
	for _, statement := range inference.tries {
		statement.ReturnType = typeRefFromType(result, statement.Span)
	}
	return result
}
