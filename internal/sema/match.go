package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkMatch(expr *ast.MatchExpr, expected *Type) Type {
	expr.ResultType = ast.TypeRef{}
	value := c.singleValue(c.checkExpression(expr.Value), expr.Value.GetSpan())
	value = c.checkSwitchTag(expr.Value, value)
	switch value.Kind {
	case Void, Task, GoPackage, GoTypeName, Nil, Null:
		c.report(expr.Value.GetSpan(), "match requires an ordinary value with a concrete type")
		value = Type{Kind: Invalid}
	}
	entry := c.snapshotNullableFlow()
	remaining := entry
	continuing := make([]nullableFlowSnapshot, 0, len(expr.Arms))
	result := Type{Kind: Invalid}
	if expected != nil {
		result = *expected
	}
	var covered matchCoverage
	for i := range expr.Arms {
		arm := &expr.Arms[i]
		if covered.exhaustive() {
			c.report(arm.Span, "unreachable match arm")
		}
		c.restoreNullableFlow(remaining)
		c.pushScope()
		total := c.checkMatchPattern(&arm.Pattern, value, false)
		if arm.Guard == nil {
			covered.add(&arm.Pattern, value, total)
		} else {
			guard, whenTrue, whenFalse := c.checkCondition(arm.Guard)
			guard = c.singleValue(guard, arm.Guard.GetSpan())
			if guard.Kind != Invalid && !guard.IsBoolean() {
				c.report(arm.Guard.GetSpan(), "match guard must be boolean")
			}
			if total {
				remaining = c.mergeNullableFlow(entry, whenFalse)
			} else {
				remaining = c.mergeNullableFlow(entry, remaining, whenFalse)
			}
			c.restoreNullableFlow(whenTrue)
		}
		if result.Kind != Invalid {
			actual := c.singleValue(c.checkStorageExpressionExpectedSlot(&arm.Value, result), arm.Value.GetSpan())
			if ordinaryMatchResult(actual) {
				c.requireAssignable(result, actual, arm.Value.GetSpan())
			} else {
				c.report(arm.Value.GetSpan(), "match arm must return an ordinary value")
			}
		} else {
			actual := c.singleValue(c.checkExpression(arm.Value), arm.Value.GetSpan())
			if actual.Kind == Nil || actual.Kind == Null {
				c.report(arm.Value.GetSpan(), "cannot infer match result from null or nil; add a result type")
			} else if !ordinaryMatchResult(actual) {
				c.report(arm.Value.GetSpan(), "match arm must return an ordinary value")
			} else if actual.Kind != Invalid {
				result = c.checkSwitchTag(arm.Value, actual)
				if c.needsStorageBoundary(result, actual, arm.Value) {
					span := arm.Value.GetSpan()
					arm.Value = &ast.UnaryExpr{Operator: ast.NonNullOperator, Operand: arm.Value,
						AssertionType: c.boundaryTypeRef(result, span), AssertionSpan: span, ImplicitNonNull: true, Span: span}
				}
			}
		}
		c.popScope()
		continuing = append(continuing, c.snapshotNullableFlow())
	}
	if !covered.exhaustive() {
		c.report(expr.Span, "non-exhaustive match; add a default arm or cover every possible value")
		continuing = append(continuing, remaining)
	}
	c.restoreNullableFlow(c.mergeNullableFlow(entry, continuing...))
	if !ordinaryMatchResult(result) || result.Kind == Nil || result.Kind == Null {
		c.report(expr.Span, "match must produce an ordinary value")
		return Type{Kind: Invalid}
	}
	if !typeNameableForEmission(result) {
		c.report(expr.Span, "match result contains a private Go type; add an explicit public interface or named type context")
		return Type{Kind: Invalid}
	}
	c.prepareGoTypeForEmission(&result, expr.Span)
	expr.ResultType = typeRefFromType(result, expr.Span)
	return result
}

func ordinaryMatchResult(value Type) bool {
	switch value.Kind {
	case Void, Task, Result, MultiValue, GoPackage, GoTypeName:
		return false
	}
	return true
}

type matchCoverage struct {
	total, whenTrue, whenFalse, whenNull, whenPresent bool
}

func (c matchCoverage) exhaustive() bool {
	return c.total || c.whenTrue && c.whenFalse || c.whenNull && c.whenPresent
}

func (c *matchCoverage) add(pattern *ast.MatchPattern, value Type, total bool) {
	c.total = c.total || total
	switch pattern.Kind {
	case ast.MatchNonNullBinding:
		c.whenPresent = value.Kind == Nullable
	case ast.MatchLiteral:
		literal, ok := pattern.Literal.(*ast.LiteralExpr)
		if !ok {
			return
		}
		if literal.Kind == ast.NullLiteral && value.Kind == Nullable {
			c.whenNull = true
		}
		if literal.Kind == ast.BooleanLiteral && value.IsBoolean() {
			if literal.Text == "true" {
				c.whenTrue = true
			} else {
				c.whenFalse = true
			}
		}
	}
}

