package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkFunction(decl *ast.FunctionDecl) {
	previousDependency := c.globalDependencyOwner
	c.globalDependencyOwner = decl.Name
	defer func() { c.globalDependencyOwner = previousDependency }()
	c.validateLabels(decl.Body)
	previousFlow := c.enterCallableFlow()
	defer c.leaveCallableFlow(previousFlow)
	c.pushTypeParameterScope(c.functionTypeParameters[decl])
	defer c.popTypeParameterScope()
	c.pushScope()
	previousControl := c.enterCallableControl()
	for _, param := range decl.Parameters {
		t := c.resolveType(param.Type)
		c.rejectResultValueType(t, param.Type.Span, "parameters")
		c.declareLocal(param.Name, t, false, nil, param.Span)
	}
	c.result = c.resolveType(decl.ReturnType)
	c.checkBlock(decl.Body, false)
	if c.result.Kind != Void && !definitelyReturns(decl.Body) {
		c.report(decl.Span, fmt.Sprintf("function %q may complete without returning %s", decl.Name, c.result.String()))
	}
	c.popScope()
	c.callableControlState = previousControl
}

func (c *Checker) checkArrow(expr *ast.ArrowExpr) Type {
	return c.checkArrowExpected(expr, Type{})
}

func (c *Checker) checkArrowExpected(expr *ast.ArrowExpr, expected Type) Type {
	if c.inFieldInitializer {
		c.fieldInitializerArrowDepth++
		defer func() { c.fieldInitializerArrowDepth-- }()
	}
	expected = c.arrowContext(expected)
	inferredParameters := c.inferArrowParameters(expr, expected)
	// Returns and super-constructor calls belong to the current callable, even
	// when an arrow captures this from its enclosing constructor.
	previousInConstructor := c.inConstructor
	c.inConstructor = false
	defer func() { c.inConstructor = previousInConstructor }()
	lexicalContext := c.enterClosureLexical()
	parameters := make([]Type, len(expr.Parameters))
	for i, parameter := range expr.Parameters {
		if inferred, ok := inferredParameters[i]; ok {
			// Context already carries type identity, including private dependency
			// types that the caller cannot name explicitly.
			parameters[i] = inferred
		} else {
			parameters[i] = c.resolveType(parameter.Type)
		}
		if parameters[i].Kind == Void {
			c.report(parameter.Type.Span, "parameters cannot have type void")
		}
		c.rejectResultValueType(parameters[i], parameter.Type.Span, "parameters")
		c.rejectTaskAPIType(parameters[i], parameter.Type.Span, "arrow parameters")
		c.declareLocal(parameter.Name, parameters[i], false, nil, parameter.Span)
	}
	previousControl := c.enterCallableControl()
	var result Type
	if expr.ReturnType != nil {
		result = c.resolveType(*expr.ReturnType)
		c.rejectTaskAPIType(result, expr.ReturnType.Span, "arrow return types")
		c.result = result
	} else if expected.Kind == Function && expected.Result != nil {
		result = *expected.Result
		c.result = result
	}
	if expr.ExpressionBody != nil {
		if result.Kind == Result {
			c.report(expr.ExpressionBody.GetSpan(), "Result arrow functions require a block body with an explicit return")
		}
		actual := c.checkExpressionExpectedSlot(&expr.ExpressionBody, result)
		if expr.ReturnType == nil && result.Kind == Invalid {
			if actual.Kind != MultiValue {
				actual = c.singleValue(actual, expr.ExpressionBody.GetSpan())
			}
			if actual.Kind == Nil {
				c.report(expr.ExpressionBody.GetSpan(), "cannot infer an arrow function return type from nil")
				result = Type{Kind: Invalid, Name: "<invalid>"}
			} else {
				result = defaultLiteralType(actual)
				c.checkNumericMaterialization(expr.ExpressionBody, result)
			}
		} else {
			c.requireAssignable(result, actual, expr.ExpressionBody.GetSpan())
		}
	} else if expr.BlockBody != nil {
		c.validateLabels(expr.BlockBody)
		var inference *arrowReturnInference
		if expr.ReturnType == nil && result.Kind == Invalid {
			inference = &arrowReturnInference{}
			c.arrowReturns = inference
			c.result = Type{}
		}
		c.checkBlock(expr.BlockBody, false)
		if inference != nil {
			result = c.finishArrowReturnInference(inference, expr)
		}
		if result.Kind != Void && result.Kind != Invalid && !definitelyReturns(expr.BlockBody) {
			c.report(expr.Span, fmt.Sprintf("arrow function may complete without returning %s", result.String()))
		}
	}
	if containsTaskType(result) {
		c.report(expr.Span, "arrow functions cannot return Task; Task is a non-escaping local capability")
	}
	c.callableControlState = previousControl
	effects := c.leaveClosureLexical(lexicalContext)
	if inference := c.checkingLocalArrow; inference != nil && inference.declaration.Value == expr {
		inference.capturedWrites = effects.writes
		inference.memberWrite = effects.memberWrite
	}
	c.publishClosureEffects(effects, expr.Span)
	c.prepareGoTypeForEmission(&result, expr.Span)
	resolved := typeRefFromType(result, expr.Span)
	expr.ResolvedReturnType = resolved
	callableParameters := make([]Type, len(parameters))
	for index, parameter := range expr.Parameters {
		callableParameters[index] = c.callableParameterType(parameter, parameters[index])
	}
	return Type{Kind: Function, Name: "function", Parameters: callableParameters, Variadic: hasVariadicParameter(expr.Parameters), Result: &result}
}
