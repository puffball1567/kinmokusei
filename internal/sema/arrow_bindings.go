package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) arrowContext(expected Type) Type {
	if expected.Kind == Nullable && expected.Element != nil {
		expected = *expected.Element
	}
	if expected.Kind == GoNamed && expected.GoType != nil {
		if signature, err := kinmokuseiTypeFromGo(expected.GoType.Underlying()); err == nil && signature.Kind == Function {
			return signature
		}
	}
	return expected
}

func (c *Checker) inferArrowParameters(arrow *ast.ArrowExpr, expected Type) map[int]Type {
	inferred := map[int]Type{}
	compatible := expected.Kind == Function && len(expected.Parameters) == len(arrow.Parameters) && expected.Variadic == hasVariadicParameter(arrow.Parameters)
	for i := range arrow.Parameters {
		parameter := &arrow.Parameters[i]
		if parameter.Type.IsSpecified() {
			continue
		}
		if !compatible {
			c.report(parameter.Span, fmt.Sprintf("cannot infer arrow parameter %q; add a type annotation or a matching function context", parameter.Name))
			continue
		}
		resolved := expected.Parameters[i]
		if parameter.Variadic {
			element := resolved
			resolved = Type{Kind: Array, Name: "array", Element: &element}
		}
		c.prepareGoTypeForEmission(&resolved, parameter.Span)
		parameter.Type = typeRefFromType(resolved, parameter.Span)
		inferred[i] = resolved
	}
	return inferred
}

// A signature available without evaluating the body supports recursion and
// forward references. Named function storage keeps its original type and ABI.
func (c *Checker) declareArrowBinding(declaration *ast.VariableDecl, declared Type) Type {
	arrow, ok := declaration.Value.(*ast.ArrowExpr)
	declaration.FunctionBinding = ok && declaration.Constant && (!declaration.Type.IsSpecified() || declared.Kind == Function)
	if !declaration.FunctionBinding {
		return declared
	}
	if declaration.Name == "main" && !declaration.Type.IsSpecified() && arrow.ReturnType == nil && len(arrow.Parameters) == 0 {
		result := builtins["void"]
		declared = Type{Kind: Function, Name: "function", Result: &result}
	}
	if declared.Kind == Function {
		return declared
	}
	if arrow.ReturnType == nil {
		return declared
	}
	parameters := make([]Type, len(arrow.Parameters))
	for i, parameter := range arrow.Parameters {
		if !parameter.Type.IsSpecified() {
			return declared
		}
		parameters[i] = c.callableParameterType(parameter, c.resolveType(parameter.Type))
	}
	result := c.resolveType(*arrow.ReturnType)
	return Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: hasVariadicParameter(arrow.Parameters), Result: &result}
}

// Declared function bodies are checked after stored globals, so their bodies can
// refer to later inferred globals just as ordinary function declarations do.
func (c *Checker) globalCheckOrder(program *ast.Program) []*ast.VariableDecl {
	var stored, functions []*ast.VariableDecl
	for _, declaration := range program.Declarations {
		if variable, ok := declaration.(*ast.VariableDecl); ok {
			if variable.FunctionBinding && c.globals[variable.Name].typeInfo.Kind == Function {
				functions = append(functions, variable)
			} else {
				stored = append(stored, variable)
			}
		}
	}
	return append(stored, functions...)
}

func (c *Checker) checkGlobalBinding(decl *ast.VariableDecl) {
	previous := c.globalDependencyOwner
	c.globalDependencyOwner = decl.Name
	defer func() { c.globalDependencyOwner = previous }()
	declared := Type{Kind: Invalid, Name: "<inferred>"}
	if decl.Type.IsSpecified() {
		declared = c.resolveType(decl.Type)
	} else if decl.FunctionBinding {
		declared = c.globals[decl.Name].typeInfo
	}
	valueType := c.checkExpressionExpectedSlot(&decl.Value, declared)
	if !decl.Type.IsSpecified() {
		declared = c.inferredVariableType(valueType, decl.Value.GetSpan())
		if !decl.Constant || !numericInitializerEmitsConstant(decl.Value) {
			c.checkNumericMaterialization(decl.Value, declared)
		}
	}
	c.requireAssignable(declared, valueType, decl.Value.GetSpan())
	if declared.Kind == Void {
		c.report(decl.GetSpan(), "variables cannot have type void")
	}
	c.rejectResultValueType(declared, decl.Type.Span, "variables")
	c.rejectTaskAPIType(declared, decl.Type.Span, "global variables")
	decl.ResolvedType = typeRefFromType(declared, decl.Span)
	if decl.Name == "main" && (!decl.FunctionBinding || declared.Kind != Function || len(declared.Parameters) != 0 || declared.Result == nil || declared.Result.Kind != Void) {
		c.report(decl.NameSpan, "main must be a const arrow with no parameters and a void return type")
	}
	c.globals[decl.Name] = valueSymbol{typeInfo: declared, declaredType: declared, constant: decl.Constant, declarationSpan: decl.NameSpan, declaration: decl}
}
