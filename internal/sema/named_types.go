package sema

import (
	"fmt"
	gotoken "go/token"
	gotypes "go/types"
	"math/big"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) predeclareNamedTypes(program *ast.Program) {
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.ClassDecl:
			class := declaration
			if _, exists := c.classes[class.Name]; !exists {
				typeParameters, typeParamScope := c.declareTypeParameters(class.TypeParameters, "generic class")
				goNamed := newNativeGoNamed(class.Name, typeParameters, gotypes.NewStruct(nil, nil))
				c.classes[class.Name] = &classSymbol{
					declarationSpan: class.NameSpan, typeParameters: typeParameters, typeParamScope: typeParamScope, goNamed: goNamed,
				}
			}
		case *ast.InterfaceDecl:
			if _, exists := c.interfaces[declaration.Name]; !exists {
				typeParameters, typeParamScope := c.declareTypeParameters(declaration.TypeParameters, "generic interface")
				goNamed := newNativeGoNamed(declaration.Name, typeParameters, gotypes.NewInterfaceType(nil, nil).Complete())
				c.interfaces[declaration.Name] = &interfaceSymbol{
					methods: map[string]methodSymbol{}, typeParameters: typeParameters, typeParamScope: typeParamScope,
					declarationSpan: declaration.NameSpan, goNamed: goNamed,
				}
			}
		case *ast.StructDecl:
			if _, exists := c.structs[declaration.Name]; !exists {
				typeParameters, typeParamScope := c.declareTypeParameters(declaration.TypeParameters, "generic struct")
				fields := map[string]Type{}
				fieldNames := map[string]string{}
				goNamed := newNativeGoNamed(declaration.Name, typeParameters, nil)
				c.structs[declaration.Name] = &structSymbol{
					fields: map[string]fieldSymbol{}, methods: map[string]methodSymbol{}, declarationSpan: declaration.NameSpan,
					typeParameters: typeParameters, typeParamScope: typeParamScope, goNamed: goNamed,
					typeInfo: Type{Kind: Struct, Name: declaration.Name, Fields: fields, FieldNames: fieldNames, TypeParameters: typeParameters, Generic: len(typeParameters) != 0},
				}
			}
		case *ast.TypeDecl:
			if _, exists := c.nativeTypes[declaration.Name]; !exists {
				typeParameters, typeParamScope := c.declareDefinedTypeParameters(declaration)
				symbol := &nativeTypeSymbol{
					declaration: declaration, typeInfo: Type{Kind: Invalid, Name: declaration.Name}, methods: map[string]methodSymbol{},
					typeParameters: typeParameters, typeParamScope: typeParamScope,
				}
				if !declaration.Alias {
					object := gotypes.NewTypeName(gotoken.NoPos, nil, declaration.Name, nil)
					symbol.goNamed = gotypes.NewNamed(object, nil, nil)
					parameters := make([]*gotypes.TypeParam, 0, len(typeParameters))
					for _, parameter := range typeParameters {
						if goParameter, ok := parameter.GoType.(*gotypes.TypeParam); ok {
							parameters = append(parameters, goParameter)
						}
					}
					if len(parameters) == len(typeParameters) && len(parameters) != 0 {
						symbol.goNamed.SetTypeParams(parameters)
					}
					symbol.typeInfo = Type{
						Kind: GoNamed, Name: declaration.Name, GoType: symbol.goNamed,
						TypeParameters: typeParameters, Generic: len(typeParameters) != 0,
					}
				}
				c.nativeTypes[declaration.Name] = symbol
			}
		case *ast.EnumDecl:
			if _, exists := c.nativeTypes[declaration.Name]; !exists {
				typeDeclaration := &ast.TypeDecl{
					Name: declaration.Name, NameSpan: declaration.NameSpan, Underlying: declaration.Underlying, Span: declaration.Span,
				}
				c.nativeTypes[declaration.Name] = &nativeTypeSymbol{
					declaration: typeDeclaration, typeInfo: Type{Kind: Invalid, Name: declaration.Name}, methods: map[string]methodSymbol{},
				}
			}
			members := make(map[string]*ast.EnumMember, len(declaration.Members))
			for index := range declaration.Members {
				member := &declaration.Members[index]
				if _, duplicate := members[member.Name]; duplicate {
					c.report(member.Span, fmt.Sprintf("duplicate enum member %q", member.Name))
				} else {
					members[member.Name] = member
				}
			}
			c.enums[declaration.Name] = &enumSymbol{declaration: declaration, members: members}
		}
	}
}

