package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/source"
)

// Imported method signatures carry Go types, not source-only qualifiers. Until
// those signatures have source-shape metadata, reject arguments whose round
// trip would erase nullability or native class/interface identity.
func (c *Checker) checkConstraintMethodArguments(origin *gotypes.Named, arguments []Type, span source.Span) bool {
	return c.checkRawConstraintMethodArguments(c.rawConstraintMethodArguments(origin, arguments), span)
}

func (c *Checker) rawConstraintMethodArguments(origin *gotypes.Named, arguments []Type) []Type {
	contract := underlyingGoInterface(origin)
	if contract == nil || contract.NumMethods() == 0 {
		return nil
	}
	methods := constraintMethods(contract)
	if symbol := c.interfaces[origin.Obj().Name()]; symbol != nil && symbol.goNamed == origin && symbol.constraintSourceMethods != nil {
		// Source method shapes survive substitution. Keep the Go interop guard
		// only for operands whose methods still have raw Go signatures.
		methods = symbol.constraintRawMethods
	}
	var result []Type
	for i, argument := range arguments {
		if i >= origin.TypeParams().Len() {
			break
		}
		parameter := origin.TypeParams().At(i)
		used := false
		for _, method := range methods {
			signature := method.Type()
			// Replacing one identity detects its use even inside nested named
			// generic types, without following recursive named underlyings.
			replaced := substituteConstraintType(signature, map[gotypes.Type]gotypes.Type{parameter: gotypes.Typ[gotypes.Int]})
			used = used || !gotypes.Identical(signature, replaced)
		}
		if used {
			result = append(result, argument)
		}
	}
	return result
}

func (c *Checker) checkRawConstraintMethodArguments(arguments []Type, span source.Span) bool {
	for _, argument := range arguments {
		storage, ok := goTypeOf(argument)
		if ok && goShapedConstraintMethodArgument(argument) {
			if restored, err := kinmokuseiTypeFromGo(storage); err == nil && sameConstraintNullability(argument, restored) {
				continue
			}
		}
		c.report(span, "method constraint arguments must preserve source type information; nullable and native-only method argument shapes are not supported")
		return false
	}
	return true
}

func goShapedConstraintMethodArgument(argument Type) bool {
	switch argument.Kind {
	case Nullable, Class, Interface, Struct, Result:
		return false
	}
	for _, child := range []*Type{argument.Element, argument.Key, argument.Result} {
		if child != nil && !goShapedConstraintMethodArgument(*child) {
			return false
		}
	}
	for _, group := range [][]Type{argument.Parameters, argument.TypeArguments, argument.Results} {
		for _, child := range group {
			if !goShapedConstraintMethodArgument(child) {
				return false
			}
		}
	}
	for _, field := range argument.Fields {
		if !goShapedConstraintMethodArgument(field) {
			return false
		}
	}
	return true
}

// Re-check source method constraints after a caller or generic owner supplies
// concrete bindings. Checking only the original A<E> declaration would miss
// a later substitution of a nullable source type for E.
func (c *Checker) checkConstraintMethodBindings(bound gotypes.Type, bindings nativeTypeBindings, span source.Span) bool {
	named, ok := gotypes.Unalias(bound).(*gotypes.Named)
	if !ok || named.TypeArgs().Len() == 0 {
		return true
	}
	symbol := c.interfaces[named.Obj().Name()]
	if symbol == nil || !symbol.constraint || symbol.goNamed != named.Origin() {
		return true
	}
	arguments := make([]Type, named.TypeArgs().Len())
	for i := range arguments {
		argument, err := kinmokuseiTypeFromGo(named.TypeArgs().At(i))
		if err != nil {
			continue // The ordinary argument checker reports unsupported types.
		}
		arguments[i] = substituteNativeTypeParameters(argument, bindings)
	}
	return c.checkConstraintMethodArguments(named.Origin(), arguments, span)
}
