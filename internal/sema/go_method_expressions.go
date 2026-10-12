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
	explicit := goExplicitReceiverSignature(receiver.GoType, selection)
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

func goExplicitReceiverSignature(receiver gotypes.Type, selection *gotypes.Selection) *gotypes.Signature {
	signature := selection.Type().(*gotypes.Signature)
	parameters := make([]*gotypes.Var, 0, signature.Params().Len()+1)
	parameters = append(parameters, gotypes.NewVar(token.NoPos, nil, "receiver", receiver))
	for index := 0; index < signature.Params().Len(); index++ {
		parameters = append(parameters, signature.Params().At(index))
	}
	return gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(parameters...), signature.Results(), signature.Variadic())
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
		if c.goTypeExpressionMethods == nil {
			c.goTypeExpressionMethods = map[source.Span]map[string]ast.CallableSignature{}
		}
		methods := map[string]ast.CallableSignature{}
		set := gotypes.NewMethodSet(receiver.GoType)
		for i := 0; i < set.Len(); i++ {
			selection := set.At(i)
			if !selection.Obj().Exported() {
				continue
			}
			explicit := goExplicitReceiverSignature(receiver.GoType, selection)
			callable, err := kinmokuseiFunctionFromGo(explicit)
			if err != nil || !c.allowUnsafeGo && goTypeContainsUnsafePointer(explicit, nil) {
				continue
			}
			inheritGoQualifier(&callable, receiver)
			call := &ast.CallExpr{Span: expr.GetSpan()}
			c.recordCallSignature(call, callable)
			methods[selection.Obj().Name()] = *call.Signature
		}
		c.goTypeExpressionMethods[expr.GetSpan()] = methods
	}
}

func (c *Checker) checkGoTypeInstantiationExpression(expr *ast.IndexExpr, base Type) Type {
	object := goTypeNameObject(base.GoType)
	if object == nil || base.GoQualifier == "" {
		c.report(expr.Span, "explicit type instantiation requires a named Go type")
		return Type{Kind: Invalid}
	}
	ref := ast.TypeRef{Go: true, Name: object.Name(), Qualifier: base.GoQualifier, GenericArguments: expr.TypeArguments, Span: expr.Span}
	// Reuse ordinary type resolution, including constraints, unsafe-pointer
	// policy and the prohibition on erasing verified text through Go storage.
	receiver := c.resolveType(ref)
	if receiver.Kind == Invalid {
		return receiver
	}
	receiver.Kind = GoTypeName
	expr.Instantiated, expr.ResolvedType = true, ref
	c.recordGoTypeExpression(expr, receiver)
	return receiver
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
	ref := ast.TypeRef{Go: true, Name: object.Name(), Qualifier: qualifier, Span: span}
	var arguments *gotypes.TypeList
	switch t := goType.(type) {
	case *gotypes.Named:
		arguments = t.TypeArgs()
	case *gotypes.Alias:
		arguments = t.TypeArgs()
	}
	if arguments != nil {
		for i := 0; i < arguments.Len(); i++ {
			argument, err := kinmokuseiTypeFromGo(arguments.At(i))
			if err != nil {
				return ast.TypeRef{}, false
			}
			inheritGoQualifier(&argument, Type{GoType: goType, GoQualifier: qualifier})
			ref.GenericArguments = append(ref.GenericArguments, typeRefFromType(argument, span))
		}
	}
	return ref, true
}
