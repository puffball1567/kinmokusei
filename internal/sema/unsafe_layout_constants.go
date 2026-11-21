package sema

import (
	"fmt"
	goast "go/ast"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Ask Go about the checked storage type, not the source operand's runtime
// value. Layout builtins do not execute their arguments. Go also determines
// whether a type parameter (including one inside an array/struct) makes the
// result nonconstant; pointer/slice headers can still have constant sizes.
func (c *Checker) checkUnsafeLayoutConstant(call *ast.CallExpr, name string, value Type) {
	if value.Kind == Invalid || len(call.Arguments) != 1 {
		return
	}
	pkg := gotypes.NewPackage("kinmokusei.synthetic/layout", "layout")
	pkg.Scope().Insert(gotypes.Unsafe.Scope().Lookup(name))
	var operand goast.Expr = goast.NewIdent("value")
	if name == "Offsetof" {
		field, ok := call.Arguments[0].(*ast.MemberExpr)
		if !ok || c.goFieldReceivers[field] == nil {
			return
		}
		// Reuse the already checked receiver; checking the expression twice
		// would duplicate dependency, capture and nullable-flow effects.
		receiver, selected := c.goFieldReceivers[field], field.ResolvedName
		if !field.GoField {
			// Source fields may be package-private. The probe is a different
			// synthetic package, so project the already-authorized field onto
			// a layout-identical struct with exported names. Preserve field
			// order, types and tags; never apply this to imported Go members.
			var ok bool
			receiver, ok = sourceLayoutProbe(receiver, field.EmbeddedPath, selected)
			if !ok {
				return
			}
			selected = "Selected"
		}
		pkg.Scope().Insert(gotypes.NewVar(0, pkg, "value", receiver))
		operand = &goast.SelectorExpr{X: operand, Sel: goast.NewIdent(selected)}
	} else if info, known := c.scalarConstant(call.Arguments[0]); known {
		// Keep untyped values intact so Go validates their default type,
		// including overflow in Sizeof(1 << 100) and 32-bit int boundaries.
		pkg.Scope().Insert(gotypes.NewConst(0, pkg, "value", info.Type, info.Value))
	} else {
		storage, ok := c.goTypeForNativeStorage(value)
		if !ok {
			return
		}
		pkg.Scope().Insert(gotypes.NewVar(0, pkg, "value", storage))
	}
	node := &goast.CallExpr{Fun: goast.NewIdent(name), Args: []goast.Expr{operand}}
	// Unlike CheckExpr, this checks layout with the selected target's Sizes.
	info, err := c.targetNumericProbe(pkg, node, false, false)
	if err != nil {
		c.report(call.Arguments[0].GetSpan(), err.Error())
		return
	}
	if c.constantValues == nil {
		c.constantValues = map[ast.Expression]gotypes.TypeAndValue{}
	}
	c.constantValues[call] = info
	call.GoConstant = info.Value != nil
}

// Retain the storage receiver without setting GoField: native fields must keep
// their source-level nullable-flow and identity behavior.
func (c *Checker) recordLayoutFieldReceiver(field *ast.MemberExpr, receiver Type) {
	storage, ok := c.goTypeForNativeStorage(receiver)
	if !ok {
		return
	}
	if c.goFieldReceivers == nil {
		c.goFieldReceivers = map[*ast.MemberExpr]gotypes.Type{}
	}
	c.goFieldReceivers[field] = storage
}

func sourceLayoutProbe(receiver gotypes.Type, path []string, name string) (*gotypes.Struct, bool) {
	underlying := receiver.Underlying()
	if pointer, ok := underlying.(*gotypes.Pointer); ok {
		underlying = pointer.Elem().Underlying()
	}
	structure, ok := underlying.(*gotypes.Struct)
	if !ok {
		return nil, false
	}
	fields := make([]*gotypes.Var, structure.NumFields())
	tags := make([]string, len(fields))
	found := false
	for i := range fields {
		field := structure.Field(i)
		exported, storage, embedded := fmt.Sprintf("Field%d", i), field.Type(), false
		switch {
		case len(path) == 0 && field.Name() == generatedIdentifier(name):
			exported, found = "Selected", true
		case len(path) != 0 && field.Name() == generatedIdentifier(path[0]):
			inner, ok := sourceLayoutProbe(storage, path[1:], name)
			if !ok {
				return nil, false
			}
			exported = fmt.Sprintf("Embedded%d", len(path))
			storage = gotypes.NewNamed(gotypes.NewTypeName(0, nil, exported, nil), inner, nil)
			embedded, found = true, true
		}
		fields[i] = gotypes.NewField(0, nil, exported, storage, embedded)
		tags[i] = structure.Tag(i)
	}
	return gotypes.NewStruct(fields, tags), found
}
