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
		if existing.goName == inherited.goName && !c.compatibleInheritedMethods(existing, inherited) {
			c.report(span, fmt.Sprintf("interface %s inherits incompatible signatures for Go method %s", name, inherited.goName))
		}
	}
}

func (c *Checker) compatibleInheritedMethods(a, b methodSymbol) bool {
	if a.fromAnonymousInterface || b.fromAnonymousInterface {
		return sameComposedMethodContract(a.typeInfo, b.typeInfo) && c.identicalMethodStorage(a.typeInfo, b.typeInfo)
	}
	return identicalMethodSignature(a.typeInfo, b.typeInfo)
}

func (c *Checker) inheritSourceInterfaceMethod(owner string, symbol *interfaceSymbol, name string, method methodSymbol, span source.Span) {
	if previous, exists := symbol.methods[name]; exists {
		if previous.goName != method.goName || !c.compatibleInheritedMethods(previous, method) {
			c.report(span, fmt.Sprintf("interface %s inherits incompatible signatures for method %s", owner, name))
			return
		}
		// A repeated method may still constrain later bases through its source
		// contract. Retain that fact even when the earlier declaration wins.
		previous.fromAnonymousInterface = previous.fromAnonymousInterface || method.fromAnonymousInterface
		symbol.methods[name] = previous
	}
	c.checkInheritedGoMethodConflict(owner, symbol, method, span)
	if _, exists := symbol.methods[name]; !exists {
		symbol.methods[name] = method
	}
}

func (c *Checker) inheritGoInterface(decl *ast.InterfaceDecl, symbol *interfaceSymbol, base Type, span source.Span) bool {
	contract := underlyingGoInterface(base.GoType)
	if !contract.IsMethodSet() {
		c.report(span, "interface extends requires a runtime Go interface, not a type-set constraint")
		return false
	}
	methods, valid := c.embeddedGoInterfaceMethods(base, contract, span, fmt.Sprintf("interface %s cannot extend %s", decl.Name, base.String()))
	for _, required := range methods {
		methodType := required.Type
		inherited := methodSymbol{typeInfo: methodType, visibility: ast.Public, goName: required.Name, goInterfaceMethod: true, goDeclaration: required.GoDeclaration}
		c.checkInheritedGoMethodConflict(decl.Name, symbol, inherited, span)
		if _, exists := symbol.methods[required.Name]; !exists {
			symbol.methods[required.Name] = inherited
		}
		method := ast.InterfaceMethod{Name: required.Name, GoName: required.Name, ReturnType: typeRefFromType(*methodType.Result, span)}
		for j, parameter := range methodType.Parameters {
			ref := typeRefFromType(parameter, span)
			variadic := methodType.Variadic && j == len(methodType.Parameters)-1
			if variadic {
				ref = ast.TypeRef{Element: &ref, Span: span}
			}
			parameterName := methodType.ParameterNames[j]
			if parameterName == "" {
				parameterName = fmt.Sprintf("arg%d", j+1)
			}
			method.Parameters = append(method.Parameters, ast.Parameter{Name: parameterName, Type: ref, Variadic: variadic})
		}
		decl.InheritedGoMethods = append(decl.InheritedGoMethods, method)
	}
	return valid
}

func goInterfaceContractOrigin(base Type, contract *gotypes.Interface) (*gotypes.Interface, nativeTypeBindings) {
	// Convert the generic origin, then substitute source arguments. Converting
	// only the Go instance would erase class identity and nullable qualifiers.
	bindings := nativeTypeBindings{}
	if named, ok := gotypes.Unalias(base.GoType).(*gotypes.Named); ok && named.TypeParams().Len() == len(base.TypeArguments) {
		contract = underlyingGoInterface(named.Origin())
		for i, argument := range base.TypeArguments {
			bindings[named.TypeParams().At(i)] = argument
		}
	}
	return contract, bindings
}

func (c *Checker) embeddedGoInterfaceMethods(base Type, contract *gotypes.Interface, span source.Span, context string) ([]GoInterfaceMethod, bool) {
	valid := true
	contract, bindings := goInterfaceContractOrigin(base, contract)
	methods := make([]GoInterfaceMethod, 0, contract.NumMethods())
	for i := 0; i < contract.NumMethods(); i++ {
		required := contract.Method(i)
		if !required.Exported() {
			c.report(span, fmt.Sprintf("%s: unexported Go method %s", context, required.Name()))
			valid = false
			continue
		}
		if !c.allowUnsafeGo && goTypeContainsUnsafePointer(required.Type(), nil) {
			c.report(span, fmt.Sprintf("%s: Go method %s uses unsafe.Pointer", context, required.Name()))
			valid = false
			continue
		}
		if reason := unsupportedGoInteropTypeReason(required.Type(), "method "+required.Name(), map[gotypes.Type]bool{}); reason != "" {
			c.report(span, fmt.Sprintf("%s: %s", context, reason))
			valid = false
			continue
		}
		methodType, err := kinmokuseiTypeFromGo(required.Type())
		if err != nil {
			c.report(span, fmt.Sprintf("%s: %v", context, err))
			valid = false
			continue
		}
		inheritGoQualifier(&methodType, base)
		methodType = substituteNativeTypeParameters(methodType, bindings)
		signature := required.Type().(*gotypes.Signature)
		methodType.ParameterNames = make([]string, signature.Params().Len())
		for j := range methodType.ParameterNames {
			methodType.ParameterNames[j] = signature.Params().At(j).Name()
		}
		methods = append(methods, GoInterfaceMethod{Name: required.Name(), GoName: required.Name(), Type: methodType, GoDeclaration: required})
	}
	return methods, valid
}

func (c *Checker) interfaceHasGoAncestor(value Type, target gotypes.Type) bool {
	for _, ancestor := range c.interfaceAncestors(value) {
		if (ancestor.Kind == GoNamed || ancestor.Kind == GoInterface) && ancestor.GoType != nil && gotypes.AssignableTo(ancestor.GoType, target) {
			return true
		}
	}
	return false
}
