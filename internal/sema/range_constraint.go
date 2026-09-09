package sema

import (
	goast "go/ast"
	gotoken "go/token"
	gotypes "go/types"
)

// goRangeCoreType finds the common underlying type without treating interface
// embeddings (intersections) as unions. Satisfies checks the entire constraint
// against each candidate, including embedded constraints and method sets.
func goRangeCoreType(parameter *gotypes.TypeParam) gotypes.Type {
	// A range probe rejects unrestricted and structurally empty type sets. The
	// candidate checks below additionally keep the Go 1.23 common-type rule,
	// even when the compiler itself runs on a newer Go toolchain.
	pkg := gotypes.NewPackage("kinmokusei.synthetic/range", "rangecheck")
	pkg.Scope().Insert(gotypes.NewVar(gotoken.NoPos, pkg, "value", parameter))
	pkg.MarkComplete()
	probe := &goast.FuncLit{
		Type: &goast.FuncType{Params: &goast.FieldList{}},
		Body: &goast.BlockStmt{List: []goast.Stmt{
			&goast.RangeStmt{X: goast.NewIdent("value"), Body: &goast.BlockStmt{}},
		}},
	}
	if err := gotypes.CheckExpr(gotoken.NewFileSet(), pkg, gotoken.NoPos, probe, nil); err != nil {
		return nil
	}
	var candidates []gotypes.Type
	seen := map[gotypes.Type]bool{}
	var collect func(gotypes.Type)
	collect = func(value gotypes.Type) {
		value = gotypes.Unalias(value)
		if seen[value] {
			return
		}
		seen[value] = true
		switch value := value.(type) {
		case *gotypes.TypeParam:
			collect(value.Constraint())
		case *gotypes.Named:
			collect(value.Underlying())
		case *gotypes.Interface:
			for i := 0; i < value.NumEmbeddeds(); i++ {
				collect(value.EmbeddedType(i))
			}
		case *gotypes.Union:
			for i := 0; i < value.Len(); i++ {
				collect(value.Term(i).Type())
			}
		default:
			candidates = append(candidates, value)
		}
	}
	collect(parameter)
	accepts := func(terms ...*gotypes.Term) bool {
		constraint := gotypes.NewInterfaceType(nil, []gotypes.Type{gotypes.NewUnion(terms)})
		constraint.Complete()
		return gotypes.Satisfies(parameter, constraint)
	}
	for _, candidate := range candidates {
		if accepts(gotypes.NewTerm(true, candidate)) {
			return candidate
		}
		if channel, ok := candidate.(*gotypes.Chan); ok && channel.Dir() != gotypes.SendOnly {
			receive := gotypes.NewChan(gotypes.RecvOnly, channel.Elem())
			both := gotypes.NewChan(gotypes.SendRecv, channel.Elem())
			if accepts(gotypes.NewTerm(true, receive), gotypes.NewTerm(true, both)) {
				return receive
			}
		}
	}
	return nil
}

// Go constraint terms describe native classes using their storage pointers.
// Restore declaration identity before checking source-level member access and
// assignments; otherwise a class element would become an unrelated Go pointer.
func (c *Checker) restoreNativeRangeType(value Type) Type {
	for i := range value.TypeArguments {
		value.TypeArguments[i] = c.restoreNativeRangeType(value.TypeArguments[i])
	}
	for _, child := range []**Type{&value.Element, &value.Key, &value.Result} {
		if *child != nil {
			restored := c.restoreNativeRangeType(**child)
			*child = &restored
		}
	}
	for i := range value.Parameters {
		value.Parameters[i] = c.restoreNativeRangeType(value.Parameters[i])
	}
	if value.Kind == GoPointer && value.GoType != nil {
		if pointer, ok := gotypes.Unalias(value.GoType).(*gotypes.Pointer); ok {
			if named, ok := gotypes.Unalias(pointer.Elem()).(*gotypes.Named); ok {
				if symbol := c.classes[named.Obj().Name()]; symbol != nil && named.Origin() == symbol.goNamed && value.Element != nil {
					return Type{Kind: Class, Name: named.Obj().Name(), TypeArguments: value.Element.TypeArguments}
				}
			}
		}
	}
	if value.Kind == GoNamed && value.GoType != nil {
		if named, ok := gotypes.Unalias(value.GoType).(*gotypes.Named); ok {
			name := named.Obj().Name()
			if symbol := c.interfaces[name]; symbol != nil && !symbol.constraint && named.Origin() == symbol.goNamed {
				return Type{Kind: Interface, Name: name, TypeArguments: value.TypeArguments}
			}
			if symbol := c.structs[name]; symbol != nil && named.Origin() == symbol.goNamed {
				bindings := make(nativeTypeBindings, len(value.TypeArguments))
				for i, argument := range value.TypeArguments {
					bindings[symbol.typeParameters[i].GoType] = argument
				}
				result := substituteNativeTypeParameters(symbol.typeInfo, bindings)
				result.GoType, result.TypeArguments = value.GoType, value.TypeArguments
				result.Generic, result.TypeParameters = false, nil
				return result
			}
		}
	}
	return value
}
