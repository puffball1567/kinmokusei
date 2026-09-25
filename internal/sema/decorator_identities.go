package sema

// Metadata uses checked types, not source spellings: transparent aliases must
// not lose nominal identity and generic instances must not share a DI key.
// Nullable consumers request the same nominal provider; nullable type arguments
// remain part of that provider's exact generic contract.
func decoratorNominalIdentity(value Type) string {
	if value.Kind == Nullable && value.Element != nil {
		value = *value.Element
	}
	var prefix string
	switch value.Kind {
	case Class:
		prefix = "type|"
	case Interface:
		prefix = "interface|"
	case Struct:
		prefix = "struct|"
	default:
		return ""
	}
	if decoratorValueHasTypeParameter(value) {
		return ""
	}
	identity := prefix + value.Name
	if len(value.TypeArguments) != 0 {
		identity += "|instance|" + decoratorValueContract(value)
	}
	return identity
}
