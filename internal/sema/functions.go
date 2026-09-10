package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) checkFunction(decl *ast.FunctionDecl) {
	c.validateLabels(decl.Body)
	previousMemberFlow := c.memberFlow
	c.memberFlow = map[memberFlowKey]memberFlowState{}
	defer func() { c.memberFlow = previousMemberFlow }()
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
	// Returns and super-constructor calls belong to the current callable, even
	// when an arrow captures this from its enclosing constructor.
	previousInConstructor := c.inConstructor
	c.inConstructor = false
	defer func() { c.inConstructor = previousInConstructor }()
	outerFlow := c.snapshotNullableFlow()
	memberRoots := map[source.Span]bool{}
	if len(c.capturedMemberRoots) != 0 {
		for declaration := range c.capturedMemberRoots[len(c.capturedMemberRoots)-1] {
			memberRoots[declaration] = true
		}
	}
	for key, state := range outerFlow.members {
		if state.nonNull {
			memberRoots[key.root] = true
		}
	}
	base := len(c.scopes)
	c.scopes = cloneValueScopes(c.scopes)
	c.memberFlow = map[memberFlowKey]memberFlowState{}
	for scopeIndex, scope := range c.scopes {
		for name, symbol := range scope {
			if !symbol.constant && symbol.declaredType.Kind == Nullable {
				symbol.typeInfo = symbol.declaredType
				c.scopes[scopeIndex][name] = symbol
			}
		}
	}
	c.callableScopeBases = append(c.callableScopeBases, base)
	c.capturedWrites = append(c.capturedWrites, map[source.Span]source.Span{})
	c.capturedMemberWrites = append(c.capturedMemberWrites, source.Span{})
	c.capturedMemberRoots = append(c.capturedMemberRoots, memberRoots)
	c.pushScope()
	parameters := make([]Type, len(expr.Parameters))
	for i, parameter := range expr.Parameters {
		parameters[i] = c.resolveType(parameter.Type)
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
	}
	if expr.ExpressionBody != nil {
		if result.Kind == Result {
			c.report(expr.ExpressionBody.GetSpan(), "Result arrow functions require a block body with an explicit return")
		}
		actual := c.checkExpressionExpectedSlot(&expr.ExpressionBody, result)
		if expr.ReturnType == nil {
			actual = c.singleValue(actual, expr.ExpressionBody.GetSpan())
			if actual.Kind == Nil {
				c.report(expr.ExpressionBody.GetSpan(), "cannot infer an arrow function return type from nil")
				result = Type{Kind: Invalid, Name: "<invalid>"}
			} else {
				result = defaultLiteralType(actual)
			}
		} else {
			c.requireAssignable(result, actual, expr.ExpressionBody.GetSpan())
		}
	} else if expr.BlockBody != nil {
		c.validateLabels(expr.BlockBody)
		if expr.ReturnType == nil {
			c.report(expr.Span, "arrow functions with a block body require an explicit return type")
			result = Type{Kind: Invalid, Name: "<invalid>"}
			c.result = result
		}
		c.checkBlock(expr.BlockBody, false)
		if result.Kind != Void && result.Kind != Invalid && !definitelyReturns(expr.BlockBody) {
			c.report(expr.Span, fmt.Sprintf("arrow function may complete without returning %s", result.String()))
		}
	}
	if containsTaskType(result) {
		c.report(expr.Span, "arrow functions cannot return Task; Task is a non-escaping local capability")
	}
	c.callableControlState = previousControl
	c.popScope()
	captured := c.capturedWrites[len(c.capturedWrites)-1]
	c.capturedWrites = c.capturedWrites[:len(c.capturedWrites)-1]
	capturedMemberWrite := c.capturedMemberWrites[len(c.capturedMemberWrites)-1]
	c.capturedMemberWrites = c.capturedMemberWrites[:len(c.capturedMemberWrites)-1]
	c.capturedMemberRoots = c.capturedMemberRoots[:len(c.capturedMemberRoots)-1]
	c.callableScopeBases = c.callableScopeBases[:len(c.callableScopeBases)-1]
	c.restoreNullableFlow(outerFlow)
	for declaration := range captured {
		c.markDeclarationEscaped(declaration, expr.Span, "a closure that can mutate it")
	}
	if capturedMemberWrite.Start.Line != 0 {
		c.invalidateAllMemberFacts(expr.Span, "a closure with possible member mutation")
		c.recordMemberWrite(expr.Span)
	}
	c.prepareGoTypeForEmission(&result, expr.Span)
	resolved := typeRefFromType(result, expr.Span)
	expr.ResolvedReturnType = resolved
	callableParameters := make([]Type, len(parameters))
	for index, parameter := range expr.Parameters {
		callableParameters[index] = c.callableParameterType(parameter, parameters[index])
	}
	return Type{Kind: Function, Name: "function", Parameters: callableParameters, Variadic: hasVariadicParameter(expr.Parameters), Result: &result}
}
