package sema

import (
	"fmt"
	goast "go/ast"
	"go/constant"
	gotoken "go/token"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkGoTypeAssertion(expr *ast.GoTypeAssertionExpr) Type {
	value := c.singleValue(c.checkExpression(expr.Value), expr.Value.GetSpan())
	asserted := c.resolveType(expr.Type)
	if value.Kind == Invalid || asserted.Kind == Invalid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	source := value
	if source.Kind == Nullable && source.Element != nil {
		source = *source.Element
	}
	if source.Kind == Class {
		if asserted.Kind != Class {
			c.report(expr.Span, fmt.Sprintf("class downcast requires class source and target types, got %s and %s", value.String(), asserted.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if source.Name == asserted.Name {
			c.report(expr.Span, fmt.Sprintf("%s already has class type %s; no downcast is needed", value.String(), asserted.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		ancestor, downcast := c.classAncestorType(asserted, source.Name)
		if !downcast || !exactType(source, ancestor) {
			upcastAncestor, upcast := c.classAncestorType(source, asserted.Name)
			if upcast && exactType(asserted, upcastAncestor) {
				c.report(expr.Span, fmt.Sprintf("%s to %s is an upcast; use ordinary assignment or argument passing", source.String(), asserted.String()))
			} else {
				c.report(expr.Span, fmt.Sprintf("classes %s and %s are not in the same inheritance chain", source.String(), asserted.String()))
			}
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		expr.ClassDowncast = true
		expr.SourceClass = source.Name
		if !expr.Checked {
			return asserted
		}
		return Type{Kind: MultiValue, Name: "checked class downcast", Results: []Type{asserted, builtins["boolean"]}}
	}
	contract := underlyingGoInterface(value.GoType)
	if contract == nil {
		c.report(expr.Value.GetSpan(), fmt.Sprintf("type assertion requires a Go interface value, got %s", value.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	assertedGoType, ok := goTypeOf(asserted)
	if !ok {
		c.report(expr.Type.Span, fmt.Sprintf("asserted type %s cannot be represented as a Go type", asserted.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !gotypes.AssertableTo(contract, assertedGoType) {
		c.report(expr.Span, fmt.Sprintf("Go interface %s cannot contain asserted type %s", value.String(), asserted.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !expr.Checked {
		return asserted
	}
	return Type{Kind: MultiValue, Name: "checked type assertion", Results: []Type{asserted, builtins["boolean"]}}
}

func (c *Checker) checkGoConversion(expr *ast.CallExpr, target Type) Type {
	converted, err := kinmokuseiTypeFromGo(target.GoType)
	if err != nil {
		c.report(expr.Callee.GetSpan(), fmt.Sprintf("Go type %s cannot be used in a conversion: %v", target.String(), err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if expr.Expanded {
		c.report(expr.Span, "spread arguments cannot be used in Go type conversions")
	}
	converted.GoQualifier = target.GoQualifier
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("conversion to %s expects 1 argument, got %d", target.String(), len(expr.Arguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return converted
	}
	value := c.singleValue(c.checkExpression(expr.Arguments[0]), expr.Arguments[0].GetSpan())
	valueGo, valueOK := goTypeOf(value)
	if isComplexType(converted) || isComplexType(value) || isUntypedGoNumeric(value) {
		return c.checkComplexConversion(expr, converted, value)
	}
	if target.GoType == nil || !valueOK || !gotypes.ConvertibleTo(valueGo, target.GoType) {
		c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("cannot convert %s to %s", value.String(), target.String()))
	}
	return converted
}

func (c *Checker) checkNativeTypeConversion(expr *ast.CallExpr, target Type) Type {
	if expr.Expanded {
		c.report(expr.Span, "spread arguments cannot be used in type conversions")
	}
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("conversion to %s expects 1 argument, got %d", target.String(), len(expr.Arguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return target
	}
	if target.Kind == Invalid {
		c.checkExpression(expr.Arguments[0])
		return target
	}
	value := c.singleValue(c.checkExpression(expr.Arguments[0]), expr.Arguments[0].GetSpan())
	targetGo, targetOK := goTypeOf(target)
	if isComplexType(target) || isComplexType(value) || isUntypedGoNumeric(value) {
		return c.checkComplexConversion(expr, target, value)
	}
	valueGo, valueOK := goTypeOf(value)
	if target.Kind == TypeParameter && value.Kind == Nil {
		valueGo, valueOK = gotypes.Typ[gotypes.UntypedNil], true
	}
	convertible := targetOK && valueOK && value.Kind != Nullable && gotypes.ConvertibleTo(valueGo, targetGo)
	if !convertible {
		c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("cannot convert %s to %s", value.String(), target.String()))
	} else if target.IsNumeric() {
		if integer, known := c.resolvedIntegerConstantValue(expr.Arguments[0]); known && !integerConstantFitsFixedType(integer, target) {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", integer.String(), target.String()))
		}
	}
	if target.Kind == TypeParameter && convertible && value.Kind != TypeParameter {
		if integer, known := c.resolvedIntegerConstantValue(expr.Arguments[0]); known {
			// ConvertibleTo checks type sets, but not the particular constant's
			// representability. CheckExpr applies Go's constant rules to every
			// possible type argument. Target-dependent int bounds are also
			// validated when checking the generated Go for the selected target.
			pkg := gotypes.NewPackage("kinmokusei.synthetic/conversion", "conversion")
			pkg.Scope().Insert(gotypes.NewTypeName(0, pkg, "Target", targetGo))
			pkg.Scope().Insert(gotypes.NewConst(0, pkg, "value", valueGo, constant.Make(integer)))
			pkg.MarkComplete()
			call := &goast.CallExpr{Fun: goast.NewIdent("Target"), Args: []goast.Expr{goast.NewIdent("value")}}
			if err := gotypes.CheckExpr(gotoken.NewFileSet(), pkg, gotoken.NoPos, call, nil); err != nil {
				c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("integer constant %s cannot be converted to every type in %s's type set", integer.String(), target.String()))
			}
		}
	}
	return target
}
