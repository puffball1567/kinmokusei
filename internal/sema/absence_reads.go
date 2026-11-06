package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/source"
)

// A missing map entry and a closed receive have the same payload-zero rules.
// Keep valid Go zeros, expose nil-backed absence as nullable, and require a
// checked pair where the source type has no nullable representation.
func (c *Checker) absenceReadResult(element Type, operation, guard string, span source.Span) Type {
	if c.unevaluatedDepth != 0 || c.zeroValueSafe(element) {
		return element
	}
	if isNullableBaseType(element) {
		return Type{Kind: Nullable, Name: "nullable", Element: &element}
	}
	c.report(span, fmt.Sprintf("%s %s has no safe zero value; use [value, %s] and check %s before using value", operation, element.String(), guard, guard))
	return Type{Kind: Invalid, Name: "<invalid>"}
}
