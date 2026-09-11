package sema

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) declareTopLevel(program *ast.Program) {
	declared := map[string]source.Span{}
	for _, decl := range program.Declarations {
		var name string
		switch decl := decl.(type) {
		case *ast.VariableDecl:
			name = decl.Name
			t := Type{Kind: Invalid, Name: "<inferred>"}
			if decl.Type.IsSpecified() {
				t = c.resolveType(decl.Type)
			}
			if t.Kind == Void {
				c.report(decl.Type.Span, "variables cannot have type void")
			}
			c.rejectResultValueType(t, decl.Type.Span, "variables")
			c.rejectTaskAPIType(t, decl.Type.Span, "global variables")
			if _, exists := declared[name]; !exists {
				c.globals[name] = valueSymbol{typeInfo: t, declaredType: t, constant: decl.Constant, declarationSpan: decl.NameSpan, declaration: decl}
			}
		case *ast.FunctionDecl:
			name = decl.Name
			typeParameters, typeParameterScope := c.declareFunctionTypeParameters(decl)
			c.functionTypeParameters[decl] = typeParameterScope
			c.pushTypeParameterScope(typeParameterScope)
			params := make([]Type, len(decl.Parameters))
			for i, param := range decl.Parameters {
				resolved := c.resolveType(param.Type)
				if resolved.Kind == Void {
					c.report(param.Type.Span, "parameters cannot have type void")
				}
				c.rejectResultValueType(resolved, param.Type.Span, "parameters")
				c.rejectTaskAPIType(resolved, param.Type.Span, "function parameters")
				params[i] = c.callableParameterType(param, resolved)
			}
			if _, exists := declared[name]; !exists {
				result := c.resolveType(decl.ReturnType)
				c.rejectTaskAPIType(result, decl.ReturnType.Span, "function return types")
				c.functions[name] = functionSymbol{
					parameters: params, typeParameters: typeParameters, typeParameterScope: typeParameterScope,
					variadic: hasVariadicParameter(decl.Parameters),
					result:   result, span: decl.Span, declarationSpan: decl.NameSpan,
				}
			}
			c.popTypeParameterScope()
		case *ast.ClassDecl:
			name = decl.Name
		case *ast.StructDecl:
			name = decl.Name
		case *ast.TypeDecl:
			name = decl.Name
		case *ast.EnumDecl:
			name = decl.Name
		case *ast.InterfaceDecl:
			name = decl.Name
		case *ast.MethodDecl:
			continue
		case *ast.CABIExportDecl:
			continue
		}
		if previous, exists := declared[name]; exists {
			c.report(decl.GetSpan(), fmt.Sprintf("duplicate top-level name %q (first declared at %d:%d)", name, previous.Start.Line, previous.Start.Column))
		} else {
			declared[name] = decl.GetSpan()
		}
		if isBuiltinTypeName(name) {
			c.report(decl.GetSpan(), fmt.Sprintf("top-level name %q conflicts with a built-in type", name))
		} else if isBuiltinValueName(name) {
			c.report(decl.GetSpan(), fmt.Sprintf("top-level name %q conflicts with a compiler built-in", name))
		}
	}
}

func (c *Checker) declareLocal(name string, t Type, constant bool, declaration *ast.VariableDecl, span source.Span) {
	scope := c.scopes[len(c.scopes)-1]
	if _, exists := scope[name]; exists {
		c.report(span, fmt.Sprintf("duplicate local name %q", name))
		return
	}
	declarationSpan := declarationNameSpan(name, span)
	if declaration != nil {
		declarationSpan = declaration.NameSpan
	} else if name == "this" {
		declarationSpan = source.Span{}
	}
	taskState := uint8(taskNotTracked)
	if t.Kind == Task {
		taskState = taskPending
	}
	scope[name] = valueSymbol{typeInfo: t, declaredType: t, constant: constant, declarationSpan: declarationSpan, declaration: declaration, taskState: taskState}
	if isBuiltinTypeName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a built-in type", name))
	} else if isBuiltinValueName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a compiler built-in", name))
	}
}

func (c *Checker) declareMultiLocal(name string, t Type, constant bool, declaration *ast.MultiVariableDecl, index int, span source.Span) {
	scope := c.scopes[len(c.scopes)-1]
	if _, exists := scope[name]; exists {
		c.report(span, fmt.Sprintf("duplicate local name %q", name))
		return
	}
	scope[name] = valueSymbol{typeInfo: t, declaredType: t, constant: constant, declarationSpan: span, multiDeclaration: declaration, multiIndex: index}
	if isBuiltinTypeName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a built-in type", name))
	} else if isBuiltinValueName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a compiler built-in", name))
	}
}

func (c *Checker) declareCatchLocal(clause *ast.CatchClause, catchType Type) {
	scope := c.scopes[len(c.scopes)-1]
	name := clause.Name
	span := clause.NameSpan
	if _, exists := scope[name]; exists {
		c.report(span, fmt.Sprintf("duplicate local name %q", name))
		return
	}
	scope[name] = valueSymbol{
		typeInfo: catchType, declaredType: catchType, constant: true,
		declarationSpan: span, catchClause: clause,
	}
	if isBuiltinTypeName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a built-in type", name))
	} else if isBuiltinValueName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a compiler built-in", name))
	}
}

