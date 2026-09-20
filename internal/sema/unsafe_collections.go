package sema

import (
	goast "go/ast"
	gotoken "go/token"
	gotypes "go/types"
)

// unsafe.Slice and SliceData require a common pointer/slice shape, including
// type-parameter bounds. A range probe cannot validate pointers to scalars.
// Check the actual Go operation and retain source element contracts separately:
// Go storage alone cannot distinguish C from C | null.
func (c *Checker) unsafeSequenceElement(value Type, pointer bool) (Type, bool) {
	if value.Kind == Nullable && value.Element != nil {
		value = *value.Element
	}
	storage, ok := c.goTypeForNativeStorage(value)
	if !ok {
		return Type{}, false
	}
	terms := []Type{value}
	if parameter, generic := storage.(*gotypes.TypeParam); generic && value.Kind == TypeParameter {
		terms = c.collectionTerms(parameter)
	}
	kind, name := Array, "SliceData"
	if pointer {
		kind, name = GoPointer, "Slice"
	}
	var element Type
	for index, term := range terms {
		shape := c.constraintArgumentShape(term)
		if shape.Kind != kind || shape.Element == nil {
			return Type{}, false
		}
		if index != 0 && !c.identicalCollectionElement(element, *shape.Element) {
			return Type{}, false
		}
		element = *shape.Element
	}
	if len(terms) == 0 {
		return Type{}, false
	}
	pkg := gotypes.NewPackage("kinmokusei.synthetic/unsafe", "unsafecheck")
	pkg.Scope().Insert(gotypes.NewPkgName(0, pkg, "unsafe", gotypes.Unsafe))
	pkg.Scope().Insert(gotypes.NewVar(0, pkg, "value", storage))
	pkg.MarkComplete()
	arguments := []goast.Expr{goast.NewIdent("value")}
	if pointer {
		arguments = append(arguments, &goast.BasicLit{Kind: gotoken.INT, Value: "1"})
	}
	call := &goast.CallExpr{Fun: &goast.SelectorExpr{X: goast.NewIdent("unsafe"), Sel: goast.NewIdent(name)}, Args: arguments}
	if _, err := evalNumericGo(pkg, call); err != nil {
		return Type{}, false
	}
	inheritGoQualifier(&element, value)
	return element, true
}