func (c *Checker) predeclareInterfaceNames(program *ast.Program) {
	for _, declaration := range program.Declarations {
		decl, ok := declaration.(*ast.InterfaceDecl)
		if !ok || !decl.Constraint {
			continue
		}
		if _, exists := c.interfaces[decl.Name]; exists {
			continue
		}
		object := gotypes.NewTypeName(gotoken.NoPos, nil, decl.Name, nil)
		c.interfaces[decl.Name] = &interfaceSymbol{
			methods: map[string]methodSymbol{}, declarationSpan: decl.NameSpan,
			goNamed: gotypes.NewNamed(object, nil, nil), constraint: true,
			constraintDeclaration: decl,
		}
	}
	// Bind all constraint parameters while every constraint name already has
	// a stable identity. Bounds referencing source constraints finish later.
	for _, declaration := range program.Declarations {
		decl, ok := declaration.(*ast.InterfaceDecl)
		if !ok || !decl.Constraint {
			continue
		}
		symbol := c.interfaces[decl.Name]
		if symbol.constraintDeclaration != decl {
			continue
		}
		identities := append([]ast.TypeParameter(nil), decl.TypeParameters...)
		for i := range identities {
			identities[i].Constraint = nil
		}
		symbol.typeParameters, symbol.typeParamScope = c.declareTypeParameters(identities, "generic constraint")
		for _, parameter := range decl.TypeParameters {
			if identity, ok := symbol.typeParamScope[parameter.Name].GoType.(*gotypes.TypeParam); ok && parameter.Constraint != nil {
				c.deferredParameterBounds[identity] = true
			}
		}
		parameters := make([]*gotypes.TypeParam, len(symbol.typeParameters))
		for i, parameter := range symbol.typeParameters {
			parameters[i] = parameter.GoType.(*gotypes.TypeParam)
		}
		symbol.goNamed.SetTypeParams(parameters)
	}
}

func (c *Checker) declareNativeConstraints(program *ast.Program) {
	c.nativeConstraintsReady = true
	for _, declaration := range program.Declarations {
		decl, ok := declaration.(*ast.InterfaceDecl)
		if !ok || !decl.Constraint {
			continue
		}
		c.completeNativeConstraint(c.interfaces[decl.Name])
	}
}

