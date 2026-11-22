package sema

import (
	"fmt"
	gotypes "go/types"
)

func (c *Checker) goStructTypeArgumentCandidates(signature *gotypes.Signature, args, explicit []Type, expanded bool) nativeTypeBindings {
	parameters := signature.TypeParams()
	hasMethods := false
	for i := 0; i < parameters.Len(); i++ {
		if contract := underlyingGoInterface(parameters.At(i).Constraint()); contract != nil && contract.NumMethods() != 0 {
			hasMethods = true
			break
		}
	}
	if !hasMethods {
		return nil
	}
	candidates := make(nativeTypeBindings, parameters.Len())
	for i := 0; i < parameters.Len(); i++ {
		candidates[parameters.At(i)] = Type{}
		if i < len(explicit) {
			candidates[parameters.At(i)] = explicit[i]
		}
	}
	for i, actual := range args {
		formal := goGenericCallParameter(signature, i, expanded)
		if formal == nil {
			continue
		}
		if converted, err := kinmokuseiTypeFromGo(formal); err == nil {
			_ = c.inferNativeTypeArguments(converted, actual, candidates)
		}
	}
	return candidates
}

// Keep the concrete argument in the call probe. For a native struct bound,
// validate its method signatures as an extra argument while leaving structural
// terms and comparable on the type parameter. This gives Go the method types
// needed to infer dependent parameters, without changing nominal identity.
func (c *Checker) goStructConstraintProbe(signature *gotypes.Signature, candidates nativeTypeBindings) (*gotypes.Signature, []Type) {
	if len(candidates) == 0 {
		return nil, nil
	}
	parameters := signature.TypeParams()
	inferenceParameters := make([]Type, parameters.Len())
	for i := range inferenceParameters {
		parameter := parameters.At(i)
		inferenceParameters[i] = Type{Kind: TypeParameter, Name: parameter.Obj().Name(), GoType: parameter}
	}
	// A structural bound such as S ~[]T can reveal T even when the call
	// takes only S. The candidates choose the methods to check, while the
	// Go probe determines the actual type arguments.
	c.inferNativeConstraintArguments(inferenceParameters, candidates)
	bounds := make([]gotypes.Type, parameters.Len())
	var witnesses []Type
	var formals []*gotypes.Var
	for i := range bounds {
		parameter := parameters.At(i)
		bounds[i] = parameter.Constraint()
		contract := underlyingGoInterface(bounds[i])
		if contract == nil {
			continue
		}
		provided := c.projectStructMethods(contract, candidates[parameter])
		if provided == nil {
			continue
		}
		bounds[i] = constraintWithoutMethods(contract)
		methods := make([]*gotypes.Func, contract.NumMethods())
		for j := range methods {
			methods[j] = contract.Method(j)
		}
		required := gotypes.NewInterfaceType(methods, nil).Complete()
		formals = append(formals, gotypes.NewVar(0, nil, "", required))
		witnesses = append(witnesses, Type{Kind: GoInterface, GoType: provided})
	}
	if len(witnesses) == 0 {
		return nil, nil
	}
	clones := make([]*gotypes.TypeParam, parameters.Len())
	replacements := make(map[gotypes.Type]gotypes.Type, parameters.Len())
	for i := range clones {
		name := parameters.At(i).Obj().Name()
		clones[i] = gotypes.NewTypeParam(gotypes.NewTypeName(0, nil, name, nil), bounds[i])
		replacements[parameters.At(i)] = clones[i]
	}
	for i, bound := range bounds {
		clones[i].SetConstraint(substituteConstraintType(bound, replacements))
	}
	for i := 0; i < signature.Params().Len(); i++ {
		formals = append(formals, signature.Params().At(i))
	}
	inputs := substituteConstraintType(gotypes.NewTuple(formals...), replacements).(*gotypes.Tuple)
	results := substituteConstraintType(signature.Results(), replacements).(*gotypes.Tuple)
	return gotypes.NewSignatureType(nil, nil, clones, inputs, results, signature.Variadic()), witnesses
}

func (c *Checker) checkGoStructConstraintArguments(signature *gotypes.Signature, arguments *gotypes.TypeList, candidates nativeTypeBindings) error {
	if candidates == nil {
		return nil
	}
	parameters := signature.TypeParams()
	bindings := make(map[gotypes.Type]gotypes.Type, parameters.Len())
	for i := 0; i < parameters.Len(); i++ {
		bindings[parameters.At(i)] = arguments.At(i)
	}
	for i := 0; i < parameters.Len(); i++ {
		parameter := parameters.At(i)
		bound := underlyingGoInterface(substituteConstraintType(parameter.Constraint(), bindings))
		if bound == nil || bound.NumMethods() == 0 {
			continue
		}
		storage := arguments.At(i)
		actual := candidates[parameter]
		if known, ok := c.goTypeForNativeStorage(actual); !ok || !gotypes.Identical(known, storage) {
			converted, err := kinmokuseiTypeFromGo(storage)
			if err != nil {
				continue // The Go probe has already checked non-native bounds.
			}
			actual = c.restoreNativeRangeType(converted)
		}
		methods, _, _ := c.nativeStructMethodSet(actual)
		valid := false
		if methods == nil {
			valid = gotypes.Satisfies(storage, bound)
		} else {
			valid = gotypes.Satisfies(storage, constraintWithoutMethods(bound)) &&
				c.nativeStructMatchesGoMethods(actual, bound, bound, nil)
		}
		if !valid {
			return fmt.Errorf("%s does not satisfy the Go method constraint for %s", actual.String(), parameter.Obj().Name())
		}
	}
	return nil
}
