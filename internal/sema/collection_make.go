package sema

import (
	"fmt"
	goast "go/ast"
	gotoken "go/token"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Unlike the element-oriented constructors, make preserves the requested
// collection type, including named types and constrained type parameters.
func (c *Checker) checkMakeCollection(expr *ast.CallExpr) Type {
	expr.Builtin = ast.MakeCall
	if expr.Expanded {
		c.report(expr.Span, "make does not accept spread arguments")
	}
	if len(expr.TypeArguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("make expects one collection type argument, got %d", len(expr.TypeArguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	target := c.resolveType(expr.TypeArguments[0])
	storage, valid := c.goTypeForNativeStorage(target)
	slice := false
	if valid && target.Kind != Nullable {
		slice, valid = c.makeCollectionShape(storage)
	} else {
		valid = false
	}
	if !valid && target.Kind != Invalid {
		c.report(expr.TypeArguments[0].Span, fmt.Sprintf("make target must be a slice, map, or channel with a compatible underlying type, got %s", target.String()))
	}
	minimum, maximum := 0, 1
	if slice {
		minimum, maximum = 1, 2
	}
	c.checkMakeSizeArguments(expr, "make", minimum, maximum)
	if slice && len(expr.Arguments) == 2 {
		length, lengthOK := c.integerContextValue(expr.Arguments[0])
		capacity, capacityOK := c.integerContextValue(expr.Arguments[1])
		if lengthOK && capacityOK && capacity.Cmp(length) < 0 {
			c.report(expr.Arguments[1].GetSpan(), "make capacity cannot be smaller than length")
		}
	}
	if !valid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	expr.MakeSliceTarget = slice
	if slice {
		expr.Signature = &ast.CallableSignature{ParameterNames: []string{"length", "capacity?"}, ParameterTypes: []string{"int", "int"}, Result: target.String()}
	} else {
		expr.Signature = &ast.CallableSignature{ParameterNames: []string{"capacity?"}, ParameterTypes: []string{"int"}, Result: target.String()}
	}
	return target
}

func (c *Checker) makeCollectionShape(target gotypes.Type) (slice, valid bool) {
	shape := gotypes.Unalias(target).Underlying()
	if parameter, ok := gotypes.Unalias(target).(*gotypes.TypeParam); ok {
		terms := c.collectionTerms(parameter)
		if len(terms) == 0 {
			return false, false
		}
		storage, ok := c.goTypeForNativeStorage(terms[0])
		if !ok {
			return false, false
		}
		shape = gotypes.Unalias(storage).Underlying()
	}
	switch shape.(type) {
	case *gotypes.Slice:
		slice = true
	case *gotypes.Map, *gotypes.Chan:
	default:
		return false, false
	}
	// Probe the complete type set, not just the representative term. In
	// particular, channel directions must be compatible, and slice/map bounds
	// must have one underlying type. A constant probe never evaluates source.
	pkg := gotypes.NewPackage("kinmokusei.synthetic/make", "makecheck")
	pkg.Scope().Insert(gotypes.NewTypeName(gotoken.NoPos, pkg, "T", target))
	pkg.MarkComplete()
	_, err := evalNumericGo(pkg, &goast.CallExpr{Fun: goast.NewIdent("make"), Args: []goast.Expr{
		goast.NewIdent("T"), &goast.BasicLit{Kind: gotoken.INT, Value: "1"},
	}})
	return slice, err == nil
}
