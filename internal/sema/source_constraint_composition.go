package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) sourceInterfaceConstraintOperand(term ast.TypeSetTerm, bound Type) constraintOperand {
	if term.Underlying {
		c.report(term.Span, "underlying constraint terms must name concrete types, not interfaces")
		return constraintOperand{}
	}
	storage, valid := c.nativeSourceInterfaceConstraint(bound, term.Span)
	if !valid {
		return constraintOperand{}
	}
	methods, bindings, _, _ := c.nativeConstraintMethods(bound)
	return constraintOperand{
		methods:       constraintMethods(underlyingGoInterface(storage)),
		sourceMethods: substituteConstraintMethods(methods, bindings), valid: true,
	}
}

func substituteConstraintMethods(methods map[string]methodSymbol, bindings nativeTypeBindings) map[string]methodSymbol {
	result := make(map[string]methodSymbol, len(methods))
	for name, method := range methods {
		method.typeInfo = substituteNativeTypeParameters(method.typeInfo, bindings)
		result[name] = method
	}
	return result
}

// This is checked source metadata, never a new runtime interface or a conversion.
// Go storage still uses the original named constraint and its type set.
func sourceConstraintMethodType(methods map[string]methodSymbol, bindings nativeTypeBindings) Type {
	bound := Type{Kind: GoInterface, Name: "source constraint"}
	for _, name := range sortedNativeConstraintMethodNames(methods) {
		method := methods[name]
		bound.GoMethods = append(bound.GoMethods, GoInterfaceMethod{
			Name: name, GoName: method.goName, Type: substituteNativeTypeParameters(method.typeInfo, bindings), DeclarationSpan: method.declarationSpan, GoDeclaration: method.goDeclaration,
		})
	}
	return bound
}

func (operand constraintOperand) rawMethodContracts() []*gotypes.Func {
	if operand.sourceMethods == nil {
		return operand.methods
	}
	return operand.rawMethods
}

// Merge by both source spelling and emitted Go identity. Comparing Go signatures
// alone would silently unify, for example, Item and Item | null or Result<T>
// and an ordinary Go pair. Preserve aliases only when their full contracts agree.
func (c *Checker) composeSourceConstraintMethods(decl *ast.InterfaceDecl, operands []constraintOperand) (map[string]methodSymbol, bool) {
	methods := map[string]methodSymbol{}
	byGoName := map[string]methodSymbol{}
	valid := true
	for _, operand := range operands {
		incoming := operand.sourceMethods
		if incoming == nil {
			incoming = map[string]methodSymbol{}
			for _, method := range operand.methods {
				if !method.Exported() {
					c.report(decl.NameSpan, fmt.Sprintf("source constraint composition cannot expose unexported Go method %s", method.Name()))
					valid = false
					continue
				}
				if !c.allowUnsafeGo && goTypeContainsUnsafePointer(method.Type(), nil) {
					c.report(decl.NameSpan, fmt.Sprintf("source constraint composition: Go method %s uses unsafe.Pointer", method.Name()))
					valid = false
					continue
				}
				if reason := unsupportedGoInteropTypeReason(method.Type(), "method "+method.Name(), map[gotypes.Type]bool{}); reason != "" {
					c.report(decl.NameSpan, "source constraint composition: "+reason)
					valid = false
					continue
				}
				callable, err := kinmokuseiTypeFromGo(method.Type())
				if err != nil {
					c.report(decl.NameSpan, fmt.Sprintf("source constraint composition: %v", err))
					valid = false
					continue
				}
				incoming[method.Name()] = methodSymbol{goName: method.Name(), typeInfo: callable, visibility: ast.Public, goInterfaceMethod: true}
			}
		}
		for _, name := range sortedNativeConstraintMethodNames(incoming) {
			method := incoming[name]
			if previous, exists := byGoName[method.goName]; exists && !sameComposedMethodContract(previous.typeInfo, method.typeInfo) {
				c.report(decl.NameSpan, fmt.Sprintf("constraint intersection has incompatible source signatures for method %s", method.goName))
				valid = false
				continue
			}
			if previous, exists := methods[name]; exists {
				if previous.goName != method.goName || !sameComposedMethodContract(previous.typeInfo, method.typeInfo) {
					c.report(decl.NameSpan, fmt.Sprintf("constraint intersection has conflicting source member %s", name))
					valid = false
				}
				continue
			}
			byGoName[method.goName] = method
			methods[name] = method
		}
	}
	diagnosticCount := len(c.diagnostics)
	c.checkInterfacePropertyContracts(decl, &interfaceSymbol{methods: methods})
	return methods, valid && len(c.diagnostics) == diagnosticCount
}

func sameComposedMethodContract(left, right Type) bool {
	return sameComposedResultEffects(left, right) && identicalMethodSignature(left, right)
}

// Implementations can bridge Result to Go's result storage, but merging source
// contracts cannot choose which failure-handling API wins by operand order.
func sameComposedResultEffects(left, right Type) bool {
	if (left.Kind == Result) != (right.Kind == Result) {
		return false
	}
	for _, pair := range [][2]*Type{{left.Element, right.Element}, {left.Key, right.Key}, {left.Result, right.Result}} {
		if pair[0] != nil && pair[1] != nil && !sameComposedResultEffects(*pair[0], *pair[1]) {
			return false
		}
	}
	for _, pair := range [][2][]Type{{left.Parameters, right.Parameters}, {left.Results, right.Results}, {left.TypeArguments, right.TypeArguments}} {
		if len(pair[0]) == len(pair[1]) {
			for i := range pair[0] {
				if !sameComposedResultEffects(pair[0][i], pair[1][i]) {
					return false
				}
			}
		}
	}
	if left.Kind == Object && right.Kind == Object {
		for name, field := range left.Fields {
			if other, exists := right.Fields[name]; exists && !sameComposedResultEffects(field, other) {
				return false
			}
		}
	}
	if isSourceAnonymousInterface(left) && isSourceAnonymousInterface(right) {
		methods, _ := anonymousInterfaceContractMethods(right)
		for _, method := range left.GoMethods {
			goName := method.GoName
			if goName == "" {
				goName = method.Name
			}
			if other, exists := methods[goName]; exists && !sameComposedResultEffects(method.Type, other) {
				return false
			}
		}
	}
	return true
}
