package sema

import (
	"fmt"
	gotoken "go/token"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

type boundInstance struct {
	origin    gotypes.Type
	arguments []gotypes.Type
	ref       ast.TypeRef
}

// Install every declaration identity before resolving bounds. Dependencies are
// completed first, so Map[K, V] sees K's comparable bound even if K comes later.
// Recursive references inside a constraint argument may use the provisional
// identity; a bare type-parameter bound is rejected by constraint resolution.
func (c *Checker) completeNativeTypeParameterBounds(parameters []ast.TypeParameter, scope map[string]Type, comparable map[string]bool, deferredOnly bool) {
	c.pushTypeParameterScope(scope)
	defer c.popTypeParameterScope()
	// Validate generic constraint applications only after all bounds exist.
	// For example, T extends Equal<T> can itself establish that T is comparable,
	// which Equal's parameter requires. Checking the provisional any rejects it.
	previousInstances := c.pendingBoundInstances
	var instances []boundInstance
	c.pendingBoundInstances = &instances
	defer func() { c.pendingBoundInstances = previousInstances }()
	if c.deferredParameterBounds == nil {
		c.deferredParameterBounds = map[*gotypes.TypeParam]bool{}
	}
	declarations := map[string]ast.TypeParameter{}
	for _, parameter := range parameters {
		if _, exists := declarations[parameter.Name]; !exists {
			declarations[parameter.Name] = parameter
		}
	}
	state := map[string]uint8{}
	var complete func(string)
	complete = func(name string) {
		parameter, exists := declarations[name]
		identity, ok := scope[name].GoType.(*gotypes.TypeParam)
		if !exists || !ok || state[name] != 0 {
			return
		}
		state[name] = 1
		if parameter.Constraint == nil || deferredOnly && !c.deferredParameterBounds[identity] {
			state[name] = 2
			return
		}
		deferred := c.nativeTypeParameterConstraintIsDeferred(*parameter.Constraint)
		visitTypeParameterReferences(*parameter.Constraint, func(dependency string) {
			complete(dependency)
			if other, ok := scope[dependency].GoType.(*gotypes.TypeParam); ok && c.deferredParameterBounds[other] {
				deferred = true
			}
		})
		if deferred && !deferredOnly {
			c.deferredParameterBounds[identity] = true
			state[name] = 2
			return
		}
		bound, valid := c.resolveNativeTypeParameterConstraint(*parameter.Constraint)
		if !valid {
			bound = gotypes.NewInterfaceType(nil, nil).Complete()
		}
		if comparable[name] && bound != gotypes.Universe.Lookup("comparable").Type() {
			bound = gotypes.NewInterfaceType(nil, []gotypes.Type{bound, gotypes.Universe.Lookup("comparable").Type()}).Complete()
		}
		identity.SetConstraint(bound)
		if valid {
			c.recordParameterRangeShape(identity, *parameter.Constraint)
		}
		delete(c.deferredParameterBounds, identity)
		state[name] = 2
	}
	for _, parameter := range parameters {
		complete(parameter.Name)
	}
	for _, instance := range instances {
		if _, err := gotypes.Instantiate(nil, instance.origin, instance.arguments, true); err != nil {
			if instance.ref.Qualifier == "" {
				c.report(instance.ref.Span, fmt.Sprintf("cannot instantiate constraint %s: %v", instance.ref.Name, err))
			} else {
				c.report(instance.ref.Span, fmt.Sprintf("cannot instantiate Go type %s.%s: %v", instance.ref.Qualifier, instance.ref.Name, err))
			}
		}
	}
}

func visitTypeParameterReferences(ref ast.TypeRef, visit func(string)) {
	if ref.Qualifier == "" && ref.Name != "" {
		visit(ref.Name)
	}
	for _, child := range []*ast.TypeRef{ref.Element, ref.Pointee, ref.Return} {
		if child != nil {
			visitTypeParameterReferences(*child, visit)
		}
	}
	for _, group := range [][]ast.TypeRef{ref.GenericArguments, ref.Parameters} {
		for _, child := range group {
			visitTypeParameterReferences(child, visit)
		}
	}
	for _, field := range ref.ObjectFields {
		visitTypeParameterReferences(field.Type, visit)
	}
}

// Substitute the whole constraint graph by declaration identity, not by name.
// Named declarations remain nominal: only their arguments are substituted, not
// their underlying types. This also bounds traversal of recursive constraints.
func substituteConstraintType(value gotypes.Type, bindings map[gotypes.Type]gotypes.Type) gotypes.Type {
	if value == nil || len(bindings) == 0 {
		return value
	}
	value = gotypes.Unalias(value)
	if replacement := bindings[value]; replacement != nil {
		return replacement
	}
	sub := func(t gotypes.Type) gotypes.Type { return substituteConstraintType(t, bindings) }
	switch value := value.(type) {
	case *gotypes.Pointer:
		return gotypes.NewPointer(sub(value.Elem()))
	case *gotypes.Slice:
		return gotypes.NewSlice(sub(value.Elem()))
	case *gotypes.Array:
		return gotypes.NewArray(sub(value.Elem()), value.Len())
	case *gotypes.Map:
		return gotypes.NewMap(sub(value.Key()), sub(value.Elem()))
	case *gotypes.Chan:
		return gotypes.NewChan(value.Dir(), sub(value.Elem()))
	case *gotypes.Named:
		if value.TypeArgs().Len() == 0 {
			return value
		}
		arguments := make([]gotypes.Type, value.TypeArgs().Len())
		for i := range arguments {
			arguments[i] = sub(value.TypeArgs().At(i))
		}
		// Argument satisfaction is checked after simultaneous substitution.
		instantiated, err := gotypes.Instantiate(nil, value.Origin(), arguments, false)
		if err == nil {
			return instantiated
		}
	case *gotypes.Interface:
		methods := make([]*gotypes.Func, value.NumExplicitMethods())
		for i := range methods {
			method := value.ExplicitMethod(i)
			methods[i] = gotypes.NewFunc(method.Pos(), method.Pkg(), method.Name(), sub(method.Type()).(*gotypes.Signature))
		}
		embedded := make([]gotypes.Type, value.NumEmbeddeds())
		for i := range embedded {
			embedded[i] = sub(value.EmbeddedType(i))
		}
		return gotypes.NewInterfaceType(methods, embedded).Complete()
	case *gotypes.Union:
		terms := make([]*gotypes.Term, value.Len())
		for i := range terms {
			terms[i] = gotypes.NewTerm(value.Term(i).Tilde(), sub(value.Term(i).Type()))
		}
		return gotypes.NewUnion(terms)
	case *gotypes.Signature:
		return gotypes.NewSignatureType(nil, nil, nil, sub(value.Params()).(*gotypes.Tuple), sub(value.Results()).(*gotypes.Tuple), value.Variadic())
	case *gotypes.Tuple:
		variables := make([]*gotypes.Var, value.Len())
		for i := range variables {
			variable := value.At(i)
			variables[i] = gotypes.NewVar(variable.Pos(), variable.Pkg(), variable.Name(), sub(variable.Type()))
		}
		return gotypes.NewTuple(variables...)
	case *gotypes.Struct:
		fields := make([]*gotypes.Var, value.NumFields())
		tags := make([]string, len(fields))
		for i := range fields {
			field := value.Field(i)
			fields[i] = gotypes.NewField(field.Pos(), field.Pkg(), field.Name(), sub(field.Type()), field.Embedded())
			tags[i] = value.Tag(i)
		}
		return gotypes.NewStruct(fields, tags)
	}
	return value
}

// Only unresolved parameters are inferred here. Known native bindings form a
// synthetic call; Go's constraint inference solves the dependent equations.
// The probe never evaluates user expressions or changes runtime representation.
func (c *Checker) inferNativeConstraintArguments(parameters []Type, bindings nativeTypeBindings) {
	missing := false
	for _, parameter := range parameters {
		missing = missing || bindings[parameter.GoType].Kind == Invalid
	}
	if !missing {
		return
	}
	// Infer from source shapes first: Go storage erases native nullability.
	// The Go probe remains responsible for the other constraint equations.
	for pass := 0; pass < len(parameters); pass++ {
		before := 0
		for _, parameter := range parameters {
			if bindings[parameter.GoType].Kind != Invalid {
				before++
			}
		}
		for _, parameter := range parameters {
			if value := bindings[parameter.GoType]; value.Kind != Invalid {
				if shape, ok := c.parameterRangeShape(parameter.GoType.(*gotypes.TypeParam)); ok {
					_ = c.inferNativeTypeArguments(shape, c.constraintArgumentShape(value), bindings)
				}
			}
		}
		after := 0
		for _, parameter := range parameters {
			if bindings[parameter.GoType].Kind != Invalid {
				after++
			}
		}
		if before == after {
			break
		}
	}
	missing = false
	for _, parameter := range parameters {
		missing = missing || bindings[parameter.GoType].Kind == Invalid
	}
	if !missing {
		return
	}
	clones := make([]*gotypes.TypeParam, len(parameters))
	replacements := map[gotypes.Type]gotypes.Type{}
	for i, parameter := range parameters {
		clones[i] = gotypes.NewTypeParam(gotypes.NewTypeName(gotoken.NoPos, nil, parameter.Name, nil), nil)
		replacements[parameter.GoType] = clones[i]
	}
	for i, parameter := range parameters {
		clones[i].SetConstraint(substituteConstraintType(parameter.GoType.(*gotypes.TypeParam).Constraint(), replacements))
	}
	var formals []*gotypes.Var
	var actuals []Type
	results := make([]*gotypes.Var, len(parameters))
	for i, parameter := range parameters {
		results[i] = gotypes.NewVar(gotoken.NoPos, nil, "", clones[i])
		if value := bindings[parameter.GoType]; value.Kind != Invalid {
			storage, ok := c.goTypeForNativeStorage(value)
			if !ok {
				return
			}
			formals = append(formals, gotypes.NewVar(gotoken.NoPos, nil, "", clones[i]))
			actuals = append(actuals, Type{Kind: GoNamed, GoType: storage})
		}
	}
	signature := gotypes.NewSignatureType(nil, nil, clones, gotypes.NewTuple(formals...), gotypes.NewTuple(results...), false)
	instance, err := inferGoGenericCall(signature, actuals, nil, false, nil)
	if err != nil {
		return
	}
	for i, parameter := range parameters {
		if bindings[parameter.GoType].Kind == Invalid {
			if value, err := kinmokuseiTypeFromGo(instance.Results().At(i).Type()); err == nil {
				bindings[parameter.GoType] = c.restoreNativeRangeType(value)
			}
		}
	}
}
