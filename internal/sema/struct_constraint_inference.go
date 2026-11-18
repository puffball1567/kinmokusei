package sema

import gotypes "go/types"

// Record constraints cannot use range's core-type inference. Match their
// nominal source terms before the Go fallback erases nullable/text arguments.
// Infer only when all matching alternatives agree, never by union term order.
func (c *Checker) inferNativeStructConstraintArguments(parameter *gotypes.TypeParam, value Type, bindings nativeTypeBindings) {
	actual := c.constraintArgumentShape(value)
	if actual.Kind != Struct {
		return
	}
	var inferred nativeTypeBindings
	for _, term := range c.collectionTerms(parameter) {
		formal := c.constraintArgumentShape(term)
		if formal.Kind != Struct || formal.Name != actual.Name {
			continue
		}
		trial := make(nativeTypeBindings, len(bindings))
		for key, value := range bindings {
			trial[key] = value
		}
		if c.inferNativeTypeArguments(formal, actual, trial) != nil {
			continue
		}
		resolved := substituteNativeTypeParameters(formal, trial)
		if !sameConstraintNullability(resolved, actual) {
			continue
		}
		left, leftOK := c.goTypeForNativeStorage(resolved)
		right, rightOK := c.goTypeForNativeStorage(actual)
		if !leftOK || !rightOK || !gotypes.Identical(left, right) {
			continue
		}
		if inferred != nil {
			if len(trial) != len(inferred) {
				return
			}
			for key, value := range trial {
				if !exactType(value, inferred[key]) {
					return
				}
			}
		}
		inferred = trial
	}
	for key, value := range inferred {
		bindings[key] = value
	}
}