func (c *Checker) completeNativeConstraint(symbol *interfaceSymbol) {
	if symbol == nil || symbol.goNamed == nil || symbol.goNamed.Underlying() != nil {
		return
	}
	decl := symbol.constraintDeclaration
	if decl == nil {
		return
	}
	if symbol.constraintResolving {
		c.report(decl.NameSpan, fmt.Sprintf("constraint declaration cycle involving %s", decl.Name))
		return
	}
	symbol.constraintResolving = true
	defer func() { symbol.constraintResolving = false }()
	// A dependency must resolve in its own declaration scope, never in the
	// scope of the class/function/constraint that happened to reference it.
	previousScopes := c.typeParameterScopes
	c.typeParameterScopes = nil
	defer func() { c.typeParameterScopes = previousScopes }()
	c.completeNativeTypeParameterBounds(decl.TypeParameters, symbol.typeParamScope, nil, true)
	c.pushTypeParameterScope(symbol.typeParamScope)
	defer c.popTypeParameterScope()
	terms := make([]*gotypes.Term, 0, len(decl.Terms))
	valid := true
	if len(decl.Terms) > 100 {
		c.report(decl.Span, "constraint declarations cannot contain more than 100 terms because the Go toolchain cannot compile larger unions")
		valid = false
	}
	for _, term := range decl.Terms {
		if candidates, shapes, handled := c.sourceConstraintTerms(term); handled {
			if len(candidates) == 0 {
				valid = false
			}
			for i, candidate := range candidates {
				for _, existing := range terms {
					if typeSetTermsOverlap(existing, candidate) {
						c.report(term.Span, fmt.Sprintf("constraint term %s overlaps an earlier term", formatTypeSetTermForDiagnostic(term)))
						valid = false
						break
					}
				}
				terms = append(terms, candidate)
				symbol.constraintTermTypes = append(symbol.constraintTermTypes, shapes[i])
			}
			continue
		}
		resolved := c.resolveType(term.Type)
		if resolved.Kind == Invalid {
			valid = false
			continue
		}
		goType, ok := goTypeOf(resolved)
		if !ok {
			goType, ok = c.goTypeForNativeStorage(resolved)
		}
		if !ok || goType == nil {
			c.report(term.Span, fmt.Sprintf("constraint term %s cannot be represented as a Go type", formatTypeRefForDiagnostic(term.Type)))
			valid = false
			continue
		}
		goType = gotypes.Unalias(goType)
		if underlyingGoInterface(goType) != nil {
			c.report(term.Span, fmt.Sprintf("constraint term %s must be a concrete type, not an interface", formatTypeRefForDiagnostic(term.Type)))
			valid = false
			continue
		}
		if term.Underlying && !gotypes.Identical(goType, goType.Underlying()) {
			c.report(term.Span, fmt.Sprintf("underlying constraint term ~%s must name its own underlying type", formatTypeRefForDiagnostic(term.Type)))
			valid = false
			continue
		}
		candidate := gotypes.NewTerm(term.Underlying, goType)
		for _, existing := range terms {
			if typeSetTermsOverlap(existing, candidate) {
				c.report(term.Span, fmt.Sprintf("constraint term %s overlaps an earlier term", formatTypeSetTermForDiagnostic(term)))
				valid = false
				break
			}
		}
		terms = append(terms, candidate)
		symbol.constraintTermTypes = append(symbol.constraintTermTypes, resolved)
	}
	if len(terms) > 100 && len(decl.Terms) <= 100 {
		c.report(decl.Span, "expanded constraint declarations cannot contain more than 100 terms because the Go toolchain cannot compile larger unions")
		valid = false
	}
	constraint := gotypes.NewInterfaceType(nil, nil)
	if valid && len(terms) != 0 {
		constraint = gotypes.NewInterfaceType(nil, []gotypes.Type{gotypes.NewUnion(terms)})
	}
	constraint.Complete()
	symbol.goNamed.SetUnderlying(constraint)
}

func typeSetTermsOverlap(left, right *gotypes.Term) bool {
	if left == nil || right == nil {
		return false
	}
	leftType, rightType := left.Type(), right.Type()
	if left.Tilde() && right.Tilde() {
		return gotypes.Identical(leftType, rightType)
	}
	if left.Tilde() {
		return gotypes.Identical(leftType, rightType.Underlying())
	}
	if right.Tilde() {
		return gotypes.Identical(rightType, leftType.Underlying())
	}
	return gotypes.Identical(leftType, rightType)
}

func formatTypeSetTermForDiagnostic(term ast.TypeSetTerm) string {
	prefix := ""
	if term.Underlying {
		prefix = "~"
	}
	return prefix + formatTypeRefForDiagnostic(term.Type)
}

func newNativeGoNamed(name string, typeParameters []Type, underlying gotypes.Type) *gotypes.Named {
	object := gotypes.NewTypeName(gotoken.NoPos, nil, name, nil)
	named := gotypes.NewNamed(object, underlying, nil)
	parameters := make([]*gotypes.TypeParam, 0, len(typeParameters))
	for _, parameter := range typeParameters {
		if goParameter, ok := parameter.GoType.(*gotypes.TypeParam); ok {
			parameters = append(parameters, goParameter)
		}
	}
	if len(parameters) == len(typeParameters) && len(parameters) != 0 {
		named.SetTypeParams(parameters)
	}
	return named
}

func (c *Checker) declareNativeTypes(program *ast.Program) {
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.TypeDecl:
			c.resolveNativeType(c.nativeTypes[declaration.Name])
		case *ast.EnumDecl:
			c.resolveNativeType(c.nativeTypes[declaration.Name])
		}
	}
}

func enumMemberGoName(enumName, memberName string) string {
	return enumName + memberGoName(memberName, ast.Public)
}

