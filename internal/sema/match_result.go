package sema

import (
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type resultMatchCoverage struct {
	success, failure matchCoverage
}

func (c *Checker) checkResultMatchArm(arm *ast.MatchArm, successType Type, covered *resultMatchCoverage) {
	pattern := &arm.Pattern
	payloadType := successType
	variant := &covered.success
	label := "ok"
	switch pattern.Kind {
	case ast.MatchResultOK:
	case ast.MatchResultFail:
		payloadType = builtins["error"]
		variant = &covered.failure
		label = "fail"
	default:
		c.report(pattern.Span, "Result match arms must use ok or fail patterns. Use _ inside a variant to discard its payload.")
		return
	}
	if variant.exhaustive() {
		c.report(arm.Span, "unreachable match arm")
	}
	if payloadType.Kind == Void && pattern.Payload != nil {
		c.report(pattern.Span, "Result<void> success pattern must be ok()")
		c.checkMatchPattern(pattern.Payload, Type{Kind: Invalid}, true)
		return
	}
	if pattern.Payload == nil {
		if payloadType.Kind != Void {
			c.report(pattern.Span, label+" pattern requires a payload pattern")
		} else if arm.Guard == nil {
			variant.total = true
		}
		return
	}
	total := c.checkMatchPattern(pattern.Payload, payloadType, true)
	if pattern.Kind == ast.MatchResultFail && pattern.Payload.Kind == ast.MatchBinding {
		binding := &pattern.Payload.Binding
		if binding.Name != "_" {
			c.trackResultError(binding.Name, binding.Span)
		}
	}
	if arm.Guard == nil {
		variant.add(pattern.Payload, payloadType, total)
	}
}

func (c *Checker) checkResultMatchExhaustiveness(span source.Span, covered resultMatchCoverage) bool {
	var missing []string
	if !covered.success.exhaustive() {
		missing = append(missing, "ok")
	}
	if !covered.failure.exhaustive() {
		missing = append(missing, "fail")
	}
	if len(missing) == 0 {
		return true
	}
	c.report(span, "non-exhaustive Result match. Missing full coverage for "+strings.Join(missing, ", "))
	return false
}
