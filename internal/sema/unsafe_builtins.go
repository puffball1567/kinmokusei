package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkUnsafeBuiltinCall(expr *ast.CallExpr) (Type, bool) {
	member, ok := expr.Callee.(*ast.MemberExpr)
	if identifier, named := expr.Callee.(*ast.IdentifierExpr); named {
		if imported, exists := c.lookupNamedGoImport(identifier.Name, identifier.Span); exists && imported.pack.path == "unsafe" {
			member = &ast.MemberExpr{Object: &ast.IdentifierExpr{Name: resolvedGoPackageAlias(imported.pack), Span: identifier.Span}, Name: identifier.Name, Span: identifier.Span}
			identifier.GoMember = member
			identifier.ResolvedDeclaration = imported.span
			ok = true
		}
	}
	if !ok {
		return Type{}, false
	}
	identifier, ok := member.Object.(*ast.IdentifierExpr)
	if !ok {
		return Type{}, false
	}
	if _, shadowed := c.lookupValue(identifier.Name, identifier.Span); shadowed {
		return Type{}, false
	}
	imported := c.lookupGoPackage(identifier.Span.Path, identifier.Name)
	if imported == nil || imported.path != "unsafe" {
		return Type{}, false
	}
	identifier.ResolvedDeclaration = imported.declaration.AliasSpan
	identifier.Name = resolvedGoPackageAlias(imported)
	kinds := map[string]ast.BuiltinCallKind{
		"Sizeof": ast.UnsafeSizeofCall, "Alignof": ast.UnsafeAlignofCall, "Offsetof": ast.UnsafeOffsetofCall,
		"Add": ast.UnsafeAddCall, "Slice": ast.UnsafeSliceCall, "SliceData": ast.UnsafeSliceDataCall,
		"String": ast.UnsafeStringCall, "StringData": ast.UnsafeStringDataCall,
	}
	kind, ok := kinds[member.Name]
	if !ok {
		return Type{}, false
	}

	expr.Builtin = kind
	member.Go = true
	member.ResolvedName = member.Name
	imported.declaration.Used = true
	qualifiedName := identifier.Name + "." + member.Name
	wantArguments := 1
	if member.Name == "Add" || member.Name == "Slice" || member.Name == "String" {
		wantArguments = 2
	}
	c.checkBuiltinCallShape(expr, qualifiedName, wantArguments, wantArguments, 0)
	arguments := make([]Type, len(expr.Arguments))
	for index, argument := range expr.Arguments {
		arguments[index] = c.singleValue(c.checkExpression(argument), argument.GetSpan())
	}

	uintptrType := Type{Kind: GoBasic, Name: "uintptr", GoType: gotypes.Typ[gotypes.Uintptr]}
	unsafePointer := Type{Kind: GoBasic, Name: "unsafe.Pointer", GoType: gotypes.Typ[gotypes.UnsafePointer], GoQualifier: resolvedGoPackageAlias(imported)}
	invalid := Type{Kind: Invalid, Name: "<invalid>"}
	argument := func(index int) (Type, bool) {
		if index >= len(arguments) {
			return invalid, false
		}
		return arguments[index], true
	}

	switch member.Name {
	case "Sizeof", "Alignof":
		value, exists := argument(0)
		if exists && value.Kind == Nil {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s requires a typed value, got nil", qualifiedName))
		} else if exists && value.Kind == Void {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s requires a value, got void", qualifiedName))
		}
		return uintptrType, true
	case "Offsetof":
		_, exists := argument(0)
		if exists {
			field, fieldOK := expr.Arguments[0].(*ast.MemberExpr)
			if !fieldOK || !field.GoField {
				c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s requires a Go struct field selector", qualifiedName))
			} else if field.GoFieldViaPointer {
				c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s field cannot be embedded through a pointer", qualifiedName))
			}
		}
		return uintptrType, true
	case "Add":
		if pointer, exists := argument(0); exists && pointer.Kind != Invalid && pointer.Kind != Nil && !isUnsafePointer(pointer) {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s pointer must be unsafe.Pointer, got %s", qualifiedName, pointer.String()))
		}
		if length, exists := argument(1); exists {
			c.checkUnsafeIntegerArgument(qualifiedName, "offset", expr.Arguments[1], length, false)
		}
		return unsafePointer, true
	case "Slice":
		pointer, exists := argument(0)
		if !exists || pointer.Kind == Invalid {
			return invalid, true
		}
		pointerGo, representable := goTypeOf(pointer)
		var selected *gotypes.Pointer
		if representable {
			selected, _ = gotypes.Unalias(pointerGo).Underlying().(*gotypes.Pointer)
		}
		if selected == nil {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s pointer must be a typed Go pointer, got %s", qualifiedName, pointer.String()))
			return invalid, true
		}
		if length, exists := argument(1); exists {
			c.checkUnsafeIntegerArgument(qualifiedName, "length", expr.Arguments[1], length, true)
		}
		element := c.collectionElementType(selected.Elem(), pointer, expr.Span)
		return Type{Kind: Array, Name: "array", Element: &element}, true
	case "SliceData":
		value, exists := argument(0)
		if !exists || value.Kind == Invalid {
			return invalid, true
		}
		valueGo, representable := goTypeOf(value)
		if !representable {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s argument must be a slice, got %s", qualifiedName, value.String()))
			return invalid, true
		}
		slice, sliceOK := gotypes.Unalias(valueGo).Underlying().(*gotypes.Slice)
		if !sliceOK {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s argument must be a slice, got %s", qualifiedName, value.String()))
			return invalid, true
		}
		element := c.collectionElementType(slice.Elem(), value, expr.Span)
		elementGo, _ := goTypeOf(element)
		return Type{Kind: GoPointer, Name: "*" + element.String(), Element: &element, GoType: gotypes.NewPointer(elementGo), GoQualifier: element.GoQualifier}, true
	case "String":
		if pointer, exists := argument(0); exists && pointer.Kind != Invalid && pointer.Kind != Nil && !isGoBytePointer(pointer) {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s pointer must be *byte, got %s", qualifiedName, pointer.String()))
		}
		if length, exists := argument(1); exists {
			c.checkUnsafeIntegerArgument(qualifiedName, "length", expr.Arguments[1], length, true)
		}
		return builtins["string"], true
	case "StringData":
		if value, exists := argument(0); exists && value.Kind != Invalid && !isGoString(value) {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s argument must be a string, got %s", qualifiedName, value.String()))
		}
		byteType := builtins["byte"]
		return Type{Kind: GoPointer, Name: "*byte", Element: &byteType, GoType: gotypes.NewPointer(gotypes.Typ[gotypes.Uint8])}, true
	default:
		return invalid, true
	}
}

