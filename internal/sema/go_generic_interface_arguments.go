package sema

import (
	gotypes "go/types"
	"slices"
)

func (c *Checker) inferImportedGoGenericCall(signature *gotypes.Signature, arguments []Type, explicit []gotypes.Type, sourceExplicit []Type, expanded bool, numeric []gotypes.TypeAndValue) (*gotypes.Signature, error) {
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
	if probe == nil {
		probe = arguments
	}
	instance, err := inferGoGenericCall(signature, probe, explicit, expanded, numeric)
	candidates := c.goStructTypeArgumentCandidates(signature, arguments, sourceExplicit, expanded)
	needsInstantiation := err != nil
	if err != nil {
		adjusted, witnesses := c.goStructConstraintProbe(signature, candidates)
		if adjusted == nil {
			return nil, err
		}
		// A witness is only part of the synthetic Go call. The original
		// arguments and the original signature are used for source checking.
		probe = append(witnesses, probe...)
		numeric = append(make([]gotypes.TypeAndValue, len(witnesses)), numeric...)
		instance, err = inferGoGenericCall(adjusted, probe, explicit, expanded, numeric)
		if err != nil {
			return nil, err
		}
	}
	if err := c.checkGoStructConstraintArguments(signature, instance.TypeArgs, candidates); err != nil {
		return nil, err
	}
	if !needsInstantiation {
		return instance.Type.(*gotypes.Signature), nil
	}
	argumentsGo := make([]gotypes.Type, instance.TypeArgs.Len())
	for i := range argumentsGo {
		argumentsGo[i] = instance.TypeArgs.At(i)
	}
	instantiated, err := gotypes.Instantiate(nil, signature, argumentsGo, false)
	if err != nil {
		return nil, err
	}
	return instantiated.(*gotypes.Signature), nil
}

func goGenericCallParameter(signature *gotypes.Signature, index int, expanded bool) gotypes.Type {
	parameters := signature.Params()
	last := parameters.Len() - 1
	if last < 0 || index > last && !signature.Variadic() {
		return nil
	}
	formal := parameters.At(min(index, last)).Type()
	if signature.Variadic() && index >= last && !expanded {
		formal = gotypes.Unalias(formal).(*gotypes.Slice).Elem()
	}
	return formal
}

func goGenericInterfaceArgumentType(signature *gotypes.Signature, index int, explicit []gotypes.Type, expanded bool) gotypes.Type {
	formal := goGenericCallParameter(signature, index, expanded)
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
	if contract == nil || !contract.IsMethodSet() {
		return nil
	}
	return c.projectStructMethods(contract, actual)
}

func (c *Checker) projectStructMethods(contract *gotypes.Interface, actual Type) *gotypes.Interface {
	if contract.NumMethods() == 0 {
		return nil
	}
	methods, bindings, pointer := c.nativeStructMethodSet(actual)
	if methods == nil {
		return nil
	}
	signatures := make([]*gotypes.Func, 0, contract.NumMethods())
	for i := 0; i < contract.NumMethods(); i++ {
		required := contract.Method(i)
		if !required.Exported() {
			provided := c.embeddedPrivateGoMethod(actual, required)
			if provided == nil {
				return nil
			}
			signatures = append(signatures, gotypes.NewFunc(0, required.Pkg(), required.Name(), provided.Type().(*gotypes.Signature)))
			continue
		}
		provided, ok := nativeConstraintMethod(methods, bindings, pointer, required.Name())
		if !ok {
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
