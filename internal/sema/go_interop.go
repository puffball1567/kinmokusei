package sema

import (
	"fmt"
	gotypes "go/types"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) declareGoPackages(program *ast.Program) {
	namedPaths := map[string]bool{}
	for _, declaration := range program.Imports {
		if declaration.Go && len(declaration.Names) != 0 {
			namedPaths[declaration.Path] = true
		}
	}
	for i := range program.Imports {
		declaration := &program.Imports[i]
		if !declaration.Go {
			continue
		}
		if len(declaration.Names) != 0 && declaration.Alias == "" {
			declaration.Alias = ast.GoImportAlias(declaration.Path)
		}
		if namedPaths[declaration.Path] {
			declaration.ResolvedAlias = ast.GoImportAlias(declaration.Path)
		}
		if declaration.Alias == "_" {
			c.report(declaration.Span, "Go package alias '_' cannot be used as a namespace")
			continue
		}
		alias := declaration.Alias
		if declaration.ResolvedAlias != "" {
			alias = declaration.ResolvedAlias
		}
		byAlias := c.goPackages[declaration.Span.Path]
		if byAlias == nil {
			byAlias = map[string]*goPackageSymbol{}
			c.goPackages[declaration.Span.Path] = byAlias
		}
		if previous, duplicate := byAlias[alias]; duplicate && (previous.path != declaration.Path || len(previous.declaration.Names) == 0 && len(declaration.Names) == 0) {
			c.report(declaration.Span, fmt.Sprintf("duplicate Go package alias %q", declaration.Alias))
			continue
		}
		if previous := byAlias[declaration.Alias]; len(declaration.Names) == 0 && previous != nil && previous.path != declaration.Path {
			c.report(declaration.Span, fmt.Sprintf("duplicate Go package alias %q", declaration.Alias))
			continue
		}
		if declaration.Path == "unsafe" && !c.allowUnsafeGo {
			c.report(declaration.PathSpan, `Go package "unsafe" requires [go.interop] unsafe = "allow"`)
			continue
		}
		if strings.HasPrefix(declaration.Path, "internal/") || strings.Contains(declaration.Path, "/internal/") {
			c.report(declaration.PathSpan, fmt.Sprintf("Go package %q is not available in current Go interop", declaration.Path))
			continue
		}
		packageInfo, err := c.goImporter.Import(declaration.Path)
		if err != nil {
			c.report(declaration.PathSpan, fmt.Sprintf("cannot load Go package %q: %v", declaration.Path, err))
			continue
		}
		if packageInfo.Name() == "main" {
			c.report(declaration.PathSpan, fmt.Sprintf("Go package %q is not available in current Go interop", declaration.Path))
			continue
		}
		imported := &goPackageSymbol{path: declaration.Path, declaration: declaration, packageInfo: packageInfo}
		// Keep an explicit namespace declaration as the canonical navigation
		// target when the same file also selects individual exports.
		if byAlias[alias] == nil || len(declaration.Names) == 0 {
			byAlias[alias] = imported
		}
		if len(declaration.Names) == 0 {
			byAlias[declaration.Alias] = imported
		}
		if len(declaration.Names) != 0 {
			c.declareNamedGoImports(imported, program)
		}
	}
}

func (c *Checker) lookupGoPackage(path, alias string) *goPackageSymbol {
	return c.goPackages[path][alias]
}

