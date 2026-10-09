package sema

import (
	"fmt"
	"go/token"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Method expressions use the type's method set, not the addressable value
// selector rules. Their first argument is the explicit receiver, including for
// promoted methods and interface methods.
func (c *Checker) checkGoMethodExpression(expr *ast.MemberExpr, receiver Type) Type {
	invalid := Type{Kind: Invalid, Name: "<invalid>"}
	if receiver.GoType == nil {
		return invalid
	}
	base := gotypes.Unalias(receiver.GoType)
	if pointer, ok := base.(*gotypes.Pointer); ok {
		base = gotypes.Unalias(pointer.Elem())
	}
	if named, ok := base.(*gotypes.Named); ok && named.TypeParams().Len() != 0 && named.TypeArgs().Len() == 0 {
		c.report(expr.Span, "Go method expressions require an instantiated receiver type")
		return invalid
	}
	if contract, ok := base.Underlying().(*gotypes.Interface); ok && !contract.IsMethodSet() {
		c.report(expr.Span, "Go constraint interfaces cannot be used as method expression receivers")
		return invalid
	}
	selection := gotypes.NewMethodSet(receiver.GoType).Lookup(nil, expr.Name)
	if selection == nil || !selection.Obj().Exported() {
		c.report(expr.NameSpan, fmt.Sprintf("Go type %s has no exported method %q in its method set", receiver.String(), expr.Name))
		return invalid
	}
	signature := selection.Type().(*gotypes.Signature)
	parameters := make([]*gotypes.Var, 0, signature.Params().Len()+1)
	parameters = append(parameters, gotypes.NewVar(token.NoPos, nil, "receiver", receiver.GoType))
	for index := 0; index < signature.Params().Len(); index++ {
		parameters = append(parameters, signature.Params().At(index))
	}
	explicit := gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(parameters...), signature.Results(), signature.Variadic())
	result, err := kinmokuseiFunctionFromGo(explicit)
	if err != nil {
		c.report(expr.Span, fmt.Sprintf("Go method expression %s.%s is not supported: %v", receiver.String(), expr.Name, err))
		return invalid
	}
	if !c.allowUnsafeGo && goTypeContainsUnsafePointer(explicit, nil) {
		c.report(expr.Span, `Go method expression uses unsafe.Pointer; set [go.interop] unsafe = "allow" to use it`)
		return invalid
	}
	inheritGoQualifier(&result, receiver)
	expr.Go = true
	expr.ResolvedName = expr.Name
	return result
}

func (c *Checker) recordGoTypeExpression(expr ast.Expression, receiver Type) {
	if receiver.Kind != GoTypeName {
		return
	}
	if ref, ok := goMethodExpressionReceiverRef(receiver.GoType, receiver.GoQualifier, expr.GetSpan()); ok {
		if c.goTypeExpressions == nil {
			c.goTypeExpressions = map[source.Span]ast.TypeRef{}
		}
		c.goTypeExpressions[expr.GetSpan()] = ref
	}
}

func goMethodExpressionReceiverRef(goType gotypes.Type, qualifier string, span source.Span) (ast.TypeRef, bool) {
	if pointer, ok := goType.(*gotypes.Pointer); ok {
		pointee, found := goMethodExpressionReceiverRef(pointer.Elem(), qualifier, span)
		return ast.TypeRef{Pointee: &pointee, Span: span}, found
	}
	object := goTypeNameObject(goType)
	if object == nil || qualifier == "" {
		return ast.TypeRef{}, false
	}
	if named, ok := gotypes.Unalias(goType).(*gotypes.Named); ok && named.TypeParams().Len() != 0 && named.TypeArgs().Len() == 0 {
		return ast.TypeRef{}, false
	}
	if contract, ok := goType.Underlying().(*gotypes.Interface); ok && !contract.IsMethodSet() {
		return ast.TypeRef{}, false
	}
	return ast.TypeRef{Go: true, Name: object.Name(), Qualifier: qualifier, Span: span}, true
}
