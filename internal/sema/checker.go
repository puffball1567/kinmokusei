package sema

import (
	"fmt"
	goast "go/ast"
	"go/constant"
	"go/importer"
	gotoken "go/token"
	gotypes "go/types"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/puffball1567/onsentamago/internal/ast"
	"github.com/puffball1567/onsentamago/internal/diagnostic"
	"github.com/puffball1567/onsentamago/internal/source"
)

type functionSymbol struct {
	parameters         []Type
	typeParameters     []Type
	typeParameterScope map[string]Type
	variadic           bool
	result             Type
	span               source.Span
	declarationSpan    source.Span
}

type valueSymbol struct {
	typeInfo              Type
	declaredType          Type
	flowInvalidated       source.Span
	flowInvalidationCause string
	flowEscaped           bool
	constant              bool
	declarationSpan       source.Span
	declaration           *ast.VariableDecl
	multiDeclaration      *ast.MultiVariableDecl
	multiIndex            int
	rangeBinding          *ast.RangeBinding
	selectCase            *ast.SelectCase
	selectIndex           int
	typeSwitchCase        *ast.TypeSwitchCase
	catchClause           *ast.CatchClause
	taskState             uint8
}

const (
	taskNotTracked uint8 = iota
	taskPending
	taskConsumed
	taskMaybeConsumed
)

// go/types rejects constant shifts above this implementation bound. Diagnose
// the same restriction while the OnsenTamago source span is still available.
const maximumGoConstantShift = 1074

type fieldSymbol struct {
	typeInfo        Type
	visibility      ast.Visibility
	goName          string
	declarationSpan source.Span
	declaringClass  string
}

type methodSymbol struct {
	typeInfo        Type
	visibility      ast.Visibility
	static          bool
	pointerReceiver bool
	goName          string
	declarationSpan source.Span
	declaringClass  string
	virtual         bool
	final           bool
	virtualOwner    string
}

type classSymbol struct {
	fields              map[string]fieldSymbol
	methods             map[string]methodSymbol
	constructor         []Type
	constructorVariadic bool
	implements          map[string]bool
	implementedTypes    []Type
	goImplements        []gotypes.Type
	typeParameters      []Type
	typeParamScope      map[string]Type
	declarationSpan     source.Span
	base                string
	baseType            Type
	ancestors           []string
	ancestorTypes       []Type
	final               bool
}

type structSymbol struct {
	fields          map[string]fieldSymbol
	methods         map[string]methodSymbol
	typeInfo        Type
	typeParameters  []Type
	typeParamScope  map[string]Type
	declarationSpan source.Span
}

type interfaceSymbol struct {
	methods         map[string]methodSymbol
	typeParameters  []Type
	typeParamScope  map[string]Type
	declarationSpan source.Span
}

type nativeTypeSymbol struct {
	declaration    *ast.TypeDecl
	typeInfo       Type
	goNamed        *gotypes.Named
	methods        map[string]methodSymbol
	typeParameters []Type
	typeParamScope map[string]Type
	state          uint8
}

type enumSymbol struct {
	declaration *ast.EnumDecl
	members     map[string]*ast.EnumMember
}

type goPackageSymbol struct {
	path        string
	declaration *ast.ImportDecl
	packageInfo *gotypes.Package
}

type loopFlowContext struct {
	continues []nullableFlowSnapshot
	breaks    []nullableFlowSnapshot
}

type breakFlowContext struct {
	breaks []nullableFlowSnapshot
}

type memberFlowKey struct {
	root source.Span
	path string
}

type memberFlowState struct {
	declaredType      Type
	nonNullType       Type
	nonNull           bool
	invalidated       source.Span
	invalidationCause string
}

type nullableFlowSnapshot struct {
	scopes  []map[string]valueSymbol
	members map[memberFlowKey]memberFlowState
}

type Checker struct {
	diagnostics                []diagnostic.Diagnostic
	functions                  map[string]functionSymbol
	globals                    map[string]valueSymbol
	scopes                     []map[string]valueSymbol
	result                     Type
	loopDepth                  int
	breakableDepth             int
	classes                    map[string]*classSymbol
	structs                    map[string]*structSymbol
	interfaces                 map[string]*interfaceSymbol
	nativeTypes                map[string]*nativeTypeSymbol
	enums                      map[string]*enumSymbol
	currentClass               string
	allowed                    map[string]map[string]bool
	goPackages                 map[string]map[string]*goPackageSymbol
	goImporter                 gotypes.Importer
	allowUnsafeGo              bool
	inConstructor              bool
	callableScopeBases         []int
	capturedWrites             []map[source.Span]source.Span
	loopFlowContexts           []loopFlowContext
	breakFlowContexts          []breakFlowContext
	suppressFlowEffects        int
	memberFlow                 map[memberFlowKey]memberFlowState
	memberTypes                map[memberFlowKey]Type
	usesTasks                  bool
	usesExceptions             bool
	nativeTypeIndirectionDepth int
	exceptionDepth             int
	catchTargets               []int
	taskOperandDepth           int
	typeParameterScopes        []map[string]Type
	functionTypeParameters     map[*ast.FunctionDecl]map[string]Type
	receiverTypeParameters     map[*ast.MethodDecl]map[string]Type
	validFallthrough           map[*ast.BranchStmt]bool
	capturedMemberWrites       []source.Span
	capturedMemberRoots        []map[source.Span]bool
}

type GoInteropPolicy struct {
	AllowUnsafe bool
}

func Check(program *ast.Program) []diagnostic.Diagnostic {
	return CheckScoped(program, nil)
}

func CheckScoped(program *ast.Program, allowed map[string]map[string]bool) []diagnostic.Diagnostic {
	return CheckScopedWithGoImporter(program, allowed, importer.Default())
}

func CheckScopedWithGoImporter(program *ast.Program, allowed map[string]map[string]bool, goImporter gotypes.Importer) []diagnostic.Diagnostic {
	return CheckScopedWithGoImporterAndPolicy(program, allowed, goImporter, GoInteropPolicy{})
}

func CheckScopedWithGoImporterAndPolicy(program *ast.Program, allowed map[string]map[string]bool, goImporter gotypes.Importer, policy GoInteropPolicy) []diagnostic.Diagnostic {
	if goImporter == nil {
		goImporter = importer.Default()
	}
	c := &Checker{
		functions: map[string]functionSymbol{}, globals: map[string]valueSymbol{},
		classes: map[string]*classSymbol{}, structs: map[string]*structSymbol{}, interfaces: map[string]*interfaceSymbol{}, nativeTypes: map[string]*nativeTypeSymbol{}, enums: map[string]*enumSymbol{}, allowed: allowed,
		goPackages: map[string]map[string]*goPackageSymbol{}, goImporter: goImporter, allowUnsafeGo: policy.AllowUnsafe,
		memberFlow: map[memberFlowKey]memberFlowState{}, memberTypes: map[memberFlowKey]Type{},
		functionTypeParameters: map[*ast.FunctionDecl]map[string]Type{},
		receiverTypeParameters: map[*ast.MethodDecl]map[string]Type{},
		validFallthrough:       map[*ast.BranchStmt]bool{},
	}
	c.installExceptionBuiltin()
	c.declareGoPackages(program)
	c.predeclareNamedTypes(program)
	c.declareNativeTypes(program)
	c.declareInterfaces(program)
	c.declareClasses(program)
	c.declareTopLevel(program)
	c.declareReceiverMethods(program)
	c.validateStructValueCycles(program)
	for _, decl := range program.Declarations {
		if decl, ok := decl.(*ast.VariableDecl); ok {
			declared := Type{Kind: Invalid, Name: "<inferred>"}
			if decl.Type.IsSpecified() {
				declared = c.resolveType(decl.Type)
			}
			valueType := c.checkExpressionExpectedSlot(&decl.Value, declared)
			if !decl.Type.IsSpecified() {
				declared = c.inferredVariableType(valueType, decl.Value.GetSpan())
			}
			c.requireAssignable(declared, valueType, decl.Value.GetSpan())
			if declared.Kind == Void {
				c.report(decl.GetSpan(), "variables cannot have type void")
			}
			c.rejectResultValueType(declared, decl.Type.Span, "variables")
			c.rejectTaskAPIType(declared, decl.Type.Span, "global variables")
			decl.ResolvedType = typeRefFromType(declared, decl.Span)
			c.globals[decl.Name] = valueSymbol{typeInfo: declared, declaredType: declared, constant: decl.Constant, declarationSpan: decl.NameSpan, declaration: decl}
		}
	}
	for _, decl := range program.Declarations {
		if decl, ok := decl.(*ast.EnumDecl); ok {
			c.checkEnum(decl)
		}
	}
	for _, decl := range program.Declarations {
		if decl, ok := decl.(*ast.ClassDecl); ok {
			c.checkClass(decl)
		}
	}
	for _, decl := range program.Declarations {
		if decl, ok := decl.(*ast.StructDecl); ok {
			c.checkStruct(decl)
		}
	}
	for _, decl := range program.Declarations {
		if decl, ok := decl.(*ast.MethodDecl); ok {
			c.pushTypeParameterScope(c.receiverTypeParameters[decl])
			receiver := decl.ReceiverType
			if receiver.IsPointer() && receiver.Pointee != nil {
				receiver = *receiver.Pointee
			}
			if _, exists := c.nativeTypes[receiver.Name]; exists {
				c.checkNativeTypeMethod(decl, decl.ReceiverName, receiver.Name)
			} else {
				c.checkStructMethod(decl, decl.ReceiverName, receiver.Name)
			}
			c.popTypeParameterScope()
		}
	}
	for _, decl := range program.Declarations {
		if decl, ok := decl.(*ast.FunctionDecl); ok {
			c.checkFunction(decl)
		}
	}
	c.checkCABIExports(program)
	c.checkGeneratedNames(program)
	c.markResolvedTypeRefs(program)
	program.UsesTasks = c.usesTasks
	program.UsesExceptions = c.usesExceptions
	return c.diagnostics
}

func (c *Checker) checkCABIExports(program *ast.Program) {
	program.CABIExports = nil
	symbols := map[string]source.Span{}
	targets := map[string]source.Span{}
	functions := map[string]*ast.FunctionDecl{}
	variables := map[string]*ast.VariableDecl{}
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			functions[declaration.Name] = declaration
		case *ast.VariableDecl:
			variables[declaration.Name] = declaration
		}
	}
	validate := func(export ast.CABIExport, generic bool) {
		if generic {
			c.report(export.NameSpan, "generic functions cannot be exported through the C ABI")
		}
		symbol := export.Symbol
		if !validCABIIdentifier(symbol) {
			c.report(export.SymbolSpan, fmt.Sprintf("C ABI symbol %q must start with an ASCII letter and contain only ASCII letters, digits, or '_'", symbol))
		} else if gotoken.Lookup(symbol).IsKeyword() {
			c.report(export.SymbolSpan, fmt.Sprintf("C ABI symbol %q is a Go keyword and cannot be generated", symbol))
		} else if symbol == "main" || symbol == "init" {
			c.report(export.SymbolSpan, fmt.Sprintf("C ABI symbol %q is reserved by the generated Go package", symbol))
		}
		if previous, exists := symbols[symbol]; exists {
			c.report(export.SymbolSpan, fmt.Sprintf("duplicate C ABI symbol %q (first declared at %d:%d)", symbol, previous.Start.Line, previous.Start.Column))
		} else {
			symbols[symbol] = export.SymbolSpan
		}
		if previous, exists := targets[export.Name]; exists {
			c.report(export.NameSpan, fmt.Sprintf("duplicate C ABI export target %q (first exported at %d:%d)", export.Name, previous.Start.Line, previous.Start.Column))
		} else {
			targets[export.Name] = export.NameSpan
		}
		for _, parameter := range export.Parameters {
			if !c.isCABITypeRef(parameter.Type, false) {
				c.report(parameter.Type.Span, fmt.Sprintf("C ABI parameter %q has unsupported type %s; use boolean, a fixed-width scalar, or an enum with a fixed-width integer underlying type", parameter.Name, formatTypeRefForDiagnostic(parameter.Type)))
			}
		}
		if !c.isCABITypeRef(export.ReturnType, true) {
			c.report(export.ReturnType.Span, fmt.Sprintf("C ABI result has unsupported type %s; use void, boolean, a fixed-width scalar, or an enum with a fixed-width integer underlying type", formatTypeRefForDiagnostic(export.ReturnType)))
		}
		program.CABIExports = append(program.CABIExports, export)
	}
	for _, declaration := range program.Declarations {
		function, ok := declaration.(*ast.FunctionDecl)
		if !ok || !function.CABIExport {
			continue
		}
		validate(ast.CABIExport{
			Name: function.Name, NameSpan: function.NameSpan, Symbol: function.CABISymbol, SymbolSpan: function.CABISymbolSpan,
			Parameters: function.Parameters, ReturnType: function.ReturnType, Span: function.CABIExportSpan,
		}, len(function.TypeParameters) != 0)
	}
	for _, declaration := range program.Declarations {
		exports, ok := declaration.(*ast.CABIExportDecl)
		if !ok {
			continue
		}
		if len(exports.Symbols) != len(exports.Names) {
			c.report(exports.Span, fmt.Sprintf("C ABI export list has %d symbols but %d names; counts must match positionally", len(exports.Symbols), len(exports.Names)))
		}
		exports.ResolvedDeclarations = make([]source.Span, len(exports.Names))
		count := min(len(exports.Symbols), len(exports.Names))
		for index := 0; index < count; index++ {
			name := exports.Names[index]
			if function := functions[name]; function != nil {
				exports.ResolvedDeclarations[index] = function.NameSpan
				validate(ast.CABIExport{
					Name: name, NameSpan: exports.NameSpans[index], Symbol: exports.Symbols[index], SymbolSpan: exports.SymbolSpans[index],
					Parameters: function.Parameters, ReturnType: function.ReturnType, Span: exports.Span,
				}, len(function.TypeParameters) != 0)
				continue
			}
			variable := variables[name]
			if variable == nil {
				c.report(exports.NameSpans[index], fmt.Sprintf("undefined C ABI export target %q", name))
				continue
			}
			exports.ResolvedDeclarations[index] = variable.NameSpan
			if !variable.Constant {
				c.report(exports.NameSpans[index], fmt.Sprintf("C ABI export target %q must be a const arrow function", name))
				continue
			}
			arrow, ok := variable.Value.(*ast.ArrowExpr)
			if !ok {
				c.report(exports.NameSpans[index], fmt.Sprintf("C ABI export target %q must be a top-level function or const arrow function", name))
				continue
			}
			if arrow.ReturnType == nil {
				c.report(exports.NameSpans[index], fmt.Sprintf("C ABI arrow export %q requires an explicit return type", name))
				continue
			}
			validate(ast.CABIExport{
				Name: name, NameSpan: exports.NameSpans[index], Symbol: exports.Symbols[index], SymbolSpan: exports.SymbolSpans[index],
				Parameters: arrow.Parameters, ReturnType: *arrow.ReturnType, Span: exports.Span,
			}, false)
		}
	}
}

func validCABIIdentifier(name string) bool {
	if len(name) == 0 || !isASCIIAlpha(name[0]) {
		return false
	}
	for index := 1; index < len(name); index++ {
		if !isASCIIAlpha(name[index]) && (name[index] < '0' || name[index] > '9') && name[index] != '_' {
			return false
		}
	}
	return true
}

func isASCIIAlpha(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func (c *Checker) isCABITypeRef(ref ast.TypeRef, allowVoid bool) bool {
	if ref.Nullable || ref.Qualifier != "" || len(ref.GenericArguments) != 0 || ref.IsArray() || ref.IsPointer() || ref.IsFunction() || ref.IsObject() {
		return false
	}
	switch ref.Name {
	case "boolean", "byte", "uint8", "int8", "int16", "int32", "int64", "uint16", "uint32", "uint64", "float32", "float", "float64", "number":
		return true
	case "void":
		return allowVoid
	default:
		if c.enums[ref.Name] == nil {
			return false
		}
		resolved := c.resolveType(ref)
		if resolved.GoType == nil {
			return false
		}
		basic, ok := gotypes.Unalias(resolved.GoType).Underlying().(*gotypes.Basic)
		if !ok {
			return false
		}
		switch basic.Kind() {
		case gotypes.Int8, gotypes.Int16, gotypes.Int32, gotypes.Int64, gotypes.Uint8, gotypes.Uint16, gotypes.Uint32, gotypes.Uint64:
			return true
		default:
			return false
		}
	}
}

func formatTypeRefForDiagnostic(ref ast.TypeRef) string {
	if ref.Nullable {
		ref.Nullable = false
		return formatTypeRefForDiagnostic(ref) + " | null"
	}
	if ref.Name != "" && !ref.IsArray() && !ref.IsPointer() && !ref.IsFunction() && !ref.IsObject() {
		if ref.Qualifier != "" {
			return ref.Qualifier + "." + ref.Name
		}
		return ref.Name
	}
	if ref.IsArray() && ref.Element != nil {
		return formatTypeRefForDiagnostic(*ref.Element) + "[]"
	}
	if ref.IsPointer() && ref.Pointee != nil {
		return "*" + formatTypeRefForDiagnostic(*ref.Pointee)
	}
	return "non-scalar type"
}

func (c *Checker) checkGeneratedNames(program *ast.Program) {
	declared := map[string]source.Span{}
	structMembers := map[string]map[string]source.Span{}
	claim := func(name string, span source.Span) {
		if c.usesTasks && (name == "__ontamaTask" || name == "__ontamaVoidTask" || name == "__ontamaResultTask" || name == "__ontamaVoidResultTask") {
			c.report(span, fmt.Sprintf("generated Go name %q is reserved by the Task runtime", name))
			return
		}
		if c.usesExceptions && (name == "__ontamaException" || name == "__ontamaThrown" || name == "__ontamaReturn" || name == "__ontamaReturnValue" || name == "__ontamaInitException" || name == "__ontamaExceptionFromError" || name == "NewException") {
			c.report(span, fmt.Sprintf("generated Go name %q is reserved by the exception runtime", name))
			return
		}
		if previous, exists := declared[name]; exists {
			c.report(span, fmt.Sprintf("generated Go name %q collides with a declaration at %d:%d", name, previous.Start.Line, previous.Start.Column))
			return
		}
		declared[name] = span
	}
	claimStructMember := func(structName, name string, span source.Span) {
		if structName == "" || name == "" {
			return
		}
		members := structMembers[structName]
		if members == nil {
			members = map[string]source.Span{}
			structMembers[structName] = members
		}
		name = generatedIdentifier(name)
		if previous, exists := members[name]; exists {
			c.report(span, fmt.Sprintf("generated Go struct member name %q collides with a member at %d:%d", name, previous.Start.Line, previous.Start.Column))
			return
		}
		members[name] = span
	}
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			claim(generatedIdentifier(declaration.Name), declaration.Span)
		case *ast.VariableDecl:
			claim(generatedIdentifier(declaration.Name), declaration.Span)
		case *ast.InterfaceDecl:
			if gotoken.Lookup(declaration.Name).IsKeyword() {
				c.report(declaration.Span, fmt.Sprintf("interface name %q is a Go keyword and cannot be generated", declaration.Name))
			}
			claim(declaration.Name, declaration.Span)
		case *ast.ClassDecl:
			if gotoken.Lookup(declaration.Name).IsKeyword() {
				c.report(declaration.Span, fmt.Sprintf("class name %q is a Go keyword and cannot be generated", declaration.Name))
			}
			claim(declaration.Name, declaration.Span)
			claim("New"+declaration.Name, declaration.Span)
			claim("__ontamaInit"+declaration.Name, declaration.Span)
			if declaration.Base != nil {
				claimStructMember(declaration.Name, declaration.Base.Name, declaration.Base.Span)
			}
			if declaration.HierarchyRoot == declaration.Name {
				claimStructMember(declaration.Name, "__ontamaRoot", declaration.Span)
			}
			for _, owner := range declaration.VirtualOwners {
				if owner == declaration.Name {
					claimStructMember(declaration.Name, "__ontama"+owner+"Self", declaration.Span)
				}
			}
			for _, field := range declaration.Fields {
				claimStructMember(declaration.Name, field.GoName, field.Span)
			}
			if declaration.Constructor != nil {
				for _, parameter := range declaration.Constructor.Parameters {
					if parameter.IsField {
						claimStructMember(declaration.Name, memberGoName(parameter.Name, parameter.Visibility), parameter.Span)
					}
				}
			}
			for _, ancestor := range declaration.Ancestors {
				claim("__ontamaUpcast"+declaration.Name+"To"+ancestor, declaration.Span)
				claim("__ontamaDowncast"+ancestor+"To"+declaration.Name, declaration.Span)
				claim("__ontamaMustDowncast"+ancestor+"To"+declaration.Name, declaration.Span)
				claim("Upcast"+declaration.Name+"To"+ancestor, declaration.Span)
				claim("Downcast"+ancestor+"To"+declaration.Name, declaration.Span)
				claim("MustDowncast"+ancestor+"To"+declaration.Name, declaration.Span)
			}
			if len(declaration.Ancestors) != 0 {
				claim("__ontama"+declaration.Name+"Projection", declaration.Span)
				claimStructMember(declaration.Name, "__ontamaAs"+declaration.Name, declaration.Span)
			}
			for _, method := range declaration.Methods {
				if method.Virtual && !method.Override && !method.Static {
					claim("__ontama"+declaration.Name+"Virtual", method.Span)
					break
				}
			}
			for _, method := range declaration.Methods {
				if method.Static {
					claim(staticMethodGoName(declaration.Name, method.GoName, method.Visibility), method.Span)
					continue
				}
				if method.Virtual || method.Override {
					claimStructMember(declaration.Name, "__ontama"+method.VirtualOwner+method.GoName, method.Span)
				}
				if !method.Override {
					claimStructMember(declaration.Name, method.GoName, method.Span)
				}
			}
		case *ast.StructDecl:
			if gotoken.Lookup(declaration.Name).IsKeyword() {
				c.report(declaration.Span, fmt.Sprintf("struct name %q is a Go keyword and cannot be generated", declaration.Name))
			}
			claim(declaration.Name, declaration.Span)
			for _, field := range declaration.Fields {
				claimStructMember(declaration.Name, field.GoName, field.Span)
			}
			for _, method := range declaration.Methods {
				claimStructMember(declaration.Name, method.GoName, method.Span)
			}
		case *ast.TypeDecl:
			if gotoken.Lookup(declaration.Name).IsKeyword() {
				c.report(declaration.Span, fmt.Sprintf("type name %q is a Go keyword and cannot be generated", declaration.Name))
			}
			claim(declaration.Name, declaration.Span)
		case *ast.EnumDecl:
			if gotoken.Lookup(declaration.Name).IsKeyword() {
				c.report(declaration.Span, fmt.Sprintf("enum name %q is a Go keyword and cannot be generated", declaration.Name))
			}
			claim(declaration.Name, declaration.Span)
			for _, member := range declaration.Members {
				if member.Name == "_" {
					c.report(member.Span, "enum member name cannot be '_'")
					continue
				}
				claim(enumMemberGoName(declaration.Name, member.Name), member.Span)
			}
		case *ast.MethodDecl:
			receiver := declaration.ReceiverType
			if receiver.IsPointer() && receiver.Pointee != nil {
				receiver = *receiver.Pointee
			}
			claimStructMember(receiver.Name, declaration.GoName, declaration.Span)
		}
	}
	for _, export := range program.CABIExports {
		claim(export.Symbol, export.SymbolSpan)
	}
	seenImports := map[string]bool{}
	for _, imported := range program.Imports {
		if imported.Go && imported.Used {
			if seenImports[imported.Path] {
				continue
			}
			seenImports[imported.Path] = true
			alias := imported.Alias
			if imported.ResolvedAlias != "" {
				alias = imported.ResolvedAlias
			}
			claim(generatedIdentifier(alias), imported.Span)
		}
	}
}

func generatedIdentifier(name string) string {
	if gotoken.Lookup(name).IsKeyword() || isGeneratedGoPredeclaredName(name) {
		return name + "_"
	}
	return name
}

func isGeneratedGoPredeclaredName(name string) bool {
	switch name {
	case "append", "cap", "clear", "close", "complex", "copy", "delete", "imag", "len", "make", "max", "min", "new", "panic", "print", "println", "real", "recover":
		return true
	default:
		return false
	}
}

func (c *Checker) predeclareNamedTypes(program *ast.Program) {
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.ClassDecl:
			class := declaration
			if _, exists := c.classes[class.Name]; !exists {
				typeParameters, typeParamScope := c.declareTypeParameters(class.TypeParameters, "generic class")
				c.classes[class.Name] = &classSymbol{
					declarationSpan: class.NameSpan, typeParameters: typeParameters, typeParamScope: typeParamScope,
				}
			}
		case *ast.InterfaceDecl:
			if _, exists := c.interfaces[declaration.Name]; !exists {
				typeParameters, typeParamScope := c.declareTypeParameters(declaration.TypeParameters, "generic interface")
				c.interfaces[declaration.Name] = &interfaceSymbol{
					methods: map[string]methodSymbol{}, typeParameters: typeParameters, typeParamScope: typeParamScope,
					declarationSpan: declaration.NameSpan,
				}
			}
		case *ast.StructDecl:
			if _, exists := c.structs[declaration.Name]; !exists {
				typeParameters, typeParamScope := c.declareTypeParameters(declaration.TypeParameters, "generic struct")
				fields := map[string]Type{}
				fieldNames := map[string]string{}
				c.structs[declaration.Name] = &structSymbol{
					fields: map[string]fieldSymbol{}, methods: map[string]methodSymbol{}, declarationSpan: declaration.NameSpan,
					typeParameters: typeParameters, typeParamScope: typeParamScope,
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
		bindings := make(map[string]Type, len(symbol.typeParameters))
		for index, parameter := range symbol.typeParameters {
			bindings[parameter.Name] = arguments[index]
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

func (c *Checker) declareInterfaces(program *ast.Program) {
	for _, declaration := range program.Declarations {
		decl, ok := declaration.(*ast.InterfaceDecl)
		if !ok {
			continue
		}
		symbol := c.interfaces[decl.Name]
		if symbol == nil {
			symbol = &interfaceSymbol{methods: map[string]methodSymbol{}, declarationSpan: decl.NameSpan}
			c.interfaces[decl.Name] = symbol
		}
		symbol.methods = map[string]methodSymbol{}
		c.pushTypeParameterScope(symbol.typeParamScope)
		for i := range decl.Methods {
			method := &decl.Methods[i]
			if _, exists := symbol.methods[method.Name]; exists {
				c.report(method.Span, fmt.Sprintf("duplicate interface method %q", method.Name))
				continue
			}
			parameters := make([]Type, len(method.Parameters))
			for j, parameter := range method.Parameters {
				resolved := c.resolveType(parameter.Type)
				c.rejectResultValueType(resolved, parameter.Type.Span, "parameters")
				c.rejectTaskAPIType(resolved, parameter.Type.Span, "interface parameters")
				parameters[j] = c.callableParameterType(parameter, resolved)
			}
			result := c.resolveType(method.ReturnType)
			c.rejectTaskAPIType(result, method.ReturnType.Span, "interface return types")
			method.GoName = memberGoName(method.Name, ast.Public)
			symbol.methods[method.Name] = methodSymbol{
				typeInfo:   Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: hasVariadicParameter(method.Parameters), Result: &result},
				visibility: ast.Public, goName: method.GoName, declarationSpan: method.NameSpan,
			}
		}
		c.popTypeParameterScope()
	}
}

func (c *Checker) declareTopLevel(program *ast.Program) {
	declared := map[string]source.Span{}
	for _, decl := range program.Declarations {
		var name string
		switch decl := decl.(type) {
		case *ast.VariableDecl:
			name = decl.Name
			t := Type{Kind: Invalid, Name: "<inferred>"}
			if decl.Type.IsSpecified() {
				t = c.resolveType(decl.Type)
			}
			if t.Kind == Void {
				c.report(decl.Type.Span, "variables cannot have type void")
			}
			c.rejectResultValueType(t, decl.Type.Span, "variables")
			c.rejectTaskAPIType(t, decl.Type.Span, "global variables")
			if _, exists := declared[name]; !exists {
				c.globals[name] = valueSymbol{typeInfo: t, declaredType: t, constant: decl.Constant, declarationSpan: decl.NameSpan, declaration: decl}
			}
		case *ast.FunctionDecl:
			name = decl.Name
			typeParameters, typeParameterScope := c.declareFunctionTypeParameters(decl)
			c.functionTypeParameters[decl] = typeParameterScope
			c.pushTypeParameterScope(typeParameterScope)
			params := make([]Type, len(decl.Parameters))
			for i, param := range decl.Parameters {
				resolved := c.resolveType(param.Type)
				if resolved.Kind == Void {
					c.report(param.Type.Span, "parameters cannot have type void")
				}
				c.rejectResultValueType(resolved, param.Type.Span, "parameters")
				c.rejectTaskAPIType(resolved, param.Type.Span, "function parameters")
				params[i] = c.callableParameterType(param, resolved)
			}
			if _, exists := declared[name]; !exists {
				result := c.resolveType(decl.ReturnType)
				c.rejectTaskAPIType(result, decl.ReturnType.Span, "function return types")
				c.functions[name] = functionSymbol{
					parameters: params, typeParameters: typeParameters, typeParameterScope: typeParameterScope,
					variadic: hasVariadicParameter(decl.Parameters),
					result:   result, span: decl.Span, declarationSpan: decl.NameSpan,
				}
			}
			c.popTypeParameterScope()
		case *ast.ClassDecl:
			name = decl.Name
		case *ast.StructDecl:
			name = decl.Name
			c.declareStruct(decl)
		case *ast.TypeDecl:
			name = decl.Name
		case *ast.EnumDecl:
			name = decl.Name
		case *ast.InterfaceDecl:
			name = decl.Name
		case *ast.MethodDecl:
			continue
		case *ast.CABIExportDecl:
			continue
		}
		if previous, exists := declared[name]; exists {
			c.report(decl.GetSpan(), fmt.Sprintf("duplicate top-level name %q (first declared at %d:%d)", name, previous.Start.Line, previous.Start.Column))
		} else {
			declared[name] = decl.GetSpan()
		}
		if isBuiltinTypeName(name) {
			c.report(decl.GetSpan(), fmt.Sprintf("top-level name %q conflicts with a built-in type", name))
		} else if isBuiltinValueName(name) {
			c.report(decl.GetSpan(), fmt.Sprintf("top-level name %q conflicts with a compiler built-in", name))
		}
	}
}

func (c *Checker) declareFunctionTypeParameters(decl *ast.FunctionDecl) ([]Type, map[string]Type) {
	if len(decl.TypeParameters) == 0 {
		return nil, nil
	}
	if decl.Name == "main" || decl.Name == "init" {
		c.report(decl.NameSpan, fmt.Sprintf("function %q cannot declare type parameters", decl.Name))
	}
	return c.declareTypeParameters(decl.TypeParameters, "generic function")
}

func (c *Checker) declareTypeParameters(parameters []ast.TypeParameter, context string) ([]Type, map[string]Type) {
	return c.declareTypeParametersWithComparable(parameters, context, nil)
}

func (c *Checker) declareDefinedTypeParameters(declaration *ast.TypeDecl) ([]Type, map[string]Type) {
	comparable := map[string]bool{}
	collectComparableTypeParameters(declaration.Underlying, comparable)
	if !declaration.Alias {
		return c.declareTypeParametersWithComparable(declaration.TypeParameters, "generic defined type", comparable)
	}
	// Generic aliases are expanded at every use site for Go 1.23 and their
	// declarations do not reach generated Go. Loading a Go constraint must not
	// retain an otherwise unused import solely for the erased declaration.
	usage := map[*ast.ImportDecl]bool{}
	for _, byAlias := range c.goPackages {
		for _, imported := range byAlias {
			usage[imported.declaration] = imported.declaration.Used
		}
	}
	parameters, scope := c.declareTypeParametersWithComparable(declaration.TypeParameters, "generic alias", comparable)
	for imported, used := range usage {
		imported.Used = used
	}
	return parameters, scope
}

func (c *Checker) declareTypeParametersWithComparable(parameters []ast.TypeParameter, context string, comparable map[string]bool) ([]Type, map[string]Type) {
	anyConstraint := gotypes.NewInterfaceType(nil, nil)
	anyConstraint.Complete()
	comparableConstraint := gotypes.Universe.Lookup("comparable").Type()
	result := make([]Type, 0, len(parameters))
	scope := make(map[string]Type, len(parameters))
	for _, parameter := range parameters {
		if parameter.Name == "_" {
			c.report(parameter.Span, context+" type parameter name cannot be '_'")
			continue
		}
		if parameter.Name == "any" {
			c.report(parameter.Span, context+" type parameter name cannot be 'any' because native parameters use the Go any constraint")
			continue
		}
		if isBuiltinTypeName(parameter.Name) || parameter.Name == "Map" || parameter.Name == "GoChannel" || parameter.Name == "GoSendChannel" || parameter.Name == "GoReceiveChannel" {
			c.report(parameter.Span, fmt.Sprintf("%s type parameter %q conflicts with a built-in type", context, parameter.Name))
			continue
		}
		if _, duplicate := scope[parameter.Name]; duplicate {
			c.report(parameter.Span, fmt.Sprintf("duplicate %s type parameter %q", context, parameter.Name))
			continue
		}
		constraint := gotypes.Type(anyConstraint)
		if parameter.Constraint != nil {
			resolved, ok := c.resolveNativeTypeParameterConstraint(*parameter.Constraint)
			if !ok {
				continue
			}
			constraint = resolved
		}
		if comparable[parameter.Name] {
			if parameter.Constraint == nil {
				constraint = comparableConstraint
			} else if constraint != comparableConstraint {
				intersection := gotypes.NewInterfaceType(nil, []gotypes.Type{constraint, comparableConstraint})
				intersection.Complete()
				constraint = intersection
			}
		}
		object := gotypes.NewTypeName(gotoken.NoPos, nil, parameter.Name, nil)
		goParameter := gotypes.NewTypeParam(object, constraint)
		typeInfo := Type{Kind: TypeParameter, Name: parameter.Name, GoType: goParameter}
		scope[parameter.Name] = typeInfo
		result = append(result, typeInfo)
	}
	return result, scope
}

func (c *Checker) resolveNativeTypeParameterConstraint(ref ast.TypeRef) (gotypes.Type, bool) {
	if ref.Qualifier == "" && ref.Name == "comparable" && !ref.Nullable && !ref.IsArray() && !ref.IsPointer() && !ref.IsFunction() && !ref.IsObject() && !ref.IsGoStruct() && len(ref.GenericArguments) == 0 {
		return gotypes.Universe.Lookup("comparable").Type(), true
	}
	if ref.Nullable || ref.IsArray() || ref.IsPointer() || ref.IsFunction() || ref.IsObject() || ref.IsGoStruct() {
		c.report(ref.Span, fmt.Sprintf("native type parameter constraint %s must be a Go interface constraint", formatTypeRefForDiagnostic(ref)))
		return nil, false
	}
	resolved := c.resolveType(ref)
	if resolved.Kind == Invalid {
		return nil, false
	}
	constraint, ok := goTypeOf(resolved)
	if !ok || underlyingGoInterface(constraint) == nil {
		c.report(ref.Span, fmt.Sprintf("native type parameter constraint %s must be a Go interface constraint", formatTypeRefForDiagnostic(ref)))
		return nil, false
	}
	return constraint, true
}

func nativeTypeArgumentSatisfies(parameter, argument Type) bool {
	goParameter, ok := parameter.GoType.(*gotypes.TypeParam)
	if !ok {
		return true
	}
	constraint, constraintOK := goParameter.Constraint().Underlying().(*gotypes.Interface)
	if !constraintOK || constraint.Empty() {
		return true
	}
	argument = defaultLiteralType(argument)
	goArgument, ok := goTypeOf(argument)
	if ok {
		return gotypes.Satisfies(goArgument, constraint)
	}
	return goParameter.Constraint() == gotypes.Universe.Lookup("comparable").Type() && argument.IsComparable()
}

func (c *Checker) validateNativeTypeArguments(parameters, arguments []Type, refs []ast.TypeRef, fallback source.Span, owner string) bool {
	valid := true
	for index := range parameters {
		if index >= len(arguments) || nativeTypeArgumentSatisfies(parameters[index], arguments[index]) {
			continue
		}
		span := fallback
		if index < len(refs) {
			span = refs[index].Span
		}
		c.report(span, fmt.Sprintf("type %s does not satisfy %s type parameter constraint for %s", arguments[index].String(), parameters[index].Name, owner))
		valid = false
	}
	return valid
}

func collectComparableTypeParameters(ref ast.TypeRef, result map[string]bool) {
	if ref.Name == "Map" && len(ref.GenericArguments) == 2 {
		collectTypeParametersRequiringComparability(ref.GenericArguments[0], result)
	}
	if ref.Element != nil {
		collectComparableTypeParameters(*ref.Element, result)
	}
	if ref.Pointee != nil {
		collectComparableTypeParameters(*ref.Pointee, result)
	}
	for _, argument := range ref.GenericArguments {
		collectComparableTypeParameters(argument, result)
	}
	for _, parameter := range ref.Parameters {
		collectComparableTypeParameters(parameter, result)
	}
	if ref.Return != nil {
		collectComparableTypeParameters(*ref.Return, result)
	}
	for _, field := range ref.ObjectFields {
		collectComparableTypeParameters(field.Type, result)
	}
}

func collectTypeParametersRequiringComparability(ref ast.TypeRef, result map[string]bool) {
	if ref.IsPointer() || ref.IsSlice() || ref.Name == "Map" || ref.IsFunction() || ref.Object || ref.GoStruct {
		return
	}
	if len(ref.GenericArguments) == 0 && ref.Element == nil {
		result[ref.Name] = true
		return
	}
	if ref.Element != nil {
		collectTypeParametersRequiringComparability(*ref.Element, result)
	}
	for _, argument := range ref.GenericArguments {
		collectTypeParametersRequiringComparability(argument, result)
	}
}

func (c *Checker) pushTypeParameterScope(scope map[string]Type) {
	c.typeParameterScopes = append(c.typeParameterScopes, scope)
}

func (c *Checker) popTypeParameterScope() {
	c.typeParameterScopes = c.typeParameterScopes[:len(c.typeParameterScopes)-1]
}

func (c *Checker) lookupTypeParameter(name string) (Type, bool) {
	for index := len(c.typeParameterScopes) - 1; index >= 0; index-- {
		parameter, ok := c.typeParameterScopes[index][name]
		if ok {
			return parameter, true
		}
	}
	return Type{}, false
}

func callableTypeForFunction(function functionSymbol) Type {
	result := function.result
	return Type{
		Kind: Function, Name: "function", Parameters: function.parameters,
		TypeParameters: function.typeParameters, Generic: len(function.typeParameters) != 0, Variadic: function.variadic, Result: &result,
	}
}

func hasVariadicParameter(parameters []ast.Parameter) bool {
	return len(parameters) != 0 && parameters[len(parameters)-1].Variadic
}

func (c *Checker) callableParameterType(parameter ast.Parameter, resolved Type) Type {
	if !parameter.Variadic {
		return resolved
	}
	if resolved.Kind != Array || resolved.Element == nil {
		c.report(parameter.Type.Span, "rest parameter type must be a slice")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return *resolved.Element
}

func (c *Checker) declareClasses(program *ast.Program) {
	declarations := map[string]*ast.ClassDecl{}
	for _, declaration := range program.Declarations {
		if class, ok := declaration.(*ast.ClassDecl); ok {
			declarations[class.Name] = class
		}
	}
	state := map[string]uint8{}
	var declare func(*ast.ClassDecl)
	declare = func(decl *ast.ClassDecl) {
		switch state[decl.Name] {
		case 2:
			return
		case 1:
			c.report(decl.NameSpan, fmt.Sprintf("inheritance cycle contains class %s", decl.Name))
			return
		}
		state[decl.Name] = 1
		if decl.Base != nil {
			base := decl.Base
			if base.Nullable || base.Qualifier != "" || base.Name == "" || base.IsArray() || base.IsPointer() || base.IsFunction() || base.IsObject() {
				c.report(base.Span, "extends expects an unqualified class name")
			} else if baseDecl := declarations[base.Name]; baseDecl != nil {
				declare(baseDecl)
				if baseDecl.Final {
					c.report(base.Span, fmt.Sprintf("cannot extend final class %s", base.Name))
				}
			} else if builtinBase := c.classes[base.Name]; builtinBase != nil {
				c.usesExceptions = true
				if builtinBase.final {
					c.report(decl.Base.Span, fmt.Sprintf("cannot extend final class %s", decl.Base.Name))
				}
			} else {
				c.report(base.Span, fmt.Sprintf("unknown base class %q", base.Name))
			}
		}
		c.declareClass(decl)
		state[decl.Name] = 2
	}
	for _, declaration := range program.Declarations {
		if class, ok := declaration.(*ast.ClassDecl); ok {
			declare(class)
		}
	}
	for name, declaration := range declarations {
		symbol := c.classes[name]
		if symbol == nil || len(symbol.ancestors) == 0 {
			continue
		}
		root := symbol.ancestors[len(symbol.ancestors)-1]
		declaration.HierarchyRoot = root
		if rootDeclaration := declarations[root]; rootDeclaration != nil {
			rootDeclaration.HierarchyRoot = root
		}
		for _, ancestor := range symbol.ancestors {
			if ancestorDeclaration := declarations[ancestor]; ancestorDeclaration != nil {
				ancestorDeclaration.HierarchyRoot = root
			}
		}
	}
}

func (c *Checker) installExceptionBuiltin() {
	stringType := builtins["string"]
	errorType := builtins["error"]
	methodResult := stringType
	c.classes["Exception"] = &classSymbol{
		fields: map[string]fieldSymbol{
			"message": {
				typeInfo: stringType, visibility: ast.Public, goName: "Message", declaringClass: "Exception",
			},
		},
		methods: map[string]methodSymbol{
			"error": {
				typeInfo: Type{Kind: Function, Name: "function", Result: &methodResult}, visibility: ast.Public,
				goName: "Error", declaringClass: "Exception",
			},
		},
		constructor:  []Type{stringType},
		implements:   map[string]bool{},
		goImplements: []gotypes.Type{errorType.GoType},
	}
}

func (c *Checker) declareClass(decl *ast.ClassDecl) {
	predeclared := c.classes[decl.Name]
	symbol := &classSymbol{
		fields: map[string]fieldSymbol{}, methods: map[string]methodSymbol{}, implements: map[string]bool{}, declarationSpan: decl.NameSpan, final: decl.Final,
	}
	if predeclared != nil {
		symbol.typeParameters = predeclared.typeParameters
		symbol.typeParamScope = predeclared.typeParamScope
	}
	c.classes[decl.Name] = symbol
	c.pushTypeParameterScope(symbol.typeParamScope)
	defer c.popTypeParameterScope()
	if decl.Base != nil {
		base := c.classes[decl.Base.Name]
		if base != nil && base.fields != nil && decl.Base.Name != decl.Name {
			baseType := c.resolveNativeClassType(*decl.Base, base)
			baseBindings := nativeClassBindings(base, baseType)
			symbol.base = decl.Base.Name
			symbol.baseType = baseType
			symbol.ancestors = append([]string{decl.Base.Name}, base.ancestors...)
			symbol.ancestorTypes = append(symbol.ancestorTypes, baseType)
			for _, ancestorType := range base.ancestorTypes {
				symbol.ancestorTypes = append(symbol.ancestorTypes, substituteNativeTypeParameters(ancestorType, baseBindings))
			}
			decl.Ancestors = append(decl.Ancestors, symbol.ancestors...)
			for _, ancestorType := range symbol.ancestorTypes {
				decl.AncestorTypes = append(decl.AncestorTypes, typeRefFromType(ancestorType, decl.Base.Span))
			}
			decl.Base.ResolvedDeclaration = base.declarationSpan
			for name, field := range base.fields {
				field.typeInfo = substituteNativeTypeParameters(field.typeInfo, baseBindings)
				symbol.fields[name] = field
			}
			for name, method := range base.methods {
				if !method.static {
					method.typeInfo = substituteNativeTypeParameters(method.typeInfo, baseBindings)
				}
				symbol.methods[name] = method
			}
			for name := range base.implements {
				symbol.implements[name] = true
			}
			for _, implemented := range base.implementedTypes {
				symbol.implementedTypes = append(symbol.implementedTypes, substituteNativeTypeParameters(implemented, baseBindings))
			}
			symbol.goImplements = append(symbol.goImplements, base.goImplements...)
		}
	}
	declareField := func(name string, typeRef ast.TypeRef, visibility ast.Visibility, span, declarationSpan source.Span, setGoName func(string)) {
		if name == "__ontamaRoot" {
			c.report(span, fmt.Sprintf("field %q is reserved for class identity", name))
			return
		}
		if method, exists := symbol.methods[name]; exists {
			c.report(span, fmt.Sprintf("field %q conflicts with method declared by class %s", name, method.declaringClass))
			return
		}
		if symbol.base != "" && name == symbol.base {
			c.report(span, fmt.Sprintf("field %q conflicts with the embedded base class", name))
			return
		}
		reservedVirtualField := name == "__ontama"+decl.Name+"Self"
		for _, inheritedMethod := range symbol.methods {
			if inheritedMethod.virtualOwner != "" && name == "__ontama"+inheritedMethod.virtualOwner+"Self" {
				reservedVirtualField = true
			}
		}
		if reservedVirtualField {
			c.report(span, fmt.Sprintf("field %q is reserved for virtual dispatch", name))
			return
		}
		if _, exists := symbol.fields[name]; exists {
			c.report(span, fmt.Sprintf("duplicate field %q", name))
			return
		}
		goName := memberGoName(name, visibility)
		setGoName(goName)
		fieldType := c.resolveType(typeRef)
		c.rejectResultValueType(fieldType, typeRef.Span, "fields")
		c.rejectTaskAPIType(fieldType, typeRef.Span, "class fields")
		symbol.fields[name] = fieldSymbol{typeInfo: fieldType, visibility: visibility, goName: goName, declarationSpan: declarationSpan, declaringClass: decl.Name}
	}
	for i := range decl.Fields {
		field := &decl.Fields[i]
		declareField(field.Name, field.Type, field.Visibility, field.Span, field.NameSpan, func(name string) { field.GoName = name })
	}
	if decl.Constructor != nil {
		c.validateLabels(decl.Constructor.Body)
		symbol.constructor = make([]Type, len(decl.Constructor.Parameters))
		for i := range decl.Constructor.Parameters {
			parameter := &decl.Constructor.Parameters[i]
			resolved := c.resolveType(parameter.Type)
			c.rejectResultValueType(resolved, parameter.Type.Span, "parameters")
			c.rejectTaskAPIType(resolved, parameter.Type.Span, "constructor parameters")
			symbol.constructor[i] = c.callableParameterType(*parameter, resolved)
			if parameter.IsField {
				declareField(parameter.Name, parameter.Type, parameter.Visibility, parameter.Span, declarationNameSpan(parameter.Name, parameter.Span), func(string) {})
			}
		}
		symbol.constructorVariadic = hasVariadicParameter(decl.Constructor.Parameters)
	}
	for _, method := range decl.Methods {
		c.validateLabels(method.Body)
		parameters := make([]Type, len(method.Parameters))
		for i, parameter := range method.Parameters {
			resolved := c.resolveType(parameter.Type)
			c.rejectResultValueType(resolved, parameter.Type.Span, "parameters")
			c.rejectTaskAPIType(resolved, parameter.Type.Span, "method parameters")
			parameters[i] = c.callableParameterType(parameter, resolved)
		}
		result := c.resolveType(method.ReturnType)
		c.rejectTaskAPIType(result, method.ReturnType.Span, "method return types")
		method.GoName = memberGoName(method.Name, method.Visibility)
		inherited, replaces := symbol.methods[method.Name]
		if field, conflicts := symbol.fields[method.Name]; conflicts {
			c.report(method.Span, fmt.Sprintf("method %q conflicts with field declared by class %s", method.Name, field.declaringClass))
		}
		if method.Static && (method.Virtual || method.Override) {
			c.report(method.Span, "static methods cannot be virtual or override")
		}
		if method.Virtual && method.Override {
			c.report(method.Span, "override already remains virtual; remove the virtual modifier")
		}
		if method.Final && !method.Override {
			c.report(method.Span, "final methods must override an inherited virtual method")
		}
		if method.Virtual && method.Visibility == ast.Private {
			c.report(method.Span, "virtual methods must be public or protected")
		}
		methodType := Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: hasVariadicParameter(method.Parameters), Result: &result}
		if method.Static && len(symbol.typeParameters) != 0 {
			methodType.TypeParameters = append([]Type(nil), symbol.typeParameters...)
			methodType.Generic = true
		}
		virtualOwner := ""
		if replaces && inherited.declaringClass == decl.Name {
			c.report(method.Span, fmt.Sprintf("duplicate method %q", method.Name))
			continue
		}
		if replaces {
			switch {
			case !method.Override:
				c.report(method.Span, fmt.Sprintf("method %q replaces inherited method from %s; add override", method.Name, inherited.declaringClass))
			case inherited.final:
				c.report(method.Span, fmt.Sprintf("method %q in %s is final and cannot be overridden", method.Name, inherited.declaringClass))
			case inherited.static:
				c.report(method.Span, fmt.Sprintf("static method %q cannot be overridden", method.Name))
			case !inherited.virtual:
				c.report(method.Span, fmt.Sprintf("method %q in %s is not virtual", method.Name, inherited.declaringClass))
			case method.Static:
				c.report(method.Span, fmt.Sprintf("override method %q cannot be static", method.Name))
			case method.Visibility != inherited.visibility:
				c.report(method.Span, fmt.Sprintf("override method %q must preserve inherited visibility", method.Name))
			case !exactType(methodType, inherited.typeInfo):
				c.report(method.Span, fmt.Sprintf("override method %q has an incompatible signature", method.Name))
			}
			virtualOwner = inherited.virtualOwner
		} else if method.Override {
			c.report(method.Span, fmt.Sprintf("method %q has override but no inherited method", method.Name))
		}
		if method.Virtual && virtualOwner == "" {
			virtualOwner = decl.Name
		}
		method.VirtualOwner = virtualOwner
		symbol.methods[method.Name] = methodSymbol{
			typeInfo: methodType, visibility: method.Visibility, static: method.Static, goName: method.GoName,
			declarationSpan: method.NameSpan, declaringClass: decl.Name,
			virtual: method.Virtual || method.Override, final: method.Final, virtualOwner: virtualOwner,
		}
	}
	owners := map[string]bool{}
	for _, method := range symbol.methods {
		if method.virtualOwner != "" {
			owners[method.virtualOwner] = true
		}
	}
	for owner := range owners {
		decl.VirtualOwners = append(decl.VirtualOwners, owner)
	}
	sort.Strings(decl.VirtualOwners)
	for _, implemented := range decl.Implements {
		if implemented.IsArray() || implemented.IsFunction() {
			c.report(implemented.Span, "implements expects an interface name")
			continue
		}
		if implemented.Qualifier != "" || implemented.Go || implemented.Name == "error" {
			contract := c.resolveType(implemented)
			goInterface := underlyingGoInterface(contract.GoType)
			if contract.Kind == Invalid {
				continue
			}
			if goInterface == nil {
				c.report(implemented.Span, fmt.Sprintf("Go type %s is not an interface", contract.String()))
				continue
			}
			duplicate := false
			for _, existing := range symbol.goImplements {
				if gotypes.Identical(existing, contract.GoType) {
					duplicate = true
					break
				}
			}
			if duplicate {
				c.report(implemented.Span, fmt.Sprintf("duplicate implemented Go interface %s", contract.String()))
				continue
			}
			symbol.goImplements = append(symbol.goImplements, contract.GoType)
			c.validateGoInterfaceImplementation(decl.Name, symbol, contract, goInterface, implemented.Span)
			continue
		}
		if _, exists := c.interfaces[implemented.Name]; !exists {
			c.report(implemented.Span, fmt.Sprintf("unknown interface %q", implemented.Name))
			continue
		}
		contractType := c.resolveType(implemented)
		if contractType.Kind == Invalid {
			continue
		}
		if contractType.Kind != Interface {
			c.report(implemented.Span, fmt.Sprintf("unknown interface %q", implemented.Name))
			continue
		}
		contract := c.interfaces[implemented.Name]
		contractName := contractType.String()
		if symbol.implements[contractName] {
			c.report(implemented.Span, fmt.Sprintf("duplicate implemented interface %q", contractName))
			continue
		}
		symbol.implements[contractName] = true
		symbol.implementedTypes = append(symbol.implementedTypes, contractType)
		bindings := nativeInterfaceBindings(contract, contractType)
		for name, required := range contract.methods {
			requiredType := substituteNativeTypeParameters(required.typeInfo, bindings)
			actual, exists := symbol.methods[name]
			switch {
			case !exists:
				c.report(decl.Span, fmt.Sprintf("class %s does not implement %s: missing method %s", decl.Name, contractName, name))
			case actual.visibility != ast.Public:
				c.report(decl.Span, fmt.Sprintf("class %s does not implement %s: method %s must be public", decl.Name, contractName, name))
			case actual.static:
				c.report(decl.Span, fmt.Sprintf("class %s does not implement %s: method %s cannot be static", decl.Name, contractName, name))
			case !exactType(actual.typeInfo, requiredType):
				c.report(decl.Span, fmt.Sprintf("class %s does not implement %s: method %s has an incompatible signature", decl.Name, contractName, name))
			}
		}
	}
}

func (c *Checker) declareStruct(decl *ast.StructDecl) {
	symbol := c.structs[decl.Name]
	if symbol == nil {
		fields := map[string]Type{}
		fieldNames := map[string]string{}
		symbol = &structSymbol{
			fields: map[string]fieldSymbol{}, methods: map[string]methodSymbol{}, declarationSpan: decl.NameSpan,
			typeInfo: Type{Kind: Struct, Name: decl.Name, Fields: fields, FieldNames: fieldNames},
		}
		c.structs[decl.Name] = symbol
	}
	c.pushTypeParameterScope(symbol.typeParamScope)
	defer c.popTypeParameterScope()
	for i := range decl.Fields {
		field := &decl.Fields[i]
		if _, exists := symbol.fields[field.Name]; exists {
			c.report(field.Span, fmt.Sprintf("duplicate struct field %q", field.Name))
			continue
		}
		fieldType := c.resolveType(field.Type)
		if fieldType.Kind == Void {
			c.report(field.Type.Span, fmt.Sprintf("struct field %q cannot have type void", field.Name))
		}
		c.rejectResultValueType(fieldType, field.Type.Span, "struct fields")
		c.rejectTaskAPIType(fieldType, field.Type.Span, "struct fields")
		goName := memberGoName(field.Name, field.Visibility)
		field.GoName = goName
		symbol.fields[field.Name] = fieldSymbol{
			typeInfo: fieldType, visibility: field.Visibility, goName: goName, declarationSpan: field.NameSpan,
		}
		symbol.typeInfo.Fields[field.Name] = fieldType
		symbol.typeInfo.FieldNames[field.Name] = goName
	}
	for _, method := range decl.Methods {
		c.declareStructMethod(symbol, method)
	}
}

func (c *Checker) declareStructMethod(symbol *structSymbol, method *ast.MethodDecl) {
	if _, exists := symbol.fields[method.Name]; exists {
		c.report(method.Span, fmt.Sprintf("struct member %q conflicts with a field", method.Name))
		return
	}
	if _, exists := symbol.methods[method.Name]; exists {
		c.report(method.Span, fmt.Sprintf("duplicate struct method %q", method.Name))
		return
	}
	parameters := make([]Type, len(method.Parameters))
	for i, parameter := range method.Parameters {
		resolved := c.resolveType(parameter.Type)
		if resolved.Kind == Void {
			c.report(parameter.Type.Span, "parameters cannot have type void")
		}
		c.rejectResultValueType(resolved, parameter.Type.Span, "parameters")
		c.rejectTaskAPIType(resolved, parameter.Type.Span, "method parameters")
		parameters[i] = c.callableParameterType(parameter, resolved)
	}
	result := c.resolveType(method.ReturnType)
	c.rejectTaskAPIType(result, method.ReturnType.Span, "method return types")
	method.GoName = memberGoName(method.Name, method.Visibility)
	symbol.methods[method.Name] = methodSymbol{
		typeInfo:   Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: hasVariadicParameter(method.Parameters), Result: &result},
		visibility: method.Visibility, pointerReceiver: method.PointerReceiver,
		goName: method.GoName, declarationSpan: method.NameSpan,
	}
}

func (c *Checker) declareReceiverMethods(program *ast.Program) {
	for _, declaration := range program.Declarations {
		method, ok := declaration.(*ast.MethodDecl)
		if !ok {
			continue
		}
		receiverRef := method.ReceiverType
		nullableReceiver := receiverRef.Nullable
		method.PointerReceiver = receiverRef.IsPointer()
		if method.PointerReceiver && receiverRef.Pointee != nil {
			receiverRef = *receiverRef.Pointee
		}
		if nullableReceiver || receiverRef.Nullable || receiverRef.Qualifier != "" || receiverRef.Name == "" || receiverRef.IsArray() || receiverRef.IsPointer() || receiverRef.IsFunction() || receiverRef.IsObject() {
			c.report(method.ReceiverType.Span, "external method receiver must be a native struct value or pointer, or a defined type value or pointer")
			continue
		}
		structure := c.structs[receiverRef.Name]
		if structure != nil {
			if structure.declarationSpan.Path != method.Span.Path {
				c.report(method.ReceiverType.Span, fmt.Sprintf("external method receiver %s must be declared in the same module", receiverRef.Name))
				continue
			}
			typeParameterScope, valid := c.prepareReceiverTypeParameters(method, structure.typeParameters, receiverRef.GenericArguments, "generic struct")
			if !valid {
				continue
			}
			c.receiverTypeParameters[method] = typeParameterScope
			c.pushTypeParameterScope(typeParameterScope)
			c.declareStructMethod(structure, method)
			c.popTypeParameterScope()
			continue
		}
		named := c.nativeTypes[receiverRef.Name]
		if named == nil {
			c.report(method.ReceiverType.Span, fmt.Sprintf("external method receiver %s is not a native struct or defined type", formatTypeRefForDiagnostic(method.ReceiverType)))
			continue
		}
		if named.declaration.NameSpan.Path != method.Span.Path {
			c.report(method.ReceiverType.Span, fmt.Sprintf("external method receiver %s must be declared in the same module", receiverRef.Name))
			continue
		}
		if named.declaration.Alias {
			c.report(method.ReceiverType.Span, fmt.Sprintf("external method receiver %s is an alias; methods require a distinct defined type", receiverRef.Name))
			continue
		}
		typeParameterScope, valid := c.prepareReceiverTypeParameters(method, named.typeParameters, receiverRef.GenericArguments, "generic defined type")
		if !valid {
			continue
		}
		c.receiverTypeParameters[method] = typeParameterScope
		c.pushTypeParameterScope(typeParameterScope)
		resolved := c.resolveNativeType(named)
		if resolved.Kind == Invalid {
			c.popTypeParameterScope()
			continue
		}
		if base, ok := resolved.GoType.(*gotypes.Named); ok {
			switch base.Underlying().(type) {
			case *gotypes.Pointer, *gotypes.Interface:
				c.report(method.ReceiverType.Span, fmt.Sprintf("defined type %s has a pointer or interface underlying type and cannot declare Go receiver methods", receiverRef.Name))
				c.popTypeParameterScope()
				continue
			}
		}
		c.declareNativeTypeMethod(named, method)
		c.popTypeParameterScope()
	}
}

func (c *Checker) prepareReceiverTypeParameters(method *ast.MethodDecl, targetParameters []Type, receiverArguments []ast.TypeRef, targetKind string) (map[string]Type, bool) {
	if len(targetParameters) == 0 {
		if len(method.TypeParameters) != 0 || len(receiverArguments) != 0 {
			c.report(method.ReceiverType.Span, fmt.Sprintf("non-generic receiver type %s cannot declare receiver type parameters", formatTypeRefForDiagnostic(method.ReceiverType)))
			return nil, false
		}
		return nil, true
	}
	valid := true
	if len(method.TypeParameters) != len(targetParameters) {
		c.report(method.NameSpan, fmt.Sprintf("external method on %s %s requires %d receiver type parameters, got %d", targetKind, formatTypeRefForDiagnostic(method.ReceiverType), len(targetParameters), len(method.TypeParameters)))
		valid = false
	}
	if len(receiverArguments) != len(targetParameters) {
		c.report(method.ReceiverType.Span, fmt.Sprintf("external method receiver %s requires %d type arguments, got %d", formatTypeRefForDiagnostic(method.ReceiverType), len(targetParameters), len(receiverArguments)))
		valid = false
	}
	_, declared := c.declareTypeParameters(method.TypeParameters, "generic receiver method")
	scope := make(map[string]Type, len(declared))
	for index, parameter := range method.TypeParameters {
		if _, exists := declared[parameter.Name]; !exists || index >= len(targetParameters) {
			continue
		}
		// Semantically use the declaration's parameter identity so existing
		// generic member substitution remains positional. The source binder may
		// use a different name; code generation retains that spelling from its
		// TypeRef while checking uses the target parameter and its constraint.
		scope[parameter.Name] = targetParameters[index]
	}
	for index, argument := range receiverArguments {
		if index >= len(method.TypeParameters) {
			break
		}
		parameter := method.TypeParameters[index]
		if argument.Name != parameter.Name || argument.Qualifier != "" || argument.Nullable || argument.IsArray() || argument.IsPointer() || argument.IsFunction() || argument.IsObject() || len(argument.GenericArguments) != 0 {
			c.report(argument.Span, fmt.Sprintf("receiver type argument %d must be receiver type parameter %s", index+1, parameter.Name))
			valid = false
		}
	}
	return scope, valid
}

func (c *Checker) declareNativeTypeMethod(symbol *nativeTypeSymbol, method *ast.MethodDecl) {
	if symbol == nil || c.resolveNativeType(symbol).Kind == Invalid {
		return
	}
	if _, exists := symbol.methods[method.Name]; exists {
		c.report(method.Span, fmt.Sprintf("duplicate defined type method %q", method.Name))
		return
	}
	parameters := make([]Type, len(method.Parameters))
	for index, parameter := range method.Parameters {
		resolved := c.resolveType(parameter.Type)
		if resolved.Kind == Void {
			c.report(parameter.Type.Span, "parameters cannot have type void")
		}
		c.rejectResultValueType(resolved, parameter.Type.Span, "parameters")
		c.rejectTaskAPIType(resolved, parameter.Type.Span, "method parameters")
		parameters[index] = c.callableParameterType(parameter, resolved)
	}
	result := c.resolveType(method.ReturnType)
	c.rejectTaskAPIType(result, method.ReturnType.Span, "method return types")
	method.GoName = memberGoName(method.Name, method.Visibility)
	methodType := Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: hasVariadicParameter(method.Parameters), Result: &result}
	symbol.methods[method.Name] = methodSymbol{
		typeInfo: methodType, visibility: method.Visibility, pointerReceiver: method.PointerReceiver,
		goName: method.GoName, declarationSpan: method.NameSpan,
	}

	named, namedOK := symbol.typeInfo.GoType.(*gotypes.Named)
	signature, signatureOK := goTypeOf(methodType)
	if !namedOK || !signatureOK {
		return
	}
	callable, ok := signature.(*gotypes.Signature)
	if !ok {
		return
	}
	receiverType := gotypes.Type(named)
	if method.PointerReceiver {
		receiverType = gotypes.NewPointer(named)
	}
	receiver := gotypes.NewVar(gotoken.NoPos, nil, method.ReceiverName, receiverType)
	methodSignature := gotypes.NewSignatureType(receiver, nil, nil, callable.Params(), callable.Results(), callable.Variadic())
	named.AddMethod(gotypes.NewFunc(gotoken.NoPos, nil, method.GoName, methodSignature))
}

func (c *Checker) validateStructValueCycles(program *ast.Program) {
	for _, declaration := range program.Declarations {
		decl, ok := declaration.(*ast.StructDecl)
		if !ok {
			continue
		}
		for _, field := range decl.Fields {
			symbol := c.structs[decl.Name]
			if symbol == nil {
				continue
			}
			fieldSymbol, exists := symbol.fields[field.Name]
			if exists && c.typeContainsStructByValue(fieldSymbol.typeInfo, decl.Name, map[string]bool{}) {
				c.report(field.Type.Span, fmt.Sprintf("struct %s recursively contains itself by value through field %q; use an explicit pointer, slice, or map indirection", decl.Name, field.Name))
			}
		}
	}
}

func (c *Checker) typeContainsStructByValue(value Type, target string, visiting map[string]bool) bool {
	switch value.Kind {
	case Struct:
		if value.Name == target {
			return true
		}
		if visiting[value.Name] {
			return false
		}
		visiting[value.Name] = true
		defer delete(visiting, value.Name)
		if symbol := c.structs[value.Name]; symbol != nil {
			for _, field := range symbol.fields {
				if c.typeContainsStructByValue(field.typeInfo, target, visiting) {
					return true
				}
			}
		}
	case FixedArray:
		return value.Element != nil && c.typeContainsStructByValue(*value.Element, target, visiting)
	case Object:
		for _, field := range value.Fields {
			if c.typeContainsStructByValue(field, target, visiting) {
				return true
			}
		}
	}
	return false
}

func underlyingGoInterface(goType gotypes.Type) *gotypes.Interface {
	if goType == nil {
		return nil
	}
	contract, _ := gotypes.Unalias(goType).Underlying().(*gotypes.Interface)
	if contract != nil {
		contract.Complete()
	}
	return contract
}

func (c *Checker) validateGoInterfaceImplementation(className string, class *classSymbol, contract Type, goInterface *gotypes.Interface, span source.Span) {
	for i := 0; i < goInterface.NumMethods(); i++ {
		required := goInterface.Method(i)
		var actual methodSymbol
		found := false
		for _, candidate := range class.methods {
			if candidate.goName == required.Name() {
				actual = candidate
				found = true
				break
			}
		}
		if !found {
			c.report(span, fmt.Sprintf("class %s cannot implement Go interface %s: missing exported method %s", className, contract.String(), required.Name()))
			continue
		}
		if actual.visibility != ast.Public {
			c.report(span, fmt.Sprintf("class %s cannot implement Go interface %s: method %s must be public", className, contract.String(), required.Name()))
			continue
		}
		if actual.static {
			c.report(span, fmt.Sprintf("class %s cannot implement Go interface %s: method %s cannot be static", className, contract.String(), required.Name()))
			continue
		}
		actualType, ok := goTypeOf(actual.typeInfo)
		if !ok || !gotypes.Identical(actualType, required.Type()) {
			c.report(span, fmt.Sprintf("class %s cannot implement Go interface %s: method %s has %s, expected %s", className, contract.String(), required.Name(), actual.typeInfo.String(), goTypeDisplayName(required.Type())))
		}
	}
}

func (c *Checker) checkClass(decl *ast.ClassDecl) {
	previousClass := c.currentClass
	previousInConstructor := c.inConstructor
	defer func() {
		c.inConstructor = previousInConstructor
	}()
	c.currentClass = decl.Name
	class := c.classes[decl.Name]
	if class != nil {
		c.pushTypeParameterScope(class.typeParamScope)
		defer c.popTypeParameterScope()
	}
	thisType := Type{Kind: Class, Name: decl.Name}
	if class != nil {
		thisType.TypeArguments = append([]Type(nil), class.typeParameters...)
	}
	if decl.Constructor == nil {
		if class := c.classes[decl.Name]; class != nil && class.base != "" {
			if base := c.classes[class.base]; base != nil && len(base.constructor) != 0 {
				c.report(decl.Span, fmt.Sprintf("class %s needs a constructor that calls super(...) because %s expects %d arguments", decl.Name, class.base, len(base.constructor)))
			}
		}
	}
	if decl.Constructor != nil {
		c.validateSuperConstructorPlacement(decl)
		previousMemberFlow := c.memberFlow
		c.memberFlow = map[memberFlowKey]memberFlowState{}
		c.inConstructor = true
		c.pushScope()
		c.declareLocal("this", thisType, true, nil, decl.Constructor.Span)
		for _, parameter := range decl.Constructor.Parameters {
			t := c.resolveType(parameter.Type)
			c.rejectResultValueType(t, parameter.Type.Span, "parameters")
			c.declareLocal(parameter.Name, t, false, nil, parameter.Span)
		}
		previousResult := c.result
		previousLoopDepth := c.loopDepth
		previousBreakableDepth := c.breakableDepth
		previousExceptionDepth := c.exceptionDepth
		previousCatchTargets := c.catchTargets
		c.loopDepth = 0
		c.breakableDepth = 0
		c.exceptionDepth = 0
		c.catchTargets = nil
		c.result = builtins["void"]
		c.checkBlock(decl.Constructor.Body, false)
		c.result = previousResult
		c.loopDepth = previousLoopDepth
		c.breakableDepth = previousBreakableDepth
		c.exceptionDepth = previousExceptionDepth
		c.catchTargets = previousCatchTargets
		c.popScope()
		c.memberFlow = previousMemberFlow
	}
	c.inConstructor = false
	c.checkClassFieldInitialization(decl)
	for _, method := range decl.Methods {
		previousMemberFlow := c.memberFlow
		c.memberFlow = map[memberFlowKey]memberFlowState{}
		c.pushScope()
		if !method.Static {
			c.declareLocal("this", thisType, true, nil, method.Span)
		}
		for _, parameter := range method.Parameters {
			t := c.resolveType(parameter.Type)
			c.rejectResultValueType(t, parameter.Type.Span, "parameters")
			c.declareLocal(parameter.Name, t, false, nil, parameter.Span)
		}
		previousResult := c.result
		previousLoopDepth := c.loopDepth
		previousBreakableDepth := c.breakableDepth
		previousExceptionDepth := c.exceptionDepth
		previousCatchTargets := c.catchTargets
		c.loopDepth = 0
		c.breakableDepth = 0
		c.exceptionDepth = 0
		c.catchTargets = nil
		c.result = c.resolveType(method.ReturnType)
		c.checkBlock(method.Body, false)
		if c.result.Kind != Void && !definitelyReturns(method.Body) {
			c.report(method.Span, fmt.Sprintf("method %q may complete without returning %s", method.Name, c.result.String()))
		}
		c.result = previousResult
		c.loopDepth = previousLoopDepth
		c.breakableDepth = previousBreakableDepth
		c.exceptionDepth = previousExceptionDepth
		c.catchTargets = previousCatchTargets
		c.popScope()
		c.memberFlow = previousMemberFlow
	}
	c.currentClass = previousClass
}

func (c *Checker) validateSuperConstructorPlacement(decl *ast.ClassDecl) {
	class := c.classes[decl.Name]
	if class == nil {
		return
	}
	firstIsSuper := false
	for index, statement := range decl.Constructor.Body.Statements {
		expression, ok := statement.(*ast.ExpressionStmt)
		if !ok {
			continue
		}
		call, ok := expression.Value.(*ast.CallExpr)
		if !ok {
			continue
		}
		name, ok := call.Callee.(*ast.IdentifierExpr)
		if !ok || name.Name != "super" {
			continue
		}
		if index == 0 {
			firstIsSuper = true
		} else {
			c.report(call.Span, "super constructor call must be the first statement")
		}
	}
	if class.base == "" {
		return
	}
	base := c.classes[class.base]
	if base != nil && len(base.constructor) != 0 && !firstIsSuper {
		c.report(decl.Constructor.Span, fmt.Sprintf("constructor for %s must call super(...) first because %s expects %d arguments", decl.Name, class.base, len(base.constructor)))
	}
}

func (c *Checker) checkStruct(decl *ast.StructDecl) {
	symbol := c.structs[decl.Name]
	if symbol != nil {
		c.pushTypeParameterScope(symbol.typeParamScope)
		defer c.popTypeParameterScope()
	}
	for _, method := range decl.Methods {
		c.checkStructMethod(method, "this", decl.Name)
	}
}

func (c *Checker) checkStructMethod(method *ast.MethodDecl, receiverName, structName string) {
	c.validateLabels(method.Body)
	valueReceiver := Type{Kind: Struct, Name: structName}
	if method.External {
		receiverRef := method.ReceiverType
		if receiverRef.IsPointer() && receiverRef.Pointee != nil {
			receiverRef = *receiverRef.Pointee
		}
		valueReceiver = c.resolveType(receiverRef)
	} else if symbol := c.structs[structName]; symbol != nil {
		valueReceiver = symbol.typeInfo
		if len(symbol.typeParameters) != 0 {
			valueReceiver.TypeArguments = append([]Type(nil), symbol.typeParameters...)
			valueReceiver.TypeParameters = nil
			valueReceiver.Generic = false
		}
	}
	receiver := valueReceiver
	if method.PointerReceiver {
		receiver = Type{Kind: GoPointer, Name: "*" + structName, Element: &valueReceiver}
	}
	c.pushScope()
	c.declareLocal(receiverName, receiver, true, nil, method.ReceiverNameSpan)
	if method.External {
		scope := c.scopes[len(c.scopes)-1]
		symbol := scope[receiverName]
		symbol.declarationSpan = method.ReceiverNameSpan
		scope[receiverName] = symbol
	}
	for _, parameter := range method.Parameters {
		t := c.resolveType(parameter.Type)
		c.rejectResultValueType(t, parameter.Type.Span, "parameters")
		c.declareLocal(parameter.Name, t, false, nil, parameter.Span)
	}
	previousResult := c.result
	previousLoopDepth := c.loopDepth
	previousBreakableDepth := c.breakableDepth
	previousExceptionDepth := c.exceptionDepth
	previousCatchTargets := c.catchTargets
	c.loopDepth = 0
	c.breakableDepth = 0
	c.exceptionDepth = 0
	c.catchTargets = nil
	c.result = c.resolveType(method.ReturnType)
	c.checkBlock(method.Body, false)
	if c.result.Kind != Void && !definitelyReturns(method.Body) {
		c.report(method.Span, fmt.Sprintf("method %q may complete without returning %s", method.Name, c.result.String()))
	}
	c.result = previousResult
	c.loopDepth = previousLoopDepth
	c.breakableDepth = previousBreakableDepth
	c.exceptionDepth = previousExceptionDepth
	c.catchTargets = previousCatchTargets
	c.popScope()
}

func (c *Checker) checkNativeTypeMethod(method *ast.MethodDecl, receiverName, typeName string) {
	c.validateLabels(method.Body)
	symbol := c.nativeTypes[typeName]
	if symbol == nil {
		return
	}
	receiverRef := method.ReceiverType
	if receiverRef.IsPointer() && receiverRef.Pointee != nil {
		receiverRef = *receiverRef.Pointee
	}
	valueReceiver := c.resolveType(receiverRef)
	if valueReceiver.Kind == Invalid || symbol.declaration.Alias {
		return
	}
	receiver := valueReceiver
	if method.PointerReceiver {
		pointerType := gotypes.NewPointer(valueReceiver.GoType)
		receiver = Type{Kind: GoPointer, Name: "*" + typeName, Element: &valueReceiver, GoType: pointerType}
	}
	c.pushScope()
	c.declareLocal(receiverName, receiver, true, nil, method.ReceiverNameSpan)
	scope := c.scopes[len(c.scopes)-1]
	receiverSymbol := scope[receiverName]
	receiverSymbol.declarationSpan = method.ReceiverNameSpan
	scope[receiverName] = receiverSymbol
	for _, parameter := range method.Parameters {
		parameterType := c.resolveType(parameter.Type)
		c.rejectResultValueType(parameterType, parameter.Type.Span, "parameters")
		c.declareLocal(parameter.Name, parameterType, false, nil, parameter.Span)
	}
	previousResult := c.result
	previousLoopDepth := c.loopDepth
	previousBreakableDepth := c.breakableDepth
	previousExceptionDepth := c.exceptionDepth
	previousCatchTargets := c.catchTargets
	c.loopDepth = 0
	c.breakableDepth = 0
	c.exceptionDepth = 0
	c.catchTargets = nil
	c.result = c.resolveType(method.ReturnType)
	c.checkBlock(method.Body, false)
	if c.result.Kind != Void && !definitelyReturns(method.Body) {
		c.report(method.Span, fmt.Sprintf("method %q may complete without returning %s", method.Name, c.result.String()))
	}
	c.result = previousResult
	c.loopDepth = previousLoopDepth
	c.breakableDepth = previousBreakableDepth
	c.exceptionDepth = previousExceptionDepth
	c.catchTargets = previousCatchTargets
	c.popScope()
}

func (c *Checker) checkClassFieldInitialization(decl *ast.ClassDecl) {
	class := c.classes[decl.Name]
	if class == nil {
		return
	}
	required := map[string]source.Span{}
	for _, field := range decl.Fields {
		symbol, exists := class.fields[field.Name]
		if !exists || symbol.typeInfo.Kind == Invalid || symbol.typeInfo.Kind == Nullable || !isNullableBaseType(symbol.typeInfo) {
			continue
		}
		required[field.Name] = field.NameSpan
	}
	if len(required) == 0 {
		return
	}
	initialized := map[string]bool{}
	if decl.Constructor != nil {
		flow := constructorInitializationBlock(decl.Constructor.Body, initialized, required)
		initialized = flow.continuing
		if initialized == nil {
			initialized = map[string]bool{}
		}
	}
	names := make([]string, 0, len(required))
	for name := range required {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if initialized[name] {
			continue
		}
		fieldType := class.fields[name].typeInfo
		c.report(required[name], fmt.Sprintf("non-null field %q of type %s must be initialized on every constructor path; assign this.%s or declare it as %s | null", name, fieldType.String(), name, fieldType.String()))
	}
}

type constructorInitializationFlow struct {
	continuing   map[string]bool
	breaks       []map[string]bool
	continues    []map[string]bool
	fallthroughs []map[string]bool
}

func constructorInitializationBlock(block *ast.BlockStmt, initial map[string]bool, required map[string]source.Span) constructorInitializationFlow {
	state := cloneFieldInitialization(initial)
	if block == nil {
		return constructorInitializationFlow{continuing: state}
	}
	var breaks []map[string]bool
	var continues []map[string]bool
	var fallthroughs []map[string]bool
	for _, statement := range block.Statements {
		if state == nil {
			break
		}
		flow := constructorInitializationStatement(statement, state, required)
		breaks = append(breaks, flow.breaks...)
		continues = append(continues, flow.continues...)
		fallthroughs = append(fallthroughs, flow.fallthroughs...)
		state = flow.continuing
	}
	return constructorInitializationFlow{continuing: state, breaks: breaks, continues: continues, fallthroughs: fallthroughs}
}

func constructorInitializationStatement(statement ast.Statement, initial map[string]bool, required map[string]source.Span) constructorInitializationFlow {
	switch statement := statement.(type) {
	case *ast.LabeledStmt:
		return constructorInitializationStatement(statement.Statement, initial, required)
	case *ast.AssignmentStmt:
		state := cloneFieldInitialization(initial)
		if statement.Operator != "" && statement.Operator != "=" {
			return constructorInitializationFlow{continuing: state}
		}
		member, ok := statement.Target.(*ast.MemberExpr)
		if !ok {
			return constructorInitializationFlow{continuing: state}
		}
		receiver, ok := member.Object.(*ast.IdentifierExpr)
		if ok && receiver.Name == "this" {
			if _, tracked := required[member.Name]; tracked {
				state[member.Name] = true
			}
		}
		return constructorInitializationFlow{continuing: state}
	case *ast.BlockStmt:
		return constructorInitializationBlock(statement, initial, required)
	case *ast.IfStmt:
		thenFlow := constructorInitializationBlock(statement.Then, initial, required)
		elseFlow := constructorInitializationFlow{continuing: cloneFieldInitialization(initial)}
		if statement.Else != nil {
			elseFlow = constructorInitializationStatement(statement.Else, initial, required)
		}
		return constructorInitializationFlow{
			continuing:   intersectCompletingFieldInitialization(thenFlow.continuing, elseFlow.continuing),
			breaks:       append(thenFlow.breaks, elseFlow.breaks...),
			continues:    append(thenFlow.continues, elseFlow.continues...),
			fallthroughs: append(thenFlow.fallthroughs, elseFlow.fallthroughs...),
		}
	case *ast.ValueSwitchStmt:
		states := make([]map[string]bool, 0, len(statement.Cases)+1)
		var continues []map[string]bool
		var incomingFallthrough map[string]bool
		hasDefault := false
		for index := range statement.Cases {
			clause := &statement.Cases[index]
			hasDefault = hasDefault || clause.Default
			caseInitial := initial
			if incomingFallthrough != nil {
				caseInitial = intersectFieldInitialization(initial, incomingFallthrough)
			}
			flow := constructorInitializationBlock(clause.Body, caseInitial, required)
			states = append(states, flow.breaks...)
			if clause.FallsThrough {
				incomingFallthrough = intersectFieldInitializationStates(flow.fallthroughs)
			} else {
				incomingFallthrough = nil
				if flow.continuing != nil {
					states = append(states, flow.continuing)
				}
			}
			continues = append(continues, flow.continues...)
		}
		if !hasDefault {
			states = append(states, cloneFieldInitialization(initial))
		}
		return constructorInitializationFlow{continuing: intersectFieldInitializationStates(states), continues: continues}
	case *ast.TypeSwitchStmt:
		states := make([]map[string]bool, 0, len(statement.Cases)+1)
		var continues []map[string]bool
		hasDefault := false
		for index := range statement.Cases {
			clause := &statement.Cases[index]
			hasDefault = hasDefault || clause.Default
			flow := constructorInitializationBlock(clause.Body, initial, required)
			states = appendConstructorCompletingStates(states, flow)
			continues = append(continues, flow.continues...)
		}
		if !hasDefault {
			states = append(states, cloneFieldInitialization(initial))
		}
		return constructorInitializationFlow{continuing: intersectFieldInitializationStates(states), continues: continues}
	case *ast.SelectStmt:
		states := make([]map[string]bool, 0, len(statement.Cases))
		var continues []map[string]bool
		for index := range statement.Cases {
			flow := constructorInitializationBlock(statement.Cases[index].Body, initial, required)
			states = appendConstructorCompletingStates(states, flow)
			continues = append(continues, flow.continues...)
		}
		return constructorInitializationFlow{continuing: intersectFieldInitializationStates(states), continues: continues}
	case *ast.WhileStmt:
		return constructorInitializationLoop(statement.Body, initial, required, statement.GuaranteedEntry || expressionAlwaysTrue(statement.Condition), false)
	case *ast.ForStmt:
		state := cloneFieldInitialization(initial)
		if statement.Initializer != nil {
			initializer := constructorInitializationStatement(statement.Initializer, state, required)
			state = initializer.continuing
			if state == nil {
				return initializer
			}
		}
		guaranteed := statement.Condition == nil || statement.GuaranteedEntry || expressionAlwaysTrue(statement.Condition)
		return constructorInitializationLoop(statement.Body, state, required, guaranteed, false)
	case *ast.ForRangeStmt:
		return constructorInitializationLoop(statement.Body, initial, required, statement.GuaranteedNonEmpty || rangeExpressionGuaranteedNonEmpty(statement.Source), true)
	case *ast.BranchStmt:
		if statement.Kind == ast.BreakBranch {
			return constructorInitializationFlow{breaks: []map[string]bool{cloneFieldInitialization(initial)}}
		}
		if statement.Kind == ast.ContinueBranch {
			return constructorInitializationFlow{continues: []map[string]bool{cloneFieldInitialization(initial)}}
		}
		if statement.Kind == ast.FallthroughBranch {
			return constructorInitializationFlow{fallthroughs: []map[string]bool{cloneFieldInitialization(initial)}}
		}
		return constructorInitializationFlow{}
	case *ast.ReturnStmt, *ast.ThrowStmt:
		return constructorInitializationFlow{}
	case *ast.TryStmt:
		body := constructorInitializationBlock(statement.Body, initial, required)
		states := []map[string]bool{}
		if body.continuing != nil {
			states = append(states, body.continuing)
		}
		for _, clause := range statement.Catches {
			caught := constructorInitializationBlock(clause.Body, initial, required)
			if caught.continuing != nil {
				states = append(states, caught.continuing)
			}
		}
		continuing := intersectFieldInitializationStates(states)
		if statement.FinallyBody != nil && continuing != nil {
			return constructorInitializationBlock(statement.FinallyBody, continuing, required)
		}
		return constructorInitializationFlow{continuing: continuing}
	default:
		// Loops and other statements do not establish initialization. In
		// particular, a loop body may execute zero times.
		return constructorInitializationFlow{continuing: cloneFieldInitialization(initial)}
	}
}

func constructorInitializationLoop(body *ast.BlockStmt, initial map[string]bool, required map[string]source.Span, guaranteed, naturalExit bool) constructorInitializationFlow {
	if !guaranteed {
		return constructorInitializationFlow{continuing: cloneFieldInitialization(initial)}
	}
	flow := constructorInitializationBlock(body, initial, required)
	exits := append([]map[string]bool(nil), flow.breaks...)
	if naturalExit {
		if flow.continuing != nil {
			exits = append(exits, flow.continuing)
		}
		exits = append(exits, flow.continues...)
	}
	return constructorInitializationFlow{continuing: intersectFieldInitializationStates(exits)}
}

func expressionAlwaysTrue(expression ast.Expression) bool {
	value, known := booleanConstantValue(expression)
	return known && value
}

func (c *Checker) expressionAlwaysTrue(expression ast.Expression) bool {
	value, known := c.resolvedBooleanConstantValue(expression, map[source.Span]bool{})
	return known && value
}

func (c *Checker) resolvedBooleanConstantValue(expression ast.Expression, seen map[source.Span]bool) (bool, bool) {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		symbol, ok := c.lookupSymbol(expression.Name, expression.Span)
		if !ok || !symbol.constant || symbol.declaration == nil || seen[symbol.declarationSpan] {
			return false, false
		}
		seen[symbol.declarationSpan] = true
		value, known := c.resolvedBooleanConstantValue(symbol.declaration.Value, seen)
		delete(seen, symbol.declarationSpan)
		return value, known
	case *ast.LiteralExpr:
		if expression.Kind != ast.BooleanLiteral {
			return false, false
		}
		return expression.Text == "true", expression.Text == "true" || expression.Text == "false"
	case *ast.UnaryExpr:
		if expression.Operator != "!" {
			return false, false
		}
		value, known := c.resolvedBooleanConstantValue(expression.Operand, seen)
		return !value, known
	case *ast.BinaryExpr:
		switch expression.Operator {
		case "&&", "||":
			left, leftKnown := c.resolvedBooleanConstantValue(expression.Left, seen)
			right, rightKnown := c.resolvedBooleanConstantValue(expression.Right, seen)
			if !leftKnown || !rightKnown {
				return false, false
			}
			if expression.Operator == "&&" {
				return left && right, true
			}
			return left || right, true
		case "==", "===", "!=", "!==":
			if left, leftKnown := c.resolvedBooleanConstantValue(expression.Left, seen); leftKnown {
				if right, rightKnown := c.resolvedBooleanConstantValue(expression.Right, seen); rightKnown {
					return compareConstantEquality(left == right, expression.Operator), true
				}
			}
			if left, leftKnown := c.resolvedIntegerConstantValue(expression.Left); leftKnown {
				if right, rightKnown := c.resolvedIntegerConstantValue(expression.Right); rightKnown {
					return compareConstantEquality(left.Cmp(right) == 0, expression.Operator), true
				}
			}
			if left, leftKnown := c.resolvedStringConstantValue(expression.Left, seen); leftKnown {
				if right, rightKnown := c.resolvedStringConstantValue(expression.Right, seen); rightKnown {
					return compareConstantEquality(left == right, expression.Operator), true
				}
			}
		case "<", "<=", ">", ">=":
			if left, leftKnown := c.resolvedIntegerConstantValue(expression.Left); leftKnown {
				if right, rightKnown := c.resolvedIntegerConstantValue(expression.Right); rightKnown {
					return compareConstantOrdering(left.Cmp(right), expression.Operator), true
				}
			}
			if left, leftKnown := c.resolvedStringConstantValue(expression.Left, seen); leftKnown {
				if right, rightKnown := c.resolvedStringConstantValue(expression.Right, seen); rightKnown {
					return compareConstantOrdering(strings.Compare(left, right), expression.Operator), true
				}
			}
		}
	}
	return false, false
}

func (c *Checker) resolvedStringConstantValue(expression ast.Expression, seen map[source.Span]bool) (string, bool) {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		symbol, ok := c.lookupSymbol(expression.Name, expression.Span)
		if !ok || !symbol.constant || symbol.declaration == nil || seen[symbol.declarationSpan] {
			return "", false
		}
		seen[symbol.declarationSpan] = true
		value, known := c.resolvedStringConstantValue(symbol.declaration.Value, seen)
		delete(seen, symbol.declarationSpan)
		return value, known
	case *ast.LiteralExpr:
		if expression.Kind != ast.StringLiteral {
			return "", false
		}
		value, err := strconv.Unquote(expression.Text)
		return value, err == nil
	case *ast.BinaryExpr:
		if expression.Operator != "+" {
			return "", false
		}
		left, leftKnown := c.resolvedStringConstantValue(expression.Left, seen)
		right, rightKnown := c.resolvedStringConstantValue(expression.Right, seen)
		if !leftKnown || !rightKnown {
			return "", false
		}
		return left + right, true
	default:
		return "", false
	}
}

func (c *Checker) rangeExpressionGuaranteedNonEmpty(expression ast.Expression) bool {
	return c.resolvedRangeExpressionGuaranteedNonEmpty(expression, map[source.Span]bool{})
}

func (c *Checker) resolvedRangeExpressionGuaranteedNonEmpty(expression ast.Expression, seen map[source.Span]bool) bool {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		symbol, ok := c.lookupSymbol(expression.Name, expression.Span)
		if !ok || !symbol.constant || symbol.declaration == nil || seen[symbol.declarationSpan] {
			return false
		}
		seen[symbol.declarationSpan] = true
		guaranteed := c.resolvedRangeExpressionGuaranteedNonEmpty(symbol.declaration.Value, seen)
		delete(seen, symbol.declarationSpan)
		return guaranteed
	case *ast.ArrayLiteralExpr:
		return len(expression.Elements) != 0
	case *ast.CallExpr:
		switch expression.Builtin {
		case ast.AppendCall:
			if len(expression.Arguments) == 0 {
				return false
			}
			if c.resolvedRangeExpressionGuaranteedNonEmpty(expression.Arguments[0], seen) {
				return true
			}
			if !expression.Expanded {
				return len(expression.Arguments) > 1
			}
			return len(expression.Arguments) == 2 && c.resolvedRangeExpressionGuaranteedNonEmpty(expression.Arguments[1], seen)
		case ast.MakeSliceCall:
			if len(expression.Arguments) == 0 {
				return false
			}
			length, known := c.resolvedIntegerConstantValue(expression.Arguments[0])
			return known && length.Sign() > 0
		default:
			return false
		}
	default:
		value, known := c.resolvedStringConstantValue(expression, seen)
		return known && value != ""
	}
}

func rangeExpressionGuaranteedNonEmpty(expression ast.Expression) bool {
	switch expression := expression.(type) {
	case *ast.ArrayLiteralExpr:
		return len(expression.Elements) != 0
	case *ast.CallExpr:
		switch expression.Builtin {
		case ast.AppendCall:
			if len(expression.Arguments) == 0 {
				return false
			}
			if rangeExpressionGuaranteedNonEmpty(expression.Arguments[0]) {
				return true
			}
			if !expression.Expanded {
				return len(expression.Arguments) > 1
			}
			return len(expression.Arguments) == 2 && rangeExpressionGuaranteedNonEmpty(expression.Arguments[1])
		case ast.MakeSliceCall:
			if len(expression.Arguments) == 0 {
				return false
			}
			length, known := integerConstantValue(expression.Arguments[0])
			return known && length.Sign() > 0
		default:
			return false
		}
	default:
		value, known := stringConstantValue(expression)
		return known && value != ""
	}
}

func booleanConstantValue(expression ast.Expression) (bool, bool) {
	switch expression := expression.(type) {
	case *ast.LiteralExpr:
		if expression.Kind != ast.BooleanLiteral {
			return false, false
		}
		return expression.Text == "true", expression.Text == "true" || expression.Text == "false"
	case *ast.UnaryExpr:
		if expression.Operator != "!" {
			return false, false
		}
		value, known := booleanConstantValue(expression.Operand)
		return !value, known
	case *ast.BinaryExpr:
		switch expression.Operator {
		case "&&", "||":
			left, leftKnown := booleanConstantValue(expression.Left)
			right, rightKnown := booleanConstantValue(expression.Right)
			if !leftKnown || !rightKnown {
				return false, false
			}
			if expression.Operator == "&&" {
				return left && right, true
			}
			return left || right, true
		case "==", "===", "!=", "!==":
			if left, leftKnown := booleanConstantValue(expression.Left); leftKnown {
				if right, rightKnown := booleanConstantValue(expression.Right); rightKnown {
					return compareConstantEquality(left == right, expression.Operator), true
				}
			}
			if left, leftKnown := integerConstantValue(expression.Left); leftKnown {
				if right, rightKnown := integerConstantValue(expression.Right); rightKnown {
					return compareConstantEquality(left.Cmp(right) == 0, expression.Operator), true
				}
			}
			if left, leftKnown := stringConstantValue(expression.Left); leftKnown {
				if right, rightKnown := stringConstantValue(expression.Right); rightKnown {
					return compareConstantEquality(left == right, expression.Operator), true
				}
			}
		case "<", "<=", ">", ">=":
			if left, leftKnown := integerConstantValue(expression.Left); leftKnown {
				if right, rightKnown := integerConstantValue(expression.Right); rightKnown {
					return compareConstantOrdering(left.Cmp(right), expression.Operator), true
				}
			}
			if left, leftKnown := stringConstantValue(expression.Left); leftKnown {
				if right, rightKnown := stringConstantValue(expression.Right); rightKnown {
					return compareConstantOrdering(strings.Compare(left, right), expression.Operator), true
				}
			}
		}
	}
	return false, false
}

func compareConstantEquality(equal bool, operator string) bool {
	if operator == "!=" || operator == "!==" {
		return !equal
	}
	return equal
}

func compareConstantOrdering(comparison int, operator string) bool {
	switch operator {
	case "<":
		return comparison < 0
	case "<=":
		return comparison <= 0
	case ">":
		return comparison > 0
	case ">=":
		return comparison >= 0
	default:
		return false
	}
}

func stringConstantValue(expression ast.Expression) (string, bool) {
	switch expression := expression.(type) {
	case *ast.LiteralExpr:
		if expression.Kind != ast.StringLiteral {
			return "", false
		}
		value, err := strconv.Unquote(expression.Text)
		return value, err == nil
	case *ast.BinaryExpr:
		if expression.Operator != "+" {
			return "", false
		}
		left, leftKnown := stringConstantValue(expression.Left)
		right, rightKnown := stringConstantValue(expression.Right)
		if !leftKnown || !rightKnown {
			return "", false
		}
		return left + right, true
	default:
		return "", false
	}
}

func appendConstructorCompletingStates(states []map[string]bool, flow constructorInitializationFlow) []map[string]bool {
	if flow.continuing != nil {
		states = append(states, flow.continuing)
	}
	return append(states, flow.breaks...)
}

func intersectCompletingFieldInitialization(states ...map[string]bool) map[string]bool {
	continuing := states[:0]
	for _, state := range states {
		if state != nil {
			continuing = append(continuing, state)
		}
	}
	return intersectFieldInitializationStates(continuing)
}

func intersectFieldInitializationStates(states []map[string]bool) map[string]bool {
	if len(states) == 0 {
		return nil
	}
	result := cloneFieldInitialization(states[0])
	for _, state := range states[1:] {
		result = intersectFieldInitialization(result, state)
	}
	return result
}

func cloneFieldInitialization(state map[string]bool) map[string]bool {
	cloned := make(map[string]bool, len(state))
	for name, initialized := range state {
		if initialized {
			cloned[name] = true
		}
	}
	return cloned
}

func intersectFieldInitialization(left, right map[string]bool) map[string]bool {
	intersection := map[string]bool{}
	for name, initialized := range left {
		if initialized && right[name] {
			intersection[name] = true
		}
	}
	return intersection
}

func (c *Checker) validateLabels(body *ast.BlockStmt) {
	if body == nil {
		return
	}
	labels := map[string]*ast.LabeledStmt{}
	used := map[string]bool{}
	var walk func(ast.Statement, func(ast.Statement))
	walk = func(statement ast.Statement, visit func(ast.Statement)) {
		if statement == nil {
			return
		}
		visit(statement)
		switch statement := statement.(type) {
		case *ast.LabeledStmt:
			if statement == nil {
				return
			}
			walk(statement.Statement, visit)
		case *ast.BlockStmt:
			if statement == nil {
				return
			}
			for _, nested := range statement.Statements {
				walk(nested, visit)
			}
		case *ast.IfStmt:
			walk(statement.Then, visit)
			walk(statement.Else, visit)
		case *ast.WhileStmt:
			walk(statement.Body, visit)
		case *ast.ForStmt:
			walk(statement.Body, visit)
		case *ast.ForRangeStmt:
			walk(statement.Body, visit)
		case *ast.SelectStmt:
			for index := range statement.Cases {
				walk(statement.Cases[index].Body, visit)
			}
		case *ast.ValueSwitchStmt:
			for index := range statement.Cases {
				walk(statement.Cases[index].Body, visit)
			}
		case *ast.TypeSwitchStmt:
			for index := range statement.Cases {
				walk(statement.Cases[index].Body, visit)
			}
		case *ast.TryStmt:
			walk(statement.Body, visit)
			for _, clause := range statement.Catches {
				walk(clause.Body, visit)
			}
			if statement.FinallyBody != nil {
				walk(statement.FinallyBody, visit)
			}
		}
	}
	walk(body, func(statement ast.Statement) {
		labeled, ok := statement.(*ast.LabeledStmt)
		if !ok {
			return
		}
		if previous, duplicate := labels[labeled.Label]; duplicate {
			c.report(labeled.LabelSpan, fmt.Sprintf("duplicate label %q; first declared at %d:%d", labeled.Label, previous.LabelSpan.Start.Line, previous.LabelSpan.Start.Column))
			return
		}
		switch labeled.Statement.(type) {
		case *ast.VariableDecl, *ast.MultiVariableDecl:
			c.report(labeled.LabelSpan, fmt.Sprintf("label %q cannot be attached to a variable declaration", labeled.Label))
		}
		labels[labeled.Label] = labeled
	})

	type controlLocation struct {
		blocks []*ast.BlockStmt
		region string
	}
	labelLocations := map[*ast.LabeledStmt]controlLocation{}
	branchLocations := map[*ast.BranchStmt]controlLocation{}
	var locate func(ast.Statement, []*ast.BlockStmt, string)
	locate = func(statement ast.Statement, blocks []*ast.BlockStmt, region string) {
		if statement == nil {
			return
		}
		switch statement := statement.(type) {
		case *ast.LabeledStmt:
			if statement == nil {
				return
			}
			labelLocations[statement] = controlLocation{blocks: append([]*ast.BlockStmt(nil), blocks...), region: region}
			locate(statement.Statement, blocks, region)
		case *ast.BranchStmt:
			branchLocations[statement] = controlLocation{blocks: append([]*ast.BlockStmt(nil), blocks...), region: region}
		case *ast.BlockStmt:
			if statement == nil {
				return
			}
			nestedBlocks := append(append([]*ast.BlockStmt(nil), blocks...), statement)
			for _, nested := range statement.Statements {
				locate(nested, nestedBlocks, region)
			}
		case *ast.IfStmt:
			locate(statement.Then, blocks, region)
			locate(statement.Else, blocks, region)
		case *ast.WhileStmt:
			locate(statement.Body, blocks, region)
		case *ast.ForStmt:
			locate(statement.Body, blocks, region)
		case *ast.ForRangeStmt:
			locate(statement.Body, blocks, region)
		case *ast.SelectStmt:
			for index := range statement.Cases {
				locate(statement.Cases[index].Body, blocks, region)
			}
		case *ast.ValueSwitchStmt:
			for index := range statement.Cases {
				locate(statement.Cases[index].Body, blocks, region)
			}
		case *ast.TypeSwitchStmt:
			for index := range statement.Cases {
				locate(statement.Cases[index].Body, blocks, region)
			}
		case *ast.TryStmt:
			base := fmt.Sprintf("try:%d", statement.Span.Start.Offset)
			locate(statement.Body, blocks, base+":body")
			for index, clause := range statement.Catches {
				locate(clause.Body, blocks, fmt.Sprintf("%s:catch:%d", base, index))
			}
			if statement.FinallyBody != nil {
				locate(statement.FinallyBody, blocks, base+":finally")
			}
		}
	}
	locate(body, nil, "root")

	var validate func(ast.Statement, []*ast.LabeledStmt)
	validate = func(statement ast.Statement, enclosing []*ast.LabeledStmt) {
		if statement == nil {
			return
		}
		switch statement := statement.(type) {
		case *ast.BranchStmt:
			if statement.Label == "" {
				return
			}
			target, exists := labels[statement.Label]
			if !exists {
				c.report(statement.LabelSpan, fmt.Sprintf("undefined label %q", statement.Label))
				return
			}
			statement.ResolvedDeclaration = target.LabelSpan
			used[statement.Label] = true
			if statement.Kind == ast.GotoBranch {
				gotoLocation := branchLocations[statement]
				targetLocation := labelLocations[target]
				if gotoLocation.region != targetLocation.region {
					c.report(statement.LabelSpan, fmt.Sprintf("goto %q cannot cross a try, catch, or finally boundary", statement.Label))
					return
				}
				if !blockPathContains(targetLocation.blocks, gotoLocation.blocks) {
					c.report(statement.LabelSpan, fmt.Sprintf("goto %q cannot jump into a nested block", statement.Label))
					return
				}
				if statement.Span.Start.Offset < target.LabelSpan.Start.Offset && len(targetLocation.blocks) != 0 {
					targetBlock := targetLocation.blocks[len(targetLocation.blocks)-1]
					for _, candidate := range targetBlock.Statements {
						if candidate.GetSpan().Start.Offset <= statement.Span.Start.Offset || candidate.GetSpan().Start.Offset >= target.LabelSpan.Start.Offset {
							continue
						}
						switch declaration := candidate.(type) {
						case *ast.VariableDecl:
							c.report(statement.LabelSpan, fmt.Sprintf("goto %q jumps over declaration of %q", statement.Label, declaration.Name))
							return
						case *ast.MultiVariableDecl:
							name := "_"
							for _, binding := range declaration.Bindings {
								if binding.Name != "_" {
									name = binding.Name
									break
								}
							}
							c.report(statement.LabelSpan, fmt.Sprintf("goto %q jumps over declaration of %q", statement.Label, name))
							return
						}
					}
				}
				return
			}
			enclosingTarget := false
			for index := len(enclosing) - 1; index >= 0; index-- {
				if enclosing[index] == target {
					enclosingTarget = true
					break
				}
			}
			if !enclosingTarget {
				c.report(statement.LabelSpan, fmt.Sprintf("label %q does not enclose this branch", statement.Label))
				return
			}
			if statement.Kind == ast.ContinueBranch && !isContinueLabelTarget(target.Statement) {
				c.report(statement.LabelSpan, fmt.Sprintf("continue label %q must target a loop", statement.Label))
			} else if statement.Kind == ast.BreakBranch && !isBreakLabelTarget(target.Statement) {
				c.report(statement.LabelSpan, fmt.Sprintf("break label %q must target a loop, switch, or select", statement.Label))
			}
		case *ast.LabeledStmt:
			if statement == nil {
				return
			}
			validate(statement.Statement, append(enclosing, statement))
		case *ast.BlockStmt:
			if statement == nil {
				return
			}
			for _, nested := range statement.Statements {
				validate(nested, enclosing)
			}
		case *ast.IfStmt:
			validate(statement.Then, enclosing)
			validate(statement.Else, enclosing)
		case *ast.WhileStmt:
			validate(statement.Body, enclosing)
		case *ast.ForStmt:
			validate(statement.Body, enclosing)
		case *ast.ForRangeStmt:
			validate(statement.Body, enclosing)
		case *ast.SelectStmt:
			for index := range statement.Cases {
				validate(statement.Cases[index].Body, enclosing)
			}
		case *ast.ValueSwitchStmt:
			for index := range statement.Cases {
				validate(statement.Cases[index].Body, enclosing)
			}
		case *ast.TypeSwitchStmt:
			for index := range statement.Cases {
				validate(statement.Cases[index].Body, enclosing)
			}
		case *ast.TryStmt:
			validate(statement.Body, enclosing)
			for _, clause := range statement.Catches {
				validate(clause.Body, enclosing)
			}
			if statement.FinallyBody != nil {
				validate(statement.FinallyBody, enclosing)
			}
		}
	}
	validate(body, nil)
	for name, label := range labels {
		if !used[name] {
			c.report(label.LabelSpan, fmt.Sprintf("label %q is declared but not used", name))
		}
	}
}

func blockPathContains(prefix, path []*ast.BlockStmt) bool {
	if len(prefix) > len(path) {
		return false
	}
	for index := range prefix {
		if prefix[index] != path[index] {
			return false
		}
	}
	return true
}

func isContinueLabelTarget(statement ast.Statement) bool {
	switch statement.(type) {
	case *ast.WhileStmt, *ast.ForStmt, *ast.ForRangeStmt:
		return true
	default:
		return false
	}
}

func isBreakLabelTarget(statement ast.Statement) bool {
	if isContinueLabelTarget(statement) {
		return true
	}
	switch statement.(type) {
	case *ast.SelectStmt, *ast.ValueSwitchStmt, *ast.TypeSwitchStmt:
		return true
	default:
		return false
	}
}

func (c *Checker) checkFunction(decl *ast.FunctionDecl) {
	c.validateLabels(decl.Body)
	previousMemberFlow := c.memberFlow
	c.memberFlow = map[memberFlowKey]memberFlowState{}
	defer func() { c.memberFlow = previousMemberFlow }()
	c.pushTypeParameterScope(c.functionTypeParameters[decl])
	defer c.popTypeParameterScope()
	c.pushScope()
	previousResult := c.result
	previousLoopDepth := c.loopDepth
	previousBreakableDepth := c.breakableDepth
	previousExceptionDepth := c.exceptionDepth
	previousCatchTargets := c.catchTargets
	c.loopDepth = 0
	c.breakableDepth = 0
	c.exceptionDepth = 0
	c.catchTargets = nil
	for _, param := range decl.Parameters {
		t := c.resolveType(param.Type)
		c.rejectResultValueType(t, param.Type.Span, "parameters")
		c.declareLocal(param.Name, t, false, nil, param.Span)
	}
	c.result = c.resolveType(decl.ReturnType)
	c.checkBlock(decl.Body, false)
	if c.result.Kind != Void && !definitelyReturns(decl.Body) {
		c.report(decl.Span, fmt.Sprintf("function %q may complete without returning %s", decl.Name, c.result.String()))
	}
	c.popScope()
	c.result = previousResult
	c.loopDepth = previousLoopDepth
	c.breakableDepth = previousBreakableDepth
	c.exceptionDepth = previousExceptionDepth
	c.catchTargets = previousCatchTargets
}

func (c *Checker) checkBlock(block *ast.BlockStmt, nested bool) {
	if nested {
		c.pushScope()
		defer c.popScope()
	}
	terminated := false
	var reachableFlow *nullableFlowSnapshot
	for _, stmt := range block.Statements {
		if _, labeled := stmt.(*ast.LabeledStmt); labeled && terminated {
			if reachableFlow != nil {
				c.suppressFlowEffects--
				c.restoreNullableFlow(*reachableFlow)
				reachableFlow = nil
			}
			terminated = false
		}
		if !terminated {
			c.checkStatement(stmt)
			terminated = statementDefinitelyStopsBlock(stmt)
			continue
		}
		if reachableFlow == nil {
			snapshot := c.snapshotNullableFlow()
			reachableFlow = &snapshot
			c.suppressFlowEffects++
		}
		c.checkStatement(stmt)
	}
	if reachableFlow != nil {
		c.suppressFlowEffects--
		c.restoreNullableFlow(*reachableFlow)
	}
}

type nullableNarrowing struct {
	name            string
	scopeIndex      int
	symbol          valueSymbol
	member          memberFlowKey
	memberType      Type
	isMember        bool
	nonNullType     Type
	nonNullWhenTrue bool
}

func (c *Checker) nullableConditionNarrowing(condition ast.Expression) (nullableNarrowing, bool) {
	binary, ok := condition.(*ast.BinaryExpr)
	if !ok || (binary.Operator != "==" && binary.Operator != "===" && binary.Operator != "!=" && binary.Operator != "!==") {
		return nullableNarrowing{}, false
	}
	operand, literal := nullableComparisonOperands(binary.Left, binary.Right)
	if operand == nil || literal == nil || literal.Kind != ast.NullLiteral {
		return nullableNarrowing{}, false
	}
	if member, ok := operand.(*ast.MemberExpr); ok {
		key, stable := c.stableMemberFlowKey(member)
		declared, typed := c.memberTypes[key]
		if !stable || !typed || declared.Kind != Nullable || declared.Element == nil {
			return nullableNarrowing{}, false
		}
		return nullableNarrowing{
			member: key, memberType: declared, isMember: true, nonNullType: *declared.Element,
			nonNullWhenTrue: binary.Operator == "!=" || binary.Operator == "!==",
		}, true
	}
	identifier, ok := operand.(*ast.IdentifierExpr)
	if !ok {
		return nullableNarrowing{}, false
	}
	for index := len(c.scopes) - 1; index >= 0; index-- {
		symbol, exists := c.scopes[index][identifier.Name]
		if !exists {
			continue
		}
		if (!symbol.constant && symbol.flowEscaped) || symbol.typeInfo.Kind != Nullable || symbol.typeInfo.Element == nil {
			return nullableNarrowing{}, false
		}
		return nullableNarrowing{
			name: identifier.Name, scopeIndex: index, symbol: symbol, nonNullType: *symbol.typeInfo.Element,
			nonNullWhenTrue: binary.Operator == "!=" || binary.Operator == "!==",
		}, true
	}
	return nullableNarrowing{}, false
}

func nullableComparisonOperands(left, right ast.Expression) (ast.Expression, *ast.LiteralExpr) {
	if _, ok := left.(*ast.IdentifierExpr); ok {
		if literal, ok := right.(*ast.LiteralExpr); ok {
			return left, literal
		}
	}
	if _, ok := left.(*ast.MemberExpr); ok {
		if literal, ok := right.(*ast.LiteralExpr); ok {
			return left, literal
		}
	}
	if _, ok := right.(*ast.IdentifierExpr); ok {
		if literal, ok := left.(*ast.LiteralExpr); ok {
			return right, literal
		}
	}
	if _, ok := right.(*ast.MemberExpr); ok {
		if literal, ok := left.(*ast.LiteralExpr); ok {
			return right, literal
		}
	}
	return nil, nil
}

func (c *Checker) applyNarrowing(narrowing nullableNarrowing) {
	if narrowing.isMember {
		c.memberFlow[narrowing.member] = memberFlowState{
			declaredType: narrowing.memberType, nonNullType: narrowing.nonNullType, nonNull: true,
		}
		return
	}
	if narrowing.scopeIndex < 0 || narrowing.scopeIndex >= len(c.scopes) {
		return
	}
	symbol, exists := c.scopes[narrowing.scopeIndex][narrowing.name]
	if !exists || symbol.declarationSpan != narrowing.symbol.declarationSpan {
		return
	}
	symbol.typeInfo = narrowing.nonNullType
	symbol.flowInvalidated = source.Span{}
	symbol.flowInvalidationCause = ""
	c.scopes[narrowing.scopeIndex][narrowing.name] = symbol
}

func (c *Checker) checkStatementBranch(statement ast.Statement) {
	if block, ok := statement.(*ast.BlockStmt); ok {
		c.checkBlock(block, true)
	} else {
		c.checkStatement(statement)
	}
}

func cloneValueScopes(scopes []map[string]valueSymbol) []map[string]valueSymbol {
	cloned := make([]map[string]valueSymbol, len(scopes))
	for index, scope := range scopes {
		cloned[index] = make(map[string]valueSymbol, len(scope))
		for name, symbol := range scope {
			cloned[index][name] = symbol
		}
	}
	return cloned
}

func cloneMemberFlow(flow map[memberFlowKey]memberFlowState) map[memberFlowKey]memberFlowState {
	cloned := make(map[memberFlowKey]memberFlowState, len(flow))
	for key, state := range flow {
		cloned[key] = state
	}
	return cloned
}

func (c *Checker) snapshotNullableFlow() nullableFlowSnapshot {
	return nullableFlowSnapshot{scopes: cloneValueScopes(c.scopes), members: cloneMemberFlow(c.memberFlow)}
}

func (c *Checker) restoreNullableFlow(snapshot nullableFlowSnapshot) {
	c.scopes = cloneValueScopes(snapshot.scopes)
	c.memberFlow = cloneMemberFlow(snapshot.members)
}

func (c *Checker) mergeNullableFlow(entry nullableFlowSnapshot, continuing ...nullableFlowSnapshot) nullableFlowSnapshot {
	branches := make([][]map[string]valueSymbol, len(continuing))
	for index, branch := range continuing {
		branches[index] = branch.scopes
	}
	merged := nullableFlowSnapshot{
		scopes:  c.mergeValueScopes(entry.scopes, branches...),
		members: cloneMemberFlow(entry.members),
	}
	if len(continuing) == 0 {
		return merged
	}
	keys := map[memberFlowKey]bool{}
	for key := range entry.members {
		keys[key] = true
	}
	for _, branch := range continuing {
		for key := range branch.members {
			keys[key] = true
		}
	}
	for key := range keys {
		var common memberFlowState
		nonNullOnEveryPath := true
		for index, branch := range continuing {
			state, exists := branch.members[key]
			if !exists || !state.nonNull {
				nonNullOnEveryPath = false
				continue
			}
			if index == 0 || !common.nonNull {
				common = state
			}
		}
		if nonNullOnEveryPath {
			common.nonNull = true
			common.invalidated = source.Span{}
			common.invalidationCause = ""
			merged.members[key] = common
			continue
		}
		state := entry.members[key]
		state.nonNull = false
		for _, branch := range continuing {
			candidate, exists := branch.members[key]
			if exists && candidate.invalidated.Start.Line != 0 {
				state = candidate
				state.nonNull = false
				break
			}
		}
		if state.declaredType.Kind != Invalid || state.invalidated.Start.Line != 0 {
			merged.members[key] = state
		} else {
			delete(merged.members, key)
		}
	}
	return merged
}

func (c *Checker) mergeValueScopes(entry []map[string]valueSymbol, continuing ...[]map[string]valueSymbol) []map[string]valueSymbol {
	merged := cloneValueScopes(entry)
	if len(continuing) == 0 {
		// The joined point is unreachable. Every terminating edge has already
		// reported pending tasks at its return, so no task remains live here.
		for _, scope := range merged {
			for name, symbol := range scope {
				if symbol.taskState != taskNotTracked {
					symbol.taskState = taskConsumed
					scope[name] = symbol
				}
			}
		}
		return merged
	}
	for scopeIndex, scope := range merged {
		for name, symbol := range scope {
			if symbol.taskState != taskNotTracked {
				state := uint8(taskNotTracked)
				for _, branch := range continuing {
					candidateState := uint8(taskPending)
					if scopeIndex < len(branch) {
						if candidate, exists := branch[scopeIndex][name]; exists && candidate.declarationSpan == symbol.declarationSpan {
							candidateState = candidate.taskState
						}
					}
					if state == taskNotTracked {
						state = candidateState
					} else if state != candidateState {
						state = taskMaybeConsumed
					}
				}
				symbol.taskState = state
			}
			declared := symbol.declaredType
			escapedOnAnyPath := symbol.flowEscaped
			for _, branch := range continuing {
				if scopeIndex < len(branch) {
					candidate, exists := branch[scopeIndex][name]
					if exists && candidate.declarationSpan == symbol.declarationSpan && candidate.flowEscaped {
						escapedOnAnyPath = true
					}
				}
			}
			symbol.flowEscaped = escapedOnAnyPath
			if declared.Kind != Nullable || declared.Element == nil {
				symbol.typeInfo = declared
				scope[name] = symbol
				continue
			}
			nonNullOnEveryPath := !escapedOnAnyPath
			for _, branch := range continuing {
				if scopeIndex >= len(branch) {
					nonNullOnEveryPath = false
					break
				}
				branchSymbol, exists := branch[scopeIndex][name]
				if !exists || branchSymbol.declarationSpan != symbol.declarationSpan || branchSymbol.typeInfo.Kind == Nullable || branchSymbol.typeInfo.Kind == Null || !c.isAssignable(*declared.Element, branchSymbol.typeInfo) {
					nonNullOnEveryPath = false
					break
				}
			}
			if nonNullOnEveryPath {
				symbol.typeInfo = *declared.Element
				symbol.flowInvalidated = source.Span{}
				symbol.flowInvalidationCause = ""
			} else {
				symbol.typeInfo = declared
				for _, branch := range continuing {
					if scopeIndex < len(branch) {
						candidate, exists := branch[scopeIndex][name]
						if exists && candidate.declarationSpan == symbol.declarationSpan && candidate.flowInvalidated.Start.Line != 0 {
							symbol.flowInvalidated = candidate.flowInvalidated
							symbol.flowInvalidationCause = candidate.flowInvalidationCause
							break
						}
					}
				}
			}
			scope[name] = symbol
		}
	}
	return merged
}

func (c *Checker) checkLoopFixedPoint(entry nullableFlowSnapshot, checkIteration func() (nullableFlowSnapshot, bool)) {
	header := nullableFlowSnapshot{scopes: cloneValueScopes(entry.scopes), members: cloneMemberFlow(entry.members)}
	limit := nullableFlowSymbolCount(entry.scopes)*2 + len(entry.members)*2 + 4
	for iteration := 0; iteration < limit; iteration++ {
		diagnosticStart := len(c.diagnostics)
		c.restoreNullableFlow(header)
		c.loopFlowContexts = append(c.loopFlowContexts, loopFlowContext{})
		backedge, fallsThrough := checkIteration()
		flow := c.loopFlowContexts[len(c.loopFlowContexts)-1]
		c.loopFlowContexts = c.loopFlowContexts[:len(c.loopFlowContexts)-1]
		backedges := append([]nullableFlowSnapshot(nil), flow.continues...)
		if fallsThrough {
			backedges = append(backedges, backedge)
		}
		next := c.mergeNullableFlow(entry, append([]nullableFlowSnapshot{entry}, backedges...)...)
		if sameNullableFlowSnapshot(header, next) {
			exits := append([]nullableFlowSnapshot{next}, flow.breaks...)
			c.restoreNullableFlow(c.mergeNullableFlow(entry, exits...))
			return
		}
		c.diagnostics = c.diagnostics[:diagnosticStart]
		header = next
	}

	// The nullable lattice is finite and the transfer is monotone, so this is a
	// defensive fallback only. Check once at the most conservative state reached
	// instead of accepting a body based on an earlier iteration.
	c.restoreNullableFlow(header)
	c.loopFlowContexts = append(c.loopFlowContexts, loopFlowContext{})
	backedge, fallsThrough := checkIteration()
	flow := c.loopFlowContexts[len(c.loopFlowContexts)-1]
	c.loopFlowContexts = c.loopFlowContexts[:len(c.loopFlowContexts)-1]
	backedges := append([]nullableFlowSnapshot(nil), flow.continues...)
	if fallsThrough {
		backedges = append(backedges, backedge)
	}
	next := c.mergeNullableFlow(entry, append([]nullableFlowSnapshot{entry}, backedges...)...)
	exits := append([]nullableFlowSnapshot{next}, flow.breaks...)
	c.restoreNullableFlow(c.mergeNullableFlow(entry, exits...))
}

func nullableFlowSymbolCount(scopes []map[string]valueSymbol) int {
	count := 0
	for _, scope := range scopes {
		for _, symbol := range scope {
			if symbol.declaredType.Kind == Nullable || symbol.taskState != taskNotTracked {
				count++
			}
		}
	}
	return count
}

func sameNullableFlow(left, right []map[string]valueSymbol) bool {
	if len(left) != len(right) {
		return false
	}
	for scopeIndex, leftScope := range left {
		rightScope := right[scopeIndex]
		for name, leftSymbol := range leftScope {
			rightSymbol, exists := rightScope[name]
			if leftSymbol.taskState != taskNotTracked {
				if !exists || rightSymbol.declarationSpan != leftSymbol.declarationSpan || rightSymbol.taskState != leftSymbol.taskState {
					return false
				}
			}
			if leftSymbol.declaredType.Kind != Nullable {
				continue
			}
			if !exists || rightSymbol.declarationSpan != leftSymbol.declarationSpan {
				return false
			}
			if leftSymbol.typeInfo.Kind != rightSymbol.typeInfo.Kind || leftSymbol.flowEscaped != rightSymbol.flowEscaped {
				return false
			}
		}
	}
	return true
}

func sameNullableFlowSnapshot(left, right nullableFlowSnapshot) bool {
	if !sameNullableFlow(left.scopes, right.scopes) || len(left.members) != len(right.members) {
		return false
	}
	for key, leftState := range left.members {
		rightState, exists := right.members[key]
		if !exists || leftState.nonNull != rightState.nonNull || leftState.invalidated != rightState.invalidated || leftState.invalidationCause != rightState.invalidationCause {
			return false
		}
	}
	return true
}

func statementDefinitelyReturns(statement ast.Statement) bool {
	if statement == nil {
		return false
	}
	if block, ok := statement.(*ast.BlockStmt); ok {
		return definitelyReturns(block)
	}
	return definitelyReturns(&ast.BlockStmt{Statements: []ast.Statement{statement}})
}

func statementDefinitelyStopsBlock(statement ast.Statement) bool {
	switch statement := statement.(type) {
	case *ast.ReturnStmt, *ast.ThrowStmt, *ast.BranchStmt:
		return true
	case *ast.LabeledStmt:
		return statementDefinitelyStopsBlock(statement.Statement)
	case *ast.BlockStmt:
		for _, nested := range statement.Statements {
			if statementDefinitelyStopsBlock(nested) {
				return true
			}
		}
	case *ast.IfStmt:
		if statement.Else == nil || !statementDefinitelyStopsBlock(statement.Then) {
			return false
		}
		return statementDefinitelyStopsBlock(statement.Else)
	case *ast.TryStmt:
		if statement.FinallyBody != nil && statementDefinitelyStopsBlock(statement.FinallyBody) {
			return true
		}
		if !statementDefinitelyStopsBlock(statement.Body) {
			return false
		}
		for _, clause := range statement.Catches {
			if !statementDefinitelyStopsBlock(clause.Body) {
				return false
			}
		}
		return true
	case *ast.SelectStmt, *ast.ValueSwitchStmt, *ast.TypeSwitchStmt:
		// A break stops its case, not the surrounding block. Reuse the existing
		// all-path return proof for compound breakable statements.
		return statementDefinitelyReturns(statement)
	}
	return false
}

func (c *Checker) checkStatement(stmt ast.Statement) {
	switch stmt := stmt.(type) {
	case *ast.LabeledStmt:
		c.invalidateControlTransferFlow(stmt.Span)
		c.checkStatement(stmt.Statement)
	case *ast.VariableDecl:
		declared := Type{Kind: Invalid, Name: "<inferred>"}
		if stmt.Type.IsSpecified() {
			declared = c.resolveType(stmt.Type)
		}
		var value Type
		if propagated, ok := stmt.Value.(*ast.PropagateExpr); ok {
			value = c.checkPropagateExpression(propagated)
		} else {
			value = c.checkExpressionExpectedSlot(&stmt.Value, declared)
		}
		if !stmt.Type.IsSpecified() {
			declared = c.inferredVariableType(value, stmt.Value.GetSpan())
		}
		if declared.Kind == Void {
			c.report(stmt.Type.Span, "variables cannot have type void")
		}
		c.rejectResultValueType(declared, stmt.Type.Span, "variables")
		c.requireAssignable(declared, value, stmt.Value.GetSpan())
		stmt.ResolvedType = typeRefFromType(declared, stmt.Span)
		c.declareLocal(stmt.Name, declared, stmt.Constant, stmt, stmt.Span)
		c.updateIdentifierFlow(stmt.Name, stmt.NameSpan, value)
	case *ast.MultiVariableDecl:
		c.checkMultiVariableDeclaration(stmt)
	case *ast.ReturnStmt:
		if c.exceptionDepth != 0 {
			stmt.CrossesTry = true
		}
		if c.inConstructor {
			c.report(stmt.Span, "constructors cannot return early; use conditional initialization and let the constructor complete")
		}
		if c.result.Kind == Result {
			c.checkResultReturn(stmt)
			c.reportPendingTasksBeforeExit()
			return
		}
		if stmt.Value == nil {
			if c.result.Kind != Void {
				c.report(stmt.Span, fmt.Sprintf("expected return value of type %s", c.result.Name))
			}
			c.reportPendingTasksBeforeExit()
			return
		}
		value := c.checkExpressionExpectedSlot(&stmt.Value, c.result)
		if c.result.Kind == Void {
			c.report(stmt.Value.GetSpan(), "void function cannot return a value")
		} else {
			c.requireAssignable(c.result, value, stmt.Value.GetSpan())
		}
		c.reportPendingTasksBeforeExit()
	case *ast.ThrowStmt:
		c.usesExceptions = true
		if stmt.Bare {
			if len(c.catchTargets) == 0 {
				c.report(stmt.Span, "bare throw may only be used inside a catch block")
			} else {
				stmt.RethrowOffset = c.catchTargets[len(c.catchTargets)-1]
			}
			c.reportPendingTasksBeforeExit()
			return
		}
		value := c.singleValue(c.checkExpression(stmt.Value), stmt.Value.GetSpan())
		c.requireAssignable(builtins["error"], value, stmt.Value.GetSpan())
		c.reportPendingTasksBeforeExit()
	case *ast.TryStmt:
		c.checkTryStatement(stmt)
	case *ast.IfStmt:
		condition := c.checkExpression(stmt.Condition)
		if condition.Kind != Invalid && condition.Kind != Boolean {
			c.report(stmt.Condition.GetSpan(), fmt.Sprintf("if condition must be boolean, got %s", condition.Name))
		}
		narrowing, hasNarrowing := c.nullableConditionNarrowing(stmt.Condition)
		entryFlow := c.snapshotNullableFlow()

		c.restoreNullableFlow(entryFlow)
		if hasNarrowing && narrowing.nonNullWhenTrue {
			c.applyNarrowing(narrowing)
		}
		c.checkBlock(stmt.Then, true)
		thenFlow := c.snapshotNullableFlow()

		c.restoreNullableFlow(entryFlow)
		if hasNarrowing && !narrowing.nonNullWhenTrue {
			c.applyNarrowing(narrowing)
		}
		if stmt.Else != nil {
			c.checkStatementBranch(stmt.Else)
		}
		elseFlow := c.snapshotNullableFlow()

		continuing := make([]nullableFlowSnapshot, 0, 2)
		if !statementDefinitelyStopsBlock(stmt.Then) {
			continuing = append(continuing, thenFlow)
		}
		if stmt.Else == nil || !statementDefinitelyStopsBlock(stmt.Else) {
			continuing = append(continuing, elseFlow)
		}
		c.restoreNullableFlow(c.mergeNullableFlow(entryFlow, continuing...))
	case *ast.BlockStmt:
		c.checkBlock(stmt, true)
	case *ast.ExpressionStmt:
		if propagated, ok := stmt.Value.(*ast.PropagateExpr); ok {
			value := c.checkPropagateExpression(propagated)
			if value.Kind != Invalid && value.Kind != Void {
				c.report(stmt.Span, fmt.Sprintf("propagated result of type %s must be bound to a variable", value.String()))
			}
			return
		}
		value := c.checkExpression(stmt.Value)
		if value.Kind == Result {
			c.report(stmt.Span, "Result values must be consumed with ?, explicitly split, or returned")
		}
		if !isAllowedExpressionStatement(stmt.Value) {
			c.report(stmt.Span, "only function calls and Go channel receives may be used as expression statements")
		}
	case *ast.AssignmentStmt:
		target := c.checkAssignmentTarget(stmt.Target)
		if target.Kind == Task {
			c.report(stmt.Target.GetSpan(), "Task bindings cannot be reassigned")
		}
		if stmt.Operator == "" || stmt.Operator == "=" {
			value := c.checkExpressionExpectedSlot(&stmt.Value, target)
			c.requireAssignable(target, value, stmt.Value.GetSpan())
			c.updateAssignmentFlow(stmt.Target, value)
		} else {
			c.markAssignmentTargetRead(stmt.Target)
			value := c.singleValue(c.checkExpression(stmt.Value), stmt.Value.GetSpan())
			operator := strings.TrimSuffix(stmt.Operator, "=")
			result := c.checkBinaryOperands(&ast.BinaryExpr{Left: stmt.Target, Operator: operator, Right: stmt.Value, Span: stmt.Span}, target, value)
			c.requireAssignable(target, result, stmt.Value.GetSpan())
			c.invalidateMemberWriteTarget(stmt.Target, stmt.Span)
		}
	case *ast.IncDecStmt:
		target := c.checkAssignmentTarget(stmt.Target)
		c.markAssignmentTargetRead(stmt.Target)
		if target.Kind != Invalid && !target.IsNumeric() {
			c.report(stmt.Span, fmt.Sprintf("operator %s requires a numeric assignable operand", stmt.Operator))
		}
		c.invalidateMemberWriteTarget(stmt.Target, stmt.Span)
	case *ast.MultiAssignmentStmt:
		c.checkMultiAssignment(stmt)
	case *ast.WhileStmt:
		entryFlow := c.snapshotNullableFlow()
		c.checkLoopFixedPoint(entryFlow, func() (nullableFlowSnapshot, bool) {
			c.checkLoopCondition(stmt.Condition)
			stmt.GuaranteedEntry = c.expressionAlwaysTrue(stmt.Condition)
			narrowing, hasNarrowing := c.nullableConditionNarrowing(stmt.Condition)
			if hasNarrowing && narrowing.nonNullWhenTrue {
				c.applyNarrowing(narrowing)
			}
			c.loopDepth++
			c.checkBlock(stmt.Body, true)
			c.loopDepth--
			return c.snapshotNullableFlow(), !statementDefinitelyStopsBlock(stmt.Body)
		})
	case *ast.ForStmt:
		c.pushScope()
		if stmt.Initializer != nil {
			if variable, ok := stmt.Initializer.(*ast.VariableDecl); ok {
				if _, propagated := variable.Value.(*ast.PropagateExpr); propagated {
					c.report(variable.Value.GetSpan(), "result propagation cannot be used in a for-loop initializer")
				}
			}
			c.checkStatement(stmt.Initializer)
		}
		entryFlow := c.snapshotNullableFlow()
		c.checkLoopFixedPoint(entryFlow, func() (nullableFlowSnapshot, bool) {
			if stmt.Condition != nil {
				c.checkLoopCondition(stmt.Condition)
				stmt.GuaranteedEntry = c.expressionAlwaysTrue(stmt.Condition)
			}
			narrowing, hasNarrowing := c.nullableConditionNarrowing(stmt.Condition)
			if hasNarrowing && narrowing.nonNullWhenTrue {
				c.applyNarrowing(narrowing)
			}
			c.loopDepth++
			c.checkBlock(stmt.Body, true)
			if stmt.Post != nil {
				c.checkStatement(stmt.Post)
			}
			c.loopDepth--
			return c.snapshotNullableFlow(), !statementDefinitelyStopsBlock(stmt.Body)
		})
		c.popScope()
	case *ast.ForRangeStmt:
		types := c.prepareForRange(stmt)
		entryFlow := c.snapshotNullableFlow()
		c.checkLoopFixedPoint(entryFlow, func() (nullableFlowSnapshot, bool) {
			c.checkForRangeBody(stmt, types)
			return c.snapshotNullableFlow(), !statementDefinitelyStopsBlock(stmt.Body)
		})
	case *ast.SelectStmt:
		c.checkSelect(stmt)
	case *ast.ValueSwitchStmt:
		c.checkValueSwitch(stmt)
	case *ast.TypeSwitchStmt:
		c.checkTypeSwitch(stmt)
	case *ast.BranchStmt:
		if stmt.Kind == ast.FallthroughBranch {
			if !c.validFallthrough[stmt] {
				c.report(stmt.Span, "fallthrough may only be used as the final statement of a non-final value switch case")
			}
			return
		}
		if stmt.Kind == ast.BreakBranch && c.loopDepth == 0 && c.breakableDepth == 0 {
			c.report(stmt.Span, "break may only be used inside a loop, switch, or select")
		}
		if stmt.Kind == ast.ContinueBranch && c.loopDepth == 0 {
			c.report(stmt.Span, "continue may only be used inside a loop")
		}
		if stmt.Kind == ast.GotoBranch {
			c.invalidateControlTransferFlow(stmt.Span)
			return
		}
		if c.suppressFlowEffects == 0 && len(c.loopFlowContexts) != 0 {
			context := &c.loopFlowContexts[len(c.loopFlowContexts)-1]
			switch {
			case stmt.Kind == ast.ContinueBranch && c.loopDepth != 0:
				context.continues = append(context.continues, c.snapshotNullableFlow())
			case stmt.Kind == ast.BreakBranch && c.loopDepth != 0 && c.breakableDepth == 0:
				context.breaks = append(context.breaks, c.snapshotNullableFlow())
			}
		}
		if c.suppressFlowEffects == 0 && stmt.Kind == ast.BreakBranch && c.breakableDepth != 0 && len(c.breakFlowContexts) != 0 {
			context := &c.breakFlowContexts[len(c.breakFlowContexts)-1]
			context.breaks = append(context.breaks, c.snapshotNullableFlow())
		}
	case *ast.CallControlStmt:
		call, ok := stmt.Value.(*ast.CallExpr)
		if !ok {
			c.checkExpression(stmt.Value)
			return
		}
		value := c.checkExpression(call)
		if value.Kind == Result {
			keyword := "defer"
			if stmt.Kind == ast.GoCall {
				keyword = "go"
			}
			c.report(call.Span, fmt.Sprintf("%s cannot discard a Result; consume it with ? in a Result function", keyword))
		}
		if call.Conversion {
			keyword := "defer"
			if stmt.Kind == ast.GoCall {
				keyword = "go"
			}
			c.report(call.Span, keyword+" requires a function or method call; type conversions are not calls")
		}
	case *ast.DetachStmt:
		c.usesTasks = true
		c.taskOperandDepth++
		task := c.singleValue(c.checkExpression(stmt.Value), stmt.Value.GetSpan())
		c.taskOperandDepth--
		if task.Kind != Task || task.Element == nil {
			if task.Kind != Invalid {
				c.report(stmt.Value.GetSpan(), fmt.Sprintf("detach requires Task<T>, got %s", task.String()))
			}
			return
		}
		c.consumeTask(stmt.Value)
		result := *task.Element
		stmt.ResultTask = result.Kind == Result
		stmt.Void = result.Kind == Void || stmt.ResultTask && result.Element != nil && result.Element.Kind == Void
		value := result
		if stmt.ResultTask && result.Element != nil {
			value = *result.Element
		}
		c.prepareGoTypeForEmission(&value, stmt.Span)
		stmt.ValueType = typeRefFromType(value, stmt.Span)
	case *ast.ChannelSendStmt:
		channelType := c.singleValue(c.checkExpression(stmt.Channel), stmt.Channel.GetSpan())
		if channelType.Kind == Nullable {
			c.report(stmt.Channel.GetSpan(), fmt.Sprintf("nullable channel %s must be checked against null before sending", channelType.String()))
			if channelType.Element == nil {
				return
			}
			channelType = *channelType.Element
		}
		goType, ok := goTypeOf(channelType)
		if !ok {
			c.report(stmt.Channel.GetSpan(), fmt.Sprintf("channel send requires a Go channel, got %s", channelType.String()))
			c.checkExpression(stmt.Value)
			return
		}
		channel, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Chan)
		if !ok {
			c.report(stmt.Channel.GetSpan(), fmt.Sprintf("channel send requires a Go channel, got %s", channelType.String()))
			c.checkExpression(stmt.Value)
			return
		}
		if channel.Dir() == gotypes.RecvOnly {
			c.report(stmt.Channel.GetSpan(), fmt.Sprintf("cannot send to receive-only channel %s", channelType.String()))
		}
		element, err := ontamaTypeFromGo(channel.Elem())
		if err != nil {
			c.report(stmt.Channel.GetSpan(), fmt.Sprintf("channel element type is not supported: %v", err))
			c.checkExpression(stmt.Value)
			return
		}
		if channelType.Element != nil {
			element = *channelType.Element
		}
		value := c.checkExpressionExpectedSlot(&stmt.Value, element)
		c.requireAssignable(element, value, stmt.Value.GetSpan())
	}
}

func (c *Checker) checkTryStatement(stmt *ast.TryStmt) {
	c.usesExceptions = true
	stmt.HandlesReturn = c.exceptionDepth == 0
	if stmt.HandlesReturn {
		stmt.ReturnType = typeRefFromType(c.result, stmt.Span)
	}
	entry := c.snapshotNullableFlow()

	c.restoreNullableFlow(entry)
	c.checkExceptionBlock(stmt.Body)
	tryFlow := c.snapshotNullableFlow()
	continuing := []nullableFlowSnapshot{}
	if !statementDefinitelyStopsBlock(stmt.Body) {
		continuing = append(continuing, tryFlow)
	}

	seenCatchTypes := []Type{}
	for _, clause := range stmt.Catches {
		catchType := c.resolveType(clause.Type)
		if catchType.Kind != Invalid && !c.isAssignable(builtins["error"], catchType) {
			c.report(clause.Type.Span, fmt.Sprintf("catch binding type must implement error, got %s", catchType.String()))
		}
		if catchType.Kind != Invalid && c.isAssignable(builtins["error"], catchType) {
			for _, earlier := range seenCatchTypes {
				if c.catchTypeCovers(earlier, catchType) {
					c.report(clause.Type.Span, fmt.Sprintf("catch for %s is unreachable because an earlier catch for %s already handles it", catchType.String(), earlier.String()))
					break
				}
			}
			seenCatchTypes = append(seenCatchTypes, catchType)
		}
		if catchType.Kind == Class {
			clause.MatchingClasses = append(clause.MatchingClasses, catchType.Name)
			for name := range c.classes {
				if name != catchType.Name && c.classExtends(name, catchType.Name) {
					clause.MatchingClasses = append(clause.MatchingClasses, name)
				}
			}
			sort.Strings(clause.MatchingClasses[1:])
		}
		c.restoreNullableFlow(c.mergeNullableFlow(entry, entry, tryFlow))
		c.pushScope()
		if clause.Name != "_" {
			c.declareCatchLocal(clause, catchType)
		}
		c.catchTargets = append(c.catchTargets, stmt.Span.Start.Offset)
		c.checkExceptionBlock(clause.Body)
		c.catchTargets = c.catchTargets[:len(c.catchTargets)-1]
		catchFlow := c.snapshotNullableFlow()
		c.popScope()
		if !statementDefinitelyStopsBlock(clause.Body) {
			continuing = append(continuing, catchFlow)
		}
	}

	c.restoreNullableFlow(c.mergeNullableFlow(entry, continuing...))
	if stmt.FinallyBody != nil {
		// finally also runs while an exception is propagating from any point in
		// the try/catch path, so it must not inherit facts established only by a
		// normally completing path.
		current := c.snapshotNullableFlow()
		c.restoreNullableFlow(c.mergeNullableFlow(entry, entry, current))
		c.checkExceptionBlock(stmt.FinallyBody)
	}
	stmt.Terminal = statementDefinitelyStopsBlock(stmt)
}

func (c *Checker) catchTypeCovers(earlier, current Type) bool {
	if exactType(earlier, builtins["error"]) || earlier.Kind == Class && earlier.Name == "Exception" {
		return true
	}
	if earlier.Kind == Class && current.Kind == Class {
		return earlier.Name == current.Name || c.classExtends(current.Name, earlier.Name)
	}
	earlierGo, earlierIsGo := goTypeOf(earlier)
	currentGo, currentIsGo := goTypeOf(current)
	return earlierIsGo && currentIsGo && gotypes.AssignableTo(currentGo, earlierGo)
}

func (c *Checker) checkExceptionBlock(block *ast.BlockStmt) {
	previousLoopDepth := c.loopDepth
	previousBreakableDepth := c.breakableDepth
	c.loopDepth = 0
	c.breakableDepth = 0
	c.exceptionDepth++
	c.checkBlock(block, true)
	c.exceptionDepth--
	c.loopDepth = previousLoopDepth
	c.breakableDepth = previousBreakableDepth
}

func (c *Checker) prepareForRange(stmt *ast.ForRangeStmt) []Type {
	sourceType := c.singleValue(c.checkExpression(stmt.Source), stmt.Source.GetSpan())
	stmt.GuaranteedNonEmpty = rangeTypeGuaranteedNonEmpty(sourceType) || c.rangeExpressionGuaranteedNonEmpty(stmt.Source)
	key, value, kind := c.rangeBindingTypes(sourceType, stmt.Source.GetSpan())
	stmt.Kind = kind
	if kind == ast.ChannelRange && len(stmt.Bindings) != 1 {
		c.report(stmt.Span, fmt.Sprintf("channel range requires exactly one binding, got %d", len(stmt.Bindings)))
	}
	if kind == ast.CollectionRange && (len(stmt.Bindings) < 1 || len(stmt.Bindings) > 2) {
		c.report(stmt.Span, fmt.Sprintf("collection range requires one or two bindings, got %d", len(stmt.Bindings)))
	}
	types := []Type{value}
	if len(stmt.Bindings) == 2 {
		types = []Type{key, value}
	}
	return types
}

func rangeTypeGuaranteedNonEmpty(t Type) bool {
	if t.Kind == FixedArray {
		return t.Length > 0
	}
	return t.Kind == GoPointer && t.Element != nil && t.Element.Kind == FixedArray && t.Element.Length > 0
}

func (c *Checker) checkForRangeBody(stmt *ast.ForRangeStmt, types []Type) {
	c.pushScope()
	for index := range stmt.Bindings {
		binding := &stmt.Bindings[index]
		actual := Type{Kind: Invalid, Name: "<invalid>"}
		if index < len(types) {
			actual = types[index]
		}
		declared := actual
		if binding.Type.IsSpecified() {
			declared = c.resolveType(binding.Type)
			c.requireAssignable(declared, actual, binding.NameSpan)
		}
		if binding.Name != "_" {
			binding.ResolvedType = typeRefFromType(declared, binding.NameSpan)
			c.declareRangeLocal(binding, declared, stmt.Constant)
		}
	}
	c.loopDepth++
	c.checkBlock(stmt.Body, true)
	c.loopDepth--
	c.popScope()
}

func (c *Checker) rangeBindingTypes(sourceType Type, span source.Span) (Type, Type, ast.ForRangeKind) {
	invalid := Type{Kind: Invalid, Name: "<invalid>"}
	if sourceType.Kind == Nullable && sourceType.Element != nil {
		sourceType = *sourceType.Element
	}
	goType, ok := goTypeOf(sourceType)
	if !ok {
		if sourceType.Kind != Invalid {
			c.report(span, fmt.Sprintf("range requires an array, slice, map, string, or receive-capable Go channel, got %s", sourceType.String()))
		}
		return invalid, invalid, ast.UnknownRange
	}
	underlying := gotypes.Unalias(goType).Underlying()
	if pointer, pointerOK := underlying.(*gotypes.Pointer); pointerOK {
		if _, arrayOK := gotypes.Unalias(pointer.Elem()).Underlying().(*gotypes.Array); arrayOK {
			underlying = gotypes.Unalias(pointer.Elem()).Underlying()
		}
	}
	switch ranged := underlying.(type) {
	case *gotypes.Chan:
		if ranged.Dir() == gotypes.SendOnly {
			c.report(span, fmt.Sprintf("cannot range over send-only channel %s", sourceType.String()))
			return invalid, invalid, ast.UnknownRange
		}
		element := c.collectionElementType(ranged.Elem(), sourceType, span)
		if sourceType.Element != nil {
			element = *sourceType.Element
		}
		return invalid, element, ast.ChannelRange
	case *gotypes.Array:
		element := c.collectionElementType(ranged.Elem(), sourceType, span)
		if sourceType.Kind == FixedArray && sourceType.Element != nil {
			element = *sourceType.Element
		}
		return builtins["int"], element, ast.CollectionRange
	case *gotypes.Slice:
		element := c.collectionElementType(ranged.Elem(), sourceType, span)
		if sourceType.Kind == Array && sourceType.Element != nil {
			element = *sourceType.Element
		}
		return builtins["int"], element, ast.CollectionRange
	case *gotypes.Map:
		key := c.collectionElementType(ranged.Key(), sourceType, span)
		value := c.collectionElementType(ranged.Elem(), sourceType, span)
		if sourceType.Key != nil {
			key = *sourceType.Key
		}
		if sourceType.Element != nil {
			value = *sourceType.Element
		}
		return key, value, ast.CollectionRange
	case *gotypes.Basic:
		if ranged.Info()&gotypes.IsString != 0 {
			return builtins["int"], builtins["int32"], ast.CollectionRange
		}
	}
	if sourceType.Kind != Invalid {
		c.report(span, fmt.Sprintf("range requires an array, slice, map, string, or receive-capable Go channel, got %s", sourceType.String()))
	}
	return invalid, invalid, ast.UnknownRange
}

func (c *Checker) checkSelect(stmt *ast.SelectStmt) {
	defaultSeen := false
	entryFlow := c.snapshotNullableFlow()
	continuing := make([]nullableFlowSnapshot, 0, len(stmt.Cases))
	c.breakFlowContexts = append(c.breakFlowContexts, breakFlowContext{})
	for index := range stmt.Cases {
		clause := &stmt.Cases[index]
		c.restoreNullableFlow(entryFlow)
		c.pushScope()
		switch clause.Kind {
		case ast.SelectDefault:
			if defaultSeen {
				c.report(clause.Span, "select may contain at most one default case")
			}
			defaultSeen = true
		case ast.SelectSend:
			c.checkStatement(&ast.ChannelSendStmt{Channel: clause.Channel, Value: clause.Value, Span: clause.Span})
		case ast.SelectReceive:
			c.checkSelectReceive(clause)
		}
		c.breakableDepth++
		c.checkBlock(clause.Body, false)
		c.breakableDepth--
		c.popScope()
		if !definitelyReturns(clause.Body) {
			continuing = append(continuing, c.snapshotNullableFlow())
		}
	}
	flow := c.breakFlowContexts[len(c.breakFlowContexts)-1]
	c.breakFlowContexts = c.breakFlowContexts[:len(c.breakFlowContexts)-1]
	continuing = append(continuing, flow.breaks...)
	c.restoreNullableFlow(c.mergeNullableFlow(entryFlow, continuing...))
}

func (c *Checker) checkSelectReceive(clause *ast.SelectCase) {
	count := len(clause.Bindings)
	if !clause.Declare {
		count = len(clause.Targets)
	}
	checked := count == 2
	receive := &ast.UnaryExpr{Operator: "<-", Operand: clause.Channel, Span: clause.Channel.GetSpan()}
	value := c.checkChannelReceive(receive, checked)
	if count == 0 {
		return
	}
	if count != 1 && count != 2 {
		c.report(clause.Span, fmt.Sprintf("select receive expects zero, one, or two targets, got %d", count))
		return
	}
	var results []Type
	if checked {
		results = c.multipleResults(value, count, clause.Channel.GetSpan())
	} else {
		results = []Type{c.singleValue(value, clause.Channel.GetSpan())}
	}
	if clause.Declare {
		for i := range clause.Bindings {
			binding := &clause.Bindings[i]
			if binding.Name == "_" {
				continue
			}
			result := Type{Kind: Invalid, Name: "<invalid>"}
			if i < len(results) {
				result = defaultLiteralType(results[i])
			}
			binding.ResolvedType = typeRefFromType(result, binding.Span)
			c.declareSelectLocal(binding.Name, result, clause.Constant, clause, i, binding.Span)
		}
		return
	}
	for i, target := range clause.Targets {
		if identifier, ok := target.(*ast.IdentifierExpr); ok && identifier.Name == "_" {
			continue
		}
		targetType := c.checkAssignmentTarget(target)
		if i < len(results) {
			c.requireAssignable(targetType, results[i], target.GetSpan())
			c.updateAssignmentFlow(target, results[i])
		}
	}
}

func (c *Checker) checkTypeSwitch(stmt *ast.TypeSwitchStmt) {
	value := c.singleValue(c.checkExpression(stmt.Value), stmt.Value.GetSpan())
	contract := underlyingGoInterface(value.GoType)
	if value.Kind != Invalid && contract == nil {
		c.report(stmt.Value.GetSpan(), fmt.Sprintf("type switch requires a Go interface value, got %s", value.String()))
	}
	defaultSeen := false
	nilSeen := false
	var caseTypes []gotypes.Type
	entryFlow := c.snapshotNullableFlow()
	continuing := make([]nullableFlowSnapshot, 0, len(stmt.Cases)+1)
	c.breakFlowContexts = append(c.breakFlowContexts, breakFlowContext{})
	for index := range stmt.Cases {
		clause := &stmt.Cases[index]
		c.restoreNullableFlow(entryFlow)
		c.pushScope()
		switch {
		case clause.Default:
			if defaultSeen {
				c.report(clause.Span, "type switch may contain at most one default case")
			}
			defaultSeen = true
		case clause.Nil:
			if nilSeen {
				c.report(clause.Span, "type switch may contain at most one nil case")
			}
			nilSeen = true
		default:
			caseType := c.resolveType(clause.Type)
			goCaseType, ok := goTypeOf(caseType)
			if !ok {
				c.report(clause.Type.Span, fmt.Sprintf("type switch case type %s cannot be represented as a Go type", caseType.String()))
			} else {
				for _, previous := range caseTypes {
					if gotypes.Identical(previous, goCaseType) {
						c.report(clause.Type.Span, fmt.Sprintf("duplicate type switch case %s", caseType.String()))
						break
					}
				}
				caseTypes = append(caseTypes, goCaseType)
				if contract != nil && !gotypes.AssertableTo(contract, goCaseType) {
					c.report(clause.Type.Span, fmt.Sprintf("Go interface %s cannot contain type switch case %s", value.String(), caseType.String()))
				}
			}
			if clause.Name != "_" {
				c.declareTypeSwitchLocal(clause.Name, caseType, clause.Constant, clause, clause.NameSpan)
			}
		}
		c.breakableDepth++
		c.checkBlock(clause.Body, false)
		c.breakableDepth--
		c.popScope()
		if !definitelyReturns(clause.Body) {
			continuing = append(continuing, c.snapshotNullableFlow())
		}
	}
	if !defaultSeen {
		continuing = append(continuing, entryFlow)
	}
	flow := c.breakFlowContexts[len(c.breakFlowContexts)-1]
	c.breakFlowContexts = c.breakFlowContexts[:len(c.breakFlowContexts)-1]
	continuing = append(continuing, flow.breaks...)
	c.restoreNullableFlow(c.mergeNullableFlow(entryFlow, continuing...))
}

func (c *Checker) checkValueSwitch(stmt *ast.ValueSwitchStmt) {
	value := c.singleValue(c.checkExpression(stmt.Value), stmt.Value.GetSpan())
	if value.Kind == Nil || value.Kind == Null {
		c.report(stmt.Value.GetSpan(), "value switch cannot infer a concrete type from nil or null")
	} else if value.Kind != Invalid && !value.IsComparable() {
		c.report(stmt.Value.GetSpan(), fmt.Sprintf("value switch expression type %s is not comparable", value.String()))
	}
	defaultSeen := false
	constantCases := map[string]source.Span{}
	entryFlow := c.snapshotNullableFlow()
	continuing := make([]nullableFlowSnapshot, 0, len(stmt.Cases)+1)
	var fallthroughFlow *nullableFlowSnapshot
	c.breakFlowContexts = append(c.breakFlowContexts, breakFlowContext{})
	for index := range stmt.Cases {
		clause := &stmt.Cases[index]
		clause.FallsThrough = false
		if clause.Body != nil && len(clause.Body.Statements) != 0 {
			if branch, ok := clause.Body.Statements[len(clause.Body.Statements)-1].(*ast.BranchStmt); ok && branch.Kind == ast.FallthroughBranch && index+1 < len(stmt.Cases) {
				clause.FallsThrough = true
				c.validFallthrough[branch] = true
			}
		}
		caseEntry := entryFlow
		if fallthroughFlow != nil {
			caseEntry = c.mergeNullableFlow(entryFlow, entryFlow, *fallthroughFlow)
		}
		c.restoreNullableFlow(caseEntry)
		if clause.Default {
			if defaultSeen {
				c.report(clause.Span, "value switch may contain at most one default case")
			}
			defaultSeen = true
		}
		for _, expression := range clause.Values {
			caseType := c.singleValue(c.checkExpressionExpected(expression, value), expression.GetSpan())
			c.requireAssignable(value, caseType, expression.GetSpan())
			if caseType.Kind != Invalid && caseType.Kind != Nil && caseType.Kind != Null && !caseType.IsComparable() {
				c.report(expression.GetSpan(), fmt.Sprintf("value switch case type %s is not comparable", caseType.String()))
			}
			if integer, known := c.resolvedIntegerConstantValue(expression); known {
				if caseType.Kind != Invalid && value.Kind != UntypedInt && value.IsInteger() && !integerConstantFitsFixedType(integer, value) {
					c.report(expression.GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", integer.String(), value.String()))
				}
				key := "number:" + integer.String()
				if _, duplicate := constantCases[key]; duplicate {
					c.report(expression.GetSpan(), fmt.Sprintf("duplicate value switch case %s", integer.String()))
				} else {
					constantCases[key] = expression.GetSpan()
				}
			} else if key, display, known := c.switchLiteralKey(expression); known {
				if _, duplicate := constantCases[key]; duplicate {
					c.report(expression.GetSpan(), "duplicate value switch case "+display)
				} else {
					constantCases[key] = expression.GetSpan()
				}
			}
		}
		c.breakableDepth++
		c.checkBlock(clause.Body, true)
		c.breakableDepth--
		caseFlow := c.snapshotNullableFlow()
		if clause.FallsThrough && valueSwitchCaseFallthroughReachable(clause) {
			fallthroughFlow = &caseFlow
		} else {
			fallthroughFlow = nil
		}
		if !clause.FallsThrough && !definitelyReturns(clause.Body) {
			continuing = append(continuing, c.snapshotNullableFlow())
		}
	}
	if !defaultSeen {
		continuing = append(continuing, entryFlow)
	}
	flow := c.breakFlowContexts[len(c.breakFlowContexts)-1]
	c.breakFlowContexts = c.breakFlowContexts[:len(c.breakFlowContexts)-1]
	continuing = append(continuing, flow.breaks...)
	c.restoreNullableFlow(c.mergeNullableFlow(entryFlow, continuing...))
}

func (c *Checker) switchLiteralKey(expression ast.Expression) (string, string, bool) {
	literal, ok := expression.(*ast.LiteralExpr)
	if !ok {
		identifier, identifierOK := expression.(*ast.IdentifierExpr)
		if !identifierOK {
			return "", "", false
		}
		symbol, found := c.lookupSymbol(identifier.Name, identifier.Span)
		if !found || !symbol.constant || symbol.declaration == nil {
			return "", "", false
		}
		literal, ok = symbol.declaration.Value.(*ast.LiteralExpr)
		if !ok {
			return "", "", false
		}
	}
	switch literal.Kind {
	case ast.StringLiteral:
		value, err := strconv.Unquote(literal.Text)
		if err != nil {
			return "", "", false
		}
		return "string:" + value, strconv.Quote(value), true
	case ast.BooleanLiteral:
		return "boolean:" + literal.Text, literal.Text, true
	case ast.NilLiteral:
		return "nil", "nil", true
	case ast.NullLiteral:
		return "null", "null", true
	case ast.FloatLiteral:
		value := constant.MakeFromLiteral(literal.Text, gotoken.FLOAT, 0)
		if value.Kind() == constant.Unknown {
			return "", "", false
		}
		return "number:" + value.ExactString(), value.ExactString(), true
	default:
		return "", "", false
	}
}

func valueSwitchCaseFallthroughReachable(clause *ast.ValueSwitchCase) bool {
	if clause == nil || !clause.FallsThrough || clause.Body == nil || len(clause.Body.Statements) == 0 {
		return false
	}
	for _, statement := range clause.Body.Statements[:len(clause.Body.Statements)-1] {
		if statementDefinitelyStopsBlock(statement) {
			return false
		}
	}
	return true
}

func isAllowedExpressionStatement(expression ast.Expression) bool {
	if _, ok := expression.(*ast.CallExpr); ok {
		return true
	}
	if await, ok := expression.(*ast.AwaitExpr); ok {
		return await.Void
	}
	unary, ok := expression.(*ast.UnaryExpr)
	return ok && unary.Operator == "<-"
}

func (c *Checker) checkMultiVariableDeclaration(stmt *ast.MultiVariableDecl) {
	value := c.checkMultipleValueExpression(stmt.Value)
	results := c.multipleResults(value, len(stmt.Bindings), stmt.Value.GetSpan())
	for i := range stmt.Bindings {
		binding := &stmt.Bindings[i]
		if binding.Name == "_" {
			continue
		}
		result := Type{Kind: Invalid, Name: "<invalid>"}
		if i < len(results) {
			result = defaultLiteralType(results[i])
		}
		binding.ResolvedType = typeRefFromType(result, binding.Span)
		c.declareMultiLocal(binding.Name, result, stmt.Constant, stmt, i, binding.Span)
		c.updateIdentifierFlow(binding.Name, binding.Span, result)
	}
}

func (c *Checker) checkMultiAssignment(stmt *ast.MultiAssignmentStmt) {
	value := c.checkMultipleValueExpression(stmt.Value)
	results := c.multipleResults(value, len(stmt.Bindings), stmt.Value.GetSpan())
	for i := range stmt.Bindings {
		binding := &stmt.Bindings[i]
		if binding.Name == "_" {
			continue
		}
		symbol, exists := c.lookupAssignmentSymbol(binding.Name, binding.Span)
		if !exists {
			c.report(binding.Span, fmt.Sprintf("undefined name %q", binding.Name))
			continue
		}
		if symbol.constant {
			c.report(binding.Span, fmt.Sprintf("cannot assign to const %q", binding.Name))
		}
		binding.ResolvedDeclaration = symbol.declarationSpan
		if i < len(results) {
			declared := symbol.declaredType
			if declared.Kind == Invalid {
				declared = symbol.typeInfo
			}
			c.requireAssignable(declared, results[i], binding.Span)
			c.updateIdentifierFlow(binding.Name, binding.Span, results[i])
		}
	}
}

func (c *Checker) checkMultipleValueExpression(expression ast.Expression) Type {
	if receive, ok := expression.(*ast.UnaryExpr); ok && receive.Operator == "<-" {
		return c.checkChannelReceive(receive, true)
	}
	if index, ok := expression.(*ast.IndexExpr); ok {
		return c.checkIndex(index, true)
	}
	return c.checkExpression(expression)
}

func (c *Checker) multipleResults(value Type, bindings int, span source.Span) []Type {
	if value.Kind == Invalid {
		return nil
	}
	if value.Kind == Result && value.Element != nil {
		results := []Type{builtins["error"]}
		if value.Element.Kind != Void {
			results = []Type{*value.Element, builtins["error"]}
		}
		if len(results) != bindings {
			c.report(span, fmt.Sprintf("Result binding count mismatch: got %d bindings for %d results", bindings, len(results)))
		}
		return results
	}
	if value.Kind != MultiValue {
		c.report(span, fmt.Sprintf("multiple binding requires a multiple-return value, got %s", value.String()))
		return nil
	}
	if len(value.Results) != bindings {
		c.report(span, fmt.Sprintf("multiple binding count mismatch: got %d bindings for %d results", bindings, len(value.Results)))
	}
	return value.Results
}

func (c *Checker) checkAssignmentTarget(expr ast.Expression) Type {
	if identifier, ok := expr.(*ast.IdentifierExpr); ok {
		symbol, exists := c.lookupAssignmentSymbol(identifier.Name, identifier.Span)
		if !exists {
			c.report(identifier.Span, fmt.Sprintf("undefined name %q", identifier.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if symbol.constant {
			c.report(identifier.Span, fmt.Sprintf("cannot assign to const %q", identifier.Name))
		}
		identifier.ResolvedDeclaration = symbol.declarationSpan
		if symbol.declaredType.Kind != Invalid {
			return symbol.declaredType
		}
		return symbol.typeInfo
	}
	target := c.checkExpression(expr)
	if member, ok := expr.(*ast.MemberExpr); ok {
		if member.Constant {
			c.report(member.Span, fmt.Sprintf("cannot assign to Go constant %q", member.Name))
		} else if !member.Addressable {
			c.report(member.Span, fmt.Sprintf("member %q is not assignable", member.Name))
		}
		if key, stable := c.stableMemberFlowKey(member); stable {
			if declared, exists := c.memberTypes[key]; exists {
				return declared
			}
		}
	} else if index, ok := expr.(*ast.IndexExpr); ok && !index.Assignable {
		c.report(index.Span, "index expression is not assignable")
	}
	return target
}

func (c *Checker) updateAssignmentFlow(target ast.Expression, value Type) {
	if identifier, ok := target.(*ast.IdentifierExpr); ok {
		c.invalidateMemberFactsRootedAt(identifier, target.GetSpan(), "an assignment to its receiver")
		c.updateIdentifierFlow(identifier.Name, identifier.Span, value)
		return
	}
	if member, ok := target.(*ast.MemberExpr); ok {
		key, stable := c.stableMemberFlowKey(member)
		declared, typed := c.memberTypes[key]
		c.recordMemberWrite(target.GetSpan())
		c.invalidateAllMemberFacts(target.GetSpan(), "a possibly aliased field assignment")
		if stable && typed && declared.Kind == Nullable && declared.Element != nil && value.Kind != Nullable && value.Kind != Null && value.Kind != Nil && value.Kind != Invalid && c.isAssignable(*declared.Element, value) {
			c.memberFlow[key] = memberFlowState{declaredType: declared, nonNullType: *declared.Element, nonNull: true}
		}
	}
}

func (c *Checker) updateIdentifierFlow(name string, span source.Span, value Type) {
	for index := len(c.scopes) - 1; index >= 0; index-- {
		symbol, exists := c.scopes[index][name]
		if !exists {
			continue
		}
		if symbol.constant && symbol.declarationSpan != span {
			return
		}
		declared := symbol.declaredType
		if declared.Kind == Invalid {
			declared = symbol.typeInfo
		}
		if !c.isAssignable(declared, value) {
			return
		}
		if symbol.declarationSpan != span {
			c.invalidateMemberFactsForDeclaration(symbol.declarationSpan, span, "an assignment to its receiver")
		}
		c.recordCapturedWrite(index, symbol.declarationSpan, span)
		symbol.typeInfo = declared
		if declared.Kind == Nullable && declared.Element != nil && !symbol.flowEscaped && value.Kind != Nullable && value.Kind != Null && value.Kind != Nil && value.Kind != Invalid && c.isAssignable(*declared.Element, value) {
			symbol.typeInfo = *declared.Element
			symbol.flowInvalidated = source.Span{}
			symbol.flowInvalidationCause = ""
		} else if declared.Kind == Nullable && symbol.declarationSpan != span {
			symbol.flowInvalidated = span
			symbol.flowInvalidationCause = "an assignment"
		}
		c.scopes[index][name] = symbol
		return
	}
}

func (c *Checker) recordCapturedWrite(scopeIndex int, declaration, cause source.Span) {
	if c.suppressFlowEffects != 0 {
		return
	}
	for index, base := range c.callableScopeBases {
		if scopeIndex >= base {
			continue
		}
		if _, exists := c.capturedWrites[index][declaration]; !exists {
			c.capturedWrites[index][declaration] = cause
		}
	}
}

func (c *Checker) markIdentifierEscaped(name string, span source.Span, cause string) {
	for index := len(c.scopes) - 1; index >= 0; index-- {
		symbol, exists := c.scopes[index][name]
		if !exists {
			continue
		}
		c.recordCapturedWrite(index, symbol.declarationSpan, span)
		symbol.flowEscaped = true
		symbol.typeInfo = symbol.declaredType
		symbol.flowInvalidated = span
		symbol.flowInvalidationCause = cause
		c.scopes[index][name] = symbol
		return
	}
}

func (c *Checker) markDeclarationEscaped(declaration, span source.Span, cause string) {
	for scopeIndex := len(c.scopes) - 1; scopeIndex >= 0; scopeIndex-- {
		for name, symbol := range c.scopes[scopeIndex] {
			if symbol.declarationSpan != declaration {
				continue
			}
			symbol.flowEscaped = true
			symbol.typeInfo = symbol.declaredType
			symbol.flowInvalidated = span
			symbol.flowInvalidationCause = cause
			c.scopes[scopeIndex][name] = symbol
			return
		}
	}
}

func (c *Checker) markAssignmentTargetRead(expr ast.Expression) {
	if identifier, ok := expr.(*ast.IdentifierExpr); ok {
		if symbol, exists := c.lookupSymbol(identifier.Name, identifier.Span); exists {
			identifier.ResolvedDeclaration = symbol.declarationSpan
		}
	}
}

func (c *Checker) checkLoopCondition(expr ast.Expression) {
	condition := c.checkExpression(expr)
	if condition.Kind != Invalid && condition.Kind != Boolean {
		c.report(expr.GetSpan(), fmt.Sprintf("loop condition must be boolean, got %s", condition.String()))
	}
}

func (c *Checker) checkExpression(expr ast.Expression) Type {
	if expr == nil {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	switch expr := expr.(type) {
	case *ast.LiteralExpr:
		switch expr.Kind {
		case ast.IntegerLiteral:
			return Type{Kind: UntypedInt, Name: "integer literal"}
		case ast.FloatLiteral:
			return builtins["float"]
		case ast.StringLiteral:
			return builtins["string"]
		case ast.BooleanLiteral:
			return builtins["boolean"]
		case ast.NilLiteral:
			return Type{Kind: Nil, Name: "nil"}
		case ast.NullLiteral:
			return Type{Kind: Null, Name: "null"}
		}
	case *ast.IdentifierExpr:
		if symbol, ok := c.lookupSymbol(expr.Name, expr.Span); ok {
			expr.ResolvedDeclaration = symbol.declarationSpan
			if symbol.typeInfo.Kind == Task && c.taskOperandDepth == 0 {
				c.report(expr.Span, "Task values may only be consumed by await or detach and cannot be copied or passed")
			}
			return symbol.typeInfo
		}
		if imported := c.lookupGoPackage(expr.Span.Path, expr.Name); imported != nil {
			expr.ResolvedDeclaration = imported.declaration.AliasSpan
			return Type{Kind: GoPackage, Name: expr.Name, GoPackage: imported}
		}
		if function, ok := c.functions[expr.Name]; ok && c.isTopLevelAllowed(expr.Span, expr.Name) {
			expr.ResolvedDeclaration = function.declarationSpan
			callable := callableTypeForFunction(function)
			if callable.Generic {
				c.report(expr.Span, fmt.Sprintf("generic function %q must be called before it can be used as a value", expr.Name))
			}
			return callable
		}
		if named, ok := c.nativeTypes[expr.Name]; ok && c.isTopLevelAllowed(expr.Span, expr.Name) {
			expr.ResolvedDeclaration = named.declaration.NameSpan
			c.report(expr.Span, fmt.Sprintf("type %q cannot be used as a value; call it with one argument to convert", expr.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		c.report(expr.Span, fmt.Sprintf("undefined name %q", expr.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	case *ast.UnaryExpr:
		return c.checkUnary(expr)
	case *ast.BinaryExpr:
		return c.checkBinary(expr)
	case *ast.GoTypeAssertionExpr:
		return c.checkGoTypeAssertion(expr)
	case *ast.TaskStartExpr:
		return c.checkTaskStart(expr)
	case *ast.AwaitExpr:
		return c.checkAwait(expr)
	case *ast.PropagateExpr:
		c.report(expr.Span, "result propagation may only be used as a variable initializer or as a void expression statement")
		return c.checkPropagateExpression(expr)
	case *ast.CallExpr:
		return c.checkCall(expr)
	case *ast.ArrowExpr:
		return c.checkArrow(expr)
	case *ast.ArrayLiteralExpr:
		return c.checkArrayLiteral(expr)
	case *ast.ObjectLiteralExpr:
		return c.checkObjectLiteral(expr)
	case *ast.GoCompositeLiteralExpr:
		return c.checkGoCompositeLiteral(expr)
	case *ast.MemberExpr:
		return c.checkMember(expr)
	case *ast.IndexExpr:
		return c.checkIndex(expr, false)
	case *ast.SliceExpr:
		return c.checkSlice(expr)
	case *ast.NewExpr:
		return c.checkNew(expr)
	case *ast.ClassUpcastExpr:
		return c.resolveType(expr.TargetType)
	}
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) checkGoTypeAssertion(expr *ast.GoTypeAssertionExpr) Type {
	value := c.singleValue(c.checkExpression(expr.Value), expr.Value.GetSpan())
	asserted := c.resolveType(expr.Type)
	if value.Kind == Invalid || asserted.Kind == Invalid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	source := value
	if source.Kind == Nullable && source.Element != nil {
		source = *source.Element
	}
	if source.Kind == Class {
		if asserted.Kind != Class {
			c.report(expr.Span, fmt.Sprintf("class downcast requires class source and target types, got %s and %s", value.String(), asserted.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if source.Name == asserted.Name {
			c.report(expr.Span, fmt.Sprintf("%s already has class type %s; no downcast is needed", value.String(), asserted.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		ancestor, downcast := c.classAncestorType(asserted, source.Name)
		if !downcast || !exactType(source, ancestor) {
			upcastAncestor, upcast := c.classAncestorType(source, asserted.Name)
			if upcast && exactType(asserted, upcastAncestor) {
				c.report(expr.Span, fmt.Sprintf("%s to %s is an upcast; use ordinary assignment or argument passing", source.String(), asserted.String()))
			} else {
				c.report(expr.Span, fmt.Sprintf("classes %s and %s are not in the same inheritance chain", source.String(), asserted.String()))
			}
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		expr.ClassDowncast = true
		expr.SourceClass = source.Name
		if !expr.Checked {
			return asserted
		}
		return Type{Kind: MultiValue, Name: "checked class downcast", Results: []Type{asserted, builtins["boolean"]}}
	}
	contract := underlyingGoInterface(value.GoType)
	if contract == nil {
		c.report(expr.Value.GetSpan(), fmt.Sprintf("type assertion requires a Go interface value, got %s", value.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	assertedGoType, ok := goTypeOf(asserted)
	if !ok {
		c.report(expr.Type.Span, fmt.Sprintf("asserted type %s cannot be represented as a Go type", asserted.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !gotypes.AssertableTo(contract, assertedGoType) {
		c.report(expr.Span, fmt.Sprintf("Go interface %s cannot contain asserted type %s", value.String(), asserted.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !expr.Checked {
		return asserted
	}
	return Type{Kind: MultiValue, Name: "checked type assertion", Results: []Type{asserted, builtins["boolean"]}}
}

func (c *Checker) checkTaskStart(expr *ast.TaskStartExpr) Type {
	c.usesTasks = true
	result := c.checkExpression(expr.Call)
	if expr.Call.Conversion {
		c.report(expr.Call.Span, "go expression requires a function or method call; type conversions are not calls")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if expr.Call.Builtin != ast.NotBuiltinCall {
		c.report(expr.Call.Span, "go expression does not support compiler built-ins; wrap the operation in a function")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if result.Kind == Invalid {
		return result
	}
	if result.Kind == MultiValue {
		c.report(expr.Call.Span, "go expression cannot start a raw multiple-result Go call; wrap it in a Result-returning OnsenTamago function")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	expr.ResultTask = result.Kind == Result
	expr.Void = result.Kind == Void || expr.ResultTask && result.Element != nil && result.Element.Kind == Void
	value := result
	if result.Kind == Result && result.Element != nil {
		value = *result.Element
	}
	c.prepareGoTypeForEmission(&value, expr.Span)
	expr.ValueType = typeRefFromType(value, expr.Span)
	return Type{Kind: Task, Name: "Task", Element: &result}
}

func (c *Checker) checkAwait(expr *ast.AwaitExpr) Type {
	c.usesTasks = true
	c.taskOperandDepth++
	task := c.singleValue(c.checkExpression(expr.Value), expr.Value.GetSpan())
	c.taskOperandDepth--
	if task.Kind != Task || task.Element == nil {
		if task.Kind != Invalid {
			c.report(expr.Value.GetSpan(), fmt.Sprintf("await requires Task<T>, got %s", task.String()))
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	c.consumeTask(expr.Value)
	result := *task.Element
	expr.ResultTask = result.Kind == Result
	expr.Void = result.Kind == Void || expr.ResultTask && result.Element != nil && result.Element.Kind == Void
	value := result
	if expr.ResultTask && result.Element != nil {
		value = *result.Element
	}
	c.prepareGoTypeForEmission(&value, expr.Span)
	expr.ValueType = typeRefFromType(value, expr.Span)
	return result
}

func (c *Checker) checkExpressionExpected(expr ast.Expression, expected Type) Type {
	if array, ok := expr.(*ast.ArrayLiteralExpr); ok && (expected.Kind == Array || expected.Kind == FixedArray) && expected.Element != nil {
		return c.checkArrayLiteralExpected(array, expected)
	}
	if object, ok := expr.(*ast.ObjectLiteralExpr); ok && expected.Kind == Object {
		return c.checkObjectLiteralExpected(object, expected)
	}
	actual := c.checkExpression(expr)
	if actual.Kind == UntypedInt && expected.IsInteger() {
		if value, known := c.resolvedIntegerConstantValue(expr); known && !integerConstantFitsFixedType(value, expected) {
			c.report(expr.GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", value.String(), expected.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
	}
	return actual
}

func (c *Checker) checkExpressionExpectedSlot(slot *ast.Expression, expected Type) Type {
	actual := c.checkExpressionExpected(*slot, expected)
	targetClass, actualClass := expected, actual
	if targetClass.Kind == Nullable && targetClass.Element != nil {
		targetClass = *targetClass.Element
	}
	if actualClass.Kind == Nullable && actualClass.Element != nil {
		actualClass = *actualClass.Element
	}
	if targetClass.Kind == Class && actualClass.Kind == Class && targetClass.Name != actualClass.Name {
		if ancestor, ok := c.classAncestorType(actualClass, targetClass.Name); ok && exactType(targetClass, ancestor) {
			*slot = &ast.ClassUpcastExpr{
				Value: *slot, SourceClass: actualClass.Name, TargetClass: targetClass.Name,
				SourceType: typeRefFromType(actualClass, (*slot).GetSpan()), TargetType: typeRefFromType(targetClass, (*slot).GetSpan()), Span: (*slot).GetSpan(),
			}
		}
	}
	return actual
}

func (c *Checker) classAncestorType(value Type, baseName string) (Type, bool) {
	class := c.classes[value.Name]
	if class == nil {
		return Type{}, false
	}
	bindings := nativeClassBindings(class, value)
	for index, ancestor := range class.ancestors {
		if ancestor == baseName && index < len(class.ancestorTypes) {
			return substituteNativeTypeParameters(class.ancestorTypes[index], bindings), true
		}
	}
	return Type{}, false
}

func (c *Checker) classExtends(className, baseName string) bool {
	class := c.classes[className]
	if class == nil {
		return false
	}
	for _, ancestor := range class.ancestors {
		if ancestor == baseName {
			return true
		}
	}
	return false
}

func (c *Checker) canAccessClassMember(visibility ast.Visibility, declaringClass string) bool {
	switch visibility {
	case ast.Public:
		return true
	case ast.Private:
		return c.currentClass == declaringClass
	case ast.Protected:
		return c.currentClass == declaringClass || (c.currentClass != "" && c.classExtends(c.currentClass, declaringClass))
	default:
		return false
	}
}

func (c *Checker) reportInaccessibleClassMember(span source.Span, kind, name string, visibility ast.Visibility) {
	label := "private"
	if visibility == ast.Protected {
		label = "protected"
	}
	c.report(span, fmt.Sprintf("%s %q is %s", kind, name, label))
}

func (c *Checker) checkArrayLiteral(expr *ast.ArrayLiteralExpr) Type {
	expr.Fixed = false
	expr.ResolvedLength = 0
	if len(expr.Elements) == 0 {
		c.report(expr.Span, "cannot infer the element type of an empty array")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	element := defaultLiteralType(c.singleValue(c.checkExpression(expr.Elements[0]), expr.Elements[0].GetSpan()))
	if element.Kind == Nil || element.Kind == Null {
		c.report(expr.Elements[0].GetSpan(), "cannot infer an array element type from nil or null")
		element = Type{Kind: Invalid, Name: "<invalid>"}
	}
	for _, item := range expr.Elements[1:] {
		actual := c.checkExpression(item)
		c.requireAssignable(element, actual, item.GetSpan())
	}
	c.prepareGoTypeForEmission(&element, expr.Span)
	expr.ResolvedElementType = typeRefFromType(element, expr.Span)
	return Type{Kind: Array, Name: "array", Element: &element}
}

func (c *Checker) checkArrayLiteralExpected(expr *ast.ArrayLiteralExpr, expected Type) Type {
	element := *expected.Element
	if expected.Kind == FixedArray && int64(len(expr.Elements)) != expected.Length {
		c.report(expr.Span, fmt.Sprintf("fixed array literal has %d elements, expected %d", len(expr.Elements), expected.Length))
	}
	for index := range expr.Elements {
		actual := c.checkExpressionExpectedSlot(&expr.Elements[index], element)
		c.requireAssignable(element, actual, expr.Elements[index].GetSpan())
	}
	c.prepareGoTypeForEmission(&element, expr.Span)
	expr.ResolvedElementType = typeRefFromType(element, expr.Span)
	expr.Fixed = expected.Kind == FixedArray
	expr.ResolvedLength = expected.Length
	return expected
}

func (c *Checker) checkObjectLiteral(expr *ast.ObjectLiteralExpr) Type {
	fields := map[string]Type{}
	fieldNames := map[string]string{}
	expr.ResolvedFieldTypes = make([]ast.TypeRef, len(expr.Fields))
	expr.ResolvedFieldNames = make([]string, len(expr.Fields))
	for i, field := range expr.Fields {
		if _, exists := fields[field.Name]; exists {
			c.report(field.Span, fmt.Sprintf("duplicate object field %q", field.Name))
		}
		fieldType := defaultLiteralType(c.singleValue(c.checkExpression(field.Value), field.Value.GetSpan()))
		if fieldType.Kind == Nil || fieldType.Kind == Null {
			c.report(field.Value.GetSpan(), fmt.Sprintf("cannot infer object field %q from nil or null", field.Name))
			fieldType = Type{Kind: Invalid, Name: "<invalid>"}
		}
		c.prepareGoTypeForEmission(&fieldType, field.Span)
		fields[field.Name] = fieldType
		fieldNames[field.Name] = memberGoName(field.Name, ast.Public)
		expr.ResolvedFieldTypes[i] = typeRefFromType(fieldType, field.Span)
		expr.ResolvedFieldNames[i] = fieldNames[field.Name]
	}
	return Type{Kind: Object, Name: "object", Fields: fields, FieldNames: fieldNames}
}

func (c *Checker) checkObjectLiteralExpected(expr *ast.ObjectLiteralExpr, expected Type) Type {
	expr.ResolvedFieldTypes = make([]ast.TypeRef, len(expr.Fields))
	expr.ResolvedFieldNames = make([]string, len(expr.Fields))
	seen := map[string]bool{}
	for index, field := range expr.Fields {
		if seen[field.Name] {
			c.report(field.Span, fmt.Sprintf("duplicate object field %q", field.Name))
		}
		seen[field.Name] = true
		fieldType, exists := expected.Fields[field.Name]
		if !exists {
			c.report(field.Span, fmt.Sprintf("object type has no field %q", field.Name))
			c.checkExpression(field.Value)
			continue
		}
		actual := c.checkExpressionExpectedSlot(&expr.Fields[index].Value, fieldType)
		c.requireAssignable(fieldType, actual, field.Value.GetSpan())
		c.prepareGoTypeForEmission(&fieldType, field.Span)
		expr.ResolvedFieldTypes[index] = typeRefFromType(fieldType, field.Span)
		expr.ResolvedFieldNames[index] = expected.FieldNames[field.Name]
	}
	for name := range expected.Fields {
		if !seen[name] {
			c.report(expr.Span, fmt.Sprintf("object literal is missing field %q", name))
		}
	}
	return expected
}

func (c *Checker) checkUnary(expr *ast.UnaryExpr) Type {
	if expr.Operator == "<-" {
		return c.checkChannelReceive(expr, false)
	}
	operand := c.singleValue(c.checkExpression(expr.Operand), expr.Operand.GetSpan())
	if expr.Operator == "&" {
		if operand.Kind == Invalid {
			return operand
		}
		if !c.isAddressableExpression(expr.Operand) {
			c.report(expr.Span, "operator & requires an addressable operand")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		pointee := operand
		var addressedIdentifier *ast.IdentifierExpr
		if identifier, ok := expr.Operand.(*ast.IdentifierExpr); ok {
			addressedIdentifier = identifier
			if symbol, exists := c.lookupSymbol(identifier.Name, identifier.Span); exists {
				pointee = symbol.declaredType
			}
		}
		var pointerGoType gotypes.Type
		if goType, ok := goTypeOf(pointee); ok {
			pointerGoType = gotypes.NewPointer(goType)
		}
		if addressedIdentifier != nil {
			c.markIdentifierEscaped(addressedIdentifier.Name, expr.Span, "taking its address")
		} else if _, ok := expr.Operand.(*ast.MemberExpr); ok {
			c.recordMemberWrite(expr.Span)
			c.invalidateAllMemberFacts(expr.Span, "taking the address of possibly aliased member storage")
		}
		return Type{Kind: GoPointer, Name: "*" + pointee.String(), Element: &pointee, GoType: pointerGoType, GoQualifier: pointee.GoQualifier}
	}
	if operand.Kind == Nullable {
		c.report(expr.Operand.GetSpan(), fmt.Sprintf("nullable value %s must be checked against null before using operator %s", operand.String(), expr.Operator))
		if operand.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		operand = *operand.Element
	}
	switch expr.Operator {
	case "!":
		if operand.Kind != Invalid && !operand.IsBoolean() {
			c.report(expr.Span, "operator ! requires a boolean operand")
		}
		return builtins["boolean"]
	case "*":
		if operand.Kind == Invalid {
			return operand
		}
		if operand.Kind == GoPointer && operand.Element != nil {
			return *operand.Element
		}
		goType, ok := goTypeOf(operand)
		if !ok {
			c.report(expr.Span, fmt.Sprintf("operator * requires a pointer operand, got %s", operand.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		pointer, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Pointer)
		if !ok {
			c.report(expr.Span, fmt.Sprintf("operator * requires a pointer operand, got %s", operand.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		result, err := ontamaTypeFromGo(pointer.Elem())
		if err != nil {
			c.report(expr.Span, err.Error())
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		inheritGoQualifier(&result, operand)
		return result
	case "^":
		if operand.Kind != Invalid && !operand.IsInteger() {
			c.report(expr.Span, "operator ^ requires an integer operand")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return operand
	default:
		if !operand.IsNumeric() && operand.Kind != Invalid {
			c.report(expr.Span, fmt.Sprintf("operator %s requires a numeric operand", expr.Operator))
		}
		return operand
	}
}

func (c *Checker) checkChannelReceive(expr *ast.UnaryExpr, checked bool) Type {
	operand := c.singleValue(c.checkExpression(expr.Operand), expr.Operand.GetSpan())
	if operand.Kind == Nullable {
		c.report(expr.Operand.GetSpan(), fmt.Sprintf("nullable channel %s must be checked against null before receiving", operand.String()))
		if operand.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		operand = *operand.Element
	}
	if operand.Kind == Invalid {
		return operand
	}
	goType, ok := goTypeOf(operand)
	if !ok {
		c.report(expr.Span, fmt.Sprintf("operator <- requires a Go channel operand, got %s", operand.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	channel, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Chan)
	if !ok {
		c.report(expr.Span, fmt.Sprintf("operator <- requires a Go channel operand, got %s", operand.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if channel.Dir() == gotypes.SendOnly {
		c.report(expr.Span, fmt.Sprintf("cannot receive from send-only channel %s", operand.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	element, err := ontamaTypeFromGo(channel.Elem())
	if err != nil {
		c.report(expr.Span, fmt.Sprintf("channel element type is not supported: %v", err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if operand.Element != nil {
		element = *operand.Element
	}
	if checked {
		return Type{Kind: MultiValue, Name: "checked channel receive", Results: []Type{element, builtins["boolean"]}}
	}
	return element
}

func (c *Checker) isAddressableExpression(expression ast.Expression) bool {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		symbol, ok := c.lookupSymbol(expression.Name, expression.Span)
		if ok {
			expression.ResolvedDeclaration = symbol.declarationSpan
		}
		return ok
	case *ast.MemberExpr:
		return expression.Addressable
	case *ast.IndexExpr:
		return expression.Addressable
	case *ast.UnaryExpr:
		return expression.Operator == "*"
	default:
		return false
	}
}

func (c *Checker) checkGoCompositeLiteral(expr *ast.GoCompositeLiteralExpr) Type {
	result := c.resolveType(expr.Type)
	if result.Kind == Invalid {
		for _, field := range expr.Fields {
			c.checkExpression(field.Value)
		}
		return result
	}
	if result.Kind == Struct {
		symbol := c.structs[result.Name]
		if symbol == nil {
			c.report(expr.Type.Span, fmt.Sprintf("unknown struct type %s", result.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		expr.ResolvedFieldNames = make([]string, len(expr.Fields))
		bindings := nativeStructBindings(symbol, result)
		seen := map[string]bool{}
		for index, field := range expr.Fields {
			if seen[field.Name] {
				c.report(field.Span, fmt.Sprintf("duplicate struct field %q", field.Name))
			}
			seen[field.Name] = true
			selected, exists := symbol.fields[field.Name]
			if !exists {
				c.report(field.Span, fmt.Sprintf("struct %s has no field %q", result.Name, field.Name))
				c.checkExpression(field.Value)
				continue
			}
			expr.ResolvedFieldNames[index] = selected.goName
			expr.Fields[index].ResolvedDeclaration = selected.declarationSpan
			expected := substituteNativeTypeParameters(selected.typeInfo, bindings)
			actual := c.checkExpressionExpectedSlot(&expr.Fields[index].Value, expected)
			c.requireAssignable(expected, actual, field.Value.GetSpan())
		}
		for name := range symbol.fields {
			if !seen[name] {
				c.report(expr.Span, fmt.Sprintf("struct literal %s is missing field %q", result.Name, name))
			}
		}
		return result
	}
	goType, ok := goTypeOf(result)
	if !ok {
		c.report(expr.Type.Span, fmt.Sprintf("type %s is not a Go struct type", result.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	structure, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Struct)
	if !ok {
		c.report(expr.Type.Span, fmt.Sprintf("Go type %s is not a struct", result.String()))
		for _, field := range expr.Fields {
			c.checkExpression(field.Value)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	seen := map[string]bool{}
	for index, field := range expr.Fields {
		if seen[field.Name] {
			c.report(field.Span, fmt.Sprintf("duplicate Go struct field %q", field.Name))
		}
		seen[field.Name] = true
		var selected *gotypes.Var
		for i := 0; i < structure.NumFields(); i++ {
			candidate := structure.Field(i)
			if candidate.Name() == field.Name {
				selected = candidate
				break
			}
		}
		if selected == nil {
			c.report(field.Span, fmt.Sprintf("Go struct %s has no field %q", result.String(), field.Name))
			c.checkExpression(field.Value)
			continue
		}
		if !selected.Exported() {
			c.report(field.Span, fmt.Sprintf("Go struct field %q is not exported", field.Name))
			c.checkExpression(field.Value)
			continue
		}
		expected, err := ontamaTypeFromGo(selected.Type())
		if err != nil {
			c.report(field.Span, fmt.Sprintf("Go struct field %s.%s is not supported: %v", result.String(), field.Name, err))
			c.checkExpression(field.Value)
			continue
		}
		inheritGoQualifier(&expected, result)
		actual := c.checkExpressionExpectedSlot(&expr.Fields[index].Value, expected)
		c.requireAssignable(expected, actual, field.Value.GetSpan())
	}
	return result
}

func (c *Checker) checkMember(expr *ast.MemberExpr) Type {
	if identifier, ok := expr.Object.(*ast.IdentifierExpr); ok {
		if identifier.Name == "super" {
			return c.checkSuperMember(expr)
		}
		if enumeration := c.enums[identifier.Name]; enumeration != nil && c.isTopLevelAllowed(identifier.Span, identifier.Name) {
			member := enumeration.members[expr.Name]
			if member == nil {
				c.report(expr.Span, fmt.Sprintf("enum %s has no member %q", identifier.Name, expr.Name))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			identifier.ResolvedDeclaration = enumeration.declaration.NameSpan
			expr.ResolvedDeclaration = member.NameSpan
			expr.ResolvedName = enumMemberGoName(identifier.Name, member.Name)
			expr.Static = true
			expr.Constant = true
			return c.resolveNativeType(c.nativeTypes[identifier.Name])
		}
		if class := c.classes[identifier.Name]; class != nil && c.isTopLevelAllowed(identifier.Span, identifier.Name) {
			method, exists := class.methods[expr.Name]
			if !exists || !method.static {
				c.report(expr.Span, fmt.Sprintf("class %s has no static method %q", identifier.Name, expr.Name))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			if !c.canAccessClassMember(method.visibility, method.declaringClass) {
				c.reportInaccessibleClassMember(expr.Span, "method", expr.Name, method.visibility)
			}
			expr.Static = true
			identifier.ResolvedDeclaration = class.declarationSpan
			expr.ResolvedDeclaration = method.declarationSpan
			owner := method.declaringClass
			if owner == "" {
				owner = identifier.Name
			}
			expr.ResolvedName = staticMethodGoName(owner, method.goName, method.visibility)
			return method.typeInfo
		}
	}
	object := c.singleValue(c.checkExpression(expr.Object), expr.Object.GetSpan())
	if object.Kind == Nullable {
		message := fmt.Sprintf("nullable value %s must be checked against null before member access", object.String())
		if invalidated, cause := c.flowInvalidation(expr.Object); invalidated.Start.Line != 0 {
			if cause == "" {
				cause = "a possible mutation"
			}
			message += fmt.Sprintf("; the previous non-null proof was invalidated by %s at %d:%d", cause, invalidated.Start.Line, invalidated.Start.Column)
		}
		c.report(expr.Object.GetSpan(), message)
		if object.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		object = *object.Element
	}
	if object.Kind == Object {
		field, ok := object.Fields[expr.Name]
		if !ok {
			c.report(expr.Span, fmt.Sprintf("object has no member %q", expr.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		expr.ResolvedName = object.FieldNames[expr.Name]
		expr.Addressable = c.isAddressableExpression(expr.Object)
		return field
	}
	if object.Kind == Class {
		class := c.classes[object.Name]
		if class == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if field, ok := class.fields[expr.Name]; ok {
			if !c.canAccessClassMember(field.visibility, field.declaringClass) {
				c.reportInaccessibleClassMember(expr.Span, "field", expr.Name, field.visibility)
			}
			expr.ResolvedName = field.goName
			expr.ResolvedDeclaration = field.declarationSpan
			expr.Addressable = true
			fieldType := substituteNativeTypeParameters(field.typeInfo, nativeClassBindings(class, object))
			if key, stable := c.stableMemberFlowKey(expr); stable {
				c.memberTypes[key] = fieldType
				if state, proven := c.memberFlow[key]; proven && state.nonNull && fieldType.Kind == Nullable {
					return state.nonNullType
				}
			}
			return fieldType
		}
		if method, ok := class.methods[expr.Name]; ok {
			if method.static {
				c.report(expr.Span, fmt.Sprintf("static method %q cannot be called on an instance", expr.Name))
			}
			if !c.canAccessClassMember(method.visibility, method.declaringClass) {
				c.reportInaccessibleClassMember(expr.Span, "method", expr.Name, method.visibility)
			}
			expr.ResolvedName = method.goName
			expr.ResolvedDeclaration = method.declarationSpan
			expr.VirtualDispatch = method.virtual
			expr.VirtualOwner = method.virtualOwner
			return substituteNativeTypeParameters(method.typeInfo, nativeClassBindings(class, object))
		}
		c.report(expr.Span, fmt.Sprintf("class %s has no member %q", object.Name, expr.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	structObject := object
	structPointer := false
	if object.Kind == GoPointer && object.Element != nil && object.Element.Kind == Struct {
		structObject = *object.Element
		structPointer = true
	}
	if structObject.Kind == Struct {
		structure := c.structs[structObject.Name]
		if structure == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if field, ok := structure.fields[expr.Name]; ok {
			expr.ResolvedName = field.goName
			expr.ResolvedDeclaration = field.declarationSpan
			expr.Addressable = structPointer || c.isAddressableExpression(expr.Object)
			return substituteNativeTypeParameters(field.typeInfo, nativeStructBindings(structure, structObject))
		}
		if method, ok := structure.methods[expr.Name]; ok {
			if method.pointerReceiver && !structPointer && !c.isAddressableExpression(expr.Object) {
				c.report(expr.Span, fmt.Sprintf("pointer method %q requires an addressable %s value", expr.Name, structObject.String()))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			expr.ResolvedName = method.goName
			expr.ResolvedDeclaration = method.declarationSpan
			return substituteNativeTypeParameters(method.typeInfo, nativeStructBindings(structure, structObject))
		}
		c.report(expr.Span, fmt.Sprintf("struct %s has no field %q or method with that name", structObject.Name, expr.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	nativeObject := object
	nativePointer := false
	if object.Kind == GoPointer && object.Element != nil && object.Element.Kind == GoNamed {
		nativeObject = *object.Element
		nativePointer = true
	}
	if nativeObject.Kind == GoNamed && nativeObject.GoQualifier == "" {
		if namedObject := goTypeNameObject(nativeObject.GoType); namedObject != nil {
			if symbol := c.nativeTypes[namedObject.Name()]; symbol != nil && !symbol.declaration.Alias {
				method, exists := symbol.methods[expr.Name]
				if !exists {
					c.report(expr.Span, fmt.Sprintf("defined type %s has no method %q", nativeObject.String(), expr.Name))
					return Type{Kind: Invalid, Name: "<invalid>"}
				}
				if method.pointerReceiver && !nativePointer && !c.isAddressableExpression(expr.Object) {
					c.report(expr.Span, fmt.Sprintf("pointer method %q requires an addressable %s value", expr.Name, nativeObject.String()))
					return Type{Kind: Invalid, Name: "<invalid>"}
				}
				if method.visibility != ast.Public && method.declarationSpan.Path != expr.Span.Path {
					c.report(expr.Span, fmt.Sprintf("method %q is private on defined type %s", expr.Name, nativeObject.String()))
				}
				expr.ResolvedName = method.goName
				expr.ResolvedDeclaration = method.declarationSpan
				return substituteNativeTypeParameters(method.typeInfo, nativeDefinedTypeBindings(symbol, nativeObject))
			}
		}
	}
	if object.Kind == Interface {
		contract := c.interfaces[object.Name]
		if contract == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		method, ok := contract.methods[expr.Name]
		if !ok {
			c.report(expr.Span, fmt.Sprintf("interface %s has no method %q", object.Name, expr.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		expr.ResolvedName = method.goName
		expr.ResolvedDeclaration = method.declarationSpan
		return substituteNativeTypeParameters(method.typeInfo, nativeInterfaceBindings(contract, object))
	}
	if object.Kind == GoPackage {
		return c.checkGoMember(expr, object.GoPackage)
	}
	if object.GoType != nil && object.Kind != GoTypeName {
		return c.checkGoValueMember(expr, object)
	}
	c.report(expr.Span, fmt.Sprintf("type %s has no members", object.String()))
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) checkSuperMember(expr *ast.MemberExpr) Type {
	class := c.classes[c.currentClass]
	if class == nil || class.base == "" {
		c.report(expr.Span, "super member access requires a derived class")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	base := c.classes[class.base]
	if base == nil {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	method, ok := base.methods[expr.Name]
	if !ok || method.static {
		c.report(expr.Span, fmt.Sprintf("base class %s has no instance method %q", class.base, expr.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if method.visibility == ast.Private {
		c.report(expr.Span, fmt.Sprintf("method %q is private in base class %s", expr.Name, method.declaringClass))
	}
	if _, ok := c.lookupSymbol("this", expr.Span); !ok {
		c.report(expr.Span, "super cannot be used in a static method")
	}
	expr.Super = true
	expr.SuperBase = class.base
	expr.ResolvedName = method.goName
	expr.VirtualOwner = method.virtualOwner
	expr.ResolvedDeclaration = method.declarationSpan
	return substituteNativeTypeParameters(method.typeInfo, nativeClassBindings(base, class.baseType))
}

func (c *Checker) flowInvalidation(expression ast.Expression) (source.Span, string) {
	if member, ok := expression.(*ast.MemberExpr); ok {
		if key, stable := c.stableMemberFlowKey(member); stable {
			state := c.memberFlow[key]
			return state.invalidated, state.invalidationCause
		}
	}
	identifier, ok := expression.(*ast.IdentifierExpr)
	if !ok {
		return source.Span{}, ""
	}
	for index := len(c.scopes) - 1; index >= 0; index-- {
		if symbol, exists := c.scopes[index][identifier.Name]; exists {
			return symbol.flowInvalidated, symbol.flowInvalidationCause
		}
	}
	return source.Span{}, ""
}

func (c *Checker) stableMemberFlowKey(expression *ast.MemberExpr) (memberFlowKey, bool) {
	if expression == nil || expression.Go || expression.GoField || !expression.Addressable || expression.ResolvedDeclaration.Start.Line == 0 {
		return memberFlowKey{}, false
	}
	root, path, ok := c.stableMemberFlowPath(expression)
	if !ok {
		return memberFlowKey{}, false
	}
	return memberFlowKey{root: root, path: path}, true
}

func (c *Checker) stableMemberFlowPath(expression ast.Expression) (source.Span, string, bool) {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		for index := len(c.scopes) - 1; index >= 0; index-- {
			symbol, exists := c.scopes[index][expression.Name]
			if !exists {
				continue
			}
			expression.ResolvedDeclaration = symbol.declarationSpan
			return symbol.declarationSpan, "", true
		}
		return source.Span{}, "", false
	case *ast.MemberExpr:
		if expression.Go || expression.GoField || !expression.Addressable || expression.ResolvedDeclaration.Start.Line == 0 {
			return source.Span{}, "", false
		}
		root, prefix, ok := c.stableMemberFlowPath(expression.Object)
		if !ok {
			return source.Span{}, "", false
		}
		field := expression.ResolvedDeclaration
		segment := field.Path + ":" + strconv.Itoa(field.Start.Offset) + ":" + strconv.Itoa(field.End.Offset)
		if prefix != "" {
			segment = prefix + "/" + segment
		}
		return root, segment, true
	default:
		return source.Span{}, "", false
	}
}

func (c *Checker) invalidateAllMemberFacts(span source.Span, cause string) {
	if c.suppressFlowEffects != 0 {
		return
	}
	for key, state := range c.memberFlow {
		if !state.nonNull {
			continue
		}
		state.nonNull = false
		state.invalidated = span
		state.invalidationCause = cause
		c.memberFlow[key] = state
	}
}

func (c *Checker) invalidateControlTransferFlow(span source.Span) {
	if c.suppressFlowEffects != 0 {
		return
	}
	for scopeIndex, scope := range c.scopes {
		for name, symbol := range scope {
			if symbol.constant || symbol.declaredType.Kind != Nullable {
				continue
			}
			symbol.typeInfo = symbol.declaredType
			symbol.flowInvalidated = span
			symbol.flowInvalidationCause = "an arbitrary control transfer"
			c.scopes[scopeIndex][name] = symbol
		}
	}
	c.invalidateAllMemberFacts(span, "an arbitrary control transfer")
}

func (c *Checker) invalidateMemberFactsRootedAt(identifier *ast.IdentifierExpr, span source.Span, cause string) {
	var declaration source.Span
	for index := len(c.scopes) - 1; index >= 0; index-- {
		if symbol, exists := c.scopes[index][identifier.Name]; exists {
			declaration = symbol.declarationSpan
			break
		}
	}
	if declaration.Start.Line == 0 {
		return
	}
	c.invalidateMemberFactsForDeclaration(declaration, span, cause)
}

func (c *Checker) invalidateMemberFactsForDeclaration(declaration, span source.Span, cause string) {
	if c.suppressFlowEffects != 0 {
		return
	}
	if len(c.capturedMemberRoots) != 0 && c.capturedMemberRoots[len(c.capturedMemberRoots)-1][declaration] {
		c.recordMemberWrite(span)
	}
	for key, state := range c.memberFlow {
		if key.root != declaration || !state.nonNull {
			continue
		}
		state.nonNull = false
		state.invalidated = span
		state.invalidationCause = cause
		c.memberFlow[key] = state
	}
}

func (c *Checker) recordMemberWrite(span source.Span) {
	if c.suppressFlowEffects != 0 || len(c.capturedMemberWrites) == 0 {
		return
	}
	index := len(c.capturedMemberWrites) - 1
	if c.capturedMemberWrites[index].Start.Line == 0 {
		c.capturedMemberWrites[index] = span
	}
}

func (c *Checker) invalidateMemberWriteTarget(target ast.Expression, span source.Span) {
	if _, ok := target.(*ast.MemberExpr); !ok {
		return
	}
	c.recordMemberWrite(span)
	c.invalidateAllMemberFacts(span, "a possibly aliased field update")
}

func (c *Checker) checkNew(expr *ast.NewExpr) Type {
	class, ok := c.classes[expr.ClassName]
	ok = ok && c.isTopLevelAllowed(expr.Span, expr.ClassName)
	if !ok {
		c.report(expr.Span, fmt.Sprintf("unknown class %q", expr.ClassName))
		for _, arg := range expr.Arguments {
			c.checkExpression(arg)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if expr.ClassName == "Exception" {
		c.usesExceptions = true
	}
	expr.ResolvedDeclaration = class.declarationSpan
	classType := c.resolveNativeClassType(ast.TypeRef{Name: expr.ClassName, GenericArguments: expr.TypeArguments, Span: expr.Span}, class)
	parameters := class.constructor
	if classType.Kind != Invalid {
		bindings := nativeClassBindings(class, classType)
		parameters = make([]Type, len(class.constructor))
		for index := range class.constructor {
			parameters[index] = substituteNativeTypeParameters(class.constructor[index], bindings)
		}
	}
	c.checkConstructorArguments(expr.Arguments, expr.Expanded, parameters, class.constructorVariadic, fmt.Sprintf("constructor %q", expr.ClassName), expr.Span)
	return classType
}

func (c *Checker) checkConstructorArguments(arguments []ast.Expression, expanded bool, parameters []Type, variadic bool, name string, span source.Span) {
	minimumArguments := len(parameters)
	if variadic {
		minimumArguments--
	}
	if expanded {
		if !variadic {
			c.report(span, fmt.Sprintf("%s is not variadic and cannot receive a spread argument", name))
		} else if len(arguments) != len(parameters) {
			c.report(span, fmt.Sprintf("spread call to %s expects %d arguments (%d fixed and one slice), got %d", name, len(parameters), minimumArguments, len(arguments)))
		}
	} else if len(arguments) < minimumArguments || (!variadic && len(arguments) != len(parameters)) {
		if variadic {
			c.report(span, fmt.Sprintf("%s expects at least %d arguments, got %d", name, minimumArguments, len(arguments)))
		} else {
			c.report(span, fmt.Sprintf("%s expects %d arguments, got %d", name, len(parameters), len(arguments)))
		}
	}
	for index, argument := range arguments {
		parameterIndex := index
		if variadic && parameterIndex >= len(parameters)-1 {
			parameterIndex = len(parameters) - 1
		}
		if parameterIndex < 0 || parameterIndex >= len(parameters) {
			c.checkExpression(argument)
			continue
		}
		expected := parameters[parameterIndex]
		if expanded && variadic && index == len(arguments)-1 {
			element := expected
			expected = Type{Kind: Array, Name: "array", Element: &element}
		}
		actual := c.checkExpressionExpectedSlot(&arguments[index], expected)
		c.requireAssignable(expected, actual, argument.GetSpan())
	}
}

func (c *Checker) checkIndex(expr *ast.IndexExpr, checked bool) Type {
	object := c.singleValue(c.checkExpression(expr.Object), expr.Object.GetSpan())
	if object.Kind == Nullable {
		c.report(expr.Object.GetSpan(), fmt.Sprintf("nullable value %s must be checked against null before indexing", object.String()))
		if object.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		object = *object.Element
	}
	index := c.singleValue(c.checkExpression(expr.Index), expr.Index.GetSpan())
	if object.Kind == Invalid {
		return object
	}
	switch object.Kind {
	case Array:
		if index.Kind != Invalid && !index.IsInteger() {
			c.report(expr.Index.GetSpan(), "array index must be an integer")
		}
		expr.Addressable = true
		expr.Assignable = true
		if object.Element != nil {
			return c.checkedIndexResult(expr, *object.Element, checked, false)
		}
	case Map:
		expr.Assignable = true
		if object.Key != nil {
			c.requireAssignable(*object.Key, index, expr.Index.GetSpan())
		}
		if object.Element != nil {
			return c.checkedIndexResult(expr, *object.Element, checked, true)
		}
	case String:
		if index.Kind != Invalid && !index.IsInteger() {
			c.report(expr.Index.GetSpan(), "string index must be an integer")
		}
		return c.checkedIndexResult(expr, builtins["byte"], checked, false)
	}
	goType, ok := goTypeOf(object)
	if !ok {
		c.report(expr.Object.GetSpan(), fmt.Sprintf("type %s cannot be indexed", object.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	underlying := gotypes.Unalias(goType).Underlying()
	if pointer, pointerOK := underlying.(*gotypes.Pointer); pointerOK {
		if array, arrayOK := gotypes.Unalias(pointer.Elem()).Underlying().(*gotypes.Array); arrayOK {
			if index.Kind != Invalid && !index.IsInteger() {
				c.report(expr.Index.GetSpan(), "array index must be an integer")
			}
			expr.Addressable = true
			expr.Assignable = true
			return c.checkedIndexResult(expr, c.collectionElementType(array.Elem(), object, expr.Span), checked, false)
		}
	}
	switch collection := underlying.(type) {
	case *gotypes.Array:
		if index.Kind != Invalid && !index.IsInteger() {
			c.report(expr.Index.GetSpan(), "array index must be an integer")
		}
		expr.Addressable = c.isAddressableExpression(expr.Object)
		expr.Assignable = expr.Addressable
		if object.Element != nil {
			return c.checkedIndexResult(expr, *object.Element, checked, false)
		}
		return c.checkedIndexResult(expr, c.collectionElementType(collection.Elem(), object, expr.Span), checked, false)
	case *gotypes.Slice:
		if index.Kind != Invalid && !index.IsInteger() {
			c.report(expr.Index.GetSpan(), "array index must be an integer")
		}
		expr.Addressable = true
		expr.Assignable = true
		if object.Element != nil {
			return c.checkedIndexResult(expr, *object.Element, checked, false)
		}
		return c.checkedIndexResult(expr, c.collectionElementType(collection.Elem(), object, expr.Span), checked, false)
	case *gotypes.Map:
		expr.Assignable = true
		key := c.collectionElementType(collection.Key(), object, expr.Span)
		if object.Key != nil {
			key = *object.Key
		}
		c.requireAssignable(key, index, expr.Index.GetSpan())
		if object.Element != nil {
			return c.checkedIndexResult(expr, *object.Element, checked, true)
		}
		return c.checkedIndexResult(expr, c.collectionElementType(collection.Elem(), object, expr.Span), checked, true)
	case *gotypes.Basic:
		if collection.Info()&gotypes.IsString != 0 {
			if index.Kind != Invalid && !index.IsInteger() {
				c.report(expr.Index.GetSpan(), "string index must be an integer")
			}
			return c.checkedIndexResult(expr, builtins["byte"], checked, false)
		}
	}
	c.report(expr.Object.GetSpan(), fmt.Sprintf("type %s cannot be indexed", object.String()))
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) checkedIndexResult(expr *ast.IndexExpr, element Type, checked, mapIndex bool) Type {
	if !checked {
		return element
	}
	if !mapIndex {
		c.report(expr.Span, "checked index binding requires a map operand")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return Type{Kind: MultiValue, Name: "checked map lookup", Results: []Type{element, builtins["boolean"]}}
}

func (c *Checker) checkSlice(expr *ast.SliceExpr) Type {
	object := c.singleValue(c.checkExpression(expr.Object), expr.Object.GetSpan())
	if object.Kind == Nullable {
		c.report(expr.Object.GetSpan(), fmt.Sprintf("nullable value %s must be checked against null before slicing", object.String()))
		if object.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		object = *object.Element
	}
	for _, bound := range []struct {
		name       string
		expression ast.Expression
	}{{"low", expr.Low}, {"high", expr.High}, {"max", expr.Max}} {
		if bound.expression == nil {
			continue
		}
		value := c.singleValue(c.checkExpression(bound.expression), bound.expression.GetSpan())
		if value.Kind != Invalid && !value.IsInteger() {
			c.report(bound.expression.GetSpan(), fmt.Sprintf("slice %s bound must be an integer, got %s", bound.name, value.String()))
		}
		if constant, known := integerConstantValue(bound.expression); known {
			if constant.Sign() < 0 {
				c.report(bound.expression.GetSpan(), fmt.Sprintf("slice %s bound cannot be negative", bound.name))
			} else if !constant.IsInt64() {
				c.report(bound.expression.GetSpan(), fmt.Sprintf("slice %s bound is out of range", bound.name))
			}
		}
	}
	if object.Kind == Invalid {
		return object
	}
	if object.Kind == Array {
		c.checkSliceConstantBounds(expr, -1)
		return object
	}
	if object.Kind == String {
		if expr.Full {
			c.report(expr.Span, "3-index slice cannot be used with string")
		}
		c.checkSliceConstantBounds(expr, -1)
		return object
	}
	goType, ok := goTypeOf(object)
	if !ok {
		c.report(expr.Object.GetSpan(), fmt.Sprintf("type %s cannot be sliced", object.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	underlying := gotypes.Unalias(goType).Underlying()
	var fixedLength int64 = -1
	if pointer, pointerOK := underlying.(*gotypes.Pointer); pointerOK {
		array, arrayOK := gotypes.Unalias(pointer.Elem()).Underlying().(*gotypes.Array)
		if !arrayOK {
			c.report(expr.Object.GetSpan(), fmt.Sprintf("type %s cannot be sliced", object.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		fixedLength = array.Len()
		element := c.collectionElementType(array.Elem(), object, expr.Span)
		c.checkSliceConstantBounds(expr, fixedLength)
		return Type{Kind: Array, Name: "array", Element: &element}
	}
	switch collection := underlying.(type) {
	case *gotypes.Array:
		if !c.isAddressableExpression(expr.Object) {
			c.report(expr.Object.GetSpan(), fmt.Sprintf("slicing fixed array %s requires an addressable operand", object.String()))
		}
		fixedLength = collection.Len()
		element := c.collectionElementType(collection.Elem(), object, expr.Span)
		c.checkSliceConstantBounds(expr, fixedLength)
		return Type{Kind: Array, Name: "array", Element: &element}
	case *gotypes.Slice:
		c.checkSliceConstantBounds(expr, fixedLength)
		return object
	case *gotypes.Basic:
		if collection.Info()&gotypes.IsString != 0 {
			if expr.Full {
				c.report(expr.Span, "3-index slice cannot be used with string")
			}
			c.checkSliceConstantBounds(expr, fixedLength)
			return object
		}
	}
	c.report(expr.Object.GetSpan(), fmt.Sprintf("type %s cannot be sliced", object.String()))
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) collectionElementType(goType gotypes.Type, owner Type, span source.Span) Type {
	element, err := ontamaTypeFromGo(goType)
	if err != nil {
		c.report(span, fmt.Sprintf("collection element type is not supported: %v", err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	inheritGoQualifier(&element, owner)
	return element
}

func (c *Checker) checkSliceConstantBounds(expr *ast.SliceExpr, fixedLength int64) {
	constant := func(expression ast.Expression) (*big.Int, bool) {
		if expression == nil {
			return nil, false
		}
		value, ok := integerConstantValue(expression)
		return value, ok && value.Sign() >= 0
	}
	low, lowOK := constant(expr.Low)
	high, highOK := constant(expr.High)
	max, maxOK := constant(expr.Max)
	if lowOK && highOK && low.Cmp(high) > 0 {
		c.report(expr.Span, "slice bounds are out of order: low exceeds high")
	}
	if highOK && maxOK && high.Cmp(max) > 0 {
		c.report(expr.Span, "slice bounds are out of order: high exceeds max")
	}
	if fixedLength < 0 {
		return
	}
	limit := big.NewInt(fixedLength)
	for _, bound := range []struct {
		name  string
		value *big.Int
		known bool
	}{{"low", low, lowOK}, {"high", high, highOK}, {"max", max, maxOK}} {
		if bound.known && bound.value.Cmp(limit) > 0 {
			c.report(expr.Span, fmt.Sprintf("slice %s bound %s exceeds fixed array length %d", bound.name, bound.value.String(), fixedLength))
		}
	}
}

func (c *Checker) checkBinary(expr *ast.BinaryExpr) Type {
	var left, right Type
	_, leftIsArrayLiteral := expr.Left.(*ast.ArrayLiteralExpr)
	_, rightIsArrayLiteral := expr.Right.(*ast.ArrayLiteralExpr)
	comparison := expr.Operator == "==" || expr.Operator == "===" || expr.Operator == "!=" || expr.Operator == "!=="
	if comparison && leftIsArrayLiteral && !rightIsArrayLiteral {
		right = c.singleValue(c.checkExpression(expr.Right), expr.Right.GetSpan())
		left = c.singleValue(c.checkExpressionExpected(expr.Left, right), expr.Left.GetSpan())
	} else {
		left = c.singleValue(c.checkExpression(expr.Left), expr.Left.GetSpan())
		if comparison && rightIsArrayLiteral {
			right = c.singleValue(c.checkExpressionExpected(expr.Right, left), expr.Right.GetSpan())
		} else {
			right = c.singleValue(c.checkExpression(expr.Right), expr.Right.GetSpan())
		}
	}
	return c.checkBinaryOperands(expr, left, right)
}

func (c *Checker) checkBinaryOperands(expr *ast.BinaryExpr, left, right Type) Type {
	switch expr.Operator {
	case "+", "-", "*", "/", "%":
		if expr.Operator == "+" && left.IsAddable() && right.IsAddable() && sameType(left, right) && (!left.IsNumeric() || !right.IsNumeric()) {
			return left
		}
		if !left.IsNumeric() || !right.IsNumeric() {
			if left.Kind != Invalid && right.Kind != Invalid {
				c.report(expr.Span, fmt.Sprintf("operator %s requires numeric operands", expr.Operator))
			}
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if expr.Operator == "%" && (!left.IsInteger() || !right.IsInteger()) {
			c.report(expr.Span, "operator % requires integer operands")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if (expr.Operator == "/" || expr.Operator == "%") && left.IsInteger() && right.IsInteger() {
			if divisor, known := c.resolvedIntegerConstantValue(expr.Right); known && divisor.Sign() == 0 {
				c.report(expr.Right.GetSpan(), fmt.Sprintf("operator %s integer divisor cannot be zero", expr.Operator))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
		}
		if !sameType(left, right) {
			c.report(expr.Span, fmt.Sprintf("operator %s cannot mix %s and %s without an explicit conversion", expr.Operator, left.Name, right.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if right.Kind == UntypedInt && left.Kind != UntypedInt {
			if value, known := c.resolvedIntegerConstantValue(expr.Right); known && !integerConstantFitsFixedType(value, left) {
				c.report(expr.Right.GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", value.String(), left.String()))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
		}
		if left.Kind == UntypedInt {
			return right
		}
		return left
	case "&", "|", "^", "&^":
		if !left.IsInteger() || !right.IsInteger() {
			if left.Kind != Invalid && right.Kind != Invalid {
				c.report(expr.Span, fmt.Sprintf("operator %s requires integer operands", expr.Operator))
			}
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if !sameType(left, right) {
			c.report(expr.Span, fmt.Sprintf("operator %s cannot mix %s and %s without an explicit conversion", expr.Operator, left.Name, right.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if left.Kind == UntypedInt && right.Kind != UntypedInt {
			if value, known := c.resolvedIntegerConstantValue(expr.Left); known && !integerConstantFitsFixedType(value, right) {
				c.report(expr.Left.GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", value.String(), right.String()))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
		}
		if right.Kind == UntypedInt && left.Kind != UntypedInt {
			if value, known := c.resolvedIntegerConstantValue(expr.Right); known && !integerConstantFitsFixedType(value, left) {
				c.report(expr.Right.GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", value.String(), left.String()))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
		}
		if left.Kind == UntypedInt {
			return right
		}
		return left
	case "<<", ">>":
		if !left.IsInteger() || !right.IsInteger() {
			if left.Kind != Invalid && right.Kind != Invalid {
				c.report(expr.Span, fmt.Sprintf("operator %s requires integer operands", expr.Operator))
			}
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if amount, known := c.resolvedIntegerConstantValue(expr.Right); known && amount.Sign() < 0 {
			c.report(expr.Right.GetSpan(), fmt.Sprintf("operator %s shift amount cannot be negative", expr.Operator))
			return Type{Kind: Invalid, Name: "<invalid>"}
		} else if known && c.integerExpressionIsCompileTimeConstant(expr.Left) && (!amount.IsUint64() || amount.Uint64() > maximumGoConstantShift) {
			c.report(expr.Right.GetSpan(), fmt.Sprintf("operator %s constant shift amount exceeds the Go implementation limit of %d", expr.Operator, maximumGoConstantShift))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return left
	case "<", "<=", ">", ">=":
		if !left.IsOrdered() || !right.IsOrdered() || !sameType(left, right) {
			if left.Kind != Invalid && right.Kind != Invalid {
				c.report(expr.Span, fmt.Sprintf("operator %s requires operands of the same ordered type", expr.Operator))
			}
		}
		return builtins["boolean"]
	case "==", "===", "!=", "!==":
		if left.Kind == Nil && right.Kind == Nil {
			c.report(expr.Span, "cannot compare nil with nil without a concrete nilable type")
		} else if left.Kind == Null && right.Kind == Null {
			c.report(expr.Span, "cannot compare null with null without a concrete nullable type")
		} else if !sameType(left, right) && left.Kind != Invalid && right.Kind != Invalid {
			c.report(expr.Span, fmt.Sprintf("cannot compare %s and %s", left.String(), right.String()))
		} else if left.Kind != Nil && right.Kind != Nil && left.Kind != Null && right.Kind != Null && (!left.IsComparable() || !right.IsComparable()) {
			c.report(expr.Span, fmt.Sprintf("type %s is not comparable", left.String()))
		}
		return builtins["boolean"]
	case "&&", "||":
		if (left.Kind != Boolean || right.Kind != Boolean) && left.Kind != Invalid && right.Kind != Invalid {
			c.report(expr.Span, fmt.Sprintf("operator %s requires boolean operands", expr.Operator))
		}
		return builtins["boolean"]
	}
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) checkCall(expr *ast.CallExpr) Type {
	defer func() {
		if !expr.Conversion && expr.Builtin == ast.NotBuiltinCall {
			c.recordMemberWrite(expr.Span)
			c.invalidateAllMemberFacts(expr.Span, "a call with unknown mutation effects")
		}
	}()
	name, ok := expr.Callee.(*ast.IdentifierExpr)
	if ok {
		if name.Name == "super" {
			return c.checkSuperConstructorCall(expr)
		}
		switch name.Name {
		case "goChannel":
			return c.checkGoChannelMake(expr)
		case "closeGoChannel":
			return c.checkGoChannelClose(expr)
		}
		if !c.hasCallBinding(name.Name, name.Span) {
			switch name.Name {
			case "ok":
				return c.checkResultConstructor(expr, true)
			case "fail":
				return c.checkResultConstructor(expr, false)
			case "len":
				return c.checkCollectionLen(expr)
			case "cap":
				return c.checkCollectionCap(expr)
			case "append":
				return c.checkCollectionAppend(expr)
			case "copy":
				return c.checkCollectionCopy(expr)
			case "delete":
				return c.checkCollectionDelete(expr)
			case "clear":
				return c.checkCollectionClear(expr)
			case "min", "max":
				return c.checkOrderedBuiltin(expr, name.Name)
			case "makeSlice":
				return c.checkMakeSlice(expr)
			case "makeMap":
				return c.checkMakeMap(expr)
			case "copyArray":
				return c.checkSliceToArray(expr, false)
			case "viewArray":
				return c.checkSliceToArray(expr, true)
			}
		}
	}
	if result, unsafeBuiltin := c.checkUnsafeBuiltinCall(expr); unsafeBuiltin {
		return result
	}
	if ok {
		if _, shadowed := c.lookupValue(name.Name, name.Span); !shadowed {
			if named, exists := c.nativeTypes[name.Name]; exists && c.isTopLevelAllowed(name.Span, name.Name) {
				name.ResolvedDeclaration = named.declaration.NameSpan
				expr.Conversion = true
				targetRef := ast.TypeRef{Name: name.Name, NameSpan: name.Span, GenericArguments: expr.TypeArguments, Span: expr.Span}
				if named.declaration.Alias && len(named.typeParameters) != 0 && len(expr.TypeArguments) == len(named.typeParameters) {
					expanded := instantiateGenericAliasTypeRef(targetRef, named.declaration)
					expr.ConversionType = &expanded
				}
				return c.checkNativeTypeConversion(expr, c.resolveNativeDefinedType(targetRef, named))
			}
		}
		if target, isConversion := LookupType(name.Name); isConversion && target.Kind != Void {
			expr.Conversion = true
			if expr.Expanded {
				c.report(expr.Span, "spread arguments cannot be used in type conversions")
			}
			if len(expr.Arguments) != 1 {
				c.report(expr.Span, fmt.Sprintf("conversion to %s expects 1 argument, got %d", name.Name, len(expr.Arguments)))
				return target
			}
			value := c.checkExpression(expr.Arguments[0])
			targetGo, targetRepresentable := goTypeOf(target)
			valueGo, valueRepresentable := goTypeOf(value)
			convertible := targetRepresentable && valueRepresentable && value.Kind != Nullable && gotypes.ConvertibleTo(valueGo, targetGo)
			if !convertible {
				c.report(expr.Span, fmt.Sprintf("cannot convert %s to %s", value.String(), target.String()))
			} else if target.IsNumeric() {
				integer, known := c.resolvedIntegerConstantValue(expr.Arguments[0])
				if known && !integerConstantFitsFixedType(integer, target) {
					c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", integer.String(), target.String()))
				}
			}
			return target
		}
	}
	var callable Type
	var callableName string
	if ok {
		if fn, exists := c.functions[name.Name]; exists && c.isTopLevelAllowed(name.Span, name.Name) {
			name.ResolvedDeclaration = fn.declarationSpan
			callable = callableTypeForFunction(fn)
			callableName = fmt.Sprintf("function %q", name.Name)
		} else if symbol, exists := c.lookupSymbol(name.Name, name.Span); exists {
			name.ResolvedDeclaration = symbol.declarationSpan
			callable = symbol.typeInfo
			callableName = fmt.Sprintf("value %q", name.Name)
		} else {
			c.report(name.Span, fmt.Sprintf("undefined function %q", name.Name))
		}
	} else {
		callable = c.checkExpression(expr.Callee)
		callableName = "expression"
	}
	if callable.Kind == Invalid {
		for _, arg := range expr.Arguments {
			c.checkExpression(arg)
		}
		return callable
	}
	if callable.Kind == GoTypeName {
		expr.Conversion = true
		return c.checkGoConversion(expr, callable)
	}
	if callable.Kind == GoNamed && callable.GoType != nil {
		if converted, err := ontamaTypeFromGo(callable.GoType); err == nil && converted.Kind == Function {
			callable = converted
		}
	}
	if callable.Kind == Nullable {
		c.report(expr.Callee.GetSpan(), fmt.Sprintf("nullable callable %s must be checked against null before calling", callable.String()))
		if callable.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		callable = *callable.Element
	}
	if callable.Kind != Function || callable.Result == nil {
		c.report(expr.Callee.GetSpan(), fmt.Sprintf("%s is not callable", callableName))
		for _, arg := range expr.Arguments {
			c.checkExpression(arg)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	c.recordCallSignature(expr, callable)
	if callable.Generic && len(callable.TypeParameters) != 0 {
		return c.checkNativeGenericCall(expr, callableName, callable)
	}
	if len(expr.TypeArguments) != 0 {
		if !callable.Generic {
			c.report(expr.Callee.GetSpan(), fmt.Sprintf("%s is not a generic Go function and cannot receive explicit type arguments", callableName))
			for _, argument := range expr.Arguments {
				c.checkExpression(argument)
			}
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return c.checkExplicitGenericCall(expr, callableName, callable)
	} else if callable.Generic {
		return c.checkInferredGenericCall(expr, callableName, callable)
	}
	if expr.Expanded {
		if !callable.Variadic || len(callable.Parameters) == 0 {
			c.report(expr.Span, fmt.Sprintf("%s is not variadic and cannot receive a spread argument", callableName))
			for _, argument := range expr.Arguments {
				c.checkExpression(argument)
			}
			return *callable.Result
		}
		expectedArguments := len(callable.Parameters)
		if len(expr.Arguments) != expectedArguments {
			c.report(expr.Span, fmt.Sprintf("spread call to %s expects %d arguments (%d fixed and one slice), got %d", callableName, expectedArguments, expectedArguments-1, len(expr.Arguments)))
		}
		fixedArguments := len(callable.Parameters) - 1
		for i, argument := range expr.Arguments {
			if i == len(expr.Arguments)-1 {
				element := callable.Parameters[len(callable.Parameters)-1]
				expected := Type{Kind: Array, Name: "array", Element: &element}
				actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], expected)
				c.requireAssignable(expected, actual, argument.GetSpan())
				continue
			}
			if i < fixedArguments {
				expected := callable.Parameters[i]
				actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], expected)
				c.requireAssignable(expected, actual, argument.GetSpan())
			} else {
				expected := callable.Parameters[len(callable.Parameters)-1]
				actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], expected)
				c.requireAssignable(expected, actual, argument.GetSpan())
			}
		}
		return *callable.Result
	}
	minimumArguments := len(callable.Parameters)
	if callable.Variadic {
		minimumArguments--
	}
	if len(expr.Arguments) < minimumArguments || (!callable.Variadic && len(expr.Arguments) != len(callable.Parameters)) {
		if callable.Variadic {
			c.report(expr.Span, fmt.Sprintf("%s expects at least %d arguments, got %d", callableName, minimumArguments, len(expr.Arguments)))
		} else {
			c.report(expr.Span, fmt.Sprintf("%s expects %d arguments, got %d", callableName, len(callable.Parameters), len(expr.Arguments)))
		}
	}
	for i, arg := range expr.Arguments {
		parameterIndex := i
		if callable.Variadic && parameterIndex >= len(callable.Parameters)-1 {
			parameterIndex = len(callable.Parameters) - 1
		}
		if parameterIndex >= 0 && parameterIndex < len(callable.Parameters) {
			expected := callable.Parameters[parameterIndex]
			actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], expected)
			c.requireAssignable(expected, actual, arg.GetSpan())
		} else {
			c.checkExpression(arg)
		}
	}
	return *callable.Result
}

func (c *Checker) checkSuperConstructorCall(expr *ast.CallExpr) Type {
	class := c.classes[c.currentClass]
	if !c.inConstructor || class == nil || class.base == "" {
		c.report(expr.Span, "super(...) may only be called from a derived-class constructor")
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	base := c.classes[class.base]
	if base == nil {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	bindings := nativeClassBindings(base, class.baseType)
	parameters := make([]Type, len(base.constructor))
	for index, parameter := range base.constructor {
		parameters[index] = substituteNativeTypeParameters(parameter, bindings)
	}
	c.checkConstructorArguments(expr.Arguments, expr.Expanded, parameters, base.constructorVariadic, fmt.Sprintf("base constructor %q", class.base), expr.Span)
	expr.SuperConstructor = true
	expr.SuperBase = class.base
	return builtins["void"]
}

func (c *Checker) recordCallSignature(expr *ast.CallExpr, callable Type) {
	if callable.Kind != Function || callable.Result == nil {
		return
	}
	signature := &ast.CallableSignature{
		ParameterNames: make([]string, len(callable.Parameters)),
		ParameterTypes: make([]string, len(callable.Parameters)),
		Result:         callable.Result.String(),
		Variadic:       callable.Variadic,
	}
	for index, parameter := range callable.Parameters {
		signature.ParameterTypes[index] = parameter.String()
	}
	if goSignature, ok := callable.GoType.(*gotypes.Signature); ok {
		for index := 0; index < goSignature.Params().Len() && index < len(signature.ParameterNames); index++ {
			signature.ParameterNames[index] = goSignature.Params().At(index).Name()
		}
	}
	expr.Signature = signature
}

func (c *Checker) checkUnsafeBuiltinCall(expr *ast.CallExpr) (Type, bool) {
	member, ok := expr.Callee.(*ast.MemberExpr)
	if !ok {
		return Type{}, false
	}
	identifier, ok := member.Object.(*ast.IdentifierExpr)
	if !ok {
		return Type{}, false
	}
	if _, shadowed := c.lookupValue(identifier.Name, identifier.Span); shadowed {
		return Type{}, false
	}
	imported := c.lookupGoPackage(identifier.Span.Path, identifier.Name)
	if imported == nil || imported.path != "unsafe" {
		return Type{}, false
	}
	identifier.ResolvedDeclaration = imported.declaration.AliasSpan
	kinds := map[string]ast.BuiltinCallKind{
		"Sizeof": ast.UnsafeSizeofCall, "Alignof": ast.UnsafeAlignofCall, "Offsetof": ast.UnsafeOffsetofCall,
		"Add": ast.UnsafeAddCall, "Slice": ast.UnsafeSliceCall, "SliceData": ast.UnsafeSliceDataCall,
		"String": ast.UnsafeStringCall, "StringData": ast.UnsafeStringDataCall,
	}
	kind, ok := kinds[member.Name]
	if !ok {
		return Type{}, false
	}

	expr.Builtin = kind
	member.Go = true
	member.ResolvedName = member.Name
	imported.declaration.Used = true
	qualifiedName := identifier.Name + "." + member.Name
	wantArguments := 1
	if member.Name == "Add" || member.Name == "Slice" || member.Name == "String" {
		wantArguments = 2
	}
	c.checkBuiltinCallShape(expr, qualifiedName, wantArguments, wantArguments, 0)
	arguments := make([]Type, len(expr.Arguments))
	for index, argument := range expr.Arguments {
		arguments[index] = c.singleValue(c.checkExpression(argument), argument.GetSpan())
	}

	uintptrType := Type{Kind: GoBasic, Name: "uintptr", GoType: gotypes.Typ[gotypes.Uintptr]}
	unsafePointer := Type{Kind: GoBasic, Name: "unsafe.Pointer", GoType: gotypes.Typ[gotypes.UnsafePointer], GoQualifier: resolvedGoPackageAlias(imported)}
	invalid := Type{Kind: Invalid, Name: "<invalid>"}
	argument := func(index int) (Type, bool) {
		if index >= len(arguments) {
			return invalid, false
		}
		return arguments[index], true
	}

	switch member.Name {
	case "Sizeof", "Alignof":
		value, exists := argument(0)
		if exists && value.Kind == Nil {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s requires a typed value, got nil", qualifiedName))
		} else if exists && value.Kind == Void {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s requires a value, got void", qualifiedName))
		}
		return uintptrType, true
	case "Offsetof":
		_, exists := argument(0)
		if exists {
			field, fieldOK := expr.Arguments[0].(*ast.MemberExpr)
			if !fieldOK || !field.GoField {
				c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s requires a Go struct field selector", qualifiedName))
			} else if field.GoFieldViaPointer {
				c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s field cannot be embedded through a pointer", qualifiedName))
			}
		}
		return uintptrType, true
	case "Add":
		if pointer, exists := argument(0); exists && pointer.Kind != Invalid && pointer.Kind != Nil && !isUnsafePointer(pointer) {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s pointer must be unsafe.Pointer, got %s", qualifiedName, pointer.String()))
		}
		if length, exists := argument(1); exists {
			c.checkUnsafeIntegerArgument(qualifiedName, "offset", expr.Arguments[1], length, false)
		}
		return unsafePointer, true
	case "Slice":
		pointer, exists := argument(0)
		if !exists || pointer.Kind == Invalid {
			return invalid, true
		}
		pointerGo, representable := goTypeOf(pointer)
		var selected *gotypes.Pointer
		if representable {
			selected, _ = gotypes.Unalias(pointerGo).Underlying().(*gotypes.Pointer)
		}
		if selected == nil {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s pointer must be a typed Go pointer, got %s", qualifiedName, pointer.String()))
			return invalid, true
		}
		if length, exists := argument(1); exists {
			c.checkUnsafeIntegerArgument(qualifiedName, "length", expr.Arguments[1], length, true)
		}
		element := c.collectionElementType(selected.Elem(), pointer, expr.Span)
		return Type{Kind: Array, Name: "array", Element: &element}, true
	case "SliceData":
		value, exists := argument(0)
		if !exists || value.Kind == Invalid {
			return invalid, true
		}
		valueGo, representable := goTypeOf(value)
		if !representable {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s argument must be a slice, got %s", qualifiedName, value.String()))
			return invalid, true
		}
		slice, sliceOK := gotypes.Unalias(valueGo).Underlying().(*gotypes.Slice)
		if !sliceOK {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s argument must be a slice, got %s", qualifiedName, value.String()))
			return invalid, true
		}
		element := c.collectionElementType(slice.Elem(), value, expr.Span)
		elementGo, _ := goTypeOf(element)
		return Type{Kind: GoPointer, Name: "*" + element.String(), Element: &element, GoType: gotypes.NewPointer(elementGo), GoQualifier: element.GoQualifier}, true
	case "String":
		if pointer, exists := argument(0); exists && pointer.Kind != Invalid && pointer.Kind != Nil && !isGoBytePointer(pointer) {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s pointer must be *byte, got %s", qualifiedName, pointer.String()))
		}
		if length, exists := argument(1); exists {
			c.checkUnsafeIntegerArgument(qualifiedName, "length", expr.Arguments[1], length, true)
		}
		return builtins["string"], true
	case "StringData":
		if value, exists := argument(0); exists && value.Kind != Invalid && !isGoString(value) {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s argument must be a string, got %s", qualifiedName, value.String()))
		}
		byteType := builtins["byte"]
		return Type{Kind: GoPointer, Name: "*byte", Element: &byteType, GoType: gotypes.NewPointer(gotypes.Typ[gotypes.Uint8])}, true
	default:
		return invalid, true
	}
}

func resolvedGoPackageAlias(imported *goPackageSymbol) string {
	if imported == nil || imported.declaration == nil {
		return ""
	}
	if imported.declaration.ResolvedAlias != "" {
		return imported.declaration.ResolvedAlias
	}
	return imported.declaration.Alias
}

func isUnsafePointer(value Type) bool {
	goType, ok := goTypeOf(value)
	return ok && gotypes.AssignableTo(goType, gotypes.Typ[gotypes.UnsafePointer])
}

func isGoBytePointer(value Type) bool {
	goType, ok := goTypeOf(value)
	return ok && gotypes.AssignableTo(goType, gotypes.NewPointer(gotypes.Typ[gotypes.Uint8]))
}

func isGoString(value Type) bool {
	goType, ok := goTypeOf(value)
	return ok && gotypes.AssignableTo(goType, gotypes.Typ[gotypes.String])
}

func (c *Checker) checkUnsafeIntegerArgument(name, role string, expression ast.Expression, value Type, nonnegative bool) {
	if value.Kind != Invalid && !value.IsInteger() {
		c.report(expression.GetSpan(), fmt.Sprintf("%s %s must be an integer, got %s", name, role, value.String()))
		return
	}
	if nonnegative {
		if constant, known := integerConstantValue(expression); known && constant.Sign() < 0 {
			c.report(expression.GetSpan(), fmt.Sprintf("%s %s cannot be negative", name, role))
		} else if known && !constant.IsInt64() {
			c.report(expression.GetSpan(), fmt.Sprintf("%s %s is out of range", name, role))
		}
	} else if constant, known := integerConstantValue(expression); known && !constant.IsInt64() {
		c.report(expression.GetSpan(), fmt.Sprintf("%s %s is out of range", name, role))
	}
}

func (c *Checker) checkGoChannelMake(expr *ast.CallExpr) Type {
	expr.Builtin = ast.MakeGoChannelCall
	if expr.Expanded {
		c.report(expr.Span, "goChannel does not accept spread arguments")
	}
	if len(expr.TypeArguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("goChannel expects one type argument, got %d", len(expr.TypeArguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	element := c.resolveType(expr.TypeArguments[0])
	elementGoType, ok := goTypeOf(element)
	if !ok {
		c.report(expr.TypeArguments[0].Span, fmt.Sprintf("type %s cannot be used as a Go channel element", element.String()))
	}
	if len(expr.Arguments) > 1 {
		c.report(expr.Span, fmt.Sprintf("goChannel expects zero or one capacity argument, got %d", len(expr.Arguments)))
	}
	for _, argument := range expr.Arguments {
		capacity := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if capacity.Kind != Invalid && !capacity.IsInteger() {
			c.report(argument.GetSpan(), fmt.Sprintf("goChannel capacity must be an integer, got %s", capacity.String()))
		}
		if constant, known := integerConstantValue(argument); known && constant.Sign() < 0 {
			c.report(argument.GetSpan(), "goChannel capacity cannot be negative")
		}
	}
	if !ok || element.Kind == Invalid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return Type{Kind: GoChannel, Name: "GoChannel", Element: &element, GoType: gotypes.NewChan(gotypes.SendRecv, elementGoType), GoQualifier: element.GoQualifier}
}

func (c *Checker) checkGoChannelClose(expr *ast.CallExpr) Type {
	expr.Builtin = ast.CloseGoChannelCall
	if len(expr.TypeArguments) != 0 {
		c.report(expr.Span, "closeGoChannel does not accept type arguments")
	}
	if expr.Expanded {
		c.report(expr.Span, "closeGoChannel does not accept spread arguments")
	}
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("closeGoChannel expects one channel argument, got %d", len(expr.Arguments)))
	}
	for _, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if value.Kind == Nullable {
			c.report(argument.GetSpan(), fmt.Sprintf("nullable channel %s must be checked against null before closing", value.String()))
			if value.Element == nil {
				continue
			}
			value = *value.Element
		}
		goType, ok := goTypeOf(value)
		if !ok {
			c.report(argument.GetSpan(), fmt.Sprintf("closeGoChannel requires a Go channel, got %s", value.String()))
			continue
		}
		channel, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Chan)
		if !ok {
			c.report(argument.GetSpan(), fmt.Sprintf("closeGoChannel requires a Go channel, got %s", value.String()))
			continue
		}
		if channel.Dir() == gotypes.RecvOnly {
			c.report(argument.GetSpan(), fmt.Sprintf("cannot close receive-only channel %s", value.String()))
		}
	}
	return builtins["void"]
}

func (c *Checker) checkCollectionLen(expr *ast.CallExpr) Type {
	expr.Builtin = ast.LenCall
	c.checkBuiltinCallShape(expr, "len", 1, 1, 0)
	for _, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if value.Kind != Invalid && !isLenCollection(value) {
			c.report(argument.GetSpan(), fmt.Sprintf("len requires a string, array, array pointer, slice, map, or channel, got %s", value.String()))
		}
	}
	return builtins["int"]
}

func (c *Checker) checkCollectionCap(expr *ast.CallExpr) Type {
	expr.Builtin = ast.CapCall
	c.checkBuiltinCallShape(expr, "cap", 1, 1, 0)
	for _, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if value.Kind != Invalid && !isCapCollection(value) {
			c.report(argument.GetSpan(), fmt.Sprintf("cap requires an array, array pointer, slice, or channel, got %s", value.String()))
		}
	}
	return builtins["int"]
}

func (c *Checker) checkCollectionAppend(expr *ast.CallExpr) Type {
	expr.Builtin = ast.AppendCall
	if len(expr.TypeArguments) != 0 {
		c.report(expr.Span, "append does not accept type arguments")
	}
	if len(expr.Arguments) == 0 {
		c.report(expr.Span, "append expects a destination slice")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	destination := c.singleValue(c.checkExpression(expr.Arguments[0]), expr.Arguments[0].GetSpan())
	element, ok := c.sliceElementType(destination, expr.Arguments[0].GetSpan())
	if !ok {
		if destination.Kind != Invalid {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("append requires a slice as its first argument, got %s", destination.String()))
		}
		for _, argument := range expr.Arguments[1:] {
			c.checkExpression(argument)
		}
		return destination
	}
	if expr.Expanded {
		if len(expr.Arguments) != 2 {
			c.report(expr.Span, fmt.Sprintf("spread append expects a destination and one expanded source, got %d arguments", len(expr.Arguments)))
		}
		if len(expr.Arguments) >= 2 {
			source := c.singleValue(c.checkExpression(expr.Arguments[1]), expr.Arguments[1].GetSpan())
			if !(source.IsString() && isBuiltinByte(element)) {
				sourceElement, sourceOK := c.sliceElementType(source, expr.Arguments[1].GetSpan())
				if !sourceOK {
					if source.Kind != Invalid {
						c.report(expr.Arguments[1].GetSpan(), fmt.Sprintf("expanded append source must be a compatible slice, got %s", source.String()))
					}
				} else if !identicalCollectionElement(element, sourceElement) {
					c.report(expr.Arguments[1].GetSpan(), fmt.Sprintf("expanded append source element %s does not match destination element %s", sourceElement.String(), element.String()))
				}
			}
		}
		for _, argument := range expr.Arguments[2:] {
			c.checkExpression(argument)
		}
		return destination
	}
	for _, argument := range expr.Arguments[1:] {
		actual := c.checkExpressionExpected(argument, element)
		c.requireAssignable(element, actual, argument.GetSpan())
	}
	return destination
}

func (c *Checker) checkCollectionCopy(expr *ast.CallExpr) Type {
	expr.Builtin = ast.CopyCall
	c.checkBuiltinCallShape(expr, "copy", 2, 2, 0)
	values := make([]Type, len(expr.Arguments))
	for i, argument := range expr.Arguments {
		values[i] = c.singleValue(c.checkExpression(argument), argument.GetSpan())
	}
	if len(values) < 2 {
		return builtins["int"]
	}
	destinationElement, destinationOK := c.sliceElementType(values[0], expr.Arguments[0].GetSpan())
	if !destinationOK {
		if values[0].Kind != Invalid {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("copy destination must be a slice, got %s", values[0].String()))
		}
		return builtins["int"]
	}
	if values[1].IsString() && isBuiltinByte(destinationElement) {
		return builtins["int"]
	}
	sourceElement, sourceOK := c.sliceElementType(values[1], expr.Arguments[1].GetSpan())
	if !sourceOK {
		if values[1].Kind != Invalid {
			c.report(expr.Arguments[1].GetSpan(), fmt.Sprintf("copy source must be a compatible slice or string for byte destinations, got %s", values[1].String()))
		}
	} else if !identicalCollectionElement(destinationElement, sourceElement) {
		c.report(expr.Arguments[1].GetSpan(), fmt.Sprintf("copy source element %s does not match destination element %s", sourceElement.String(), destinationElement.String()))
	}
	return builtins["int"]
}

func (c *Checker) checkCollectionDelete(expr *ast.CallExpr) Type {
	expr.Builtin = ast.DeleteCall
	c.checkBuiltinCallShape(expr, "delete", 2, 2, 0)
	values := make([]Type, len(expr.Arguments))
	for i, argument := range expr.Arguments {
		values[i] = c.singleValue(c.checkExpression(argument), argument.GetSpan())
	}
	if len(values) < 2 {
		return builtins["void"]
	}
	key, _, ok := c.mapCollectionTypes(values[0], expr.Arguments[0].GetSpan())
	if !ok {
		if values[0].Kind != Invalid {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("delete requires a map as its first argument, got %s", values[0].String()))
		}
		return builtins["void"]
	}
	c.requireAssignable(key, values[1], expr.Arguments[1].GetSpan())
	return builtins["void"]
}

func (c *Checker) checkCollectionClear(expr *ast.CallExpr) Type {
	expr.Builtin = ast.ClearCall
	c.checkBuiltinCallShape(expr, "clear", 1, 1, 0)
	for _, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if value.Kind != Invalid && !isClearCollection(value) {
			c.report(argument.GetSpan(), fmt.Sprintf("clear requires a slice or map, got %s", value.String()))
		}
	}
	return builtins["void"]
}

func (c *Checker) checkOrderedBuiltin(expr *ast.CallExpr, name string) Type {
	if name == "min" {
		expr.Builtin = ast.MinCall
	} else {
		expr.Builtin = ast.MaxCall
	}
	if len(expr.TypeArguments) != 0 {
		c.report(expr.Span, fmt.Sprintf("%s does not accept type arguments", name))
	}
	if expr.Expanded {
		c.report(expr.Span, fmt.Sprintf("%s does not accept spread arguments", name))
	}
	if len(expr.Arguments) == 0 {
		c.report(expr.Span, fmt.Sprintf("%s expects at least 1 argument, got 0", name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}

	values := make([]Type, len(expr.Arguments))
	result := Type{Kind: Invalid, Name: "<invalid>"}
	for index, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		values[index] = value
		if value.Kind != Invalid && !value.IsOrdered() {
			c.report(argument.GetSpan(), fmt.Sprintf("%s requires ordered operands, got %s", name, value.String()))
		}
		if result.Kind == Invalid || (result.Kind == UntypedInt && value.Kind != Invalid && value.Kind != UntypedInt) {
			result = value
		}
	}
	if result.Kind == Invalid {
		return result
	}
	for index, value := range values {
		if value.Kind == Invalid {
			continue
		}
		if !sameType(result, value) {
			c.report(expr.Arguments[index].GetSpan(), fmt.Sprintf("%s operands must have one ordered type; got %s and %s", name, result.String(), value.String()))
			continue
		}
		if integer, known := c.resolvedIntegerConstantValue(expr.Arguments[index]); known && value.Kind == UntypedInt && !integerConstantFitsFixedType(integer, result) {
			c.report(expr.Arguments[index].GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", integer.String(), result.String()))
		}
	}
	return result
}

func (c *Checker) checkMakeSlice(expr *ast.CallExpr) Type {
	expr.Builtin = ast.MakeSliceCall
	if expr.Expanded {
		c.report(expr.Span, "makeSlice does not accept spread arguments")
	}
	if len(expr.TypeArguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("makeSlice expects one element type argument, got %d", len(expr.TypeArguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	element := c.resolveType(expr.TypeArguments[0])
	if !isCollectionElementType(element) {
		c.report(expr.TypeArguments[0].Span, fmt.Sprintf("type %s cannot be used as a slice element", element.String()))
	}
	c.checkMakeSizeArguments(expr, "makeSlice", 1, 2)
	if len(expr.Arguments) >= 2 {
		length, lengthOK := nonnegativeIntegerConstant(expr.Arguments[0])
		capacity, capacityOK := nonnegativeIntegerConstant(expr.Arguments[1])
		if lengthOK && capacityOK && capacity.Cmp(length) < 0 {
			c.report(expr.Arguments[1].GetSpan(), "makeSlice capacity cannot be smaller than length")
		}
	}
	if element.Kind == Invalid {
		return element
	}
	return Type{Kind: Array, Name: "array", Element: &element}
}

func (c *Checker) checkMakeMap(expr *ast.CallExpr) Type {
	expr.Builtin = ast.MakeMapCall
	if expr.Expanded {
		c.report(expr.Span, "makeMap does not accept spread arguments")
	}
	if len(expr.TypeArguments) != 2 {
		c.report(expr.Span, fmt.Sprintf("makeMap expects key and value type arguments, got %d", len(expr.TypeArguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	key := c.resolveType(expr.TypeArguments[0])
	value := c.resolveType(expr.TypeArguments[1])
	if key.Kind != Invalid && !key.IsComparable() {
		c.report(expr.TypeArguments[0].Span, fmt.Sprintf("type %s cannot be used as a Map key", key.String()))
	}
	if !isCollectionElementType(value) {
		c.report(expr.TypeArguments[1].Span, fmt.Sprintf("type %s cannot be used as a Map value", value.String()))
	}
	c.checkMakeSizeArguments(expr, "makeMap", 0, 1)
	return Type{Kind: Map, Name: "Map", Key: &key, Element: &value}
}

func (c *Checker) checkSliceToArray(expr *ast.CallExpr, view bool) Type {
	name := "copyArray"
	expr.Builtin = ast.CopyArrayCall
	if view {
		name = "viewArray"
		expr.Builtin = ast.ViewArrayCall
	}
	if expr.Expanded {
		c.report(expr.Span, fmt.Sprintf("%s does not accept spread arguments", name))
	}
	if len(expr.TypeArguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("%s expects one fixed array type argument, got %d", name, len(expr.TypeArguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	target := c.resolveType(expr.TypeArguments[0])
	targetGo, targetOK := goTypeOf(target)
	if targetOK {
		_, targetOK = gotypes.Unalias(targetGo).Underlying().(*gotypes.Array)
	}
	if target.Kind != Invalid && !targetOK {
		c.report(expr.TypeArguments[0].Span, fmt.Sprintf("%s target must be a fixed array type, got %s", name, target.String()))
	}
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("%s expects one slice argument, got %d", name, len(expr.Arguments)))
	}
	var source Type
	for i, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if i == 0 {
			source = value
		}
	}
	if len(expr.Arguments) == 1 && source.Kind != Invalid {
		sourceGo, sourceOK := goTypeOf(source)
		if sourceOK {
			_, sourceOK = gotypes.Unalias(sourceGo).Underlying().(*gotypes.Slice)
		}
		if !sourceOK {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s requires a slice source, got %s", name, source.String()))
		} else if targetOK {
			conversionTarget := targetGo
			if view {
				conversionTarget = gotypes.NewPointer(targetGo)
			}
			if !gotypes.ConvertibleTo(sourceGo, conversionTarget) {
				c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("cannot convert slice %s to %s target %s", source.String(), name, target.String()))
			}
		}
	}
	if !targetOK || target.Kind == Invalid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !view {
		return target
	}
	return Type{Kind: GoPointer, Name: "*" + target.String(), Element: &target, GoType: gotypes.NewPointer(targetGo), GoQualifier: target.GoQualifier}
}

func (c *Checker) checkBuiltinCallShape(expr *ast.CallExpr, name string, minimum, maximum, typeArguments int) {
	if len(expr.TypeArguments) != typeArguments {
		c.report(expr.Span, fmt.Sprintf("%s expects %d type arguments, got %d", name, typeArguments, len(expr.TypeArguments)))
	}
	if expr.Expanded {
		c.report(expr.Span, fmt.Sprintf("%s does not accept spread arguments", name))
	}
	if len(expr.Arguments) < minimum || len(expr.Arguments) > maximum {
		if minimum == maximum {
			c.report(expr.Span, fmt.Sprintf("%s expects %d arguments, got %d", name, minimum, len(expr.Arguments)))
		} else {
			c.report(expr.Span, fmt.Sprintf("%s expects between %d and %d arguments, got %d", name, minimum, maximum, len(expr.Arguments)))
		}
	}
}

func (c *Checker) checkMakeSizeArguments(expr *ast.CallExpr, name string, minimum, maximum int) {
	if len(expr.Arguments) < minimum || len(expr.Arguments) > maximum {
		c.report(expr.Span, fmt.Sprintf("%s expects between %d and %d size arguments, got %d", name, minimum, maximum, len(expr.Arguments)))
	}
	for _, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if value.Kind != Invalid && !value.IsInteger() {
			c.report(argument.GetSpan(), fmt.Sprintf("%s size must be an integer, got %s", name, value.String()))
		}
		if constant, known := integerConstantValue(argument); known {
			if constant.Sign() < 0 {
				c.report(argument.GetSpan(), fmt.Sprintf("%s size cannot be negative", name))
			} else if !constant.IsInt64() {
				c.report(argument.GetSpan(), fmt.Sprintf("%s size is out of range", name))
			}
		}
	}
}

func (c *Checker) sliceElementType(value Type, span source.Span) (Type, bool) {
	if value.Kind == Nullable && value.Element != nil {
		return c.sliceElementType(*value.Element, span)
	}
	if value.Kind == Array && value.Element != nil {
		return *value.Element, true
	}
	goType, ok := goTypeOf(value)
	if !ok {
		return Type{}, false
	}
	slice, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Slice)
	if !ok {
		return Type{}, false
	}
	return c.collectionElementType(slice.Elem(), value, span), true
}

func (c *Checker) mapCollectionTypes(value Type, span source.Span) (Type, Type, bool) {
	if value.Kind == Nullable && value.Element != nil {
		return c.mapCollectionTypes(*value.Element, span)
	}
	if value.Kind == Map && value.Key != nil && value.Element != nil {
		return *value.Key, *value.Element, true
	}
	goType, ok := goTypeOf(value)
	if !ok {
		return Type{}, Type{}, false
	}
	mapping, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Map)
	if !ok {
		return Type{}, Type{}, false
	}
	return c.collectionElementType(mapping.Key(), value, span), c.collectionElementType(mapping.Elem(), value, span), true
}

func isLenCollection(value Type) bool {
	if value.Kind == Nullable && value.Element != nil {
		return isLenCollection(*value.Element)
	}
	if value.Kind == Array || value.Kind == FixedArray || value.Kind == Map || value.Kind == String || value.Kind == GoChannel {
		return true
	}
	return goCollectionAcceptsLenOrCap(value, true)
}

func isCapCollection(value Type) bool {
	if value.Kind == Nullable && value.Element != nil {
		return isCapCollection(*value.Element)
	}
	if value.Kind == Array || value.Kind == FixedArray || value.Kind == GoChannel {
		return true
	}
	return goCollectionAcceptsLenOrCap(value, false)
}

func isClearCollection(value Type) bool {
	if value.Kind == Nullable && value.Element != nil {
		return isClearCollection(*value.Element)
	}
	if value.Kind == Array || value.Kind == Map {
		return true
	}
	goType, ok := goTypeOf(value)
	if !ok {
		return false
	}
	switch gotypes.Unalias(goType).Underlying().(type) {
	case *gotypes.Slice, *gotypes.Map:
		return true
	default:
		return false
	}
}

func goCollectionAcceptsLenOrCap(value Type, allowLenOnly bool) bool {
	goType, ok := goTypeOf(value)
	if !ok {
		return false
	}
	underlying := gotypes.Unalias(goType).Underlying()
	switch collection := underlying.(type) {
	case *gotypes.Array, *gotypes.Slice, *gotypes.Chan:
		return true
	case *gotypes.Map:
		return allowLenOnly
	case *gotypes.Basic:
		return allowLenOnly && collection.Info()&gotypes.IsString != 0
	case *gotypes.Pointer:
		_, ok := gotypes.Unalias(collection.Elem()).Underlying().(*gotypes.Array)
		return ok
	default:
		return false
	}
}

func isBuiltinByte(value Type) bool {
	goType, ok := goTypeOf(value)
	return ok && gotypes.Identical(goType, gotypes.Typ[gotypes.Uint8])
}

func identicalCollectionElement(left, right Type) bool {
	if left.Kind == Nullable || right.Kind == Nullable {
		return left.Kind == Nullable && right.Kind == Nullable && left.Element != nil && right.Element != nil && identicalCollectionElement(*left.Element, *right.Element)
	}
	leftGo, leftOK := goTypeOf(left)
	rightGo, rightOK := goTypeOf(right)
	if leftOK && rightOK {
		return gotypes.Identical(leftGo, rightGo)
	}
	return left.Kind == right.Kind && left.Name == right.Name && (left.Kind == Class || left.Kind == Interface || left.Kind == Object)
}

func isCollectionElementType(value Type) bool {
	switch value.Kind {
	case Invalid, Void, GoPackage, GoTypeName, Nil, Null, MultiValue, Result:
		return false
	default:
		return true
	}
}

func isNullableBaseType(value Type) bool {
	switch value.Kind {
	case Class, Interface, Array, Map, Function, GoChannel, GoPointer:
		return true
	case Invalid, Void, Nil, Null, MultiValue, Result, Nullable, GoPackage, GoTypeName, Object, FixedArray:
		return false
	default:
		return isNilable(value)
	}
}

func nonnegativeIntegerConstant(expression ast.Expression) (*big.Int, bool) {
	value, ok := integerConstantValue(expression)
	return value, ok && value.Sign() >= 0
}

func integerConstantValue(expression ast.Expression) (*big.Int, bool) {
	return integerConstantValueWithResolver(expression, nil)
}

func integerConstantValueWithResolver(expression ast.Expression, resolve func(*ast.IdentifierExpr) (*big.Int, bool)) (*big.Int, bool) {
	switch expression := expression.(type) {
	case *ast.LiteralExpr:
		if expression.Kind != ast.IntegerLiteral {
			return nil, false
		}
		value, ok := new(big.Int).SetString(expression.Text, 10)
		return value, ok
	case *ast.UnaryExpr:
		value, ok := integerConstantValueWithResolver(expression.Operand, resolve)
		if !ok {
			return nil, false
		}
		switch expression.Operator {
		case "+":
			return value, true
		case "-":
			return new(big.Int).Neg(value), true
		case "^":
			return new(big.Int).Not(value), true
		default:
			return nil, false
		}
	case *ast.BinaryExpr:
		left, leftOK := integerConstantValueWithResolver(expression.Left, resolve)
		right, rightOK := integerConstantValueWithResolver(expression.Right, resolve)
		if !leftOK || !rightOK {
			return nil, false
		}
		switch expression.Operator {
		case "+":
			return new(big.Int).Add(left, right), true
		case "-":
			return new(big.Int).Sub(left, right), true
		case "*":
			return new(big.Int).Mul(left, right), true
		case "/":
			if right.Sign() != 0 {
				return new(big.Int).Quo(left, right), true
			}
		case "%":
			if right.Sign() != 0 {
				return new(big.Int).Rem(left, right), true
			}
		case "&":
			return new(big.Int).And(left, right), true
		case "|":
			return new(big.Int).Or(left, right), true
		case "^":
			return new(big.Int).Xor(left, right), true
		case "&^":
			return new(big.Int).AndNot(left, right), true
		case "<<":
			if right.Sign() >= 0 && right.IsUint64() {
				return new(big.Int).Lsh(left, uint(right.Uint64())), true
			}
		case ">>":
			if right.Sign() >= 0 && right.IsUint64() {
				return new(big.Int).Rsh(left, uint(right.Uint64())), true
			}
		}
	case *ast.CallExpr:
		name, ok := expression.Callee.(*ast.IdentifierExpr)
		if !ok || expression.Expanded || len(expression.Arguments) != 1 {
			return nil, false
		}
		target, ok := LookupType(name.Name)
		if !ok || !target.IsInteger() {
			return nil, false
		}
		return integerConstantValueWithResolver(expression.Arguments[0], resolve)
	case *ast.IdentifierExpr:
		if resolve != nil {
			return resolve(expression)
		}
	}
	return nil, false
}

func (c *Checker) resolvedIntegerConstantValue(expression ast.Expression) (*big.Int, bool) {
	seen := map[*ast.VariableDecl]bool{}
	var resolve func(*ast.IdentifierExpr) (*big.Int, bool)
	resolve = func(identifier *ast.IdentifierExpr) (*big.Int, bool) {
		symbol, ok := c.lookupSymbol(identifier.Name, identifier.Span)
		if !ok || !symbol.constant || symbol.declaration == nil || seen[symbol.declaration] {
			return nil, false
		}
		seen[symbol.declaration] = true
		value, known := integerConstantValueWithResolver(symbol.declaration.Value, resolve)
		delete(seen, symbol.declaration)
		return value, known
	}
	return integerConstantValueWithResolver(expression, resolve)
}

func (c *Checker) integerExpressionIsCompileTimeConstant(expression ast.Expression) bool {
	seen := map[*ast.VariableDecl]bool{}
	var check func(ast.Expression) bool
	check = func(expression ast.Expression) bool {
		switch expression := expression.(type) {
		case *ast.IdentifierExpr:
			symbol, ok := c.lookupSymbol(expression.Name, expression.Span)
			if !ok || !symbol.constant || symbol.declaration == nil || seen[symbol.declaration] {
				return false
			}
			seen[symbol.declaration] = true
			constant := check(symbol.declaration.Value)
			delete(seen, symbol.declaration)
			return constant
		case *ast.LiteralExpr:
			return expression.Kind == ast.IntegerLiteral
		case *ast.UnaryExpr:
			return check(expression.Operand)
		case *ast.BinaryExpr:
			return check(expression.Left) && check(expression.Right)
		case *ast.CallExpr:
			name, ok := expression.Callee.(*ast.IdentifierExpr)
			if !ok || expression.Expanded || len(expression.Arguments) != 1 {
				return false
			}
			target, ok := LookupType(name.Name)
			return ok && target.IsInteger() && check(expression.Arguments[0])
		case *ast.MemberExpr:
			return expression.Constant
		default:
			return false
		}
	}
	return check(expression)
}

func integerConstantFitsFixedType(value *big.Int, target Type) bool {
	goType, ok := goTypeOf(target)
	if !ok {
		return true
	}
	basic, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Basic)
	if !ok {
		return true
	}
	var bits uint
	var signed bool
	switch basic.Kind() {
	case gotypes.Int8:
		bits, signed = 8, true
	case gotypes.Uint8:
		bits = 8
	case gotypes.Uint:
		return value.Sign() >= 0
	case gotypes.Int16:
		bits, signed = 16, true
	case gotypes.Uint16:
		bits = 16
	case gotypes.Int32:
		bits, signed = 32, true
	case gotypes.Uint32:
		bits = 32
	case gotypes.Int64:
		bits, signed = 64, true
	case gotypes.Uint64:
		bits = 64
	default:
		// int, uint, and uintptr depend on the selected build target. Generated
		// Go validation remains authoritative until target sizes are part of sema.
		return true
	}
	if signed {
		limit := new(big.Int).Lsh(big.NewInt(1), bits-1)
		minimum := new(big.Int).Neg(new(big.Int).Set(limit))
		maximum := new(big.Int).Sub(limit, big.NewInt(1))
		return value.Cmp(minimum) >= 0 && value.Cmp(maximum) <= 0
	}
	if value.Sign() < 0 {
		return false
	}
	limit := new(big.Int).Lsh(big.NewInt(1), bits)
	return value.Cmp(limit) < 0
}

func (c *Checker) checkNativeGenericCall(expr *ast.CallExpr, callableName string, callable Type) Type {
	if expr.Expanded && !callable.Variadic {
		c.report(expr.Span, fmt.Sprintf("%s is not variadic and cannot receive a spread argument", callableName))
	}
	if len(expr.TypeArguments) > len(callable.TypeParameters) {
		c.report(expr.Span, fmt.Sprintf("%s has %d type parameters, got %d explicit type arguments", callableName, len(callable.TypeParameters), len(expr.TypeArguments)))
	}
	bindings := map[string]Type{}
	for index, argument := range expr.TypeArguments {
		resolved := c.resolveType(argument)
		if resolved.Kind == Invalid {
			continue
		}
		if !validNativeTypeArgument(resolved) {
			c.report(argument.Span, fmt.Sprintf("type %s cannot be used as a generic function type argument", resolved.String()))
			continue
		}
		if index < len(callable.TypeParameters) {
			bindings[callable.TypeParameters[index].Name] = resolved
		}
	}
	actualTypes := make([]Type, len(expr.Arguments))
	for index, argument := range expr.Arguments {
		actualTypes[index] = c.singleValue(c.checkExpression(argument), argument.GetSpan())
	}
	minimumArguments := len(callable.Parameters)
	if callable.Variadic {
		minimumArguments--
	}
	if expr.Expanded && callable.Variadic && len(expr.Arguments) != len(callable.Parameters) {
		c.report(expr.Span, fmt.Sprintf("spread call to %s expects %d arguments (%d fixed and one slice), got %d", callableName, len(callable.Parameters), minimumArguments, len(expr.Arguments)))
	} else if !expr.Expanded && (len(expr.Arguments) < minimumArguments || (!callable.Variadic && len(expr.Arguments) != len(callable.Parameters))) {
		if callable.Variadic {
			c.report(expr.Span, fmt.Sprintf("%s expects at least %d arguments, got %d", callableName, minimumArguments, len(expr.Arguments)))
		} else {
			c.report(expr.Span, fmt.Sprintf("%s expects %d arguments, got %d", callableName, len(callable.Parameters), len(expr.Arguments)))
		}
	}
	for index := range actualTypes {
		parameterIndex := index
		if callable.Variadic && parameterIndex >= len(callable.Parameters)-1 {
			parameterIndex = len(callable.Parameters) - 1
		}
		if parameterIndex < 0 || parameterIndex >= len(callable.Parameters) {
			continue
		}
		expected := callable.Parameters[parameterIndex]
		if expr.Expanded && callable.Variadic && index == len(actualTypes)-1 {
			element := expected
			expected = Type{Kind: Array, Name: "array", Element: &element}
		}
		if err := inferNativeTypeArguments(expected, actualTypes[index], bindings); err != nil {
			c.report(expr.Arguments[index].GetSpan(), fmt.Sprintf("cannot infer type arguments for %s from argument %d: %v", callableName, index+1, err))
		}
	}
	missing := make([]string, 0, len(callable.TypeParameters))
	for _, parameter := range callable.TypeParameters {
		if _, ok := bindings[parameter.Name]; !ok {
			missing = append(missing, parameter.Name)
		}
	}
	if len(missing) != 0 {
		c.report(expr.Span, fmt.Sprintf("cannot infer type argument%s %s for %s; provide explicit type arguments", pluralSuffix(len(missing)), strings.Join(missing, ", "), callableName))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	arguments := make([]Type, len(callable.TypeParameters))
	for index, parameter := range callable.TypeParameters {
		arguments[index] = bindings[parameter.Name]
	}
	if !c.validateNativeTypeArguments(callable.TypeParameters, arguments, expr.TypeArguments, expr.Span, callableName) {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	parameters := make([]Type, len(callable.Parameters))
	for index, parameter := range callable.Parameters {
		parameters[index] = substituteNativeTypeParameters(parameter, bindings)
	}
	result := substituteNativeTypeParameters(*callable.Result, bindings)
	instantiated := Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: callable.Variadic, Result: &result}
	c.recordCallSignature(expr, instantiated)
	for index := range actualTypes {
		parameterIndex := index
		if instantiated.Variadic && parameterIndex >= len(parameters)-1 {
			parameterIndex = len(parameters) - 1
		}
		if parameterIndex < 0 || parameterIndex >= len(parameters) {
			continue
		}
		expected := parameters[parameterIndex]
		if expr.Expanded && instantiated.Variadic && index == len(actualTypes)-1 {
			element := expected
			expected = Type{Kind: Array, Name: "array", Element: &element}
		}
		c.requireAssignable(expected, actualTypes[index], expr.Arguments[index].GetSpan())
	}
	return result
}

func validNativeTypeArgument(value Type) bool {
	switch value.Kind {
	case Invalid, Void, MultiValue, Result, Task, GoPackage, GoTypeName, Nil, Null:
		return false
	default:
		return true
	}
}

func inferNativeTypeArguments(formal, actual Type, bindings map[string]Type) error {
	if formal.Kind == TypeParameter {
		actual = defaultLiteralType(actual)
		if existing, ok := bindings[formal.Name]; ok {
			if !sameType(existing, actual) {
				return fmt.Errorf("%s was already inferred as %s, not %s", formal.Name, existing.String(), actual.String())
			}
			return nil
		}
		if !validNativeTypeArgument(actual) {
			return fmt.Errorf("%s cannot be inferred from %s", formal.Name, actual.String())
		}
		bindings[formal.Name] = actual
		return nil
	}
	if formal.Kind != actual.Kind {
		return nil
	}
	switch formal.Kind {
	case Nullable, Array, FixedArray, GoPointer, Result, Task, GoChannel:
		if formal.Element != nil && actual.Element != nil {
			return inferNativeTypeArguments(*formal.Element, *actual.Element, bindings)
		}
	case Map:
		if formal.Key != nil && actual.Key != nil {
			if err := inferNativeTypeArguments(*formal.Key, *actual.Key, bindings); err != nil {
				return err
			}
		}
		if formal.Element != nil && actual.Element != nil {
			return inferNativeTypeArguments(*formal.Element, *actual.Element, bindings)
		}
	case Function:
		if len(formal.Parameters) == len(actual.Parameters) {
			for index := range formal.Parameters {
				if err := inferNativeTypeArguments(formal.Parameters[index], actual.Parameters[index], bindings); err != nil {
					return err
				}
			}
		}
		if formal.Result != nil && actual.Result != nil {
			return inferNativeTypeArguments(*formal.Result, *actual.Result, bindings)
		}
	case Object:
		for name, formalField := range formal.Fields {
			if actualField, ok := actual.Fields[name]; ok {
				if err := inferNativeTypeArguments(formalField, actualField, bindings); err != nil {
					return err
				}
			}
		}
	case GoNamed:
		if len(formal.TypeArguments) == len(actual.TypeArguments) {
			for index := range formal.TypeArguments {
				if err := inferNativeTypeArguments(formal.TypeArguments[index], actual.TypeArguments[index], bindings); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func substituteNativeTypeParameters(value Type, bindings map[string]Type) Type {
	if len(bindings) == 0 {
		return value
	}
	return substituteNativeTypeParametersSeen(value, bindings, map[string]bool{})
}

func substituteNativeTypeParametersSeen(value Type, bindings map[string]Type, visiting map[string]bool) Type {
	if value.Kind == TypeParameter {
		if replacement, ok := bindings[value.Name]; ok {
			return replacement
		}
		return value
	}
	if value.Kind == Struct {
		if visiting[value.Name] {
			result := value
			result.Fields = nil
			result.TypeArguments = append([]Type(nil), value.TypeArguments...)
			for index := range result.TypeArguments {
				result.TypeArguments[index] = substituteNativeTypeParametersSeen(result.TypeArguments[index], bindings, visiting)
			}
			result.TypeParameters = nil
			result.Generic = false
			return result
		}
		visiting[value.Name] = true
		defer delete(visiting, value.Name)
	}
	result := value
	result.Parameters = append([]Type(nil), value.Parameters...)
	for index := range result.Parameters {
		result.Parameters[index] = substituteNativeTypeParametersSeen(result.Parameters[index], bindings, visiting)
	}
	result.TypeParameters = nil
	if value.Result != nil {
		item := substituteNativeTypeParametersSeen(*value.Result, bindings, visiting)
		result.Result = &item
	}
	if value.Element != nil {
		item := substituteNativeTypeParametersSeen(*value.Element, bindings, visiting)
		result.Element = &item
	}
	if value.Key != nil {
		item := substituteNativeTypeParametersSeen(*value.Key, bindings, visiting)
		result.Key = &item
	}
	if value.Fields != nil {
		result.Fields = make(map[string]Type, len(value.Fields))
		for name, field := range value.Fields {
			result.Fields[name] = substituteNativeTypeParametersSeen(field, bindings, visiting)
		}
	}
	result.Results = append([]Type(nil), value.Results...)
	for index := range result.Results {
		result.Results[index] = substituteNativeTypeParametersSeen(result.Results[index], bindings, visiting)
	}
	result.TypeArguments = append([]Type(nil), value.TypeArguments...)
	for index := range result.TypeArguments {
		result.TypeArguments[index] = substituteNativeTypeParametersSeen(result.TypeArguments[index], bindings, visiting)
	}
	result.Generic = false
	if value.Kind == GoNamed && len(result.TypeArguments) != 0 {
		if named, ok := value.GoType.(*gotypes.Named); ok {
			object := named.Obj()
			if object != nil {
				goArguments := make([]gotypes.Type, len(result.TypeArguments))
				valid := true
				for index, argument := range result.TypeArguments {
					goArgument, ok := goTypeOf(argument)
					if !ok {
						valid = false
						break
					}
					goArguments[index] = goArgument
				}
				if valid {
					if instantiated, err := gotypes.Instantiate(nil, named.Origin(), goArguments, true); err == nil {
						result.GoType = instantiated
						names := make([]string, len(result.TypeArguments))
						for index := range result.TypeArguments {
							names[index] = result.TypeArguments[index].String()
						}
						result.Name = object.Name() + "<" + strings.Join(names, ", ") + ">"
					}
				}
			}
		}
	}
	if value.Kind == Array || value.Kind == FixedArray || value.Kind == Map || value.Kind == Function || value.Kind == Object || value.Kind == Nullable || value.Kind == Result || value.Kind == Task {
		result.GoType = nil
	}
	if value.Kind == GoPointer && result.Element != nil {
		result.GoType = nil
		if element, ok := goTypeOf(*result.Element); ok {
			result.GoType = gotypes.NewPointer(element)
		}
	}
	if value.Kind == GoChannel && result.Element != nil {
		direction := gotypes.SendRecv
		if value.GoType != nil {
			if channel, ok := gotypes.Unalias(value.GoType).Underlying().(*gotypes.Chan); ok {
				direction = channel.Dir()
			}
		}
		result.GoType = nil
		if element, ok := goTypeOf(*result.Element); ok {
			result.GoType = gotypes.NewChan(direction, element)
		}
	}
	return result
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func (c *Checker) checkExplicitGenericCall(expr *ast.CallExpr, callableName string, callable Type) Type {
	signature, ok := callable.GoType.(*gotypes.Signature)
	if !ok || signature.TypeParams() == nil || signature.TypeParams().Len() == 0 {
		c.report(expr.Callee.GetSpan(), fmt.Sprintf("%s has invalid generic Go type information", callableName))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if len(expr.TypeArguments) > signature.TypeParams().Len() {
		c.report(expr.Span, fmt.Sprintf("%s has %d Go type parameters, got %d explicit type arguments", callableName, signature.TypeParams().Len(), len(expr.TypeArguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	typeArguments := make([]gotypes.Type, len(expr.TypeArguments))
	for i := range expr.TypeArguments {
		resolved := c.resolveType(expr.TypeArguments[i])
		if resolved.Kind == Invalid {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		goType, valid := goTypeOf(resolved)
		if !valid {
			c.report(expr.TypeArguments[i].Span, fmt.Sprintf("type argument %s cannot be represented as a Go type", resolved.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		typeArguments[i] = goType
	}
	actualTypes := make([]Type, len(expr.Arguments))
	for i, argument := range expr.Arguments {
		actualTypes[i] = c.singleValue(c.checkExpression(argument), argument.GetSpan())
	}
	instantiatedSignature, err := inferGoGenericCall(signature, actualTypes, typeArguments, expr.Expanded)
	if err != nil {
		c.report(expr.Span, fmt.Sprintf("cannot apply explicit Go type arguments to %s: %v", callableName, err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	converted, err := ontamaFunctionFromGo(instantiatedSignature)
	if err != nil {
		c.report(expr.Span, fmt.Sprintf("instantiated Go call to %s is not supported: %v", callableName, err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	c.recordCallSignature(expr, converted)
	return *converted.Result
}

func (c *Checker) checkInferredGenericCall(expr *ast.CallExpr, callableName string, callable Type) Type {
	signature, ok := callable.GoType.(*gotypes.Signature)
	if !ok || signature.TypeParams() == nil || signature.TypeParams().Len() == 0 {
		c.report(expr.Callee.GetSpan(), fmt.Sprintf("%s has invalid generic Go type information", callableName))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	actualTypes := make([]Type, len(expr.Arguments))
	for i, argument := range expr.Arguments {
		actualTypes[i] = c.singleValue(c.checkExpression(argument), argument.GetSpan())
	}
	instantiated, err := inferGoGenericCall(signature, actualTypes, nil, expr.Expanded)
	if err != nil {
		c.report(expr.Span, fmt.Sprintf("cannot infer Go type arguments for %s: %v", callableName, err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	converted, err := ontamaFunctionFromGo(instantiated)
	if err != nil {
		c.report(expr.Span, fmt.Sprintf("inferred Go call to %s is not supported: %v", callableName, err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	c.recordCallSignature(expr, converted)
	return *converted.Result
}

func inferGoGenericCall(signature *gotypes.Signature, actualTypes []Type, explicitTypeArguments []gotypes.Type, expanded bool) (*gotypes.Signature, error) {
	packageInfo := gotypes.NewPackage("ontama.synthetic/generic", "generic")
	functionName := "__ontama_generic_function"
	functionIdentifier := goast.NewIdent(functionName)
	if existing := packageInfo.Scope().Insert(gotypes.NewFunc(0, packageInfo, functionName, signature)); existing != nil {
		return nil, fmt.Errorf("cannot create inference scope")
	}
	arguments := make([]goast.Expr, len(actualTypes))
	for i, actual := range actualTypes {
		switch actual.Kind {
		case UntypedInt:
			arguments[i] = &goast.BasicLit{Kind: gotoken.INT, Value: "0"}
		case Nil:
			arguments[i] = goast.NewIdent("nil")
		default:
			goType, ok := goTypeOf(actual)
			if !ok {
				return nil, fmt.Errorf("argument %d has non-Go-representable type %s", i+1, actual.String())
			}
			name := fmt.Sprintf("__ontama_argument_%d", i)
			if existing := packageInfo.Scope().Insert(gotypes.NewVar(0, packageInfo, name, goType)); existing != nil {
				return nil, fmt.Errorf("cannot create inference argument %d", i+1)
			}
			arguments[i] = goast.NewIdent(name)
		}
	}
	packageInfo.MarkComplete()
	var functionExpression goast.Expr = functionIdentifier
	if len(explicitTypeArguments) != 0 {
		typeExpressions := make([]goast.Expr, len(explicitTypeArguments))
		for i, argumentType := range explicitTypeArguments {
			name := fmt.Sprintf("__ontama_type_argument_%d", i)
			if existing := packageInfo.Scope().Insert(gotypes.NewTypeName(0, packageInfo, name, argumentType)); existing != nil {
				return nil, fmt.Errorf("cannot create explicit type argument %d", i+1)
			}
			typeExpressions[i] = goast.NewIdent(name)
		}
		if len(typeExpressions) == 1 {
			functionExpression = &goast.IndexExpr{X: functionIdentifier, Index: typeExpressions[0]}
		} else {
			functionExpression = &goast.IndexListExpr{X: functionIdentifier, Indices: typeExpressions}
		}
	}
	call := &goast.CallExpr{Fun: functionExpression, Args: arguments}
	if expanded {
		call.Ellipsis = gotoken.Pos(1)
	}
	info := &gotypes.Info{
		Types:     map[goast.Expr]gotypes.TypeAndValue{},
		Uses:      map[*goast.Ident]gotypes.Object{},
		Instances: map[*goast.Ident]gotypes.Instance{},
	}
	if err := gotypes.CheckExpr(gotoken.NewFileSet(), packageInfo, gotoken.NoPos, call, info); err != nil {
		return nil, err
	}
	instance, ok := info.Instances[functionIdentifier]
	if !ok {
		return nil, fmt.Errorf("Go type checker did not report an inferred instance")
	}
	instantiated, ok := instance.Type.(*gotypes.Signature)
	if !ok {
		return nil, fmt.Errorf("Go type checker returned %T instead of a function signature", instance.Type)
	}
	return instantiated, nil
}

func (c *Checker) checkGoConversion(expr *ast.CallExpr, target Type) Type {
	converted, err := ontamaTypeFromGo(target.GoType)
	if err != nil {
		c.report(expr.Callee.GetSpan(), fmt.Sprintf("Go type %s cannot be used in a conversion: %v", target.String(), err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if expr.Expanded {
		c.report(expr.Span, "spread arguments cannot be used in Go type conversions")
	}
	converted.GoQualifier = target.GoQualifier
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("conversion to %s expects 1 argument, got %d", target.String(), len(expr.Arguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return converted
	}
	value := c.singleValue(c.checkExpression(expr.Arguments[0]), expr.Arguments[0].GetSpan())
	valueGo, valueOK := goTypeOf(value)
	if target.GoType == nil || !valueOK || !gotypes.ConvertibleTo(valueGo, target.GoType) {
		c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("cannot convert %s to %s", value.String(), target.String()))
	}
	return converted
}

func (c *Checker) checkNativeTypeConversion(expr *ast.CallExpr, target Type) Type {
	if expr.Expanded {
		c.report(expr.Span, "spread arguments cannot be used in type conversions")
	}
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("conversion to %s expects 1 argument, got %d", target.String(), len(expr.Arguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return target
	}
	if target.Kind == Invalid {
		c.checkExpression(expr.Arguments[0])
		return target
	}
	value := c.singleValue(c.checkExpression(expr.Arguments[0]), expr.Arguments[0].GetSpan())
	targetGo, targetOK := goTypeOf(target)
	valueGo, valueOK := goTypeOf(value)
	if !targetOK || !valueOK || value.Kind == Nullable || !gotypes.ConvertibleTo(valueGo, targetGo) {
		c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("cannot convert %s to %s", value.String(), target.String()))
	} else if target.IsNumeric() {
		if integer, known := c.resolvedIntegerConstantValue(expr.Arguments[0]); known && !integerConstantFitsFixedType(integer, target) {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", integer.String(), target.String()))
		}
	}
	return target
}

func (c *Checker) checkArrow(expr *ast.ArrowExpr) Type {
	outerFlow := c.snapshotNullableFlow()
	memberRoots := map[source.Span]bool{}
	if len(c.capturedMemberRoots) != 0 {
		for declaration := range c.capturedMemberRoots[len(c.capturedMemberRoots)-1] {
			memberRoots[declaration] = true
		}
	}
	for key, state := range outerFlow.members {
		if state.nonNull {
			memberRoots[key.root] = true
		}
	}
	base := len(c.scopes)
	c.scopes = cloneValueScopes(c.scopes)
	c.memberFlow = map[memberFlowKey]memberFlowState{}
	for scopeIndex, scope := range c.scopes {
		for name, symbol := range scope {
			if !symbol.constant && symbol.declaredType.Kind == Nullable {
				symbol.typeInfo = symbol.declaredType
				c.scopes[scopeIndex][name] = symbol
			}
		}
	}
	c.callableScopeBases = append(c.callableScopeBases, base)
	c.capturedWrites = append(c.capturedWrites, map[source.Span]source.Span{})
	c.capturedMemberWrites = append(c.capturedMemberWrites, source.Span{})
	c.capturedMemberRoots = append(c.capturedMemberRoots, memberRoots)
	c.pushScope()
	parameters := make([]Type, len(expr.Parameters))
	for i, parameter := range expr.Parameters {
		parameters[i] = c.resolveType(parameter.Type)
		if parameters[i].Kind == Void {
			c.report(parameter.Type.Span, "parameters cannot have type void")
		}
		c.rejectResultValueType(parameters[i], parameter.Type.Span, "parameters")
		c.rejectTaskAPIType(parameters[i], parameter.Type.Span, "arrow parameters")
		c.declareLocal(parameter.Name, parameters[i], false, nil, parameter.Span)
	}
	previousResult := c.result
	previousLoopDepth := c.loopDepth
	previousBreakableDepth := c.breakableDepth
	previousExceptionDepth := c.exceptionDepth
	previousCatchTargets := c.catchTargets
	c.loopDepth = 0
	c.breakableDepth = 0
	c.exceptionDepth = 0
	c.catchTargets = nil
	var result Type
	if expr.ReturnType != nil {
		result = c.resolveType(*expr.ReturnType)
		c.rejectTaskAPIType(result, expr.ReturnType.Span, "arrow return types")
		c.result = result
	}
	if expr.ExpressionBody != nil {
		if result.Kind == Result {
			c.report(expr.ExpressionBody.GetSpan(), "Result arrow functions require a block body with an explicit return")
		}
		actual := c.checkExpressionExpectedSlot(&expr.ExpressionBody, result)
		if expr.ReturnType == nil {
			actual = c.singleValue(actual, expr.ExpressionBody.GetSpan())
			if actual.Kind == Nil {
				c.report(expr.ExpressionBody.GetSpan(), "cannot infer an arrow function return type from nil")
				result = Type{Kind: Invalid, Name: "<invalid>"}
			} else {
				result = defaultLiteralType(actual)
			}
		} else {
			c.requireAssignable(result, actual, expr.ExpressionBody.GetSpan())
		}
	} else if expr.BlockBody != nil {
		c.validateLabels(expr.BlockBody)
		if expr.ReturnType == nil {
			c.report(expr.Span, "arrow functions with a block body require an explicit return type")
			result = Type{Kind: Invalid, Name: "<invalid>"}
			c.result = result
		}
		c.checkBlock(expr.BlockBody, false)
		if result.Kind != Void && result.Kind != Invalid && !definitelyReturns(expr.BlockBody) {
			c.report(expr.Span, fmt.Sprintf("arrow function may complete without returning %s", result.String()))
		}
	}
	if containsTaskType(result) {
		c.report(expr.Span, "arrow functions cannot return Task; Task is a non-escaping local capability")
	}
	c.result = previousResult
	c.loopDepth = previousLoopDepth
	c.breakableDepth = previousBreakableDepth
	c.exceptionDepth = previousExceptionDepth
	c.catchTargets = previousCatchTargets
	c.popScope()
	captured := c.capturedWrites[len(c.capturedWrites)-1]
	c.capturedWrites = c.capturedWrites[:len(c.capturedWrites)-1]
	capturedMemberWrite := c.capturedMemberWrites[len(c.capturedMemberWrites)-1]
	c.capturedMemberWrites = c.capturedMemberWrites[:len(c.capturedMemberWrites)-1]
	c.capturedMemberRoots = c.capturedMemberRoots[:len(c.capturedMemberRoots)-1]
	c.callableScopeBases = c.callableScopeBases[:len(c.callableScopeBases)-1]
	c.restoreNullableFlow(outerFlow)
	for declaration := range captured {
		c.markDeclarationEscaped(declaration, expr.Span, "a closure that can mutate it")
	}
	if capturedMemberWrite.Start.Line != 0 {
		c.invalidateAllMemberFacts(expr.Span, "a closure with possible member mutation")
		c.recordMemberWrite(expr.Span)
	}
	c.prepareGoTypeForEmission(&result, expr.Span)
	resolved := typeRefFromType(result, expr.Span)
	expr.ResolvedReturnType = resolved
	callableParameters := make([]Type, len(parameters))
	for index, parameter := range expr.Parameters {
		callableParameters[index] = c.callableParameterType(parameter, parameters[index])
	}
	return Type{Kind: Function, Name: "function", Parameters: callableParameters, Variadic: hasVariadicParameter(expr.Parameters), Result: &result}
}

func (c *Checker) singleValue(value Type, span source.Span) Type {
	if value.Kind == Result {
		c.report(span, "Result values must be consumed with ?, explicitly split, or returned")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if value.Kind != MultiValue {
		return value
	}
	c.report(span, fmt.Sprintf("multiple values %s require destructuring", value.String()))
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) checkResultConstructor(expr *ast.CallExpr, success bool) Type {
	if c.result.Kind != Result || c.result.Element == nil {
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		c.report(expr.Span, "ok and fail may only be used inside a Result-returning function")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if len(expr.TypeArguments) != 0 {
		c.report(expr.Span, "Result constructors do not accept type arguments")
	}
	if expr.Expanded {
		c.report(expr.Span, "Result constructors do not accept spread arguments")
	}
	if success {
		expr.Builtin = ast.ResultOKCall
		expected := 1
		if c.result.Element.Kind == Void {
			expected = 0
		}
		expr.Signature = &ast.CallableSignature{Result: c.result.String()}
		if expected == 1 {
			expr.Signature.ParameterNames = []string{"value"}
			expr.Signature.ParameterTypes = []string{c.result.Element.String()}
		}
		if len(expr.Arguments) != expected {
			c.report(expr.Span, fmt.Sprintf("ok for %s expects %d arguments, got %d", c.result.String(), expected, len(expr.Arguments)))
		}
		for i, argument := range expr.Arguments {
			actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], *c.result.Element)
			if i == 0 && expected == 1 {
				c.requireAssignable(*c.result.Element, actual, argument.GetSpan())
			}
		}
		return c.result
	}
	expr.Builtin = ast.ResultFailCall
	expr.Signature = &ast.CallableSignature{
		ParameterNames: []string{"error"}, ParameterTypes: []string{builtins["error"].String()}, Result: c.result.String(),
	}
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("fail expects 1 error argument, got %d", len(expr.Arguments)))
	}
	for i, argument := range expr.Arguments {
		actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], builtins["error"])
		if i == 0 {
			c.requireAssignable(builtins["error"], actual, argument.GetSpan())
		}
	}
	return c.result
}

func (c *Checker) checkResultReturn(stmt *ast.ReturnStmt) {
	if c.result.Element == nil {
		return
	}
	stmt.ResultType = typeRefFromType(*c.result.Element, stmt.Span)
	if stmt.Value == nil {
		c.report(stmt.Span, fmt.Sprintf("Result function must return ok(...) or fail(...), expected %s", c.result.String()))
		return
	}
	value := c.checkExpressionExpectedSlot(&stmt.Value, c.result)
	if call, ok := stmt.Value.(*ast.CallExpr); ok {
		switch call.Builtin {
		case ast.ResultOKCall:
			stmt.ResultKind = ast.ResultSuccessReturn
			return
		case ast.ResultFailCall:
			stmt.ResultKind = ast.ResultFailureReturn
			return
		}
	}
	if value.Kind == Result && exactType(c.result, value) {
		stmt.ResultKind = ast.ResultForwardReturn
		return
	}
	if value.Kind == MultiValue {
		c.report(stmt.Value.GetSpan(), "Go multiple results are not implicitly converted to Result; propagate them with ? and return ok(value)")
		return
	}
	if value.Kind != Invalid {
		c.report(stmt.Value.GetSpan(), fmt.Sprintf("Result function must return ok(...) or fail(...), got %s", value.String()))
	}
}

func (c *Checker) checkPropagateExpression(expr *ast.PropagateExpr) Type {
	operand := c.checkExpression(expr.Value)
	if c.result.Kind != Result || c.result.Element == nil {
		c.report(expr.Span, "operator ? may only be used inside a Result-returning function")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	var value Type
	switch {
	case operand.Kind == Result && operand.Element != nil:
		value = *operand.Element
	case operand.Kind == MultiValue && len(operand.Results) == 2 && c.isAssignable(builtins["error"], operand.Results[1]):
		value = operand.Results[0]
	case c.isAssignable(builtins["error"], operand):
		value = builtins["void"]
	default:
		if operand.Kind != Invalid {
			c.report(expr.Value.GetSpan(), fmt.Sprintf("operator ? requires Result<T>, (T, error), or error; got %s", operand.String()))
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	c.prepareGoTypeForEmission(&value, expr.Span)
	resultElement := *c.result.Element
	c.prepareGoTypeForEmission(&resultElement, expr.Span)
	expr.ValueType = typeRefFromType(value, expr.Span)
	expr.ResultType = typeRefFromType(resultElement, expr.Span)
	expr.ErrorName = fmt.Sprintf("__ontama_result_error_%d", expr.Span.Start.Offset)
	return value
}

func (c *Checker) rejectResultValueType(value Type, span source.Span, context string) {
	if containsResultType(value) {
		c.report(span, fmt.Sprintf("Result may only be used as a function or method return type, not for %s", context))
	}
}

func (c *Checker) rejectTaskAPIType(value Type, span source.Span, context string) {
	if containsTaskType(value) {
		c.report(span, fmt.Sprintf("%s cannot contain Task; Task is a non-escaping local capability", context))
	}
}

func containsTaskType(value Type) bool {
	return containsTaskTypeSeen(value, map[string]bool{})
}

func containsTaskTypeSeen(value Type, visiting map[string]bool) bool {
	if value.Kind == Task {
		return true
	}
	if value.Kind == Struct {
		if visiting[value.Name] {
			return false
		}
		visiting[value.Name] = true
		defer delete(visiting, value.Name)
	}
	if value.Element != nil && containsTaskTypeSeen(*value.Element, visiting) {
		return true
	}
	if value.Key != nil && containsTaskTypeSeen(*value.Key, visiting) {
		return true
	}
	if value.Result != nil && containsTaskTypeSeen(*value.Result, visiting) {
		return true
	}
	for _, parameter := range value.Parameters {
		if containsTaskTypeSeen(parameter, visiting) {
			return true
		}
	}
	if value.Kind == Object || value.Kind == Struct {
		for _, field := range value.Fields {
			if containsTaskTypeSeen(field, visiting) {
				return true
			}
		}
	}
	return false
}

func containsResultType(value Type) bool {
	return containsResultTypeSeen(value, map[string]bool{})
}

func containsResultTypeSeen(value Type, visiting map[string]bool) bool {
	if value.Kind == Task {
		return false
	}
	if value.Kind == Result {
		return true
	}
	if value.Kind == Struct {
		if visiting[value.Name] {
			return false
		}
		visiting[value.Name] = true
		defer delete(visiting, value.Name)
	}
	if value.Element != nil && containsResultTypeSeen(*value.Element, visiting) {
		return true
	}
	if value.Key != nil && containsResultTypeSeen(*value.Key, visiting) {
		return true
	}
	if value.Result != nil && containsResultTypeSeen(*value.Result, visiting) {
		return true
	}
	for _, parameter := range value.Parameters {
		if containsResultTypeSeen(parameter, visiting) {
			return true
		}
	}
	if value.Kind == Object || value.Kind == Struct {
		for _, field := range value.Fields {
			if containsResultTypeSeen(field, visiting) {
				return true
			}
		}
	}
	return false
}

func (c *Checker) resolveTypeThroughNativeIndirection(ref ast.TypeRef) Type {
	c.nativeTypeIndirectionDepth++
	defer func() { c.nativeTypeIndirectionDepth-- }()
	return c.resolveType(ref)
}

func (c *Checker) resolveType(ref ast.TypeRef) Type {
	if ref.Nullable {
		baseRef := ref
		baseRef.Nullable = false
		base := c.resolveType(baseRef)
		if base.Kind == Invalid {
			return base
		}
		if base.Kind == Nullable {
			c.report(ref.Span, "nullable types cannot be nested")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if !isNullableBaseType(base) {
			c.report(ref.Span, fmt.Sprintf("type %s cannot be nullable because it has no nil representation", base.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return Type{Kind: Nullable, Name: "nullable", Element: &base}
	}
	if ref.IsPointer() {
		pointee := c.resolveTypeThroughNativeIndirection(*ref.Pointee)
		if pointee.Kind == Invalid {
			return pointee
		}
		if containsTaskType(pointee) {
			c.report(ref.Pointee.Span, "Task cannot be nested inside a pointer type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		goType, ok := goTypeOf(pointee)
		if !ok && pointee.Kind != Struct && pointee.Kind != FixedArray {
			c.report(ref.Span, fmt.Sprintf("type %s cannot be used as a Go pointer target", pointee.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		var pointerType gotypes.Type
		if ok {
			pointerType = gotypes.NewPointer(goType)
		}
		return Type{Kind: GoPointer, Name: "*" + pointee.String(), Element: &pointee, GoType: pointerType, GoQualifier: pointee.GoQualifier}
	}
	if ref.IsArray() {
		element := Type{}
		if ref.IsSlice() {
			element = c.resolveTypeThroughNativeIndirection(*ref.Element)
		} else {
			element = c.resolveType(*ref.Element)
		}
		if containsTaskType(element) {
			c.report(ref.Element.Span, "Task cannot be nested inside an array type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if containsResultType(element) {
			c.report(ref.Element.Span, "Result cannot be nested inside an array type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if ref.IsFixedArray() {
			if element.Kind == Invalid {
				return element
			}
			elementGoType, ok := goTypeOf(element)
			if !ok && element.Kind != Struct && element.Kind != FixedArray {
				c.report(ref.Element.Span, fmt.Sprintf("type %s cannot be used as a fixed array element", element.String()))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			var arrayType gotypes.Type
			if ok {
				arrayType = gotypes.NewArray(elementGoType, *ref.FixedLength)
			}
			return Type{Kind: FixedArray, Name: "fixed array", Element: &element, Length: *ref.FixedLength, GoType: arrayType}
		}
		return Type{Kind: Array, Name: "array", Element: &element}
	}
	if ref.IsFunction() {
		parameters := make([]Type, len(ref.Parameters))
		for i, parameter := range ref.Parameters {
			resolved := c.resolveTypeThroughNativeIndirection(parameter)
			if ref.Variadic && i == len(ref.Parameters)-1 {
				parameters[i] = c.callableParameterType(ast.Parameter{Type: parameter, Variadic: true}, resolved)
			} else {
				parameters[i] = resolved
			}
		}
		result := c.resolveTypeThroughNativeIndirection(*ref.Return)
		for _, parameter := range parameters {
			if containsTaskType(parameter) {
				c.report(ref.Span, "Task is not supported inside a function type")
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			if containsResultType(parameter) {
				c.report(ref.Span, "Result is not supported inside a function type")
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
		}
		if containsResultType(result) {
			c.report(ref.Span, "Result is not supported inside a function type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if containsTaskType(result) {
			c.report(ref.Span, "Task is not supported inside a function type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: ref.Variadic, Result: &result}
	}
	if ref.IsObject() {
		fields := map[string]Type{}
		fieldNames := map[string]string{}
		for _, field := range ref.ObjectFields {
			if _, duplicate := fields[field.Name]; duplicate {
				c.report(field.Span, fmt.Sprintf("duplicate object type field %q", field.Name))
				continue
			}
			fieldType := c.resolveType(field.Type)
			if fieldType.Kind == Void {
				c.report(field.Type.Span, fmt.Sprintf("object field %q cannot have type void", field.Name))
			}
			if containsResultType(fieldType) {
				c.report(field.Type.Span, fmt.Sprintf("object field %q cannot contain Result", field.Name))
				fieldType = Type{Kind: Invalid, Name: "<invalid>"}
			}
			if containsTaskType(fieldType) {
				c.report(field.Type.Span, fmt.Sprintf("object field %q cannot contain Task", field.Name))
				fieldType = Type{Kind: Invalid, Name: "<invalid>"}
			}
			fields[field.Name] = fieldType
			fieldNames[field.Name] = memberGoName(field.Name, ast.Public)
		}
		return Type{Kind: Object, Name: "object", Fields: fields, FieldNames: fieldNames}
	}
	if ref.Qualifier == "" {
		if len(ref.GenericArguments) == 0 {
			if parameter, ok := c.lookupTypeParameter(ref.Name); ok {
				return parameter
			}
		}
		if named, ok := c.nativeTypes[ref.Name]; ok && c.isTopLevelAllowed(ref.Span, ref.Name) {
			return c.resolveNativeDefinedType(ref, named)
		}
	}
	if ref.Name == "Result" {
		if len(ref.GenericArguments) != 1 {
			c.report(ref.Span, "Result expects one type argument")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		element := c.resolveType(ref.GenericArguments[0])
		if element.Kind == Invalid {
			return element
		}
		if containsTaskType(element) {
			c.report(ref.GenericArguments[0].Span, "Task cannot be nested inside Result")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if element.Kind == Result {
			c.report(ref.GenericArguments[0].Span, "nested Result types are not supported")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if element.Kind == MultiValue || element.Kind == GoPackage || element.Kind == GoTypeName || element.Kind == Nil {
			c.report(ref.GenericArguments[0].Span, fmt.Sprintf("type %s cannot be used as a Result value", element.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return Type{Kind: Result, Name: "Result", Element: &element}
	}
	if ref.Name == "Task" {
		if len(ref.GenericArguments) != 1 {
			c.report(ref.Span, "Task expects one type argument")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		element := c.resolveType(ref.GenericArguments[0])
		if element.Kind == Invalid {
			return element
		}
		if element.Kind == Task || element.Kind == MultiValue || element.Kind == GoPackage || element.Kind == GoTypeName || element.Kind == Nil || element.Kind == Null {
			c.report(ref.GenericArguments[0].Span, fmt.Sprintf("type %s cannot be used as a Task result", element.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return Type{Kind: Task, Name: "Task", Element: &element}
	}
	if ref.Name == "Map" {
		if len(ref.GenericArguments) != 2 {
			c.report(ref.Span, "Map expects two type arguments")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		key := c.resolveTypeThroughNativeIndirection(ref.GenericArguments[0])
		value := c.resolveTypeThroughNativeIndirection(ref.GenericArguments[1])
		if containsTaskType(key) || containsTaskType(value) {
			c.report(ref.Span, "Task cannot be nested inside a Map type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if containsResultType(key) || containsResultType(value) {
			c.report(ref.Span, "Result cannot be nested inside a Map type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if key.Kind != Invalid && !key.IsComparable() {
			c.report(ref.GenericArguments[0].Span, fmt.Sprintf("type %s cannot be used as a Map key", key.String()))
		}
		return Type{Kind: Map, Name: "Map", Key: &key, Element: &value}
	}
	if ref.Name == "GoChannel" || ref.Name == "GoSendChannel" || ref.Name == "GoReceiveChannel" {
		if len(ref.GenericArguments) != 1 {
			c.report(ref.Span, fmt.Sprintf("%s expects one type argument", ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		element := c.resolveTypeThroughNativeIndirection(ref.GenericArguments[0])
		if containsTaskType(element) {
			c.report(ref.GenericArguments[0].Span, "Task cannot be used as a Go channel element")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		elementGoType, ok := goTypeOf(element)
		if !ok {
			c.report(ref.GenericArguments[0].Span, fmt.Sprintf("type %s cannot be used as a Go channel element", element.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		direction := gotypes.SendRecv
		if ref.Name == "GoSendChannel" {
			direction = gotypes.SendOnly
		} else if ref.Name == "GoReceiveChannel" {
			direction = gotypes.RecvOnly
		}
		return Type{Kind: GoChannel, Name: ref.Name, Element: &element, GoType: gotypes.NewChan(direction, elementGoType), GoQualifier: element.GoQualifier}
	}
	if ref.Qualifier == "" {
		if symbol, ok := c.structs[ref.Name]; ok && c.isTopLevelAllowed(ref.Span, ref.Name) {
			return c.resolveNativeStructType(ref, symbol)
		}
	}
	if ref.Qualifier != "" {
		imported := c.lookupGoPackage(ref.Span.Path, ref.Qualifier)
		if imported == nil || imported.packageInfo == nil {
			c.report(ref.Span, fmt.Sprintf("unknown Go package alias %q", ref.Qualifier))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		object := imported.packageInfo.Scope().Lookup(ref.Name)
		typeName, ok := object.(*gotypes.TypeName)
		if !ok || !typeName.Exported() {
			c.report(ref.Span, fmt.Sprintf("Go package %q has no exported type %q", imported.path, ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		goType := typeName.Type()
		if len(ref.GenericArguments) != 0 {
			parameterCount := goTypeParameterCount(goType)
			if parameterCount == 0 {
				c.report(ref.Span, fmt.Sprintf("Go type %s.%s is not generic", ref.Qualifier, ref.Name))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			if len(ref.GenericArguments) != parameterCount {
				c.report(ref.Span, fmt.Sprintf("Go type %s.%s expects %d type arguments, got %d", ref.Qualifier, ref.Name, parameterCount, len(ref.GenericArguments)))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			typeArguments := make([]gotypes.Type, len(ref.GenericArguments))
			valid := true
			for i := range ref.GenericArguments {
				resolved := c.resolveType(ref.GenericArguments[i])
				argument, ok := goTypeOf(resolved)
				if !ok {
					c.report(ref.GenericArguments[i].Span, fmt.Sprintf("type argument %s cannot be represented as a Go type", resolved.String()))
					valid = false
					continue
				}
				typeArguments[i] = argument
			}
			if !valid {
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			instantiated, instantiateErr := gotypes.Instantiate(nil, goType, typeArguments, true)
			if instantiateErr != nil {
				c.report(ref.Span, fmt.Sprintf("cannot instantiate Go type %s.%s: %v", ref.Qualifier, ref.Name, instantiateErr))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			goType = instantiated
		}
		result, err := ontamaTypeFromGo(goType)
		if err != nil {
			c.report(ref.Span, fmt.Sprintf("Go type %s.%s is not supported: %v", ref.Qualifier, ref.Name, err))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if !c.allowUnsafeGo && goTypeContainsUnsafePointer(goType, nil) {
			c.report(ref.Span, fmt.Sprintf(`Go type %s.%s uses unsafe.Pointer; set [go.interop] unsafe = "allow" to use it`, ref.Qualifier, ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		imported.declaration.Used = true
		alias := imported.declaration.Alias
		if imported.declaration.ResolvedAlias != "" {
			alias = imported.declaration.ResolvedAlias
		}
		applyGoQualifier(&result, imported.path, alias)
		return result
	}
	if contract, ok := c.interfaces[ref.Name]; ok && c.isTopLevelAllowed(ref.Span, ref.Name) {
		return c.resolveNativeInterfaceType(ref, contract)
	}
	if class, ok := c.classes[ref.Name]; ok && c.isTopLevelAllowed(ref.Span, ref.Name) {
		return c.resolveNativeClassType(ref, class)
	}
	if len(ref.GenericArguments) != 0 {
		c.report(ref.Span, fmt.Sprintf("generic type %q is not supported", ref.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if t, ok := LookupType(ref.Name); ok {
		return t
	}
	c.report(ref.Span, fmt.Sprintf("unknown type %q", ref.Name))
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) resolveNativeClassType(ref ast.TypeRef, symbol *classSymbol) Type {
	want := len(symbol.typeParameters)
	got := len(ref.GenericArguments)
	if want == 0 {
		if got != 0 {
			c.report(ref.Span, fmt.Sprintf("class %s is not generic", ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return Type{Kind: Class, Name: ref.Name}
	}
	if got != want {
		c.report(ref.Span, fmt.Sprintf("generic class %s expects %d type arguments, got %d", ref.Name, want, got))
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
			c.report(ref.GenericArguments[index].Span, fmt.Sprintf("type %s cannot be used as a generic class type argument", argument.String()))
			valid = false
		}
	}
	if !valid || !c.validateNativeTypeArguments(symbol.typeParameters, arguments, ref.GenericArguments, ref.Span, "generic class "+ref.Name) {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return Type{Kind: Class, Name: ref.Name, TypeArguments: arguments}
}

func nativeClassBindings(symbol *classSymbol, instantiated Type) map[string]Type {
	if symbol == nil || len(symbol.typeParameters) == 0 || len(symbol.typeParameters) != len(instantiated.TypeArguments) {
		return nil
	}
	bindings := make(map[string]Type, len(symbol.typeParameters))
	for index, parameter := range symbol.typeParameters {
		bindings[parameter.Name] = instantiated.TypeArguments[index]
	}
	return bindings
}

func (c *Checker) resolveNativeStructType(ref ast.TypeRef, symbol *structSymbol) Type {
	want := len(symbol.typeParameters)
	got := len(ref.GenericArguments)
	if want == 0 {
		if got != 0 {
			c.report(ref.Span, fmt.Sprintf("struct %s is not generic", ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return symbol.typeInfo
	}
	if got != want {
		c.report(ref.Span, fmt.Sprintf("generic struct %s expects %d type arguments, got %d", ref.Name, want, got))
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
			c.report(ref.GenericArguments[index].Span, fmt.Sprintf("type %s cannot be used as a generic struct type argument", argument.String()))
			valid = false
		}
	}
	if !valid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !c.validateNativeTypeArguments(symbol.typeParameters, arguments, ref.GenericArguments, ref.Span, "generic struct "+ref.Name) {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	bindings := make(map[string]Type, want)
	for index, parameter := range symbol.typeParameters {
		bindings[parameter.Name] = arguments[index]
	}
	result := substituteNativeTypeParameters(symbol.typeInfo, bindings)
	result.TypeParameters = nil
	result.TypeArguments = arguments
	result.Generic = false
	return result
}

func nativeStructBindings(symbol *structSymbol, instantiated Type) map[string]Type {
	if symbol == nil || len(symbol.typeParameters) == 0 || len(symbol.typeParameters) != len(instantiated.TypeArguments) {
		return nil
	}
	bindings := make(map[string]Type, len(symbol.typeParameters))
	for index, parameter := range symbol.typeParameters {
		bindings[parameter.Name] = instantiated.TypeArguments[index]
	}
	return bindings
}

func nativeDefinedTypeBindings(symbol *nativeTypeSymbol, instantiated Type) map[string]Type {
	if symbol == nil || len(symbol.typeParameters) == 0 || len(symbol.typeParameters) != len(instantiated.TypeArguments) {
		return nil
	}
	bindings := make(map[string]Type, len(symbol.typeParameters))
	for index, parameter := range symbol.typeParameters {
		bindings[parameter.Name] = instantiated.TypeArguments[index]
	}
	return bindings
}

func (c *Checker) resolveNativeInterfaceType(ref ast.TypeRef, symbol *interfaceSymbol) Type {
	want := len(symbol.typeParameters)
	got := len(ref.GenericArguments)
	if want == 0 {
		if got != 0 {
			c.report(ref.Span, fmt.Sprintf("interface %s is not generic", ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return Type{Kind: Interface, Name: ref.Name}
	}
	if got != want {
		c.report(ref.Span, fmt.Sprintf("generic interface %s expects %d type arguments, got %d", ref.Name, want, got))
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
			c.report(ref.GenericArguments[index].Span, fmt.Sprintf("type %s cannot be used as a generic interface type argument", argument.String()))
			valid = false
		}
	}
	if !valid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !c.validateNativeTypeArguments(symbol.typeParameters, arguments, ref.GenericArguments, ref.Span, "generic interface "+ref.Name) {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return Type{Kind: Interface, Name: ref.Name, TypeArguments: arguments}
}

func nativeInterfaceBindings(symbol *interfaceSymbol, instantiated Type) map[string]Type {
	if symbol == nil || len(symbol.typeParameters) == 0 || len(symbol.typeParameters) != len(instantiated.TypeArguments) {
		return nil
	}
	bindings := make(map[string]Type, len(symbol.typeParameters))
	for index, parameter := range symbol.typeParameters {
		bindings[parameter.Name] = instantiated.TypeArguments[index]
	}
	return bindings
}

func goTypeParameterCount(goType gotypes.Type) int {
	switch goType := goType.(type) {
	case *gotypes.Named:
		if goType.TypeParams() != nil {
			return goType.TypeParams().Len()
		}
	case *gotypes.Alias:
		if goType.TypeParams() != nil {
			return goType.TypeParams().Len()
		}
	}
	return 0
}

func memberGoName(name string, visibility ast.Visibility) string {
	if visibility != ast.Public {
		return name
	}
	r, size := utf8.DecodeRuneInString(name)
	if size == 0 {
		return name
	}
	return string(unicode.ToUpper(r)) + name[size:]
}

func staticMethodGoName(className, methodName string, visibility ast.Visibility) string {
	if visibility == ast.Public {
		return generatedIdentifier(className + methodName)
	}
	return generatedIdentifier("__ontamaStatic" + className + methodName)
}

func (c *Checker) markResolvedTypeRefs(program *ast.Program) {
	var activeTypeParameters map[string]source.Span
	var visitType func(*ast.TypeRef)
	visitType = func(ref *ast.TypeRef) {
		if ref == nil {
			return
		}
		if ref.Qualifier == "" {
			if declaration, ok := activeTypeParameters[ref.Name]; ok {
				ref.TypeParameter = true
				ref.ResolvedDeclaration = declaration
			} else if named, ok := c.nativeTypes[ref.Name]; ok {
				ref.NativeNamed = true
				ref.ResolvedDeclaration = named.declaration.NameSpan
			} else if contract, ok := c.interfaces[ref.Name]; ok {
				ref.Interface = true
				ref.ResolvedDeclaration = contract.declarationSpan
			} else if class, ok := c.classes[ref.Name]; ok {
				ref.ResolvedDeclaration = class.declarationSpan
			} else if structure, ok := c.structs[ref.Name]; ok {
				ref.Struct = true
				ref.ResolvedDeclaration = structure.declarationSpan
			}
		} else if imported := c.lookupGoPackage(ref.Span.Path, ref.Qualifier); imported != nil {
			ref.QualifierDeclaration = imported.declaration.AliasSpan
		}
		for i := range ref.GenericArguments {
			visitType(&ref.GenericArguments[i])
		}
		visitType(ref.Element)
		visitType(ref.Pointee)
		for i := range ref.Parameters {
			visitType(&ref.Parameters[i])
		}
		visitType(ref.Return)
		for i := range ref.ObjectFields {
			visitType(&ref.ObjectFields[i].Type)
		}
		if ref.Qualifier == "" {
			if named, ok := c.nativeTypes[ref.Name]; ok && named.typeInfo.Kind != Invalid && named.declaration.Alias && len(named.typeParameters) != 0 && len(ref.GenericArguments) == len(named.typeParameters) {
				expanded := instantiateGenericAliasTypeRef(*ref, named.declaration)
				visitType(&expanded)
				ref.LoweredType = &expanded
			}
		}
	}
	visitTypeParameters := func(parameters []ast.TypeParameter) {
		for index := range parameters {
			visitType(parameters[index].Constraint)
		}
	}
	var visitExpression func(ast.Expression)
	var visitStatement func(ast.Statement)
	visitExpression = func(expression ast.Expression) {
		switch expression := expression.(type) {
		case *ast.UnaryExpr:
			visitExpression(expression.Operand)
		case *ast.PropagateExpr:
			visitExpression(expression.Value)
			visitType(&expression.ValueType)
			visitType(&expression.ResultType)
		case *ast.TaskStartExpr:
			visitExpression(expression.Call)
			visitType(&expression.ValueType)
		case *ast.AwaitExpr:
			visitExpression(expression.Value)
			visitType(&expression.ValueType)
		case *ast.BinaryExpr:
			visitExpression(expression.Left)
			visitExpression(expression.Right)
		case *ast.GoTypeAssertionExpr:
			visitExpression(expression.Value)
			visitType(&expression.Type)
		case *ast.CallExpr:
			visitExpression(expression.Callee)
			for i := range expression.TypeArguments {
				visitType(&expression.TypeArguments[i])
			}
			for _, argument := range expression.Arguments {
				visitExpression(argument)
			}
			visitType(expression.ConversionType)
		case *ast.ArrowExpr:
			for i := range expression.Parameters {
				visitType(&expression.Parameters[i].Type)
			}
			visitType(expression.ReturnType)
			visitType(&expression.ResolvedReturnType)
			visitExpression(expression.ExpressionBody)
			if expression.BlockBody != nil {
				visitStatement(expression.BlockBody)
			}
		case *ast.ArrayLiteralExpr:
			visitType(&expression.ResolvedElementType)
			for _, element := range expression.Elements {
				visitExpression(element)
			}
		case *ast.ObjectLiteralExpr:
			for i := range expression.ResolvedFieldTypes {
				visitType(&expression.ResolvedFieldTypes[i])
			}
			for _, field := range expression.Fields {
				visitExpression(field.Value)
			}
		case *ast.GoCompositeLiteralExpr:
			visitType(&expression.Type)
			for _, field := range expression.Fields {
				visitExpression(field.Value)
			}
		case *ast.MemberExpr:
			visitExpression(expression.Object)
		case *ast.IndexExpr:
			visitExpression(expression.Object)
			visitExpression(expression.Index)
		case *ast.SliceExpr:
			visitExpression(expression.Object)
			visitExpression(expression.Low)
			visitExpression(expression.High)
			visitExpression(expression.Max)
		case *ast.NewExpr:
			for index := range expression.TypeArguments {
				visitType(&expression.TypeArguments[index])
			}
			for _, argument := range expression.Arguments {
				visitExpression(argument)
			}
		case *ast.ClassUpcastExpr:
			visitExpression(expression.Value)
		}
	}
	visitStatement = func(statement ast.Statement) {
		if statement == nil {
			return
		}
		switch statement := statement.(type) {
		case *ast.VariableDecl:
			visitType(&statement.Type)
			visitType(&statement.ResolvedType)
			visitExpression(statement.Value)
		case *ast.MultiVariableDecl:
			for i := range statement.Bindings {
				visitType(&statement.Bindings[i].ResolvedType)
			}
			visitExpression(statement.Value)
		case *ast.BlockStmt:
			for _, child := range statement.Statements {
				visitStatement(child)
			}
		case *ast.ReturnStmt:
			visitExpression(statement.Value)
		case *ast.ThrowStmt:
			visitExpression(statement.Value)
		case *ast.TryStmt:
			visitStatement(statement.Body)
			for _, clause := range statement.Catches {
				visitType(&clause.Type)
				visitStatement(clause.Body)
			}
			if statement.FinallyBody != nil {
				visitStatement(statement.FinallyBody)
			}
		case *ast.IfStmt:
			visitExpression(statement.Condition)
			visitStatement(statement.Then)
			if statement.Else != nil {
				visitStatement(statement.Else)
			}
		case *ast.ExpressionStmt:
			visitExpression(statement.Value)
		case *ast.AssignmentStmt:
			visitExpression(statement.Target)
			visitExpression(statement.Value)
		case *ast.IncDecStmt:
			visitExpression(statement.Target)
		case *ast.MultiAssignmentStmt:
			visitExpression(statement.Value)
		case *ast.WhileStmt:
			visitExpression(statement.Condition)
			visitStatement(statement.Body)
		case *ast.ForStmt:
			if statement.Initializer != nil {
				visitStatement(statement.Initializer)
			}
			visitExpression(statement.Condition)
			if statement.Post != nil {
				visitStatement(statement.Post)
			}
			visitStatement(statement.Body)
		case *ast.ForRangeStmt:
			for index := range statement.Bindings {
				visitType(&statement.Bindings[index].Type)
				visitType(&statement.Bindings[index].ResolvedType)
			}
			visitExpression(statement.Source)
			visitStatement(statement.Body)
		case *ast.SelectStmt:
			for i := range statement.Cases {
				clause := &statement.Cases[i]
				for binding := range clause.Bindings {
					visitType(&clause.Bindings[binding].ResolvedType)
				}
				visitExpression(clause.Channel)
				visitExpression(clause.Value)
				for _, target := range clause.Targets {
					visitExpression(target)
				}
				visitStatement(clause.Body)
			}
		case *ast.ValueSwitchStmt:
			visitExpression(statement.Value)
			for i := range statement.Cases {
				clause := &statement.Cases[i]
				for _, value := range clause.Values {
					visitExpression(value)
				}
				visitStatement(clause.Body)
			}
		case *ast.TypeSwitchStmt:
			visitExpression(statement.Value)
			for i := range statement.Cases {
				clause := &statement.Cases[i]
				visitType(&clause.Type)
				visitStatement(clause.Body)
			}
		case *ast.CallControlStmt:
			visitExpression(statement.Value)
		case *ast.DetachStmt:
			visitExpression(statement.Value)
			visitType(&statement.ValueType)
		case *ast.ChannelSendStmt:
			visitExpression(statement.Channel)
			visitExpression(statement.Value)
		}
	}
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.VariableDecl:
			visitType(&declaration.Type)
			visitType(&declaration.ResolvedType)
			visitExpression(declaration.Value)
		case *ast.FunctionDecl:
			activeTypeParameters = make(map[string]source.Span, len(declaration.TypeParameters))
			for _, parameter := range declaration.TypeParameters {
				activeTypeParameters[parameter.Name] = parameter.NameSpan
			}
			visitTypeParameters(declaration.TypeParameters)
			for i := range declaration.Parameters {
				visitType(&declaration.Parameters[i].Type)
			}
			visitType(&declaration.ReturnType)
			visitStatement(declaration.Body)
			activeTypeParameters = nil
		case *ast.MethodDecl:
			activeTypeParameters = make(map[string]source.Span, len(declaration.TypeParameters))
			for _, parameter := range declaration.TypeParameters {
				activeTypeParameters[parameter.Name] = parameter.NameSpan
			}
			visitTypeParameters(declaration.TypeParameters)
			visitType(&declaration.ReceiverType)
			for i := range declaration.Parameters {
				visitType(&declaration.Parameters[i].Type)
			}
			visitType(&declaration.ReturnType)
			visitStatement(declaration.Body)
			activeTypeParameters = nil
		case *ast.ClassDecl:
			activeTypeParameters = make(map[string]source.Span, len(declaration.TypeParameters))
			for _, parameter := range declaration.TypeParameters {
				activeTypeParameters[parameter.Name] = parameter.NameSpan
			}
			visitTypeParameters(declaration.TypeParameters)
			if declaration.Base != nil {
				visitType(declaration.Base)
			}
			for i := range declaration.Implements {
				visitType(&declaration.Implements[i])
			}
			for i := range declaration.Fields {
				visitType(&declaration.Fields[i].Type)
			}
			if declaration.Constructor != nil {
				for i := range declaration.Constructor.Parameters {
					visitType(&declaration.Constructor.Parameters[i].Type)
				}
				visitStatement(declaration.Constructor.Body)
			}
			for _, method := range declaration.Methods {
				for i := range method.Parameters {
					visitType(&method.Parameters[i].Type)
				}
				visitType(&method.ReturnType)
				visitStatement(method.Body)
			}
			activeTypeParameters = nil
		case *ast.InterfaceDecl:
			activeTypeParameters = make(map[string]source.Span, len(declaration.TypeParameters))
			for _, parameter := range declaration.TypeParameters {
				activeTypeParameters[parameter.Name] = parameter.NameSpan
			}
			visitTypeParameters(declaration.TypeParameters)
			for i := range declaration.Methods {
				method := &declaration.Methods[i]
				for j := range method.Parameters {
					visitType(&method.Parameters[j].Type)
				}
				visitType(&method.ReturnType)
			}
			activeTypeParameters = nil
		case *ast.StructDecl:
			activeTypeParameters = make(map[string]source.Span, len(declaration.TypeParameters))
			for _, parameter := range declaration.TypeParameters {
				activeTypeParameters[parameter.Name] = parameter.NameSpan
			}
			visitTypeParameters(declaration.TypeParameters)
			for i := range declaration.Fields {
				visitType(&declaration.Fields[i].Type)
			}
			for _, method := range declaration.Methods {
				for i := range method.Parameters {
					visitType(&method.Parameters[i].Type)
				}
				visitType(&method.ReturnType)
				visitStatement(method.Body)
			}
			activeTypeParameters = nil
		case *ast.TypeDecl:
			activeTypeParameters = make(map[string]source.Span, len(declaration.TypeParameters))
			for _, parameter := range declaration.TypeParameters {
				activeTypeParameters[parameter.Name] = parameter.NameSpan
			}
			visitTypeParameters(declaration.TypeParameters)
			visitType(&declaration.Underlying)
			activeTypeParameters = nil
		case *ast.EnumDecl:
			visitType(&declaration.Underlying)
			for index := range declaration.Members {
				visitExpression(declaration.Members[index].Value)
			}
		}
	}
}

func instantiateGenericAliasTypeRef(ref ast.TypeRef, declaration *ast.TypeDecl) ast.TypeRef {
	bindings := make(map[string]ast.TypeRef, len(declaration.TypeParameters))
	for index, parameter := range declaration.TypeParameters {
		if index < len(ref.GenericArguments) {
			bindings[parameter.Name] = ref.GenericArguments[index]
		}
	}
	return substituteNativeTypeRefParameters(declaration.Underlying, bindings)
}

func substituteNativeTypeRefParameters(ref ast.TypeRef, bindings map[string]ast.TypeRef) ast.TypeRef {
	if ref.Qualifier == "" && !ref.IsArray() && !ref.IsPointer() && !ref.IsFunction() && !ref.IsObject() && !ref.IsGoStruct() && len(ref.GenericArguments) == 0 {
		if replacement, ok := bindings[ref.Name]; ok {
			return replacement
		}
	}
	result := ref
	result.LoweredType = nil
	result.GenericArguments = append([]ast.TypeRef(nil), ref.GenericArguments...)
	for index := range result.GenericArguments {
		result.GenericArguments[index] = substituteNativeTypeRefParameters(result.GenericArguments[index], bindings)
	}
	if ref.Element != nil {
		element := substituteNativeTypeRefParameters(*ref.Element, bindings)
		result.Element = &element
	}
	if ref.Pointee != nil {
		pointee := substituteNativeTypeRefParameters(*ref.Pointee, bindings)
		result.Pointee = &pointee
	}
	result.Parameters = append([]ast.TypeRef(nil), ref.Parameters...)
	for index := range result.Parameters {
		result.Parameters[index] = substituteNativeTypeRefParameters(result.Parameters[index], bindings)
	}
	if ref.Return != nil {
		returnType := substituteNativeTypeRefParameters(*ref.Return, bindings)
		result.Return = &returnType
	}
	result.ObjectFields = append([]ast.ObjectTypeField(nil), ref.ObjectFields...)
	for index := range result.ObjectFields {
		result.ObjectFields[index].Type = substituteNativeTypeRefParameters(result.ObjectFields[index].Type, bindings)
	}
	return result
}

func typeRefFromType(t Type, span source.Span) ast.TypeRef {
	if t.Kind == TypeParameter {
		return ast.TypeRef{Name: t.Name, TypeParameter: true, Span: span}
	}
	if t.Kind == Nullable && t.Element != nil {
		ref := typeRefFromType(*t.Element, span)
		ref.Nullable = true
		ref.Span = span
		return ref
	}
	if t.Kind == GoPointer && t.Element != nil {
		pointee := typeRefFromType(*t.Element, span)
		return ast.TypeRef{Pointee: &pointee, Span: span}
	}
	if t.Kind == Array && t.Element != nil {
		element := typeRefFromType(*t.Element, span)
		return ast.TypeRef{Element: &element, Span: span}
	}
	if t.Kind == FixedArray && t.Element != nil {
		element := typeRefFromType(*t.Element, span)
		length := t.Length
		return ast.TypeRef{Element: &element, FixedLength: &length, Span: span}
	}
	if t.Kind == Map && t.Key != nil && t.Element != nil {
		return ast.TypeRef{Name: "Map", GenericArguments: []ast.TypeRef{typeRefFromType(*t.Key, span), typeRefFromType(*t.Element, span)}, Span: span}
	}
	if t.Kind == Result && t.Element != nil {
		return ast.TypeRef{Name: "Result", GenericArguments: []ast.TypeRef{typeRefFromType(*t.Element, span)}, Span: span}
	}
	if t.Kind == Task && t.Element != nil {
		return ast.TypeRef{Name: "Task", GenericArguments: []ast.TypeRef{typeRefFromType(*t.Element, span)}, Span: span}
	}
	if t.Kind == GoChannel && t.Element != nil {
		name := "GoChannel"
		if channel, ok := gotypes.Unalias(t.GoType).Underlying().(*gotypes.Chan); ok {
			if channel.Dir() == gotypes.SendOnly {
				name = "GoSendChannel"
			} else if channel.Dir() == gotypes.RecvOnly {
				name = "GoReceiveChannel"
			}
		}
		return ast.TypeRef{Name: name, GenericArguments: []ast.TypeRef{typeRefFromType(*t.Element, span)}, Go: true, Span: span}
	}
	if t.Kind == Object {
		names := make([]string, 0, len(t.Fields))
		for name := range t.Fields {
			names = append(names, name)
		}
		sort.Strings(names)
		fields := make([]ast.ObjectTypeField, 0, len(names))
		for _, name := range names {
			goName := t.FieldNames[name]
			if goName == "" {
				goName = name
			}
			fields = append(fields, ast.ObjectTypeField{Name: goName, JSONName: name, Type: typeRefFromType(t.Fields[name], span), Span: span})
		}
		return ast.TypeRef{Object: true, ObjectFields: fields, Span: span}
	}
	if t.Kind == Struct {
		arguments := make([]ast.TypeRef, len(t.TypeArguments))
		for index := range t.TypeArguments {
			arguments[index] = typeRefFromType(t.TypeArguments[index], span)
		}
		return ast.TypeRef{Name: t.Name, GenericArguments: arguments, Struct: true, Span: span}
	}
	if t.Kind == Class {
		arguments := make([]ast.TypeRef, len(t.TypeArguments))
		for index := range t.TypeArguments {
			arguments[index] = typeRefFromType(t.TypeArguments[index], span)
		}
		return ast.TypeRef{Name: t.Name, GenericArguments: arguments, Span: span}
	}
	if t.Kind == GoStruct {
		fields := make([]ast.ObjectTypeField, len(t.GoFields))
		for index, field := range t.GoFields {
			fields[index] = ast.ObjectTypeField{
				Name: field.Name, GoTag: field.Tag, GoEmbedded: field.Embedded,
				Type: typeRefFromType(field.Type, span), Span: span,
			}
		}
		return ast.TypeRef{GoStruct: true, ObjectFields: fields, Go: true, Span: span}
	}
	if t.Kind == Interface {
		arguments := make([]ast.TypeRef, len(t.TypeArguments))
		for index := range t.TypeArguments {
			arguments[index] = typeRefFromType(t.TypeArguments[index], span)
		}
		return ast.TypeRef{Name: t.Name, GenericArguments: arguments, Interface: true, Span: span}
	}
	if object := goTypeNameObject(t.GoType); object != nil {
		name := t.Name
		name = object.Name()
		arguments := make([]ast.TypeRef, len(t.TypeArguments))
		for i := range t.TypeArguments {
			arguments[i] = typeRefFromType(t.TypeArguments[i], span)
		}
		return ast.TypeRef{Name: name, Qualifier: t.GoQualifier, GenericArguments: arguments, Go: true, Span: span}
	}
	if t.Kind == GoBasic {
		return ast.TypeRef{Name: t.Name, Qualifier: t.GoQualifier, Go: true, Span: span}
	}
	if t.Kind != Function {
		return ast.TypeRef{Name: t.Name, Span: span}
	}
	parameters := make([]ast.TypeRef, len(t.Parameters))
	for i, parameter := range t.Parameters {
		parameters[i] = typeRefFromType(parameter, span)
		if t.Variadic && i == len(t.Parameters)-1 {
			element := parameters[i]
			parameters[i] = ast.TypeRef{Element: &element, Span: span}
		}
	}
	result := typeRefFromType(*t.Result, span)
	return ast.TypeRef{Parameters: parameters, Return: &result, Variadic: t.Variadic, Span: span}
}

func goTypeNameObject(goType gotypes.Type) *gotypes.TypeName {
	switch goType := goType.(type) {
	case *gotypes.Named:
		return goType.Obj()
	case *gotypes.Alias:
		return goType.Obj()
	default:
		return nil
	}
}

func (c *Checker) requireAssignable(target, value Type, span source.Span) {
	if value.Kind == GoPackage {
		c.report(span, "a Go package namespace cannot be used as a value")
		return
	}
	if value.Kind == GoTypeName {
		c.report(span, fmt.Sprintf("Go type %s cannot be used as a value", value.String()))
		return
	}
	if value.Kind == MultiValue {
		c.report(span, fmt.Sprintf("multiple values %s require destructuring", value.String()))
		return
	}
	if value.Kind == Result {
		c.report(span, "Result values must be consumed with ?, explicitly split, or returned")
		return
	}
	if !c.isAssignable(target, value) {
		c.report(span, fmt.Sprintf("cannot use %s as %s", value.String(), target.String()))
	}
}

func (c *Checker) inferredVariableType(value Type, span source.Span) Type {
	switch value.Kind {
	case Nil, Null:
		c.report(span, "cannot infer a variable type from nil or null; add an explicit nilable or nullable type")
		return Type{Kind: Invalid, Name: "<invalid>"}
	case GoPackage, GoTypeName:
		return value
	case MultiValue:
		c.report(span, fmt.Sprintf("multiple values %s require destructuring", value.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	case Result:
		c.report(span, "Result values must be consumed with ?, explicitly split, or returned")
		return Type{Kind: Invalid, Name: "<invalid>"}
	default:
		return defaultLiteralType(value)
	}
}

func (c *Checker) isAssignable(target, value Type) bool {
	if target.Kind == Invalid || value.Kind == Invalid {
		return true
	}
	if target.Kind == Nullable && target.Element != nil {
		if value.Kind == Null {
			return true
		}
		if value.Kind == Nil {
			return false
		}
		if value.Kind == Nullable {
			return value.Element != nil && c.isAssignable(*target.Element, *value.Element)
		}
		return c.isAssignable(*target.Element, value)
	}
	if value.Kind == Nullable || value.Kind == Null || target.Kind == Null {
		return value.Kind == Null && target.Kind == Null
	}
	if target.Kind == Interface && value.Kind == Class {
		class := c.classes[value.Name]
		if class == nil {
			return false
		}
		if class.implements[target.String()] {
			return true
		}
		bindings := nativeClassBindings(class, value)
		for _, implemented := range class.implementedTypes {
			if exactType(target, substituteNativeTypeParameters(implemented, bindings)) {
				return true
			}
		}
		return false
	}
	if target.Kind == Class && value.Kind == Class {
		if ancestor, ok := c.classAncestorType(value, target.Name); ok {
			return exactType(target, ancestor)
		}
	}
	if value.Kind == Struct {
		if contract := underlyingGoInterface(target.GoType); contract != nil && contract.NumMethods() == 0 {
			return true
		}
	}
	if value.Kind == Class && underlyingGoInterface(target.GoType) != nil {
		class := c.classes[value.Name]
		if class == nil {
			return false
		}
		for _, declared := range class.goImplements {
			if gotypes.AssignableTo(declared, target.GoType) || gotypes.Identical(declared, target.GoType) {
				return true
			}
		}
		return false
	}
	return assignable(target, value)
}

func exactType(left, right Type) bool { return assignable(left, right) && assignable(right, left) }

func (c *Checker) declareLocal(name string, t Type, constant bool, declaration *ast.VariableDecl, span source.Span) {
	scope := c.scopes[len(c.scopes)-1]
	if _, exists := scope[name]; exists {
		c.report(span, fmt.Sprintf("duplicate local name %q", name))
		return
	}
	declarationSpan := declarationNameSpan(name, span)
	if declaration != nil {
		declarationSpan = declaration.NameSpan
	} else if name == "this" {
		declarationSpan = source.Span{}
	}
	taskState := uint8(taskNotTracked)
	if t.Kind == Task {
		taskState = taskPending
	}
	scope[name] = valueSymbol{typeInfo: t, declaredType: t, constant: constant, declarationSpan: declarationSpan, declaration: declaration, taskState: taskState}
	if isBuiltinTypeName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a built-in type", name))
	} else if isBuiltinValueName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a compiler built-in", name))
	}
}

func (c *Checker) declareMultiLocal(name string, t Type, constant bool, declaration *ast.MultiVariableDecl, index int, span source.Span) {
	scope := c.scopes[len(c.scopes)-1]
	if _, exists := scope[name]; exists {
		c.report(span, fmt.Sprintf("duplicate local name %q", name))
		return
	}
	scope[name] = valueSymbol{typeInfo: t, declaredType: t, constant: constant, declarationSpan: span, multiDeclaration: declaration, multiIndex: index}
	if isBuiltinTypeName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a built-in type", name))
	} else if isBuiltinValueName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a compiler built-in", name))
	}
}

func (c *Checker) declareCatchLocal(clause *ast.CatchClause, catchType Type) {
	scope := c.scopes[len(c.scopes)-1]
	name := clause.Name
	span := clause.NameSpan
	if _, exists := scope[name]; exists {
		c.report(span, fmt.Sprintf("duplicate local name %q", name))
		return
	}
	scope[name] = valueSymbol{
		typeInfo: catchType, declaredType: catchType, constant: true,
		declarationSpan: span, catchClause: clause,
	}
	if isBuiltinTypeName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a built-in type", name))
	} else if isBuiltinValueName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a compiler built-in", name))
	}
}

func (c *Checker) declareRangeLocal(binding *ast.RangeBinding, t Type, constant bool) {
	scope := c.scopes[len(c.scopes)-1]
	if _, exists := scope[binding.Name]; exists {
		c.report(binding.NameSpan, fmt.Sprintf("duplicate local name %q", binding.Name))
		return
	}
	scope[binding.Name] = valueSymbol{typeInfo: t, declaredType: t, constant: constant, declarationSpan: binding.NameSpan, rangeBinding: binding}
	if isBuiltinTypeName(binding.Name) {
		c.report(binding.NameSpan, fmt.Sprintf("local name %q conflicts with a built-in type", binding.Name))
	} else if isBuiltinValueName(binding.Name) {
		c.report(binding.NameSpan, fmt.Sprintf("local name %q conflicts with a compiler built-in", binding.Name))
	}
}

func (c *Checker) declareSelectLocal(name string, t Type, constant bool, clause *ast.SelectCase, index int, span source.Span) {
	scope := c.scopes[len(c.scopes)-1]
	if _, exists := scope[name]; exists {
		c.report(span, fmt.Sprintf("duplicate local name %q", name))
		return
	}
	scope[name] = valueSymbol{typeInfo: t, declaredType: t, constant: constant, declarationSpan: span, selectCase: clause, selectIndex: index}
	if isBuiltinTypeName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a built-in type", name))
	} else if isBuiltinValueName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a compiler built-in", name))
	}
}

func (c *Checker) declareTypeSwitchLocal(name string, t Type, constant bool, clause *ast.TypeSwitchCase, span source.Span) {
	scope := c.scopes[len(c.scopes)-1]
	if _, exists := scope[name]; exists {
		c.report(span, fmt.Sprintf("duplicate local name %q", name))
		return
	}
	scope[name] = valueSymbol{typeInfo: t, declaredType: t, constant: constant, declarationSpan: span, typeSwitchCase: clause}
	if isBuiltinTypeName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a built-in type", name))
	} else if isBuiltinValueName(name) {
		c.report(span, fmt.Sprintf("local name %q conflicts with a compiler built-in", name))
	}
}

func declarationNameSpan(name string, span source.Span) source.Span {
	if span.Path == "" {
		return source.Span{}
	}
	result := span
	result.End.Offset = result.Start.Offset + len(name)
	result.End.Line = result.Start.Line
	result.End.Column = result.Start.Column + utf8.RuneCountInString(name)
	return result
}

func isBuiltinValueName(name string) bool {
	return name == "goChannel" || name == "closeGoChannel" || strings.HasPrefix(name, "__ontama_")
}

func isBuiltinTypeName(name string) bool {
	if name == "Result" || name == "Task" {
		return true
	}
	_, builtin := LookupType(name)
	return builtin
}

func (c *Checker) hasCallBinding(name string, span source.Span) bool {
	if _, exists := c.lookupValue(name, span); exists {
		return true
	}
	if _, exists := c.functions[name]; exists && c.isTopLevelAllowed(span, name) {
		return true
	}
	return c.lookupGoPackage(span.Path, name) != nil
}

func (c *Checker) lookupValue(name string, span source.Span) (Type, bool) {
	symbol, ok := c.lookupSymbol(name, span)
	return symbol.typeInfo, ok
}

func (c *Checker) lookupSymbol(name string, span source.Span) (valueSymbol, bool) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if symbol, ok := c.scopes[i][name]; ok {
			if symbol.declaration != nil {
				symbol.declaration.Used = true
			}
			if symbol.multiDeclaration != nil {
				symbol.multiDeclaration.Bindings[symbol.multiIndex].Used = true
			}
			if symbol.rangeBinding != nil {
				symbol.rangeBinding.Used = true
			}
			if symbol.selectCase != nil {
				symbol.selectCase.Bindings[symbol.selectIndex].Used = true
			}
			if symbol.typeSwitchCase != nil {
				symbol.typeSwitchCase.Used = true
			}
			if symbol.catchClause != nil {
				symbol.catchClause.Used = true
			}
			return symbol, true
		}
	}
	if !c.isTopLevelAllowed(span, name) {
		return valueSymbol{}, false
	}
	symbol, ok := c.globals[name]
	return symbol, ok
}

func (c *Checker) lookupAssignmentSymbol(name string, span source.Span) (valueSymbol, bool) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if symbol, ok := c.scopes[i][name]; ok {
			if symbol.rangeBinding != nil {
				symbol.rangeBinding.Assigned = true
			}
			return symbol, true
		}
	}
	if !c.isTopLevelAllowed(span, name) {
		return valueSymbol{}, false
	}
	symbol, ok := c.globals[name]
	return symbol, ok
}

func (c *Checker) isTopLevelAllowed(span source.Span, name string) bool {
	if name == "Exception" {
		return true
	}
	if c.allowed == nil {
		return true
	}
	allowed, exists := c.allowed[span.Path]
	return exists && allowed[name]
}

func (c *Checker) declareGoPackages(program *ast.Program) {
	for i := range program.Imports {
		declaration := &program.Imports[i]
		if !declaration.Go {
			continue
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
		if _, duplicate := byAlias[alias]; duplicate {
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
		byAlias[alias] = &goPackageSymbol{path: declaration.Path, declaration: declaration, packageInfo: packageInfo}
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
		result, err = ontamaTypeFromGo(object.Type())
		expression.Constant = true
	case *gotypes.Func:
		result, err = ontamaFunctionFromGo(object.Type().(*gotypes.Signature))
	case *gotypes.Var:
		result, err = ontamaTypeFromGo(object.Type())
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
		result, err = ontamaTypeFromGo(object.Type())
		expression.Addressable = addressable || indirect || goTypeIsPointer(receiver.GoType)
		expression.GoField = true
		expression.GoFieldViaPointer = goFieldEmbeddedViaPointer(receiver.GoType, index)
	case *gotypes.Func:
		result, err = ontamaFunctionFromGo(object.Type().(*gotypes.Signature))
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
			for alias, imported := range c.goPackages[span.Path] {
				if imported.path != packagePath {
					continue
				}
				applyGoQualifier(t, packagePath, alias)
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

func ontamaFunctionFromGo(signature *gotypes.Signature) (Type, error) {
	return ontamaFunctionFromGoSeen(signature, map[gotypes.Type]bool{})
}

func ontamaFunctionFromGoSeen(signature *gotypes.Signature, visiting map[gotypes.Type]bool) (Type, error) {
	if signature.TypeParams() != nil && signature.TypeParams().Len() != 0 {
		return Type{Kind: Function, Name: "generic function", Generic: true, GoType: signature, Result: &Type{Kind: Invalid, Name: "<generic result>"}}, nil
	}
	parameters := make([]Type, signature.Params().Len())
	for i := range parameters {
		parameterType := signature.Params().At(i).Type()
		if signature.Variadic() && i == len(parameters)-1 {
			slice, ok := gotypes.Unalias(parameterType).(*gotypes.Slice)
			if !ok {
				return Type{}, fmt.Errorf("variadic parameter %d has unexpected Go type %s", i+1, parameterType.String())
			}
			parameterType = slice.Elem()
		}
		converted, err := ontamaTypeFromGoSeen(parameterType, visiting)
		if err != nil {
			return Type{}, fmt.Errorf("parameter %d: %w", i+1, err)
		}
		parameters[i] = converted
	}
	var result Type
	switch signature.Results().Len() {
	case 0:
		result = builtins["void"]
	case 1:
		converted, err := ontamaTypeFromGoSeen(signature.Results().At(0).Type(), visiting)
		if err != nil {
			return Type{}, fmt.Errorf("result: %w", err)
		}
		result = converted
	default:
		results := make([]Type, signature.Results().Len())
		for i := range results {
			converted, err := ontamaTypeFromGoSeen(signature.Results().At(i).Type(), visiting)
			if err != nil {
				return Type{}, fmt.Errorf("result %d: %w", i+1, err)
			}
			results[i] = converted
		}
		result = Type{Kind: MultiValue, Name: "multiple values", Results: results}
	}
	return Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: signature.Variadic(), Result: &result, GoType: signature}, nil
}

func ontamaTypeFromGo(goType gotypes.Type) (Type, error) {
	return ontamaTypeFromGoSeen(goType, map[gotypes.Type]bool{})
}

func ontamaTypeFromGoSeen(goType gotypes.Type, visiting map[gotypes.Type]bool) (Type, error) {
	if parameter, ok := goType.(*gotypes.TypeParam); ok {
		return Type{Kind: TypeParameter, Name: parameter.Obj().Name(), GoType: parameter}, nil
	}
	if visiting[goType] {
		return Type{Kind: GoNamed, Name: goTypeDisplayName(goType), GoType: goType}, nil
	}
	visiting[goType] = true
	defer delete(visiting, goType)
	switch goType := goType.(type) {
	case *gotypes.Basic:
		switch goType.Kind() {
		case gotypes.Bool, gotypes.UntypedBool:
			return builtins["boolean"], nil
		case gotypes.String, gotypes.UntypedString:
			return builtins["string"], nil
		case gotypes.Int:
			return builtins["int"], nil
		case gotypes.Int8:
			return builtins["int8"], nil
		case gotypes.Int16:
			return builtins["int16"], nil
		case gotypes.Int32, gotypes.UntypedRune:
			return builtins["int32"], nil
		case gotypes.Int64:
			return builtins["int64"], nil
		case gotypes.Uint:
			return builtins["uint"], nil
		case gotypes.Uint8:
			return builtins["byte"], nil
		case gotypes.Uint16:
			return builtins["uint16"], nil
		case gotypes.Uint32:
			return builtins["uint32"], nil
		case gotypes.Uint64:
			return builtins["uint64"], nil
		case gotypes.Float32:
			return builtins["float32"], nil
		case gotypes.Float64, gotypes.UntypedFloat:
			return builtins["float"], nil
		case gotypes.UntypedInt:
			return Type{Kind: UntypedInt, Name: "integer literal"}, nil
		case gotypes.Uintptr, gotypes.UnsafePointer,
			gotypes.Complex64, gotypes.Complex128, gotypes.UntypedComplex:
			return Type{Kind: GoBasic, Name: goType.Name(), GoType: goType}, nil
		default:
			return Type{}, fmt.Errorf("Go type %s is not supported", goType.String())
		}
	case *gotypes.Named:
		arguments, err := ontamaTypeArgumentsFromGoSeen(goType.TypeParams(), goType.TypeArgs(), visiting)
		if err != nil {
			return Type{}, err
		}
		if signature, ok := goType.Underlying().(*gotypes.Signature); ok {
			converted, err := ontamaFunctionFromGoSeen(signature, visiting)
			if err != nil {
				return Type{}, err
			}
			converted.Name = goTypeDisplayName(goType)
			converted.GoType = goType
			converted.TypeArguments = arguments
			return converted, nil
		}
		return Type{Kind: GoNamed, Name: goTypeDisplayName(goType), GoType: goType, TypeArguments: arguments}, nil
	case *gotypes.Alias:
		arguments, err := ontamaTypeArgumentsFromGoSeen(goType.TypeParams(), goType.TypeArgs(), visiting)
		if err != nil {
			return Type{}, err
		}
		unalias := gotypes.Unalias(goType)
		if signature, ok := unalias.Underlying().(*gotypes.Signature); ok {
			converted, err := ontamaFunctionFromGoSeen(signature, visiting)
			if err != nil {
				return Type{}, err
			}
			converted.Name = goTypeDisplayName(goType)
			converted.GoType = goType
			converted.TypeArguments = arguments
			return converted, nil
		}
		return Type{Kind: GoNamed, Name: goTypeDisplayName(goType), GoType: goType, TypeArguments: arguments}, nil
	case *gotypes.Signature:
		converted, err := ontamaFunctionFromGoSeen(goType, visiting)
		if err != nil {
			return Type{}, err
		}
		converted.GoType = goType
		return converted, nil
	case *gotypes.Pointer:
		element, err := ontamaTypeFromGoSeen(goType.Elem(), visiting)
		if err != nil {
			return Type{}, err
		}
		return Type{Kind: GoPointer, Name: "*" + element.String(), Element: &element, GoType: goType}, nil
	case *gotypes.Slice:
		element, err := ontamaTypeFromGoSeen(goType.Elem(), visiting)
		if err != nil {
			return Type{}, err
		}
		return Type{Kind: Array, Name: "array", Element: &element}, nil
	case *gotypes.Array:
		element, err := ontamaTypeFromGoSeen(goType.Elem(), visiting)
		if err != nil {
			return Type{}, err
		}
		return Type{Kind: FixedArray, Name: "fixed array", Element: &element, Length: goType.Len(), GoType: goType}, nil
	case *gotypes.Map:
		key, err := ontamaTypeFromGoSeen(goType.Key(), visiting)
		if err != nil {
			return Type{}, err
		}
		value, err := ontamaTypeFromGoSeen(goType.Elem(), visiting)
		if err != nil {
			return Type{}, err
		}
		return Type{Kind: Map, Name: "Map", Key: &key, Element: &value}, nil
	case *gotypes.Chan:
		element, err := ontamaTypeFromGoSeen(goType.Elem(), visiting)
		if err != nil {
			return Type{}, err
		}
		return Type{Kind: GoChannel, Name: "Go channel", Element: &element, GoType: goType}, nil
	case *gotypes.Struct:
		fields := make([]GoStructField, goType.NumFields())
		for index := 0; index < goType.NumFields(); index++ {
			field := goType.Field(index)
			converted, err := ontamaTypeFromGoSeen(field.Type(), visiting)
			if err != nil {
				return Type{}, fmt.Errorf("field %s: %w", field.Name(), err)
			}
			fields[index] = GoStructField{Name: field.Name(), Type: converted, Tag: goType.Tag(index), Embedded: field.Embedded()}
		}
		return Type{Kind: GoStruct, Name: goTypeDisplayName(goType), GoType: goType, GoFields: fields}, nil
	default:
		return Type{}, fmt.Errorf("Go type %s is not supported", goType.String())
	}
}

func goTypeContainsUnsafePointer(goType gotypes.Type, seen map[gotypes.Type]bool) bool {
	if goType == nil {
		return false
	}
	if seen == nil {
		seen = map[gotypes.Type]bool{}
	}
	if seen[goType] {
		return false
	}
	seen[goType] = true
	switch typed := goType.(type) {
	case *gotypes.Basic:
		return typed.Kind() == gotypes.UnsafePointer
	case *gotypes.Alias:
		return goTypeContainsUnsafePointer(gotypes.Unalias(typed), seen) || goTypeListContainsUnsafe(typed.TypeArgs(), seen)
	case *gotypes.Named:
		if goTypeListContainsUnsafe(typed.TypeArgs(), seen) {
			return true
		}
		_, basicUnderlying := typed.Underlying().(*gotypes.Basic)
		return basicUnderlying && goTypeContainsUnsafePointer(typed.Underlying(), seen)
	case *gotypes.Pointer:
		return goTypeContainsUnsafePointer(typed.Elem(), seen)
	case *gotypes.Slice:
		return goTypeContainsUnsafePointer(typed.Elem(), seen)
	case *gotypes.Array:
		return goTypeContainsUnsafePointer(typed.Elem(), seen)
	case *gotypes.Map:
		return goTypeContainsUnsafePointer(typed.Key(), seen) || goTypeContainsUnsafePointer(typed.Elem(), seen)
	case *gotypes.Chan:
		return goTypeContainsUnsafePointer(typed.Elem(), seen)
	case *gotypes.Signature:
		return goTupleContainsUnsafe(typed.Params(), seen) || goTupleContainsUnsafe(typed.Results(), seen)
	case *gotypes.Tuple:
		return goTupleContainsUnsafe(typed, seen)
	case *gotypes.Interface:
		for index := 0; index < typed.NumExplicitMethods(); index++ {
			if goTypeContainsUnsafePointer(typed.ExplicitMethod(index).Type(), seen) {
				return true
			}
		}
	case *gotypes.TypeParam:
		return goTypeContainsUnsafePointer(typed.Constraint(), seen)
	}
	return false
}

func goTupleContainsUnsafe(tuple *gotypes.Tuple, seen map[gotypes.Type]bool) bool {
	if tuple == nil {
		return false
	}
	for index := 0; index < tuple.Len(); index++ {
		if goTypeContainsUnsafePointer(tuple.At(index).Type(), seen) {
			return true
		}
	}
	return false
}

func goTypeListContainsUnsafe(types *gotypes.TypeList, seen map[gotypes.Type]bool) bool {
	if types == nil {
		return false
	}
	for index := 0; index < types.Len(); index++ {
		if goTypeContainsUnsafePointer(types.At(index), seen) {
			return true
		}
	}
	return false
}

func ontamaTypeArgumentsFromGo(parameters *gotypes.TypeParamList, arguments *gotypes.TypeList) ([]Type, error) {
	return ontamaTypeArgumentsFromGoSeen(parameters, arguments, map[gotypes.Type]bool{})
}

func ontamaTypeArgumentsFromGoSeen(parameters *gotypes.TypeParamList, arguments *gotypes.TypeList, visiting map[gotypes.Type]bool) ([]Type, error) {
	parameterCount := 0
	if parameters != nil {
		parameterCount = parameters.Len()
	}
	argumentCount := 0
	if arguments != nil {
		argumentCount = arguments.Len()
	}
	if parameterCount != 0 && argumentCount == 0 {
		return nil, fmt.Errorf("generic Go type requires %d type arguments", parameterCount)
	}
	converted := make([]Type, argumentCount)
	for i := range converted {
		argument, err := ontamaTypeFromGoSeen(arguments.At(i), visiting)
		if err != nil {
			return nil, fmt.Errorf("type argument %d: %w", i+1, err)
		}
		converted[i] = argument
	}
	return converted, nil
}

func goTypeDisplayName(goType gotypes.Type) string {
	return gotypes.TypeString(goType, func(pkg *gotypes.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Name()
	})
}

func (c *Checker) consumeTask(expression ast.Expression) {
	if _, direct := expression.(*ast.TaskStartExpr); direct {
		return
	}
	identifier, ok := expression.(*ast.IdentifierExpr)
	if !ok {
		c.report(expression.GetSpan(), "await and detach require a Task variable or a direct go expression")
		return
	}
	for index := len(c.scopes) - 1; index >= 0; index-- {
		symbol, exists := c.scopes[index][identifier.Name]
		if !exists {
			continue
		}
		for _, callableBase := range c.callableScopeBases {
			if index < callableBase {
				c.report(identifier.Span, fmt.Sprintf("Task %q cannot be captured by a closure", identifier.Name))
				return
			}
		}
		switch symbol.taskState {
		case taskPending:
			symbol.taskState = taskConsumed
		case taskConsumed:
			c.report(identifier.Span, fmt.Sprintf("Task %q has already been consumed", identifier.Name))
		case taskMaybeConsumed:
			c.report(identifier.Span, fmt.Sprintf("Task %q may already have been consumed on another control-flow path", identifier.Name))
		default:
			c.report(identifier.Span, fmt.Sprintf("value %q is not a consumable Task binding", identifier.Name))
		}
		c.scopes[index][identifier.Name] = symbol
		return
	}
}

func (c *Checker) reportUnconsumedTasks(scope map[string]valueSymbol) {
	names := make([]string, 0, len(scope))
	for name := range scope {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		symbol := scope[name]
		switch symbol.taskState {
		case taskPending:
			c.report(symbol.declarationSpan, fmt.Sprintf("Task %q must be consumed with await or detach before leaving its scope", name))
		case taskMaybeConsumed:
			c.report(symbol.declarationSpan, fmt.Sprintf("Task %q is not consumed on every control-flow path", name))
		}
	}
}

func (c *Checker) reportPendingTasksBeforeExit() {
	for _, scope := range c.scopes {
		c.reportUnconsumedTasks(scope)
	}
}

func (c *Checker) pushScope() { c.scopes = append(c.scopes, map[string]valueSymbol{}) }
func (c *Checker) popScope() {
	if len(c.scopes) != 0 {
		c.reportUnconsumedTasks(c.scopes[len(c.scopes)-1])
	}
	c.scopes = c.scopes[:len(c.scopes)-1]
}
func (c *Checker) report(span source.Span, message string) {
	for _, existing := range c.diagnostics {
		if existing.Message == message && existing.Span.Path == span.Path && existing.Span.Start == span.Start && existing.Span.End == span.End {
			return
		}
	}
	c.diagnostics = append(c.diagnostics, diagnostic.Diagnostic{Message: message, Span: span})
}

func definitelyReturns(block *ast.BlockStmt) bool {
	for _, stmt := range block.Statements {
		switch stmt := stmt.(type) {
		case *ast.ReturnStmt:
			return true
		case *ast.ThrowStmt:
			return true
		case *ast.LabeledStmt:
			if statementDefinitelyReturns(stmt.Statement) {
				return true
			}
		case *ast.TryStmt:
			if stmt.Terminal {
				return true
			}
		case *ast.IfStmt:
			if stmt.Else == nil {
				continue
			}
			thenReturns := definitelyReturns(stmt.Then)
			elseReturns := false
			switch branch := stmt.Else.(type) {
			case *ast.BlockStmt:
				elseReturns = definitelyReturns(branch)
			case *ast.IfStmt:
				elseReturns = definitelyReturns(&ast.BlockStmt{Statements: []ast.Statement{branch}})
			}
			if thenReturns && elseReturns {
				return true
			}
		case *ast.SelectStmt:
			if len(stmt.Cases) == 0 {
				return true
			}
			allReturn := true
			for i := range stmt.Cases {
				if !definitelyReturns(stmt.Cases[i].Body) {
					allReturn = false
					break
				}
			}
			if allReturn {
				return true
			}
		case *ast.ValueSwitchStmt:
			if valueSwitchDefinitelyReturns(stmt) {
				return true
			}
		case *ast.TypeSwitchStmt:
			hasDefault := false
			allReturn := len(stmt.Cases) != 0
			for i := range stmt.Cases {
				hasDefault = hasDefault || stmt.Cases[i].Default
				if !definitelyReturns(stmt.Cases[i].Body) {
					allReturn = false
				}
			}
			if hasDefault && allReturn {
				return true
			}
		}
	}
	return false
}

func valueSwitchDefinitelyReturns(statement *ast.ValueSwitchStmt) bool {
	if statement == nil || len(statement.Cases) == 0 {
		return false
	}
	caseReturns := make([]bool, len(statement.Cases))
	hasDefault := false
	for index := len(statement.Cases) - 1; index >= 0; index-- {
		clause := &statement.Cases[index]
		hasDefault = hasDefault || clause.Default
		caseReturns[index] = definitelyReturns(clause.Body)
		if !caseReturns[index] && index+1 < len(statement.Cases) && valueSwitchCaseFallthroughReachable(clause) {
			caseReturns[index] = caseReturns[index+1]
		}
	}
	if !hasDefault {
		return false
	}
	for _, returns := range caseReturns {
		if !returns {
			return false
		}
	}
	return true
}
