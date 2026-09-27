package sema

import (
	goast "go/ast"
	gotoken "go/token"
	gotypes "go/types"
)

// CheckExpr has no Sizes option and always assumes a 64-bit int. Preserve its
// context-free handling of untyped runtime shifts, then check materialized
// expressions with the selected target. No source expression is executed.
func (c *Checker) evalNumericGo(pkg *gotypes.Package, expr goast.Expr) (gotypes.TypeAndValue, error) {
	value, err := evalNumericGo(pkg, expr)
	if c.goSizes == nil || c.goSizes.Sizeof(gotypes.Typ[gotypes.Int]) == 8 {
		return value, err
	}
	if err == nil && value.Value == nil {
		if basic, ok := value.Type.(*gotypes.Basic); ok && basic.Info()&gotypes.IsUntyped != 0 {
			// The eventual assignment/argument context supplies the type. A var
			// probe here would default to int and reject valid uint64 shifts.
			return value, nil
		}
		return c.targetNumericProbe(pkg, expr, false, isVoidGoExpression(value))
	}
	constant, constantErr := c.targetNumericProbe(pkg, expr, true, false)
	if err == nil || constantErr == nil {
		return constant, constantErr
	}
	// A 64-bit intermediate can overflow a conversion which is valid on a
	// 32-bit target (e.g. uint32(^uint(0))). Retry in value/statement contexts
	// too, so runtime expressions containing that constant are not rejected.
	runtimeValue, runtimeErr := c.targetNumericProbe(pkg, expr, false, false)
	if runtimeErr == nil {
		return runtimeValue, nil
	}
	if _, call := expr.(*goast.CallExpr); call {
		if statement, statementErr := c.targetNumericProbe(pkg, expr, false, true); statementErr == nil {
			return statement, nil
		}
	}
	return value, err
}

func isVoidGoExpression(value gotypes.TypeAndValue) bool {
	tuple, ok := value.Type.(*gotypes.Tuple)
	return ok && tuple.Len() == 0
}

func (c *Checker) targetNumericProbe(scope *gotypes.Package, expr goast.Expr, constant, statement bool) (gotypes.TypeAndValue, error) {
	pkg := gotypes.NewPackage("kinmokusei.synthetic/target", "target")
	if scope != nil {
		for _, name := range scope.Scope().Names() {
			pkg.Scope().Insert(scope.Scope().Lookup(name))
		}
	}
	var declaration goast.Decl
	if statement {
		declaration = &goast.FuncDecl{Name: goast.NewIdent("_"), Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ExprStmt{X: expr}}}}
	} else {
		kind := gotoken.VAR
		if constant {
			kind = gotoken.CONST
		}
		declaration = &goast.GenDecl{Tok: kind, Specs: []goast.Spec{&goast.ValueSpec{Names: []*goast.Ident{goast.NewIdent("_")}, Values: []goast.Expr{expr}}}}
	}
	// Keep a valid file extent even for expression trees carrying positions.
	// go/types consults it when checking version-dependent nested calls.
	fset := gotoken.NewFileSet()
	end := int(expr.End())
	if end < 1 {
		end = 1
	}
	positions := fset.AddFile("<numeric>", 1, end+1)
	file := &goast.File{Name: goast.NewIdent("target"), Decls: []goast.Decl{declaration}, FileStart: positions.Pos(0), FileEnd: positions.Pos(end + 1)}
	info := &gotypes.Info{Types: map[goast.Expr]gotypes.TypeAndValue{}}
	checker := gotypes.NewChecker(&gotypes.Config{Sizes: c.goSizes}, fset, pkg, info)
	err := checker.Files([]*goast.File{file})
	return info.Types[expr], err
}