func (c *Checker) checkGoMember(expression *ast.MemberExpr, imported *goPackageSymbol) Type {
	expression.Go = true
	if imported == nil || imported.packageInfo == nil {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	object := imported.packageInfo.Scope().Lookup(expression.Name)
	if object == nil || !object.Exported() {
		c.report(expression.Span, fmt.Sprintf("Go package %q has no exported member %q", imported.path, expression.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	var result Type
	var err error
	switch object := object.(type) {
	case *gotypes.Const:
		result, err = kinmokuseiTypeFromGo(object.Type())
		expression.Constant = true
		if basic, ok := object.Type().(*gotypes.Basic); ok && basic.Info()&gotypes.IsUntyped != 0 && basic.Info()&(gotypes.IsFloat|gotypes.IsComplex) != 0 {
			result = Type{Kind: GoBasic, Name: basic.Name(), GoType: basic}
			if c.numericValues == nil {
				c.numericValues = map[ast.Expression]gotypes.TypeAndValue{}
			}
			c.numericValues[expression] = gotypes.TypeAndValue{Type: object.Type(), Value: object.Val()}
		}
	case *gotypes.Func:
		result, err = kinmokuseiFunctionFromGo(object.Type().(*gotypes.Signature))
	case *gotypes.Var:
		result, err = kinmokuseiTypeFromGo(object.Type())
		expression.Addressable = true
	case *gotypes.TypeName:
		result = Type{Kind: GoTypeName, Name: imported.declaration.Alias + "." + object.Name(), GoType: object.Type()}
	default:
		err = fmt.Errorf("%s symbols are not supported", goObjectKind(object))
	}
	if err != nil {
		c.report(expression.Span, fmt.Sprintf("Go member %s.%s is not supported: %v", expression.Object.(*ast.IdentifierExpr).Name, expression.Name, err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !c.allowUnsafeGo && goTypeContainsUnsafePointer(result.GoType, nil) {
		c.report(expression.Span, fmt.Sprintf(`Go member %s.%s uses unsafe.Pointer; set [go.interop] unsafe = "allow" to use it`, expression.Object.(*ast.IdentifierExpr).Name, expression.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	imported.declaration.Used = true
	alias := imported.declaration.Alias
	if imported.declaration.ResolvedAlias != "" {
		alias = imported.declaration.ResolvedAlias
	}
	applyGoQualifier(&result, imported.path, alias)
	expression.ResolvedName = expression.Name
	return result
}

func (c *Checker) checkGoValueMember(expression *ast.MemberExpr, receiver Type) Type {
	expression.Go = true
	if receiver.Kind == GoInterface {
		for _, method := range receiver.GoMethods {
			if method.Name == expression.Name {
				if !c.allowUnsafeGo && goTypeContainsUnsafePointer(method.Type.GoType, nil) {
					c.report(expression.Span, "Go method uses unsafe.Pointer; set [go.interop] unsafe = \"allow\" to use it")
					return Type{Kind: Invalid, Name: "<invalid>"}
				}
				expression.ResolvedName = method.Name
				return method.Type
			}
		}
		c.report(expression.Span, fmt.Sprintf("Go type %s has no exported member %q", receiver.String(), expression.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	addressable := c.isAddressableExpression(expression.Object)
	object, index, indirect := gotypes.LookupFieldOrMethod(receiver.GoType, addressable, nil, expression.Name)
	if object == nil {
		if !addressable {
			if candidate, _, _ := gotypes.LookupFieldOrMethod(receiver.GoType, true, nil, expression.Name); candidate != nil {
				if _, method := candidate.(*gotypes.Func); method {
					c.report(expression.Span, fmt.Sprintf("pointer method %q requires an addressable %s value", expression.Name, receiver.String()))
					return Type{Kind: Invalid, Name: "<invalid>"}
				}
			}
		}
		c.report(expression.Span, fmt.Sprintf("Go type %s has no exported member %q", receiver.String(), expression.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !object.Exported() {
		c.report(expression.Span, fmt.Sprintf("Go member %s.%s is not exported", receiver.String(), expression.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	var result Type
	var err error
	switch object := object.(type) {
	case *gotypes.Var:
		result, err = kinmokuseiTypeFromGo(object.Type())
		expression.Addressable = addressable || indirect || goTypeIsPointer(receiver.GoType)
		expression.GoField = true
		expression.GoFieldViaPointer = goFieldEmbeddedViaPointer(receiver.GoType, index)
	case *gotypes.Func:
		result, err = kinmokuseiFunctionFromGo(object.Type().(*gotypes.Signature))
	default:
		err = fmt.Errorf("member kind %T is not supported", object)
	}
	if err != nil {
		c.report(expression.Span, fmt.Sprintf("Go member %s.%s is not supported: %v", receiver.String(), expression.Name, err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !c.allowUnsafeGo && goTypeContainsUnsafePointer(result.GoType, nil) {
		c.report(expression.Span, fmt.Sprintf(`Go member %s.%s uses unsafe.Pointer; set [go.interop] unsafe = "allow" to use it`, receiver.String(), expression.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	inheritGoQualifier(&result, receiver)
	expression.ResolvedName = expression.Name
	return result
}

func goFieldEmbeddedViaPointer(receiver gotypes.Type, index []int) bool {
	current := gotypes.Unalias(receiver)
	if pointer, ok := current.Underlying().(*gotypes.Pointer); ok {
		current = gotypes.Unalias(pointer.Elem())
	}
	for depth, fieldIndex := range index {
		structure, ok := current.Underlying().(*gotypes.Struct)
		if !ok || fieldIndex < 0 || fieldIndex >= structure.NumFields() {
			return false
		}
		field := structure.Field(fieldIndex)
		if depth == len(index)-1 {
			return false
		}
		current = gotypes.Unalias(field.Type())
		if _, ok := current.Underlying().(*gotypes.Pointer); ok {
			return true
		}
	}
	return false
}

func goTypeIsPointer(goType gotypes.Type) bool {
	if goType == nil {
		return false
	}
	_, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Pointer)
	return ok
}

func applyGoQualifier(t *Type, packagePath, alias string) {
	if t == nil {
		return
	}
	if goTypePackagePath(t.GoType) == packagePath {
		t.GoQualifier = alias
	}
	for i := range t.Parameters {
		applyGoQualifier(&t.Parameters[i], packagePath, alias)
	}
	for i := range t.TypeArguments {
		applyGoQualifier(&t.TypeArguments[i], packagePath, alias)
	}
	for i := range t.GoFields {
		applyGoQualifier(&t.GoFields[i].Type, packagePath, alias)
	}
	for i := range t.GoMethods {
		applyGoQualifier(&t.GoMethods[i].Type, packagePath, alias)
	}
	for i := range t.Results {
		applyGoQualifier(&t.Results[i], packagePath, alias)
	}
	applyGoQualifier(t.Result, packagePath, alias)
	applyGoQualifier(t.Element, packagePath, alias)
	applyGoQualifier(t.Key, packagePath, alias)
}

func inheritGoQualifier(target *Type, source Type) {
	if target == nil || source.GoQualifier == "" {
		return
	}
	packagePath := goTypePackagePath(source.GoType)
	if packagePath == "" && source.Element != nil {
		packagePath = goTypePackagePath(source.Element.GoType)
	}
	if packagePath != "" {
		applyGoQualifier(target, packagePath, source.GoQualifier)
	}
}

func (c *Checker) prepareGoTypeForEmission(t *Type, span source.Span) {
	if t == nil || t.Kind == Invalid {
		return
	}
	if t.Kind == GoNamed && t.GoQualifier == "" {
		packagePath := goTypePackagePath(t.GoType)
		if packagePath != "" {
			for _, imported := range c.goPackages[span.Path] {
				if imported.path != packagePath {
					continue
				}
				applyGoQualifier(t, packagePath, resolvedGoPackageAlias(imported))
				imported.declaration.Used = true
				break
			}
			if t.GoQualifier == "" {
				c.report(span, fmt.Sprintf("Go type %s requires an explicit import go alias for %q when emitted as an inferred type", t.String(), packagePath))
			}
		}
	}
	for i := range t.Parameters {
		c.prepareGoTypeForEmission(&t.Parameters[i], span)
	}
	for i := range t.TypeArguments {
		c.prepareGoTypeForEmission(&t.TypeArguments[i], span)
	}
	for i := range t.GoFields {
		c.prepareGoTypeForEmission(&t.GoFields[i].Type, span)
	}
	for i := range t.GoMethods {
		c.prepareGoTypeForEmission(&t.GoMethods[i].Type, span)
	}
	for i := range t.Results {
		c.prepareGoTypeForEmission(&t.Results[i], span)
	}
	c.prepareGoTypeForEmission(t.Result, span)
	c.prepareGoTypeForEmission(t.Element, span)
	c.prepareGoTypeForEmission(t.Key, span)
}

func goTypePackagePath(goType gotypes.Type) string {
	if goType == nil {
		return ""
	}
	if basic, ok := goType.(*gotypes.Basic); ok && basic.Kind() == gotypes.UnsafePointer {
		return "unsafe"
	}
	if alias, ok := goType.(*gotypes.Alias); ok {
		if object := alias.Obj(); object != nil && object.Pkg() != nil {
			return object.Pkg().Path()
		}
	}
	switch goType := gotypes.Unalias(goType).(type) {
	case *gotypes.Named:
		if object := goType.Obj(); object != nil && object.Pkg() != nil {
			return object.Pkg().Path()
		}
	case *gotypes.Pointer:
		return goTypePackagePath(goType.Elem())
	}
	return ""
}

func goObjectKind(object gotypes.Object) string {
	switch object.(type) {
	case *gotypes.TypeName:
		return "type"
	case *gotypes.Var:
		return "variable"
	default:
		return "Go"
	}
}
