package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

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
	value := builtins["void"]
	if statement.Value != nil {
		value = c.singleValue(c.checkExpression(statement.Value), statement.Value.GetSpan())
	}
	c.arrowReturns.values = append(c.arrowReturns.values, arrowReturnValue{statement, value})
}

func (c *Checker) finishArrowReturnInference(inference *arrowReturnInference, arrow *ast.ArrowExpr) Type {
	result := builtins["void"]
	if len(inference.values) != 0 {
		result = defaultLiteralType(inference.values[0].typeInfo)
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
		c.requireAssignable(result, returned.typeInfo, returned.statement.Value.GetSpan())
		c.checkNumericMaterialization(returned.statement.Value, result)
		c.applyClassUpcast(&returned.statement.Value, result, returned.typeInfo)
	}
	c.prepareGoTypeForEmission(&result, arrow.Span)
	for _, statement := range inference.tries {
		statement.ReturnType = typeRefFromType(result, statement.Span)
	}
	return result
}
