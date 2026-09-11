package sema

import (
	"fmt"
	gotypes "go/types"
	"math/big"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) checkExpressionExpected(expr ast.Expression, expected Type) Type {
	if arrow, ok := expr.(*ast.ArrowExpr); ok {
		return c.checkArrowExpected(arrow, expected)
	}
	if array, ok := expr.(*ast.ArrayLiteralExpr); ok && (expected.Kind == Array || expected.Kind == FixedArray) && expected.Element != nil {
		return c.checkArrayLiteralExpected(array, expected)
	}
	if object, ok := expr.(*ast.ObjectLiteralExpr); ok && expected.Kind == Object {
		return c.checkObjectLiteralExpected(object, expected)
	}
	actual := c.checkExpression(expr)
	if !c.checkNumericMaterialization(expr, expected) {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if actual.Kind == UntypedInt && expected.IsInteger() {
		if value, known := c.resolvedIntegerConstantValue(expr); known && !integerConstantFitsFixedType(value, expected) {
			c.report(expr.GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", value.String(), expected.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
	}
	return actual
}

func (c *Checker) checkExpressionExpectedSlot(slot *ast.Expression, expected Type) Type {
	actual := c.checkExpressionExpected(*slot, expected)
	c.applyClassUpcast(slot, expected, actual)
	return actual
}

func (c *Checker) checkArrayLiteral(expr *ast.ArrayLiteralExpr) Type {
	expr.Fixed = false
	expr.ResolvedLength = 0
	if len(expr.Elements) == 0 {
		c.report(expr.Span, "cannot infer the element type of an empty array")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	element := defaultLiteralType(c.singleValue(c.checkExpression(expr.Elements[0]), expr.Elements[0].GetSpan()))
	c.checkNumericMaterialization(expr.Elements[0], element)
	if element.Kind == Nil || element.Kind == Null {
		c.report(expr.Elements[0].GetSpan(), "cannot infer an array element type from nil or null")
		element = Type{Kind: Invalid, Name: "<invalid>"}
	}
	for _, item := range expr.Elements[1:] {
		actual := c.checkExpressionExpected(item, element)
		c.requireAssignable(element, actual, item.GetSpan())
	}
	c.prepareGoTypeForEmission(&element, expr.Span)
	expr.ResolvedElementType = typeRefFromType(element, expr.Span)
	return Type{Kind: Array, Name: "array", Element: &element}
}

func (c *Checker) checkArrayLiteralExpected(expr *ast.ArrayLiteralExpr, expected Type) Type {
	element := *expected.Element
	if expected.Kind == FixedArray && int64(len(expr.Elements)) != expected.Length {
		c.report(expr.Span, fmt.Sprintf("fixed array literal has %d elements, expected %d", len(expr.Elements), expected.Length))
	}
	for index := range expr.Elements {
		actual := c.checkExpressionExpectedSlot(&expr.Elements[index], element)
		c.requireAssignable(element, actual, expr.Elements[index].GetSpan())
	}
	c.prepareGoTypeForEmission(&element, expr.Span)
	expr.ResolvedElementType = typeRefFromType(element, expr.Span)
	expr.Fixed = expected.Kind == FixedArray
	expr.ResolvedLength = expected.Length
	return expected
}

func (c *Checker) checkObjectLiteral(expr *ast.ObjectLiteralExpr) Type {
	fields := map[string]Type{}
	fieldNames := map[string]string{}
	expr.ResolvedFieldTypes = make([]ast.TypeRef, len(expr.Fields))
	expr.ResolvedFieldNames = make([]string, len(expr.Fields))
	for i, field := range expr.Fields {
		if _, exists := fields[field.Name]; exists {
			c.report(field.Span, fmt.Sprintf("duplicate object field %q", field.Name))
		}
		fieldType := defaultLiteralType(c.singleValue(c.checkExpression(field.Value), field.Value.GetSpan()))
		c.checkNumericMaterialization(field.Value, fieldType)
		if fieldType.Kind == Nil || fieldType.Kind == Null {
			c.report(field.Value.GetSpan(), fmt.Sprintf("cannot infer object field %q from nil or null", field.Name))
			fieldType = Type{Kind: Invalid, Name: "<invalid>"}
		}
		c.prepareGoTypeForEmission(&fieldType, field.Span)
		fields[field.Name] = fieldType
		fieldNames[field.Name] = memberGoName(field.Name, ast.Public)
		expr.ResolvedFieldTypes[i] = typeRefFromType(fieldType, field.Span)
		expr.ResolvedFieldNames[i] = fieldNames[field.Name]
	}
	return Type{Kind: Object, Name: "object", Fields: fields, FieldNames: fieldNames}
}

func (c *Checker) checkObjectLiteralExpected(expr *ast.ObjectLiteralExpr, expected Type) Type {
	expr.ResolvedFieldTypes = make([]ast.TypeRef, len(expr.Fields))
	expr.ResolvedFieldNames = make([]string, len(expr.Fields))
	seen := map[string]bool{}
	for index, field := range expr.Fields {
		if seen[field.Name] {
			c.report(field.Span, fmt.Sprintf("duplicate object field %q", field.Name))
		}
		seen[field.Name] = true
		fieldType, exists := expected.Fields[field.Name]
		if !exists {
			c.report(field.Span, fmt.Sprintf("object type has no field %q", field.Name))
			c.checkExpression(field.Value)
			continue
		}
		actual := c.checkExpressionExpectedSlot(&expr.Fields[index].Value, fieldType)
		c.requireAssignable(fieldType, actual, field.Value.GetSpan())
		c.prepareGoTypeForEmission(&fieldType, field.Span)
		expr.ResolvedFieldTypes[index] = typeRefFromType(fieldType, field.Span)
		expr.ResolvedFieldNames[index] = expected.FieldNames[field.Name]
	}
	for name := range expected.Fields {
		if !seen[name] {
			c.report(expr.Span, fmt.Sprintf("object literal is missing field %q", name))
		}
	}
	return expected
}

func (c *Checker) checkUnary(expr *ast.UnaryExpr) Type {
	if expr.Operator == "<-" {
		return c.checkChannelReceive(expr, false)
	}
	operand := c.singleValue(c.checkExpression(expr.Operand), expr.Operand.GetSpan())
	if expr.Operator == "&" {
		if operand.Kind == Invalid {
			return operand
		}
		if !c.isAddressableExpression(expr.Operand) {
			c.report(expr.Span, "operator & requires an addressable operand")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		pointee := operand
		var addressedIdentifier *ast.IdentifierExpr
		if identifier, ok := expr.Operand.(*ast.IdentifierExpr); ok {
			addressedIdentifier = identifier
			if symbol, exists := c.lookupSymbol(identifier.Name, identifier.Span); exists {
				pointee = symbol.declaredType
			}
		}
		var pointerGoType gotypes.Type
		if goType, ok := goTypeOf(pointee); ok {
			pointerGoType = gotypes.NewPointer(goType)
		}
		if addressedIdentifier != nil {
			c.markIdentifierEscaped(addressedIdentifier.Name, expr.Span, "taking its address")
		} else if _, ok := expr.Operand.(*ast.MemberExpr); ok {
			c.recordMemberWrite(expr.Span)
			c.invalidateAllMemberFacts(expr.Span, "taking the address of possibly aliased member storage")
		}
		return Type{Kind: GoPointer, Name: "*" + pointee.String(), Element: &pointee, GoType: pointerGoType, GoQualifier: pointee.GoQualifier}
	}
	if operand.Kind == Nullable {
		c.report(expr.Operand.GetSpan(), fmt.Sprintf("nullable value %s must be checked against null before using operator %s", operand.String(), expr.Operator))
		if operand.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		operand = *operand.Element
	}
	switch expr.Operator {
	case "!":
		if operand.Kind != Invalid && !operand.IsBoolean() {
			c.report(expr.Span, "operator ! requires a boolean operand")
		}
		return builtins["boolean"]
	case "*":
		if operand.Kind == Invalid {
			return operand
		}
		if operand.Kind == GoPointer && operand.Element != nil {
			return *operand.Element
		}
		goType, ok := goTypeOf(operand)
		if !ok {
			c.report(expr.Span, fmt.Sprintf("operator * requires a pointer operand, got %s", operand.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		pointer, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Pointer)
		if !ok {
			c.report(expr.Span, fmt.Sprintf("operator * requires a pointer operand, got %s", operand.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		result, err := kinmokuseiTypeFromGo(pointer.Elem())
		if err != nil {
			c.report(expr.Span, err.Error())
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		inheritGoQualifier(&result, operand)
		return result
	case "^":
		if operand.Kind != Invalid && !operand.IsInteger() {
			c.report(expr.Span, "operator ^ requires an integer operand")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return operand
	default:
		if !operand.IsNumeric() && operand.Kind != Invalid {
			c.report(expr.Span, fmt.Sprintf("operator %s requires a numeric operand", expr.Operator))
		}
		if isComplexType(operand) || isUntypedGoNumeric(operand) {
			return c.checkComplexUnary(expr, operand)
		}
		return operand
	}
}

func (c *Checker) checkChannelReceive(expr *ast.UnaryExpr, checked bool) Type {
	operand := c.singleValue(c.checkExpression(expr.Operand), expr.Operand.GetSpan())
	if operand.Kind == Nullable {
		c.report(expr.Operand.GetSpan(), fmt.Sprintf("nullable channel %s must be checked against null before receiving", operand.String()))
		if operand.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		operand = *operand.Element
	}
	if operand.Kind == Invalid {
		return operand
	}
	goType, ok := goTypeOf(operand)
	if !ok {
		c.report(expr.Span, fmt.Sprintf("operator <- requires a Go channel operand, got %s", operand.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	channel, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Chan)
	if !ok {
		c.report(expr.Span, fmt.Sprintf("operator <- requires a Go channel operand, got %s", operand.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if channel.Dir() == gotypes.SendOnly {
		c.report(expr.Span, fmt.Sprintf("cannot receive from send-only channel %s", operand.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	element, err := kinmokuseiTypeFromGo(channel.Elem())
	if err != nil {
		c.report(expr.Span, fmt.Sprintf("channel element type is not supported: %v", err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if operand.Element != nil {
		element = *operand.Element
	}
	if checked {
		return Type{Kind: MultiValue, Name: "checked channel receive", Results: []Type{element, builtins["boolean"]}}
	}
	return element
}

func (c *Checker) isAddressableExpression(expression ast.Expression) bool {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		if expression.GoMember != nil {
			return expression.GoMember.Addressable
		}
		symbol, ok := c.lookupSymbol(expression.Name, expression.Span)
		if ok {
			expression.ResolvedDeclaration = symbol.declarationSpan
			if symbol.declaration != nil && symbol.declaration.FunctionBinding {
				return false
			}
		}
		return ok
	case *ast.MemberExpr:
		return expression.Addressable
	case *ast.IndexExpr:
		return expression.Addressable
	case *ast.UnaryExpr:
		return expression.Operator == "*"
	default:
		return false
	}
}

func (c *Checker) checkGoCompositeLiteral(expr *ast.GoCompositeLiteralExpr) Type {
	result := c.resolveType(expr.Type)
	if result.Kind == Invalid {
		for _, field := range expr.Fields {
			c.checkExpression(field.Value)
		}
		return result
	}
	literalStructure := result
	if result.Kind == GoNamed && result.GoQualifier == "" {
		if object := goTypeNameObject(result.GoType); object != nil {
			if named := c.nativeTypes[object.Name()]; named != nil && !named.declaration.Alias {
				underlying := c.nativeDefinedUnderlying(named, result)
				if underlying.Kind == Struct {
					literalStructure = underlying
				}
			}
		}
	}
	if literalStructure.Kind == Struct {
		symbol := c.structs[literalStructure.Name]
		if symbol == nil {
			c.report(expr.Type.Span, fmt.Sprintf("unknown struct type %s", literalStructure.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		expr.ResolvedFieldNames = make([]string, len(expr.Fields))
		bindings := nativeStructBindings(symbol, literalStructure)
		seen := map[string]bool{}
		for index, field := range expr.Fields {
			if seen[field.Name] {
				c.report(field.Span, fmt.Sprintf("duplicate struct field %q", field.Name))
			}
			seen[field.Name] = true
			selected, exists := symbol.fields[field.Name]
			if !exists {
				c.report(field.Span, fmt.Sprintf("struct %s has no field %q", literalStructure.Name, field.Name))
				c.checkExpression(field.Value)
				continue
			}
			expr.ResolvedFieldNames[index] = selected.goName
			expr.Fields[index].ResolvedDeclaration = selected.declarationSpan
			expected := substituteNativeTypeParameters(selected.typeInfo, bindings)
			actual := c.checkExpressionExpectedSlot(&expr.Fields[index].Value, expected)
			c.requireAssignable(expected, actual, field.Value.GetSpan())
		}
		for name := range symbol.fields {
			if !seen[name] {
				c.report(expr.Span, fmt.Sprintf("struct literal %s is missing field %q", literalStructure.Name, name))
			}
		}
		return result
	}
	goType, ok := goTypeOf(result)
	if !ok {
		c.report(expr.Type.Span, fmt.Sprintf("type %s is not a Go struct type", result.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	structure, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Struct)
	if !ok {
		c.report(expr.Type.Span, fmt.Sprintf("Go type %s is not a struct", result.String()))
		for _, field := range expr.Fields {
			c.checkExpression(field.Value)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	seen := map[string]bool{}
	for index, field := range expr.Fields {
		if seen[field.Name] {
			c.report(field.Span, fmt.Sprintf("duplicate Go struct field %q", field.Name))
		}
		seen[field.Name] = true
		var selected *gotypes.Var
		for i := 0; i < structure.NumFields(); i++ {
			candidate := structure.Field(i)
			if candidate.Name() == field.Name {
				selected = candidate
				break
			}
		}
		if selected == nil {
			c.report(field.Span, fmt.Sprintf("Go struct %s has no field %q", result.String(), field.Name))
			c.checkExpression(field.Value)
			continue
		}
		if !selected.Exported() {
			c.report(field.Span, fmt.Sprintf("Go struct field %q is not exported", field.Name))
			c.checkExpression(field.Value)
			continue
		}
		expected, err := kinmokuseiTypeFromGo(selected.Type())
		if err != nil {
			c.report(field.Span, fmt.Sprintf("Go struct field %s.%s is not supported: %v", result.String(), field.Name, err))
			c.checkExpression(field.Value)
			continue
		}
		inheritGoQualifier(&expected, result)
		actual := c.checkExpressionExpectedSlot(&expr.Fields[index].Value, expected)
		c.requireAssignable(expected, actual, field.Value.GetSpan())
	}
	return result
}

func (c *Checker) checkIndex(expr *ast.IndexExpr, checked bool) Type {
	object := c.singleValue(c.checkExpression(expr.Object), expr.Object.GetSpan())
	if object.Kind == Nullable {
		c.report(expr.Object.GetSpan(), fmt.Sprintf("nullable value %s must be checked against null before indexing", object.String()))
		if object.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		object = *object.Element
	}
	index := c.singleValue(c.checkExpression(expr.Index), expr.Index.GetSpan())
	if object.Kind == Invalid {
		return object
	}
	switch object.Kind {
	case Array:
		c.checkSequenceIndex(expr.Index, index, -1, "array")
		expr.Addressable = true
		expr.Assignable = true
		if object.Element != nil {
			return c.checkedIndexResult(expr, *object.Element, checked, false)
		}
	case Map:
		expr.Assignable = true
		if object.Key != nil {
			c.checkNumericMaterialization(expr.Index, *object.Key)
			c.requireAssignable(*object.Key, index, expr.Index.GetSpan())
		}
		if object.Element != nil {
			return c.checkedIndexResult(expr, *object.Element, checked, true)
		}
	case String:
		c.checkSequenceIndex(expr.Index, index, c.constantStringLength(expr.Object), "string")
		return c.checkedIndexResult(expr, builtins["byte"], checked, false)
	}
	goType, ok := goTypeOf(object)
	if !ok {
		c.report(expr.Object.GetSpan(), fmt.Sprintf("type %s cannot be indexed", object.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	underlying := gotypes.Unalias(goType).Underlying()
	if pointer, pointerOK := underlying.(*gotypes.Pointer); pointerOK {
		if array, arrayOK := gotypes.Unalias(pointer.Elem()).Underlying().(*gotypes.Array); arrayOK {
			c.checkSequenceIndex(expr.Index, index, array.Len(), "array")
			expr.Addressable = true
			expr.Assignable = true
			return c.checkedIndexResult(expr, c.collectionElementType(array.Elem(), object, expr.Span), checked, false)
		}
	}
	switch collection := underlying.(type) {
	case *gotypes.Array:
		c.checkSequenceIndex(expr.Index, index, collection.Len(), "array")
		expr.Addressable = c.isAddressableExpression(expr.Object)
		expr.Assignable = expr.Addressable
		if object.Element != nil {
			return c.checkedIndexResult(expr, *object.Element, checked, false)
		}
		return c.checkedIndexResult(expr, c.collectionElementType(collection.Elem(), object, expr.Span), checked, false)
	case *gotypes.Slice:
		c.checkSequenceIndex(expr.Index, index, -1, "array")
		expr.Addressable = true
		expr.Assignable = true
		if object.Element != nil {
			return c.checkedIndexResult(expr, *object.Element, checked, false)
		}
		return c.checkedIndexResult(expr, c.collectionElementType(collection.Elem(), object, expr.Span), checked, false)
	case *gotypes.Map:
		expr.Assignable = true
		key := c.collectionElementType(collection.Key(), object, expr.Span)
		if object.Key != nil {
			key = *object.Key
		}
		c.checkNumericMaterialization(expr.Index, key)
		c.requireAssignable(key, index, expr.Index.GetSpan())
		if object.Element != nil {
			return c.checkedIndexResult(expr, *object.Element, checked, true)
		}
		return c.checkedIndexResult(expr, c.collectionElementType(collection.Elem(), object, expr.Span), checked, true)
	case *gotypes.Basic:
		if collection.Info()&gotypes.IsString != 0 {
			c.checkSequenceIndex(expr.Index, index, c.constantStringLength(expr.Object), "string")
			return c.checkedIndexResult(expr, builtins["byte"], checked, false)
		}
	}
	c.report(expr.Object.GetSpan(), fmt.Sprintf("type %s cannot be indexed", object.String()))
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) checkedIndexResult(expr *ast.IndexExpr, element Type, checked, mapIndex bool) Type {
	if !checked {
		return element
	}
	if !mapIndex {
		c.report(expr.Span, "checked index binding requires a map operand")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return Type{Kind: MultiValue, Name: "checked map lookup", Results: []Type{element, builtins["boolean"]}}
}

func (c *Checker) checkSlice(expr *ast.SliceExpr) Type {
	object := c.singleValue(c.checkExpression(expr.Object), expr.Object.GetSpan())
	if object.Kind == Nullable {
		c.report(expr.Object.GetSpan(), fmt.Sprintf("nullable value %s must be checked against null before slicing", object.String()))
		if object.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		object = *object.Element
	}
	for _, bound := range []struct {
		name       string
		expression ast.Expression
	}{{"low", expr.Low}, {"high", expr.High}, {"max", expr.Max}} {
		if bound.expression == nil {
			continue
		}
		value := c.singleValue(c.checkExpression(bound.expression), bound.expression.GetSpan())
		if value.Kind != Invalid && !c.isIntegerContext(bound.expression, value) {
			c.report(bound.expression.GetSpan(), fmt.Sprintf("slice %s bound must be an integer, got %s", bound.name, value.String()))
		}
		if constant, known := c.integerContextValue(bound.expression); known {
			if constant.Sign() < 0 {
				c.report(bound.expression.GetSpan(), fmt.Sprintf("slice %s bound cannot be negative", bound.name))
			} else if !constant.IsInt64() {
				c.report(bound.expression.GetSpan(), fmt.Sprintf("slice %s bound is out of range", bound.name))
			}
		}
	}
	if object.Kind == Invalid {
		return object
	}
	if object.Kind == Array {
		c.checkSliceConstantBounds(expr, -1)
		return object
	}
	if object.Kind == String {
		if expr.Full {
			c.report(expr.Span, "3-index slice cannot be used with string")
		}
		c.checkSliceConstantBounds(expr, -1)
		return object
	}
	goType, ok := goTypeOf(object)
	if !ok {
		c.report(expr.Object.GetSpan(), fmt.Sprintf("type %s cannot be sliced", object.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	underlying := gotypes.Unalias(goType).Underlying()
	var fixedLength int64 = -1
	if pointer, pointerOK := underlying.(*gotypes.Pointer); pointerOK {
		array, arrayOK := gotypes.Unalias(pointer.Elem()).Underlying().(*gotypes.Array)
		if !arrayOK {
			c.report(expr.Object.GetSpan(), fmt.Sprintf("type %s cannot be sliced", object.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		fixedLength = array.Len()
		element := c.collectionElementType(array.Elem(), object, expr.Span)
		c.checkSliceConstantBounds(expr, fixedLength)
		return Type{Kind: Array, Name: "array", Element: &element}
	}
	switch collection := underlying.(type) {
	case *gotypes.Array:
		if !c.isAddressableExpression(expr.Object) {
			c.report(expr.Object.GetSpan(), fmt.Sprintf("slicing fixed array %s requires an addressable operand", object.String()))
		}
		fixedLength = collection.Len()
		element := c.collectionElementType(collection.Elem(), object, expr.Span)
		c.checkSliceConstantBounds(expr, fixedLength)
		return Type{Kind: Array, Name: "array", Element: &element}
	case *gotypes.Slice:
		c.checkSliceConstantBounds(expr, fixedLength)
		return object
	case *gotypes.Basic:
		if collection.Info()&gotypes.IsString != 0 {
			if expr.Full {
				c.report(expr.Span, "3-index slice cannot be used with string")
			}
			c.checkSliceConstantBounds(expr, fixedLength)
			return object
		}
	}
	c.report(expr.Object.GetSpan(), fmt.Sprintf("type %s cannot be sliced", object.String()))
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) collectionElementType(goType gotypes.Type, owner Type, span source.Span) Type {
	element, err := kinmokuseiTypeFromGo(goType)
	if err != nil {
		c.report(span, fmt.Sprintf("collection element type is not supported: %v", err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	inheritGoQualifier(&element, owner)
	return element
}

func (c *Checker) checkSliceConstantBounds(expr *ast.SliceExpr, fixedLength int64) {
	constant := func(expression ast.Expression) (*big.Int, bool) {
		if expression == nil {
			return nil, false
		}
		value, ok := c.integerContextValue(expression)
		return value, ok && value.Sign() >= 0
	}
	low, lowOK := constant(expr.Low)
	high, highOK := constant(expr.High)
	max, maxOK := constant(expr.Max)
	if lowOK && highOK && low.Cmp(high) > 0 {
		c.report(expr.Span, "slice bounds are out of order: low exceeds high")
	}
	if highOK && maxOK && high.Cmp(max) > 0 {
		c.report(expr.Span, "slice bounds are out of order: high exceeds max")
	}
	if fixedLength < 0 {
		return
	}
	limit := big.NewInt(fixedLength)
	for _, bound := range []struct {
		name  string
		value *big.Int
		known bool
	}{{"low", low, lowOK}, {"high", high, highOK}, {"max", max, maxOK}} {
		if bound.known && bound.value.Cmp(limit) > 0 {
			c.report(expr.Span, fmt.Sprintf("slice %s bound %s exceeds fixed array length %d", bound.name, bound.value.String(), fixedLength))
		}
	}
}

func (c *Checker) checkBinary(expr *ast.BinaryExpr) Type {
	var left, right Type
	_, leftIsArrayLiteral := expr.Left.(*ast.ArrayLiteralExpr)
	_, rightIsArrayLiteral := expr.Right.(*ast.ArrayLiteralExpr)
	comparison := expr.Operator == "==" || expr.Operator == "===" || expr.Operator == "!=" || expr.Operator == "!=="
	if comparison && leftIsArrayLiteral && !rightIsArrayLiteral {
		right = c.singleValue(c.checkExpression(expr.Right), expr.Right.GetSpan())
		left = c.singleValue(c.checkExpressionExpected(expr.Left, right), expr.Left.GetSpan())
	} else {
		left = c.singleValue(c.checkExpression(expr.Left), expr.Left.GetSpan())
		if comparison && rightIsArrayLiteral {
			right = c.singleValue(c.checkExpressionExpected(expr.Right, left), expr.Right.GetSpan())
		} else {
			right = c.singleValue(c.checkExpression(expr.Right), expr.Right.GetSpan())
		}
	}
	return c.checkBinaryOperands(expr, left, right)
}

func (c *Checker) checkBinaryOperands(expr *ast.BinaryExpr, left, right Type) Type {
	if isComplexType(left) || isComplexType(right) || isUntypedGoNumeric(left) || isUntypedGoNumeric(right) {
		return c.checkComplexBinary(expr, left, right)
	}
	switch expr.Operator {
	case "+", "-", "*", "/", "%":
		if expr.Operator == "+" && left.IsAddable() && right.IsAddable() && sameType(left, right) && (!left.IsNumeric() || !right.IsNumeric()) {
			return left
		}
		if !left.IsNumeric() || !right.IsNumeric() {
			if left.Kind != Invalid && right.Kind != Invalid {
				c.report(expr.Span, fmt.Sprintf("operator %s requires numeric operands", expr.Operator))
			}
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if expr.Operator == "%" && (!left.IsInteger() || !right.IsInteger()) {
			c.report(expr.Span, "operator % requires integer operands")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if (expr.Operator == "/" || expr.Operator == "%") && left.IsInteger() && right.IsInteger() {
			if divisor, known := c.resolvedIntegerConstantValue(expr.Right); known && divisor.Sign() == 0 {
				c.report(expr.Right.GetSpan(), fmt.Sprintf("operator %s integer divisor cannot be zero", expr.Operator))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
		}
		if !sameType(left, right) {
			c.report(expr.Span, fmt.Sprintf("operator %s cannot mix %s and %s without an explicit conversion", expr.Operator, left.Name, right.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if right.Kind == UntypedInt && left.Kind != UntypedInt {
			if value, known := c.resolvedIntegerConstantValue(expr.Right); known && !integerConstantFitsFixedType(value, left) {
				c.report(expr.Right.GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", value.String(), left.String()))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
		}
		if left.Kind == UntypedInt {
			return right
		}
		return left
	case "&", "|", "^", "&^":
		if !left.IsInteger() || !right.IsInteger() {
			if left.Kind != Invalid && right.Kind != Invalid {
				c.report(expr.Span, fmt.Sprintf("operator %s requires integer operands", expr.Operator))
			}
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if !sameType(left, right) {
			c.report(expr.Span, fmt.Sprintf("operator %s cannot mix %s and %s without an explicit conversion", expr.Operator, left.Name, right.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if left.Kind == UntypedInt && right.Kind != UntypedInt {
			if value, known := c.resolvedIntegerConstantValue(expr.Left); known && !integerConstantFitsFixedType(value, right) {
				c.report(expr.Left.GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", value.String(), right.String()))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
		}
		if right.Kind == UntypedInt && left.Kind != UntypedInt {
			if value, known := c.resolvedIntegerConstantValue(expr.Right); known && !integerConstantFitsFixedType(value, left) {
				c.report(expr.Right.GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", value.String(), left.String()))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
		}
		if left.Kind == UntypedInt {
			return right
		}
		return left
	case "<<", ">>":
		if !left.IsInteger() || !right.IsInteger() {
			if left.Kind != Invalid && right.Kind != Invalid {
				c.report(expr.Span, fmt.Sprintf("operator %s requires integer operands", expr.Operator))
			}
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if amount, known := c.resolvedIntegerConstantValue(expr.Right); known && amount.Sign() < 0 {
			c.report(expr.Right.GetSpan(), fmt.Sprintf("operator %s shift amount cannot be negative", expr.Operator))
			return Type{Kind: Invalid, Name: "<invalid>"}
		} else if known && c.integerExpressionIsCompileTimeConstant(expr.Left) && (!amount.IsUint64() || amount.Uint64() > maximumGoConstantShift) {
			c.report(expr.Right.GetSpan(), fmt.Sprintf("operator %s constant shift amount exceeds the Go implementation limit of %d", expr.Operator, maximumGoConstantShift))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return left
	case "<", "<=", ">", ">=":
		if !left.IsOrdered() || !right.IsOrdered() || !sameType(left, right) {
			if left.Kind != Invalid && right.Kind != Invalid {
				c.report(expr.Span, fmt.Sprintf("operator %s requires operands of the same ordered type", expr.Operator))
			}
		}
		return builtins["boolean"]
	case "==", "===", "!=", "!==":
		if left.Kind == Nil && right.Kind == Nil {
			c.report(expr.Span, "cannot compare nil with nil without a concrete nilable type")
		} else if left.Kind == Null && right.Kind == Null {
			c.report(expr.Span, "cannot compare null with null without a concrete nullable type")
		} else if !sameType(left, right) && left.Kind != Invalid && right.Kind != Invalid {
			c.report(expr.Span, fmt.Sprintf("cannot compare %s and %s", left.String(), right.String()))
		} else if left.Kind != Nil && right.Kind != Nil && left.Kind != Null && right.Kind != Null && (!left.IsComparable() || !right.IsComparable()) {
			c.report(expr.Span, fmt.Sprintf("type %s is not comparable", left.String()))
		}
		return builtins["boolean"]
	case "&&", "||":
		if (left.Kind != Boolean || right.Kind != Boolean) && left.Kind != Invalid && right.Kind != Invalid {
			c.report(expr.Span, fmt.Sprintf("operator %s requires boolean operands", expr.Operator))
		}
		return builtins["boolean"]
	}
	return Type{Kind: Invalid, Name: "<invalid>"}
}