func (c *Checker) declareRangeLocal(binding *ast.RangeBinding, t Type, constant bool) {
	scope := c.scopes[len(c.scopes)-1]
	if _, exists := scope[binding.Name]; exists {
		c.report(binding.NameSpan, fmt.Sprintf("duplicate local name %q", binding.Name))
		return
	}
	scope[binding.Name] = valueSymbol{typeInfo: t, declaredType: t, constant: constant, declarationSpan: binding.NameSpan, rangeBinding: binding}
	if isBuiltinTypeName(binding.Name) {
		c.report(binding.NameSpan, fmt.Sprintf("local name %q conflicts with a built-in type", binding.Name))
	} else if isBuiltinValueName(binding.Name) {
		c.report(binding.NameSpan, fmt.Sprintf("local name %q conflicts with a compiler built-in", binding.Name))
	}
}

func (c *Checker) declareSelectLocal(name string, t Type, constant bool, clause *ast.SelectCase, index int, span source.Span) {
	scope := c.scopes[len(c.scopes)-1]
	if _, exists := scope[name]; exists {
		c.report(span, fmt.Sprintf("duplicate local name %q", name))
		return
	}
	scope[name] = valueSymbol{typeInfo: t, declaredType: t, constant: constant, declarationSpan: span, selectCase: clause, selectIndex: index}
	if isBuiltinTypeName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a built-in type", name))
	} else if isBuiltinValueName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a compiler built-in", name))
	}
}

func (c *Checker) declareTypeSwitchLocal(name string, t Type, constant bool, clause *ast.TypeSwitchCase, span source.Span) {
	scope := c.scopes[len(c.scopes)-1]
	if _, exists := scope[name]; exists {
		c.report(span, fmt.Sprintf("duplicate local name %q", name))
		return
	}
	scope[name] = valueSymbol{typeInfo: t, declaredType: t, constant: constant, declarationSpan: span, typeSwitchCase: clause}
	if isBuiltinTypeName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a built-in type", name))
	} else if isBuiltinValueName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a compiler built-in", name))
	}
}

func declarationNameSpan(name string, span source.Span) source.Span {
	if span.Path == "" {
		return source.Span{}
	}
	result := span
	result.End.Offset = result.Start.Offset + len(name)
	result.End.Line = result.Start.Line
	result.End.Column = result.Start.Column + utf8.RuneCountInString(name)
	return result
}

func isBuiltinValueName(name string) bool {
	return name == "goChannel" || name == "closeGoChannel" || strings.HasPrefix(name, "__kinmokusei_")
}

func isBuiltinTypeName(name string) bool {
	if name == "Result" || name == "Task" {
		return true
	}
	_, builtin := LookupType(name)
	return builtin
}

func (c *Checker) hasCallBinding(name string, span source.Span) bool {
	if _, exists := c.lookupValue(name, span); exists {
		return true
	}
	if _, exists := c.functions[name]; exists && c.isTopLevelAllowed(span, name) {
		return true
	}
	return c.lookupGoPackage(span.Path, name) != nil
}

func (c *Checker) lookupValue(name string, span source.Span) (Type, bool) {
	symbol, ok := c.lookupSymbol(name, span)
	return symbol.typeInfo, ok
}

func (c *Checker) lookupSymbol(name string, span source.Span) (valueSymbol, bool) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if symbol, ok := c.scopes[i][name]; ok {
			if symbol.declaration != nil {
				symbol.declaration.Used = true
			}
			if symbol.multiDeclaration != nil {
				symbol.multiDeclaration.Bindings[symbol.multiIndex].Used = true
			}
			if symbol.rangeBinding != nil {
				symbol.rangeBinding.Used = true
			}
			if symbol.selectCase != nil {
				symbol.selectCase.Bindings[symbol.selectIndex].Used = true
			}
			if symbol.typeSwitchCase != nil {
				symbol.typeSwitchCase.Used = true
			}
			if symbol.catchClause != nil {
				symbol.catchClause.Used = true
			}
			return symbol, true
		}
	}
	if !c.isTopLevelAllowed(span, name) {
		return valueSymbol{}, false
	}
	symbol, ok := c.globals[name]
	return symbol, ok
}

func (c *Checker) lookupAssignmentSymbol(name string, span source.Span) (valueSymbol, bool) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if symbol, ok := c.scopes[i][name]; ok {
			if symbol.rangeBinding != nil {
				symbol.rangeBinding.Assigned = true
			}
			return symbol, true
		}
	}
	if !c.isTopLevelAllowed(span, name) {
		return valueSymbol{}, false
	}
	symbol, ok := c.globals[name]
	return symbol, ok
}

func (c *Checker) isTopLevelAllowed(span source.Span, name string) bool {
	if name == "Exception" {
		return true
	}
	if c.allowed == nil {
		return true
	}
	allowed, exists := c.allowed[span.Path]
	return exists && allowed[name]
}

func (c *Checker) pushScope() { c.scopes = append(c.scopes, map[string]valueSymbol{}) }
func (c *Checker) popScope() {
	if len(c.scopes) != 0 {
		c.reportUnconsumedTasks(c.scopes[len(c.scopes)-1])
	}
	c.scopes = c.scopes[:len(c.scopes)-1]
}
