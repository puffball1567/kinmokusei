package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Source constraints retain a normalized union plus any method requirements.
// Expand references while retaining source shapes (Go storage erases nullable
// class elements). Emission keeps the named references, not this expansion.
func (c *Checker) sourceConstraintOperand(term ast.TypeSetTerm) (constraintOperand, bool) {
	ref := term.Type
	if ref.Qualifier != "" || ref.Nullable || ref.IsArray() || ref.IsPointer() || ref.IsFunction() || ref.IsObject() || ref.IsGoStruct() {
		return constraintOperand{}, false
	}
	if _, parameter := c.lookupTypeParameter(ref.Name); parameter {
		return constraintOperand{}, false
	}
	symbol := c.interfaces[ref.Name]
	if symbol == nil || !symbol.constraint || !c.isTopLevelAllowed(ref.Span, ref.Name) {
		return constraintOperand{}, false
	}
	if term.Underlying {
		c.report(term.Span, fmt.Sprintf("underlying constraint term ~%s cannot name a constraint", formatTypeRefForDiagnostic(ref)))
		return constraintOperand{}, true
	}
	instance, ok := c.instantiateNativeConstraint(ref, symbol)
	if !ok || !symbol.constraintValid {
		return constraintOperand{}, true
	}
	contract := underlyingGoInterface(instance)
	if contract == nil {
		return constraintOperand{}, true
	}
	operand := constraintOperand{methods: constraintMethods(contract), valid: true}
	if contract.NumEmbeddeds() == 0 {
		return operand, true
	}
	union, ok := contract.EmbeddedType(0).(*gotypes.Union)
	if !ok || union.Len() != len(symbol.constraintTermTypes) {
		return constraintOperand{}, true
	}
	bindings := make(nativeTypeBindings, len(symbol.typeParameters))
	for i, argument := range ref.GenericArguments {
		bindings[symbol.typeParameters[i].GoType] = c.resolveType(argument)
	}
	terms := make([]*gotypes.Term, union.Len())
	shapes := make([]Type, union.Len())
	for i := range terms {
		terms[i] = union.Term(i)
		shapes[i] = substituteNativeTypeParameters(symbol.constraintTermTypes[i], bindings)
	}
	operand.terms, operand.shapes, operand.restricted = terms, shapes, true
	return operand, true
}

func (c *Checker) instantiateNativeConstraint(ref ast.TypeRef, symbol *interfaceSymbol) (gotypes.Type, bool) {
	if c.nativeConstraintsReady {
		c.completeNativeConstraint(symbol)
	}
	if symbol.goNamed == nil || symbol.goNamed.Underlying() == nil {
		return nil, false
	}
	if len(ref.GenericArguments) != len(symbol.typeParameters) {
		c.report(ref.Span, fmt.Sprintf("constraint %s expects %d type arguments, got %d", ref.Name, len(symbol.typeParameters), len(ref.GenericArguments)))
		return nil, false
	}
	if len(symbol.typeParameters) == 0 {
		return symbol.goNamed, true
	}
	arguments := make([]Type, len(ref.GenericArguments))
	goArguments := make([]gotypes.Type, len(arguments))
	valid := true
	for i, argument := range ref.GenericArguments {
		arguments[i] = c.resolveType(argument)
		if arguments[i].Kind == Invalid {
			valid = false
			continue
		}
		if !validNativeTypeArgument(arguments[i]) {
			c.report(argument.Span, fmt.Sprintf("type %s cannot be used as a constraint type argument", arguments[i].String()))
			valid = false
			continue
		}
		var ok bool
		goArguments[i], ok = c.goTypeForNativeStorage(arguments[i])
		if !ok {
			c.report(argument.Span, fmt.Sprintf("type %s cannot be represented as a constraint type argument", arguments[i].String()))
			valid = false
		}
	}
	if !valid {
		return nil, false
	}
	if !c.checkConstraintMethodArguments(symbol.goNamed, arguments, ref.Span) {
		return nil, false
	}
	if c.pendingBoundInstances == nil && !c.validateNativeTypeArguments(symbol.typeParameters, arguments, ref.GenericArguments, ref.Span, "constraint "+ref.Name) {
		return nil, false
	}
	instance, err := gotypes.Instantiate(nil, symbol.goNamed, goArguments, false)
	if err != nil {
		c.report(ref.Span, fmt.Sprintf("cannot instantiate constraint %s: %v", ref.Name, err))
		return nil, false
	}
	if c.pendingBoundInstances != nil {
		*c.pendingBoundInstances = append(*c.pendingBoundInstances, boundInstance{origin: symbol.goNamed, arguments: goArguments, ref: ref})
	}
	return instance, true
}