func (c *Checker) checkEnum(declaration *ast.EnumDecl) {
	symbol := c.nativeTypes[declaration.Name]
	typeInfo := c.resolveNativeType(symbol)
	underlying := c.resolveType(declaration.Underlying)
	if underlying.Kind != Invalid && !underlying.IsInteger() {
		c.report(declaration.Underlying.Span, fmt.Sprintf("enum underlying type must be an integer type, got %s", underlying.String()))
	}
	if len(declaration.Members) == 0 {
		c.report(declaration.Span, "enum must declare at least one member")
		return
	}
	previous := big.NewInt(-1)
	for index := range declaration.Members {
		member := &declaration.Members[index]
		value := new(big.Int)
		if member.Value == nil {
			value.Add(previous, big.NewInt(1))
		} else {
			actual := c.checkExpressionExpectedSlot(&member.Value, typeInfo)
			c.requireAssignable(typeInfo, actual, member.Value.GetSpan())
			resolved, known := c.resolvedIntegerConstantValue(member.Value)
			if !known || !c.integerExpressionIsCompileTimeConstant(member.Value) {
				c.report(member.Value.GetSpan(), "enum initializer must be an integer constant expression without enum-member references")
				continue
			}
			value.Set(resolved)
		}
		if typeInfo.Kind != Invalid && !integerConstantFitsFixedType(value, typeInfo) {
			c.report(member.Span, fmt.Sprintf("enum value %s cannot be represented as %s", value.String(), declaration.Name))
		}
		member.ResolvedValue = value.String()
		previous.Set(value)
	}
}

