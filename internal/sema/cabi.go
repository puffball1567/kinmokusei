package sema

import (
	"fmt"
	gotoken "go/token"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) checkCABIExports(program *ast.Program) {
	program.CABIExports = nil
	symbols := map[string]source.Span{}
	targets := map[string]source.Span{}
	functions := map[string]*ast.FunctionDecl{}
	variables := map[string]*ast.VariableDecl{}
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			functions[declaration.Name] = declaration
		case *ast.VariableDecl:
			variables[declaration.Name] = declaration
		}
	}
	validate := func(export ast.CABIExport, generic bool) {
		if generic {
			c.report(export.NameSpan, "generic functions cannot be exported through the C ABI")
		}
		symbol := export.Symbol
		if !validCABIIdentifier(symbol) {
			c.report(export.SymbolSpan, fmt.Sprintf("C ABI symbol %q must start with an ASCII letter and contain only ASCII letters, digits, or '_'", symbol))
		} else if gotoken.Lookup(symbol).IsKeyword() {
			c.report(export.SymbolSpan, fmt.Sprintf("C ABI symbol %q is a Go keyword and cannot be generated", symbol))
		} else if symbol == "main" || symbol == "init" {
			c.report(export.SymbolSpan, fmt.Sprintf("C ABI symbol %q is reserved by the generated Go package", symbol))
		}
		if previous, exists := symbols[symbol]; exists {
			c.report(export.SymbolSpan, fmt.Sprintf("duplicate C ABI symbol %q (first declared at %d:%d)", symbol, previous.Start.Line, previous.Start.Column))
		} else {
			symbols[symbol] = export.SymbolSpan
		}
		if previous, exists := targets[export.Name]; exists {
			c.report(export.NameSpan, fmt.Sprintf("duplicate C ABI export target %q (first exported at %d:%d)", export.Name, previous.Start.Line, previous.Start.Column))
		} else {
			targets[export.Name] = export.NameSpan
		}
		for _, parameter := range export.Parameters {
			if !c.isCABITypeRef(parameter.Type, false) {
				c.report(parameter.Type.Span, fmt.Sprintf("C ABI parameter %q has unsupported type %s; use boolean, a fixed-width scalar, or an enum with a fixed-width integer underlying type", parameter.Name, formatTypeRefForDiagnostic(parameter.Type)))
			}
		}
		if !c.isCABITypeRef(export.ReturnType, true) {
			c.report(export.ReturnType.Span, fmt.Sprintf("C ABI result has unsupported type %s; use void, boolean, a fixed-width scalar, or an enum with a fixed-width integer underlying type", formatTypeRefForDiagnostic(export.ReturnType)))
		}
		program.CABIExports = append(program.CABIExports, export)
	}
	for _, declaration := range program.Declarations {
		function, ok := declaration.(*ast.FunctionDecl)
		if !ok || !function.CABIExport {
			continue
		}
		validate(ast.CABIExport{
			Name: function.Name, NameSpan: function.NameSpan, Symbol: function.CABISymbol, SymbolSpan: function.CABISymbolSpan,
			Parameters: function.Parameters, ReturnType: function.ReturnType, Span: function.CABIExportSpan,
		}, len(function.TypeParameters) != 0)
	}
	for _, declaration := range program.Declarations {
		exports, ok := declaration.(*ast.CABIExportDecl)
		if !ok {
			continue
		}
		if len(exports.Symbols) != len(exports.Names) {
			c.report(exports.Span, fmt.Sprintf("C ABI export list has %d symbols but %d names; counts must match positionally", len(exports.Symbols), len(exports.Names)))
		}
		exports.ResolvedDeclarations = make([]source.Span, len(exports.Names))
		count := min(len(exports.Symbols), len(exports.Names))
		for index := 0; index < count; index++ {
			name := exports.Names[index]
			if function := functions[name]; function != nil {
				exports.ResolvedDeclarations[index] = function.NameSpan
				validate(ast.CABIExport{
					Name: name, NameSpan: exports.NameSpans[index], Symbol: exports.Symbols[index], SymbolSpan: exports.SymbolSpans[index],
					Parameters: function.Parameters, ReturnType: function.ReturnType, Span: exports.Span,
				}, len(function.TypeParameters) != 0)
				continue
			}
			variable := variables[name]
			if variable == nil {
				c.report(exports.NameSpans[index], fmt.Sprintf("undefined C ABI export target %q", name))
				continue
			}
			exports.ResolvedDeclarations[index] = variable.NameSpan
			if !variable.Constant {
				c.report(exports.NameSpans[index], fmt.Sprintf("C ABI export target %q must be a const arrow function", name))
				continue
			}
			arrow, ok := variable.Value.(*ast.ArrowExpr)
			if !ok {
				c.report(exports.NameSpans[index], fmt.Sprintf("C ABI export target %q must be a top-level function or const arrow function", name))
				continue
			}
			if arrow.ReturnType == nil {
				c.report(exports.NameSpans[index], fmt.Sprintf("C ABI arrow export %q requires an explicit return type", name))
				continue
			}
			validate(ast.CABIExport{
				Name: name, NameSpan: exports.NameSpans[index], Symbol: exports.Symbols[index], SymbolSpan: exports.SymbolSpans[index],
				Parameters: arrow.Parameters, ReturnType: *arrow.ReturnType, Span: exports.Span,
			}, false)
		}
	}
}

func validCABIIdentifier(name string) bool {
	if len(name) == 0 || !isASCIIAlpha(name[0]) {
		return false
	}
	for index := 1; index < len(name); index++ {
		if !isASCIIAlpha(name[index]) && (name[index] < '0' || name[index] > '9') && name[index] != '_' {
			return false
		}
	}
	return true
}

func isASCIIAlpha(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func (c *Checker) isCABITypeRef(ref ast.TypeRef, allowVoid bool) bool {
	if ref.Nullable || ref.Qualifier != "" || len(ref.GenericArguments) != 0 || ref.IsArray() || ref.IsPointer() || ref.IsFunction() || ref.IsObject() {
		return false
	}
	switch ref.Name {
	case "boolean", "byte", "uint8", "int8", "int16", "int32", "int64", "uint16", "uint32", "uint64", "float32", "float", "float64", "number":
		return true
	case "void":
		return allowVoid
	default:
		if c.enums[ref.Name] == nil {
			return false
		}
		resolved := c.resolveType(ref)
		if resolved.GoType == nil {
			return false
		}
		basic, ok := gotypes.Unalias(resolved.GoType).Underlying().(*gotypes.Basic)
		if !ok {
			return false
		}
		switch basic.Kind() {
		case gotypes.Int8, gotypes.Int16, gotypes.Int32, gotypes.Int64, gotypes.Uint8, gotypes.Uint16, gotypes.Uint32, gotypes.Uint64:
			return true
		default:
			return false
		}
	}
}

func formatTypeRefForDiagnostic(ref ast.TypeRef) string {
	if ref.Nullable {
		ref.Nullable = false
		return formatTypeRefForDiagnostic(ref) + " | null"
	}
	if ref.Name != "" && !ref.IsArray() && !ref.IsPointer() && !ref.IsFunction() && !ref.IsObject() {
		if ref.Qualifier != "" {
			return ref.Qualifier + "." + ref.Name
		}
		return ref.Name
	}
	if ref.IsArray() && ref.Element != nil {
		return formatTypeRefForDiagnostic(*ref.Element) + "[]"
	}
	if ref.IsPointer() && ref.Pointee != nil {
		return "*" + formatTypeRefForDiagnostic(*ref.Pointee)
	}
	return "non-scalar type"
}