func (c *Checker) checkMatchPattern(p *ast.MatchPattern, value Type, extracted bool) bool {
	switch p.Kind {
	case ast.MatchWildcard:
		return true
	case ast.MatchBinding, ast.MatchNonNullBinding:
		nonNull := p.Kind == ast.MatchNonNullBinding
		if nonNull {
			if value.Kind == Nullable && value.Element != nil {
				value = *value.Element
			} else if value.Kind != Invalid {
				c.report(p.Span, fmt.Sprintf("non-null match pattern requires a nullable value, got %s", value.String()))
				value = Type{Kind: Invalid}
			}
		}
		binding := &p.Binding
		binding.ResolvedType = typeRefFromType(value, binding.Span)
		binding.NonNull = false
		if binding.Name != "_" {
			if (extracted || nonNull) && c.needsStorageBoundary(value, value, nil) {
				binding.ResolvedType = c.boundaryTypeRef(value, binding.Span)
				binding.NonNull = true
			}
			c.declareLocal(binding.Name, value, true, nil, binding.Span)
		}
		return !nonNull
	case ast.MatchLiteral:
		c.checkMatchLiteral(p.Literal, value)
	case ast.MatchRecord:
		return c.checkMatchRecord(p, value)
	case ast.MatchSequence:
		return c.checkMatchSequence(p, value)
	}
	return false
}

func (c *Checker) checkMatchLiteral(literal ast.Expression, value Type) {
	if regex, ok := literal.(*ast.RegexExpr); ok {
		c.checkRegexLiteral(regex)
		c.requireAssignable(builtins["string"], value, literal.GetSpan())
		return
	}
	if isAbsenceLiteral(literal) {
		if value.Kind != Invalid && value.Kind != Nullable {
			c.report(literal.GetSpan(), fmt.Sprintf("null match pattern requires a nullable value, got %s", value.String()))
		}
		return
	}
	actual := c.checkExpressionExpected(literal, value)
	c.requireAssignable(value, actual, literal.GetSpan())
	if value.Kind != Invalid && !value.IsComparable() {
		c.report(literal.GetSpan(), fmt.Sprintf("match literal cannot be compared with %s", value.String()))
	}
}

func (c *Checker) checkMatchRecord(p *ast.MatchPattern, value Type) bool {
	if shape, native := c.nativeDefinedShape(value); native {
		value = shape
	}
	total := value.Kind == Object || value.Kind == Struct
	if value.Kind != Invalid && !total {
		c.report(p.Span, fmt.Sprintf("record pattern requires a value struct or object, got %s", value.String()))
	}
	seen := make(map[string]bool, len(p.Fields))
	for i := range p.Fields {
		field := &p.Fields[i]
		if seen[field.Name] {
			c.report(field.NameSpan, fmt.Sprintf("duplicate record pattern field %q", field.Name))
		}
		seen[field.Name] = true
		resolved := Type{Kind: Invalid}
		found := false
		switch value.Kind {
		case Object:
			resolved, found = value.Fields[field.Name]
			field.ResolvedName = value.FieldNames[field.Name]
		case Struct:
			if structure := c.structs[value.Name]; structure != nil {
				if direct, ok := structure.fields[field.Name]; ok {
					resolved, found = substituteNativeTypeParameters(direct.typeInfo, nativeStructBindings(structure, value)), true
					field.ResolvedName, field.ResolvedDeclaration = direct.goName, direct.declarationSpan
					if direct.visibility != ast.Public && direct.declarationSpan.Path != field.NameSpan.Path {
						c.report(field.NameSpan, fmt.Sprintf("record pattern field %q is private", field.Name))
					}
				}
			}
		}
		if !found && (value.Kind == Object || value.Kind == Struct) {
			c.report(field.NameSpan, fmt.Sprintf("%s has no direct record pattern field %q", value.String(), field.Name))
		}
		total = c.checkMatchPattern(&field.Pattern, resolved, true) && found && total
	}
	return total
}

func (c *Checker) checkMatchSequence(p *ast.MatchPattern, value Type) bool {
	value = c.collectionOperandShape(value)
	p.FixedLength = value.Kind == FixedArray
	total := p.FixedLength && int64(len(p.Elements)) == value.Length
	element := Type{Kind: Invalid}
	if value.Kind == Array || value.Kind == FixedArray {
		if value.Element != nil {
			element = *value.Element
		}
		if p.FixedLength && !total {
			c.report(p.Span, fmt.Sprintf("sequence pattern has %d elements; %s requires %d", len(p.Elements), value.String(), value.Length))
		}
	} else if value.Kind != Invalid {
		c.report(p.Span, fmt.Sprintf("sequence pattern requires an array or slice, got %s", value.String()))
	}
	for i := range p.Elements {
		total = c.checkMatchPattern(&p.Elements[i], element, true) && total
	}
	return total
}