func (c *Checker) resolveNativeType(symbol *nativeTypeSymbol) Type {
	if symbol == nil {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if symbol.state == 2 {
		return symbol.typeInfo
	}
	if symbol.state == 1 {
		if !symbol.declaration.Alias && symbol.goNamed != nil && c.nativeTypeIndirectionDepth > 0 {
			return symbol.typeInfo
		}
		c.report(symbol.declaration.Underlying.Span, fmt.Sprintf("type declaration cycle contains %q", symbol.declaration.Name))
		symbol.typeInfo = Type{Kind: Invalid, Name: symbol.declaration.Name}
		return symbol.typeInfo
	}
	symbol.state = 1
	c.pushTypeParameterScope(symbol.typeParamScope)
	underlying := c.resolveType(symbol.declaration.Underlying)
	c.popTypeParameterScope()
	symbol.underlying = underlying
	if !symbol.declaration.Alias && underlying.Kind == Struct && underlying.GoType == nil && !c.structGoTypesFinalized {
		symbol.state = 0
		return symbol.typeInfo
	}
	if !symbol.declaration.Alias && underlying.Kind == GoNamed && !c.structGoTypesFinalized {
		if object := goTypeNameObject(underlying.GoType); object != nil {
			if dependency := c.nativeTypes[object.Name()]; dependency != nil && dependency.state != 2 {
				symbol.state = 0
				return symbol.typeInfo
			}
		}
	}
	invalidBoundary := underlying.Kind == Invalid || underlying.Kind == Void || underlying.Kind == MultiValue || underlying.Kind == Result || underlying.Kind == Task || underlying.Kind == GoPackage || underlying.Kind == GoTypeName || underlying.Kind == Nil || underlying.Kind == Null
	if underlying.Kind == TypeParameter && !symbol.declaration.Alias {
		c.report(symbol.declaration.Underlying.Span, "a generic defined type cannot use a type parameter directly as its underlying type; wrap it in a slice, array, map, pointer, or other concrete type")
		invalidBoundary = true
	}
	if invalidBoundary {
		if underlying.Kind != Invalid && underlying.Kind != TypeParameter {
			c.report(symbol.declaration.Underlying.Span, fmt.Sprintf("type %s cannot be used as the underlying type of %s", underlying.String(), symbol.declaration.Name))
		}
		symbol.typeInfo = Type{Kind: Invalid, Name: symbol.declaration.Name}
	} else if symbol.declaration.Alias {
		symbol.typeInfo = underlying
	} else {
		underlyingGo, ok := goTypeOf(underlying)
		if !ok {
			c.report(symbol.declaration.Underlying.Span, fmt.Sprintf("type %s cannot yet be used as a distinct defined type; use an alias or a native struct", underlying.String()))
			symbol.typeInfo = Type{Kind: Invalid, Name: symbol.declaration.Name}
		} else {
			named := symbol.goNamed
			if named == nil {
				object := gotypes.NewTypeName(gotoken.NoPos, nil, symbol.declaration.Name, nil)
				named = gotypes.NewNamed(object, nil, nil)
				symbol.goNamed = named
			}
			if len(symbol.typeParameters) != 0 {
				parameters := make([]*gotypes.TypeParam, len(symbol.typeParameters))
				for index, parameter := range symbol.typeParameters {
					goParameter, ok := parameter.GoType.(*gotypes.TypeParam)
					if !ok {
						c.report(symbol.declaration.TypeParameters[index].Span, "generic defined type parameter could not be represented in Go")
						symbol.typeInfo = Type{Kind: Invalid, Name: symbol.declaration.Name}
						symbol.state = 2
						return symbol.typeInfo
					}
					parameters[index] = goParameter
				}
				if named.TypeParams().Len() == 0 {
					named.SetTypeParams(parameters)
				}
			}
			named.SetUnderlying(gotypes.Unalias(underlyingGo).Underlying())
			symbol.typeInfo = Type{
				Kind: GoNamed, Name: symbol.declaration.Name, GoType: named,
				TypeParameters: symbol.typeParameters, Generic: len(symbol.typeParameters) != 0,
			}
		}
	}
	symbol.state = 2
	return symbol.typeInfo
}

func (c *Checker) resolveNativeDefinedType(ref ast.TypeRef, symbol *nativeTypeSymbol) Type {
	base := c.resolveNativeType(symbol)
	if base.Kind == Invalid {
		return base
	}
	want := len(symbol.typeParameters)
	got := len(ref.GenericArguments)
	if want == 0 {
		if got != 0 {
			c.report(ref.Span, fmt.Sprintf("defined type %s is not generic", ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return base
	}
	if got != want {
		c.report(ref.Span, fmt.Sprintf("generic defined type %s expects %d type arguments, got %d", ref.Name, want, got))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	arguments := make([]Type, got)
	valid := true
	for index := range ref.GenericArguments {
		arguments[index] = c.resolveType(ref.GenericArguments[index])
		argument := arguments[index]
		if argument.Kind == Invalid {
			valid = false
			continue
		}
		if argument.Kind == Void || argument.Kind == Result || argument.Kind == Task || argument.Kind == MultiValue || argument.Kind == GoPackage || argument.Kind == GoTypeName || argument.Kind == Nil || argument.Kind == Null {
			c.report(ref.GenericArguments[index].Span, fmt.Sprintf("type %s cannot be used as a generic defined type argument", argument.String()))
			valid = false
			continue
		}
	}
	if !valid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if symbol.declaration.Alias {
		if !c.validateNativeTypeArguments(symbol.typeParameters, arguments, ref.GenericArguments, ref.Span, "generic alias "+ref.Name) {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		bindings := make(nativeTypeBindings, len(symbol.typeParameters))
		for index, parameter := range symbol.typeParameters {
			bindings[parameter.GoType] = arguments[index]
		}
		return substituteNativeTypeParameters(base, bindings)
	}
	goArguments := make([]gotypes.Type, got)
	for index, argument := range arguments {
		goArgument, ok := goTypeOf(argument)
		if !ok {
			c.report(ref.GenericArguments[index].Span, fmt.Sprintf("type %s cannot yet be used as a generic defined type argument", argument.String()))
			valid = false
			continue
		}
		goArguments[index] = goArgument
	}
	if !valid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	instantiated, err := gotypes.Instantiate(nil, base.GoType, goArguments, true)
	if err != nil {
		c.report(ref.Span, fmt.Sprintf("cannot instantiate generic defined type %s: %v", ref.Name, err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	names := make([]string, len(arguments))
	for index := range arguments {
		names[index] = arguments[index].String()
	}
	return Type{
		Kind: GoNamed, Name: ref.Name + "<" + strings.Join(names, ", ") + ">", GoType: instantiated,
		TypeArguments: arguments,
	}
}
