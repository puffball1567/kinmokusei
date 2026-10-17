package sema

// A structural interface can conceal storage on source value types too. Probe
// both exact method sets and infer only arguments exposed by those methods;
// hidden open fields remain conservative at an opaque Go/reflection boundary.
func (c *Checker) structuralNativeTextContract(template Type, parameters []Type, contract Type, visiting []Type, unknown bool) bool {
	for _, pointer := range []bool{false, true} {
		candidate := template
		candidate.TypeArguments = parameters
		if pointer {
			pointee := candidate
			candidate = Type{Kind: GoPointer, Element: &pointee}
		}
		methods, owner, methodPointer, native := c.nativeConstraintMethods(candidate)
		if !native {
			continue
		}
		bindings := textContractOpenBindings(parameters)
		matches := true
		for _, required := range contract.GoMethods {
			provided, found := nativeConstraintMethod(methods, owner, methodPointer, required.GoName)
			if !found || c.inferNativeTypeArguments(provided, required.Type, bindings) != nil {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		arguments, valid := c.textContractTypeArguments(parameters, bindings)
		if !valid {
			continue
		}
		for i, parameter := range parameters {
			bindings[parameter.GoType] = arguments[i]
		}
		instance := substituteNativeTypeParameters(candidate, bindings)
		if c.nativeSourceInterfaceArgumentSatisfies(contract, instance) && c.containsTextContract(instance, visiting, unknown) {
			return true
		}
	}
	return false
}
