package sema

// Embedding declares conformance to a source interface and its bases. The
// effective method set still has to match after promotion and shadowing.
func (c *Checker) nativeStructSatisfiesSourceInterface(target, value Type) bool {
	if target.Kind != Interface {
		return false
	}
	base := value
	if base.Kind == GoPointer && base.Element != nil {
		base = *base.Element
	}
	if base.Kind != Struct && c.nativeDefinedSymbol(base) == nil {
		return false
	}
	return c.structEmbedsSourceInterface(base, target) && c.nativeSourceInterfaceArgumentSatisfies(target, value)
}

// Distinct instantiations of the same struct can declare different embedded
// contracts. Walk them independently while bounding recursive pointer paths.
func (c *Checker) structEmbedsSourceInterface(base, target Type) bool {
	active := map[string]bool{}
	seen := map[string][]Type{}
	var search func(Type) bool
	search = func(current Type) bool {
		if current.Kind == GoPointer && current.Element != nil {
			current = *current.Element
		}
		if current.Kind == Interface {
			return c.interfaceExtends(current, target)
		}
		if defined := c.nativeDefinedSymbol(current); defined != nil {
			current = c.nativeDefinedUnderlying(defined, current)
		}
		structure := c.structs[current.Name]
		if current.Kind != Struct || structure == nil || active[current.Name] {
			return false
		}
		for _, previous := range seen[current.Name] {
			if exactType(previous, current) && c.sourceStorageContractsMatch(previous, current) {
				return false
			}
		}
		seen[current.Name] = append(seen[current.Name], current)
		active[current.Name] = true
		defer delete(active, current.Name)
		bindings := nativeStructBindings(structure, current)
		for _, name := range structure.embedded {
			if search(substituteNativeTypeParameters(structure.fields[name].typeInfo, bindings)) {
				return true
			}
		}
		return false
	}
	return search(base)
}
