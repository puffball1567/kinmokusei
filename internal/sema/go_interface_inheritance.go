package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Imported contracts keep their exported Go spelling. Source implementations
// are matched by the public Go name, just as for direct `implements pkg.I`.
func classMethodByGoName(class *classSymbol, name string) (methodSymbol, bool) {
	for _, method := range class.methods {
		if method.goName == name {
			return method, true
		}
	}
	for sourceName, method := range class.methods {
		if memberGoName(sourceName, ast.Public) == name {
			return method, true
		}
	}
	return methodSymbol{}, false
}

func (c *Checker) checkInheritedGoMethodConflict(name string, symbol *interfaceSymbol, inherited methodSymbol, span source.Span) {
	for _, existing := range symbol.methods {
		if existing.goName == inherited.goName && !exactType(existing.typeInfo, inherited.typeInfo) {
			c.report(span, fmt.Sprintf("interface %s inherits incompatible signatures for Go method %s", name, inherited.goName))
		}
	}
}

func (c *Checker) inheritGoInterface(decl *ast.InterfaceDecl, symbol *interfaceSymbol, base Type, span source.Span) bool {
	name := decl.Name
	contract := underlyingGoInterface(base.GoType)
	if !contract.IsMethodSet() {
		c.report(span, "interface extends requires a runtime Go interface, not a type-set constraint")
		return false
	}
	valid := true
	// Convert the generic origin, then substitute source arguments. Converting
	// only the Go instance would erase class identity and nullable qualifiers.
	bindings := nativeTypeBindings{}
	if named, ok := gotypes.Unalias(base.GoType).(*gotypes.Named); ok && named.TypeParams().Len() == len(base.TypeArguments) {
		contract = underlyingGoInterface(named.Origin())
		for i, argument := range base.TypeArguments {
			bindings[named.TypeParams().At(i)] = argument
		}
	}
	for i := 0; i < contract.NumMethods(); i++ {
		required := contract.Method(i)
		if !required.Exported() {
			c.report(span, fmt.Sprintf("interface %s cannot extend %s: unexported Go method %s", name, base.String(), required.Name()))
			valid = false
			continue
		}
		if !c.allowUnsafeGo && goTypeContainsUnsafePointer(required.Type(), nil) {
			c.report(span, fmt.Sprintf("interface %s cannot extend %s: Go method %s uses unsafe.Pointer", name, base.String(), required.Name()))
			valid = false
			continue
		}
		if reason := unsupportedGoInteropTypeReason(required.Type(), "method "+required.Name(), map[gotypes.Type]bool{}); reason != "" {
			c.report(span, fmt.Sprintf("interface %s cannot extend %s: %s", name, base.String(), reason))
			valid = false
			continue
		}
		methodType, err := kinmokuseiTypeFromGo(required.Type())
		if err != nil {
			c.report(span, fmt.Sprintf("interface %s cannot extend %s: %v", name, base.String(), err))
			valid = false
			continue
		}
		inheritGoQualifier(&methodType, base)
		methodType = substituteNativeTypeParameters(methodType, bindings)
		inherited := methodSymbol{typeInfo: methodType, visibility: ast.Public, goName: required.Name(), goInterfaceMethod: true}
		c.checkInheritedGoMethodConflict(name, symbol, inherited, span)
		if _, exists := symbol.methods[required.Name()]; !exists {
			symbol.methods[required.Name()] = inherited
		}
		method := ast.InterfaceMethod{Name: required.Name(), GoName: required.Name(), ReturnType: typeRefFromType(*methodType.Result, span)}
		signature := required.Type().(*gotypes.Signature)
		for j, parameter := range methodType.Parameters {
			ref := typeRefFromType(parameter, span)
			variadic := methodType.Variadic && j == len(methodType.Parameters)-1
			if variadic {
				ref = ast.TypeRef{Element: &ref, Span: span}
			}
			parameterName := signature.Params().At(j).Name()
			if parameterName == "" {
				parameterName = fmt.Sprintf("arg%d", j+1)
			}
			method.Parameters = append(method.Parameters, ast.Parameter{Name: parameterName, Type: ref, Variadic: variadic})
		}
		decl.InheritedGoMethods = append(decl.InheritedGoMethods, method)
	}
	return valid
}

func (c *Checker) interfaceHasGoAncestor(value Type, target gotypes.Type) bool {
	for _, ancestor := range c.interfaceAncestors(value) {
		if ancestor.Kind == GoNamed && ancestor.GoType != nil && gotypes.AssignableTo(ancestor.GoType, target) {
			return true
		}
	}
	return false
}
