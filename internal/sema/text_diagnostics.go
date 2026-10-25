package sema

// Explain the existing opaque-interface rejection without relaxing its storage
// checks. A plain type mismatch must not suggest that changing text storage is
// sufficient, and source structural interfaces have their own assignment rules.
func (c *Checker) goInterfaceTextDiagnostic(target, value Type) string {
	for target.Kind == Nullable && target.Element != nil {
		target = *target.Element
	}
	if target.Kind == TypeParameter || isSourceAnonymousInterface(target) || exactType(target, value) || underlyingGoInterface(target.GoType) == nil || !c.textEscapesThroughGoInterface(value) {
		return ""
	}
	reason := "verified UTF-8 string storage may be exposed to unvalidated writes or calls through an opaque Go interface"
	if !c.containsDeclaredVerifiedText(value) {
		reason = "generic storage may hide verified UTF-8 strings; its safety through an opaque Go interface cannot be established"
	}
	return ": " + reason + "; use bstring/raw input storage, then validate with string(raw) before constructing the verified value"
}