func resolvedGoPackageAlias(imported *goPackageSymbol) string {
	if imported == nil || imported.declaration == nil {
		return ""
	}
	if imported.declaration.ResolvedAlias != "" {
		return imported.declaration.ResolvedAlias
	}
	return imported.declaration.Alias
}

func isUnsafePointer(value Type) bool {
	goType, ok := goTypeOf(value)
	return ok && gotypes.AssignableTo(goType, gotypes.Typ[gotypes.UnsafePointer])
}

func isGoBytePointer(value Type) bool {
	goType, ok := goTypeOf(value)
	return ok && gotypes.AssignableTo(goType, gotypes.NewPointer(gotypes.Typ[gotypes.Uint8]))
}

func isGoString(value Type) bool {
	goType, ok := goTypeOf(value)
	return ok && gotypes.AssignableTo(goType, gotypes.Typ[gotypes.String])
}

func (c *Checker) checkUnsafeIntegerArgument(name, role string, expression ast.Expression, value Type, nonnegative bool) {
	if value.Kind != Invalid && !value.IsInteger() {
		c.report(expression.GetSpan(), fmt.Sprintf("%s %s must be an integer, got %s", name, role, value.String()))
		return
	}
	if nonnegative {
		if constant, known := integerConstantValue(expression); known && constant.Sign() < 0 {
			c.report(expression.GetSpan(), fmt.Sprintf("%s %s cannot be negative", name, role))
		} else if known && !constant.IsInt64() {
			c.report(expression.GetSpan(), fmt.Sprintf("%s %s is out of range", name, role))
		}
	} else if constant, known := integerConstantValue(expression); known && !constant.IsInt64() {
		c.report(expression.GetSpan(), fmt.Sprintf("%s %s is out of range", name, role))
	}
}
