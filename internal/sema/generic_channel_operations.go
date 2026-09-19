package sema

import (
	goast "go/ast"
	gotoken "go/token"
	gotypes "go/types"
)

// Preserve the source element (not just its Go storage), including class
// identity and nullability. Direction unions need not have one core type.
func (c *Checker) genericChannelElement(parameter *gotypes.TypeParam, send bool) (Type, bool) {
	terms := c.collectionTerms(parameter)
	var element Type
	var storage gotypes.Type
	for i, term := range terms {
		shape := c.constraintArgumentShape(term)
		goType, ok := c.goTypeForNativeStorage(shape)
		if !ok {
			return Type{}, false
		}
		channel, ok := goType.Underlying().(*gotypes.Chan)
		if !ok || send && channel.Dir() == gotypes.RecvOnly || !send && channel.Dir() == gotypes.SendOnly {
			return Type{}, false
		}
		next, err := kinmokuseiTypeFromGo(channel.Elem())
		if err != nil {
			return Type{}, false
		}
		if shape.Element != nil {
			next = *shape.Element
		}
		next = c.restoreNativeRangeType(next)
		if i != 0 && (!gotypes.Identical(storage, channel.Elem()) || !c.identicalCollectionElement(element, next)) {
			return Type{}, false
		}
		element, storage = next, channel.Elem()
	}
	if len(terms) == 0 {
		return Type{}, false
	}
	// Check the complete bound too: normalized shapes must not make an empty
	// intersection or an otherwise invalid Go operation appear usable.
	pkg := gotypes.NewPackage("kinmokusei.synthetic/channel", "channelcheck")
	pkg.Scope().Insert(gotypes.NewVar(gotoken.NoPos, pkg, "channel", parameter))
	pkg.Scope().Insert(gotypes.NewVar(gotoken.NoPos, pkg, "element", storage))
	pkg.MarkComplete()
	var probe goast.Expr = &goast.UnaryExpr{Op: gotoken.ARROW, X: goast.NewIdent("channel")}
	if send {
		probe = &goast.FuncLit{
			Type: &goast.FuncType{Params: &goast.FieldList{}},
			Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.SendStmt{Chan: goast.NewIdent("channel"), Value: goast.NewIdent("element")},
			}},
		}
	}
	if err := gotypes.CheckExpr(gotoken.NewFileSet(), pkg, gotoken.NoPos, probe, nil); err != nil {
		return Type{}, false
	}
	return element, true
}
