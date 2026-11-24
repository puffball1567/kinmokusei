package sema

import (
	"fmt"
	"go/constant"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkMatch(expr *ast.MatchExpr, expected *Type) Type {
	expr.ResultType = ast.TypeRef{}
	expr.EnumExhaustive = false
	value := c.checkMatchSubject(expr)
	entry := c.snapshotNullableFlow()
	remaining := entry
	continuing := make([]nullableFlowSnapshot, 0, len(expr.Arms))
	result := Type{Kind: Invalid}
	if expected != nil {
		result = *expected
	}
	var covered matchCoverage
	var resultCovered resultMatchCoverage
	for i := range expr.Arms {
		arm := &expr.Arms[i]
		c.restoreNullableFlow(remaining)
		c.pushScope()
		total := false
		if expr.SubjectKind != ast.MatchValueSubject {
			c.checkResultMatchArm(arm, *value.Element, &resultCovered)
		} else {
			if covered.exhaustive() {
				c.report(arm.Span, "unreachable match arm")
			}
			total = c.checkMatchPattern(&arm.Pattern, value, false)
			if arm.Guard == nil {
				covered.add(&arm.Pattern, value, total)
			}
		}
		if arm.Guard != nil {
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
	exhaustive := covered.exhaustive()
	if expr.SubjectKind != ast.MatchValueSubject {
		exhaustive = c.checkResultMatchExhaustiveness(expr.Span, resultCovered)
	} else if !exhaustive {
		exhaustive = c.checkMatchExhaustiveness(expr, value)
	}
	if !exhaustive {
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

func (c *Checker) checkMatchSubject(expr *ast.MatchExpr) Type {
	expr.SubjectKind = ast.MatchValueSubject
	value := c.checkExpression(expr.Value)
	if value.Kind == Result && value.Element != nil {
		if call, ok := expr.Value.(*ast.CallExpr); ok && (call.Builtin == ast.ResultOKCall || call.Builtin == ast.ResultFailCall) {
			c.report(call.Span, "Result constructors must be returned before they can be matched")
		}
		expr.SubjectKind = ast.MatchResultSubject
		if value.Element.Kind == Void {
			expr.SubjectKind = ast.MatchVoidResultSubject
		}
		return value
	}
	value = c.checkSwitchTag(expr.Value, c.singleValue(value, expr.Value.GetSpan()))
	switch value.Kind {
	case Void, Task, GoPackage, GoTypeName, Nil, Null:
		c.report(expr.Value.GetSpan(), "match requires a value with a concrete type or a Result")
		return Type{Kind: Invalid}
	}
	return value
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
	slice                                             sliceMatchCoverage
}

func (c matchCoverage) exhaustive() bool {
	return c.total || c.whenTrue && c.whenFalse || c.whenNull && c.whenPresent || c.slice.exhaustive()
}

func (c *matchCoverage) add(pattern *ast.MatchPattern, value Type, total bool) {
	c.total = c.total || total
	switch pattern.Kind {
	case ast.MatchSequence:
		if pattern.CoversSliceLength {
			c.slice.add(pattern)
		}
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

func (c *Checker) matchEnum(value Type) *enumSymbol {
	if symbol := c.nativeDefinedSymbol(value); symbol != nil {
		return c.enums[symbol.declaration.Name]
	}
	return nil
}

func (c *Checker) checkMatchExhaustiveness(expr *ast.MatchExpr, value Type) bool {
	enum := c.matchEnum(value)
	if enum == nil {
		c.report(expr.Span, "non-exhaustive match; add a default arm or cover every possible value")
		return false
	}
	c.ensureEnumChecked(enum.declaration)
	covered := make(map[string]bool, len(enum.declaration.Members))
	for _, arm := range expr.Arms {
		if arm.Guard != nil || arm.Pattern.Kind != ast.MatchLiteral {
			continue
		}
		if info, known := c.scalarConstant(arm.Pattern.Literal); known {
			if integer := constant.ToInt(info.Value); integer.Kind() == constant.Int {
				covered[integer.ExactString()] = true
			}
		}
	}
	var missing []string
	for _, member := range enum.declaration.Members {
		if member.ResolvedValue != "" && !covered[member.ResolvedValue] {
			missing = append(missing, member.Name)
			covered[member.ResolvedValue] = true // report one name for each missing value
		}
	}
	if len(missing) != 0 {
		c.report(expr.Span, "non-exhaustive match; missing enum members: "+strings.Join(missing, ", ")+"; add an unguarded arm or a default")
		return false
	}
	expr.EnumExhaustive = true
	return true
}

func (c *Checker) checkMatchPattern(p *ast.MatchPattern, value Type, extracted bool) bool {
	switch p.Kind {
	case ast.MatchWildcard:
		return true
	case ast.MatchBinding, ast.MatchNonNullBinding, ast.MatchSequenceRest:
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
			if c.declareLocal(binding.Name, value, true, nil, binding.Span) {
				scope := c.scopes[len(c.scopes)-1]
				symbol := scope[binding.Name]
				symbol.matchBinding = binding
				scope[binding.Name] = symbol
			}
		}
		return !nonNull
	case ast.MatchLiteral:
		c.checkMatchLiteral(p.Literal, value)
	case ast.MatchRecord:
		return c.checkMatchRecord(p, value)
	case ast.MatchSequence:
		return c.checkMatchSequence(p, value)
	case ast.MatchResultOK, ast.MatchResultFail:
		if value.Kind != Invalid {
			c.report(p.Span, "ok and fail patterns require a Result subject")
		}
		if p.Payload != nil {
			c.checkMatchPattern(p.Payload, Type{Kind: Invalid}, true)
		}
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
	if member, ok := literal.(*ast.MemberExpr); ok && actual.Kind != Invalid {
		enum := c.matchEnum(actual)
		if enum == nil || !member.Constant || enum.members[member.Name] == nil ||
			member.ResolvedDeclaration != enum.members[member.Name].NameSpan {
			c.report(member.Span, "qualified match pattern must name an enum member")
			return
		}
	}
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
