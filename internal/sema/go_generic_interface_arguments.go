package sema

import (
	gotypes "go/types"
	"slices"
)

func (c *Checker) inferImportedGoGenericCall(signature *gotypes.Signature, arguments []Type, explicit []gotypes.Type, expanded bool, numeric []gotypes.TypeAndValue) (*gotypes.Signature, error) {
	var probe []Type
	for i, argument := range arguments {
		formal := goGenericInterfaceArgumentType(signature, i, explicit, expanded)
		if projected := c.projectStructInterfaceArgument(formal, argument); projected != nil {
			if probe == nil {
				probe = slices.Clone(arguments)
			}
			probe[i] = Type{Kind: GoInterface, GoType: projected}
		}
	}
	if probe != nil {
		arguments = probe
	}
	return inferGoGenericCall(signature, arguments, explicit, expanded, numeric)
}

func goGenericInterfaceArgumentType(signature *gotypes.Signature, index int, explicit []gotypes.Type, expanded bool) gotypes.Type {
	parameters := signature.Params()
	last := parameters.Len() - 1
	if last < 0 || index > last && !signature.Variadic() {
		return nil
	}
	formal := parameters.At(min(index, last)).Type()
	if signature.Variadic() && index >= last && !expanded {
		formal = gotypes.Unalias(formal).(*gotypes.Slice).Elem()
	}
	parameter, ok := gotypes.Unalias(formal).(*gotypes.TypeParam)
	if !ok {
		return formal
	}
	boundIndex := parameter.Index()
	if boundIndex >= 0 && boundIndex < len(explicit) && signature.TypeParams().At(boundIndex) == parameter {
		return explicit[boundIndex]
	}
	// A bound is not the argument's type. Replacing an inferred concrete
	// type argument with an interface would change results and constraints.
	return nil
}

// Give Go's inference probe the signatures it would find on the generated
// struct. The probe only projects arguments already expected to be interfaces.
// Final argument checking still sees the original types and source contracts.
func (c *Checker) projectStructInterfaceArgument(formal gotypes.Type, actual Type) *gotypes.Interface {
	contract := underlyingGoInterface(formal)
	if contract == nil || !contract.IsMethodSet() || contract.NumMethods() == 0 {
		return nil
	}
	methods, bindings, pointer := c.nativeStructMethodSet(actual)
	if len(methods) == 0 {
		return nil
	}
	signatures := make([]*gotypes.Func, 0, contract.NumMethods())
	for i := 0; i < contract.NumMethods(); i++ {
		required := contract.Method(i)
		provided, ok := nativeConstraintMethod(methods, bindings, pointer, required.Name())
		if !ok || !required.Exported() {
			return nil
		}
		storage, ok := c.goTypeForNativeStorage(provided)
		signature, callable := storage.(*gotypes.Signature)
		if !ok || !callable {
			return nil
		}
		signatures = append(signatures, gotypes.NewFunc(0, nil, required.Name(), signature))
	}
	return gotypes.NewInterfaceType(signatures, nil).Complete()
}
