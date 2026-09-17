package sema

import (
	"fmt"
	goast "go/ast"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Validate every type-set member, without range's common-underlying-type rule.
// Synthetic variables only describe types: source operands are never evaluated.
func genericCollectionOperation(name string, values ...gotypes.Type) bool {
	pkg := gotypes.NewPackage("kinmokusei.synthetic/collection", "collection")
	arguments := make([]goast.Expr, len(values))
	for i, value := range values {
		name := fmt.Sprintf("arg%d", i)
		pkg.Scope().Insert(gotypes.NewVar(0, pkg, name, value))
		arguments[i] = goast.NewIdent(name)
	}
	pkg.MarkComplete()
	_, err := evalNumericGo(pkg, &goast.CallExpr{Fun: goast.NewIdent(name), Args: arguments})
	return err == nil
}

// delete needs a common key, not a common map value type. Keep this separate
// from range/index metadata so enabling deletion cannot enable invalid indexing.
func (c *Checker) parameterDeleteKey(parameter *gotypes.TypeParam) (Type, bool) {
	if key, ok := c.parameterDeleteKeys[parameter]; ok {
		return key, key.Kind != Invalid
	}
	key := Type{Kind: Invalid}
	operand, problem := c.normalizeImportedConstraint(parameter.Constraint(), nil, nil, map[gotypes.Type]bool{})
	if problem == "" && len(operand.terms) != 0 {
		if mapping, ok := gotypes.Unalias(operand.terms[0].Type()).Underlying().(*gotypes.Map); ok && genericCollectionOperation("delete", parameter, mapping.Key()) {
			if converted, err := kinmokuseiTypeFromGo(mapping.Key()); err == nil {
				key = c.restoreNativeRangeType(converted)
			}
		}
	}
	if c.parameterDeleteKeys == nil {
		c.parameterDeleteKeys = map[*gotypes.TypeParam]Type{}
	}
	c.parameterDeleteKeys[parameter] = key
	return key, key.Kind != Invalid
}

// Preserve source-only qualifiers erased by Go storage, also for map unions
// whose differing value types prevent recording a common range shape.
func (c *Checker) recordParameterDeleteKey(parameter *gotypes.TypeParam, ref ast.TypeRef) {
	delete(c.parameterDeleteKeys, parameter)
	key, ok := c.parameterDeleteKey(parameter)
	if !ok {
		return
	}
	symbol := c.interfaces[ref.Name]
	if ref.Qualifier != "" || symbol == nil || !symbol.constraint || len(ref.GenericArguments) != len(symbol.typeParameters) {
		return
	}
	bindings := make(nativeTypeBindings, len(symbol.typeParameters))
	for i, argument := range ref.GenericArguments {
		bindings[symbol.typeParameters[i].GoType] = c.resolveType(argument)
	}
	var sourceKey *Type
	for _, term := range symbol.constraintTermTypes {
		shape := c.constraintArgumentShape(substituteNativeTypeParameters(term, bindings))
		if shape.Kind != Map || shape.Key == nil {
			continue
		}
		if sourceKey != nil && !sameConstraintNullability(*sourceKey, *shape.Key) {
			c.parameterDeleteKeys[parameter] = Type{Kind: Invalid}
			return
		}
		sourceKey = shape.Key
	}
	if sourceKey != nil {
		key = *sourceKey
	}
	c.parameterDeleteKeys[parameter] = key
}
