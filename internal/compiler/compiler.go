package compiler

import (
	"fmt"
	"go/build"
	"go/importer"
	gotypes "go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/codegen"
	"ontama.local/ontama/internal/diagnostic"
	"ontama.local/ontama/internal/lexer"
	"ontama.local/ontama/internal/parser"
	"ontama.local/ontama/internal/product"
	"ontama.local/ontama/internal/project"
	"ontama.local/ontama/internal/sema"
	"ontama.local/ontama/stdlib"
)

type Result struct {
	Program     *ast.Program
	Diagnostics []diagnostic.Diagnostic
	goImporter  gotypes.Importer
}

type GoExport struct {
	Name   string
	Detail string
	Kind   string
}

type GoMember struct {
	Name   string
	Detail string
	Kind   string
}

type GoFunctionSignature struct {
	ParameterNames []string
	ParameterTypes []string
	Result         string
	Variadic       bool
}

type CABIArtifacts struct {
	GoSource    []byte
	Gateway     []byte
	Header      []byte
	Manifest    []byte
	Fingerprint string
}

type CFFIArtifacts struct {
	Package string
	Source  []byte
}

// GenerateCFFI validates an incoming C FFI manifest and emits the private cgo
// package consumed through ordinary checked Go interop.
func GenerateCFFI(manifest []byte) (CFFIArtifacts, error) {
	generated, err := codegen.GenerateCFFI(manifest)
	if err != nil {
		return CFFIArtifacts{}, err
	}
	return CFFIArtifacts{Package: generated.Package, Source: generated.Source}, nil
}

// GoPackageExports returns the exported API selected by the same Go importer,
// module graph, build target, and build tags used for this compilation result.
func (r Result) GoPackageExports(path string) ([]GoExport, error) {
	goImporter := r.goImporter
	if goImporter == nil {
		goImporter = importer.Default()
	}
	packageInfo, err := goImporter.Import(path)
	if err != nil {
		return nil, err
	}
	names := packageInfo.Scope().Names()
	result := make([]GoExport, 0, len(names))
	for _, name := range names {
		object := packageInfo.Scope().Lookup(name)
		if object == nil || !object.Exported() {
			continue
		}
		kind := "value"
		switch object.(type) {
		case *gotypes.Const:
			kind = "constant"
		case *gotypes.Func, *gotypes.Builtin:
			kind = "function"
		case *gotypes.TypeName:
			kind = "type"
		case *gotypes.Var:
			kind = "variable"
		}
		detail := gotypes.ObjectString(object, func(pkg *gotypes.Package) string {
			if pkg == nil {
				return ""
			}
			return pkg.Name()
		})
		result = append(result, GoExport{Name: name, Detail: detail, Kind: kind})
	}
	return result, nil
}

// GoTypeMembers returns exported fields and methods selectable on a Go named
// type. Addressable value receivers include pointer-receiver methods, matching
// the selector rules used by semantic checking.
func (r Result) GoTypeMembers(path, name string, pointer, addressable bool) ([]GoMember, bool, error) {
	goImporter := r.goImporter
	if goImporter == nil {
		goImporter = importer.Default()
	}
	packageInfo, err := goImporter.Import(path)
	if err != nil {
		return nil, false, err
	}
	object, ok := packageInfo.Scope().Lookup(name).(*gotypes.TypeName)
	if !ok || !object.Exported() {
		return nil, false, nil
	}
	receiver := object.Type()
	if pointer {
		receiver = gotypes.NewPointer(receiver)
	}
	qualifier := func(pkg *gotypes.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Name()
	}
	members := map[string]GoMember{}
	methodReceiver := receiver
	if addressable && !pointer {
		methodReceiver = gotypes.NewPointer(receiver)
	}
	methodSet := gotypes.NewMethodSet(methodReceiver)
	for index := 0; index < methodSet.Len(); index++ {
		selection := methodSet.At(index)
		method, ok := selection.Obj().(*gotypes.Func)
		if !ok || !method.Exported() {
			continue
		}
		members[method.Name()] = GoMember{Name: method.Name(), Detail: gotypes.SelectionString(selection, qualifier), Kind: "method"}
	}
	fieldNames := map[string]bool{}
	collectGoFieldNames(receiver, map[gotypes.Type]bool{}, fieldNames)
	for fieldName := range fieldNames {
		selected, _, _ := gotypes.LookupFieldOrMethod(receiver, addressable, nil, fieldName)
		field, ok := selected.(*gotypes.Var)
		if !ok || !field.IsField() || !field.Exported() {
			continue
		}
		members[field.Name()] = GoMember{Name: field.Name(), Detail: gotypes.ObjectString(field, qualifier), Kind: "field"}
	}
	result := make([]GoMember, 0, len(members))
	for _, member := range members {
		result = append(result, member)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, true, nil
}

func collectGoFieldNames(goType gotypes.Type, seen map[gotypes.Type]bool, names map[string]bool) {
	goType = gotypes.Unalias(goType)
	if pointer, ok := goType.(*gotypes.Pointer); ok {
		goType = gotypes.Unalias(pointer.Elem())
	}
	if seen[goType] {
		return
	}
	seen[goType] = true
	structure, ok := goType.Underlying().(*gotypes.Struct)
	if !ok {
		return
	}
	for index := 0; index < structure.NumFields(); index++ {
		field := structure.Field(index)
		if field.Exported() {
			names[field.Name()] = true
		}
		if field.Embedded() {
			collectGoFieldNames(field.Type(), seen, names)
		}
	}
}

// GoPackageFunctionSignature returns the selected toolchain signature for one
// exported package function. It is used by editor features while source may be
// temporarily incomplete and therefore unavailable to semantic checking.
func (r Result) GoPackageFunctionSignature(path, name string) (GoFunctionSignature, bool, error) {
	goImporter := r.goImporter
	if goImporter == nil {
		goImporter = importer.Default()
	}
	packageInfo, err := goImporter.Import(path)
	if err != nil {
		return GoFunctionSignature{}, false, err
	}
	object := packageInfo.Scope().Lookup(name)
	function, ok := object.(*gotypes.Func)
	if !ok || !function.Exported() {
		return GoFunctionSignature{}, false, nil
	}
	signature, ok := function.Type().(*gotypes.Signature)
	if !ok {
		return GoFunctionSignature{}, false, nil
	}
	return goFunctionSignature(signature), true, nil
}

// GoTypeMethodSignature returns the signature of an exported method selectable
// on a Go named type under the same addressability rules as semantic checking.
func (r Result) GoTypeMethodSignature(path, typeName string, pointer, addressable bool, methodName string) (GoFunctionSignature, bool, error) {
	goImporter := r.goImporter
	if goImporter == nil {
		goImporter = importer.Default()
	}
	packageInfo, err := goImporter.Import(path)
	if err != nil {
		return GoFunctionSignature{}, false, err
	}
	object, ok := packageInfo.Scope().Lookup(typeName).(*gotypes.TypeName)
	if !ok || !object.Exported() {
		return GoFunctionSignature{}, false, nil
	}
	receiver := object.Type()
	if pointer {
		receiver = gotypes.NewPointer(receiver)
	}
	selected, _, _ := gotypes.LookupFieldOrMethod(receiver, addressable, nil, methodName)
	method, ok := selected.(*gotypes.Func)
	if !ok || !method.Exported() {
		return GoFunctionSignature{}, false, nil
	}
	signature, ok := method.Type().(*gotypes.Signature)
	if !ok {
		return GoFunctionSignature{}, false, nil
	}
	return goFunctionSignature(signature), true, nil
}

func goFunctionSignature(signature *gotypes.Signature) GoFunctionSignature {
	result := GoFunctionSignature{
		ParameterNames: make([]string, signature.Params().Len()),
		ParameterTypes: make([]string, signature.Params().Len()),
		Variadic:       signature.Variadic(),
	}
	for index := 0; index < signature.Params().Len(); index++ {
		parameter := signature.Params().At(index)
		result.ParameterNames[index] = parameter.Name()
		parameterType := parameter.Type()
		if result.Variadic && index == signature.Params().Len()-1 {
			if slice, sliceOK := parameterType.(*gotypes.Slice); sliceOK {
				parameterType = slice.Elem()
			}
		}
		result.ParameterTypes[index] = formatGoTypeForSignature(parameterType)
	}
	switch signature.Results().Len() {
	case 0:
		result.Result = "void"
	case 1:
		result.Result = formatGoTypeForSignature(signature.Results().At(0).Type())
	default:
		items := make([]string, signature.Results().Len())
		for index := range items {
			items[index] = formatGoTypeForSignature(signature.Results().At(index).Type())
		}
		result.Result = "(" + strings.Join(items, ", ") + ")"
	}
	return result
}

func formatGoTypeForSignature(goType gotypes.Type) string {
	switch value := goType.(type) {
	case *gotypes.Basic:
		switch value.Kind() {
		case gotypes.Bool, gotypes.UntypedBool:
			return "boolean"
		case gotypes.String, gotypes.UntypedString:
			return "string"
		case gotypes.Int:
			return "int"
		case gotypes.Int32, gotypes.UntypedRune:
			return "int32"
		case gotypes.Int64:
			return "int64"
		case gotypes.Uint8:
			return "byte"
		case gotypes.Float32:
			return "float32"
		case gotypes.Float64, gotypes.UntypedFloat:
			return "float"
		default:
			return value.Name()
		}
	case *gotypes.Pointer:
		return "*" + formatGoTypeForSignature(value.Elem())
	case *gotypes.Slice:
		return formatGoTypeForSignature(value.Elem()) + "[]"
	case *gotypes.Array:
		return fmt.Sprintf("[%d]%s", value.Len(), formatGoTypeForSignature(value.Elem()))
	case *gotypes.Map:
		return "Map<" + formatGoTypeForSignature(value.Key()) + ", " + formatGoTypeForSignature(value.Elem()) + ">"
	case *gotypes.Chan:
		name := "GoChannel"
		if value.Dir() == gotypes.SendOnly {
			name = "GoSendChannel"
		} else if value.Dir() == gotypes.RecvOnly {
			name = "GoReceiveChannel"
		}
		return name + "<" + formatGoTypeForSignature(value.Elem()) + ">"
	case *gotypes.Named:
		return formatGoNamedType(value.Obj(), value.TypeArgs())
	case *gotypes.Alias:
		return formatGoNamedType(value.Obj(), value.TypeArgs())
	case *gotypes.TypeParam:
		return value.Obj().Name()
	case *gotypes.Signature:
		parameters := make([]string, value.Params().Len())
		for index := range parameters {
			parameterType := value.Params().At(index).Type()
			if value.Variadic() && index == len(parameters)-1 {
				if slice, ok := parameterType.(*gotypes.Slice); ok {
					parameterType = slice.Elem()
				}
				parameters[index] = "..."
			}
			parameters[index] += formatGoTypeForSignature(parameterType)
		}
		return "(" + strings.Join(parameters, ", ") + ") => " + formatGoTupleResult(value.Results())
	default:
		return gotypes.TypeString(goType, func(pkg *gotypes.Package) string {
			if pkg == nil {
				return ""
			}
			return pkg.Name()
		})
	}
}

func formatGoNamedType(object *gotypes.TypeName, arguments *gotypes.TypeList) string {
	name := object.Name()
	if object.Pkg() != nil {
		name = object.Pkg().Name() + "." + name
	}
	if arguments == nil || arguments.Len() == 0 {
		return name
	}
	formatted := make([]string, arguments.Len())
	for index := range formatted {
		formatted[index] = formatGoTypeForSignature(arguments.At(index))
	}
	return name + "<" + strings.Join(formatted, ", ") + ">"
}

func formatGoTupleResult(results *gotypes.Tuple) string {
	if results == nil || results.Len() == 0 {
		return "void"
	}
	if results.Len() == 1 {
		return formatGoTypeForSignature(results.At(0).Type())
	}
	formatted := make([]string, results.Len())
	for index := range formatted {
		formatted[index] = formatGoTypeForSignature(results.At(index).Type())
	}
	return "(" + strings.Join(formatted, ", ") + ")"
}

func CheckFiles(paths []string) (Result, error) {
	return CheckFilesWithOverlay(paths, nil)
}

// CheckFilesWithOverlay checks paths while substituting the supplied in-memory
// source text for matching files. Overlay keys may be relative or absolute and
// are normalized in the same way as module paths.
func CheckFilesWithOverlay(paths []string, overlay map[string]string) (Result, error) {
	ordered := append([]string(nil), paths...)
	sort.Strings(ordered)
	var lockedRoot string
	var lockedTarget *project.BuildTarget
	allowUnsafeGo := false
	if len(ordered) != 0 {
		root, found, rootErr := project.FindRoot(ordered[0])
		if rootErr != nil {
			return Result{}, rootErr
		}
		if found {
			manifest, lock, lockErr := project.ValidateLockedFiles(root)
			if lockErr != nil {
				return Result{}, lockErr
			}
			lockedRoot = root
			target := lock.Target
			lockedTarget = &target
			allowUnsafeGo = manifest.AllowsUnsafeGoInterop()
		}
	}
	normalizedOverlay := make(map[string]string, len(overlay))
	for path, input := range overlay {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return Result{}, err
		}
		normalizedOverlay[filepath.Clean(absolute)] = input
	}
	linkBase := ""
	if len(ordered) != 0 {
		if absolute, absoluteErr := filepath.Abs(ordered[0]); absoluteErr == nil {
			linkBase = filepath.Dir(absolute)
		}
	}
	loader := &moduleLoader{
		states:   map[string]int{},
		programs: map[string]*ast.Program{},
		paths:    map[string]string{},
		merged:   &ast.Program{},
		linkBase: linkBase,
		overlay:  normalizedOverlay,
	}
	for _, path := range ordered {
		if err := loader.load(filepath.Clean(path), nil); err != nil {
			return Result{}, err
		}
	}
	goImporter := goImporterForProgram(loader.merged, ordered, lockedRoot, lockedTarget)
	if len(loader.diagnostics) == 0 {
		allowed := loader.linkModules(ordered)
		if len(loader.diagnostics) == 0 {
			loader.diagnostics = append(loader.diagnostics, sema.CheckScopedWithGoImporterAndPolicy(loader.merged, allowed, goImporter, sema.GoInteropPolicy{AllowUnsafe: allowUnsafeGo})...)
			return Result{Program: loader.merged, Diagnostics: loader.diagnostics, goImporter: goImporter}, nil
		}
	}
	return Result{Program: loader.merged, Diagnostics: loader.diagnostics, goImporter: goImporter}, nil
}

func goImporterForProgram(program *ast.Program, rootPaths []string, lockedRoot string, lockedTarget *project.BuildTarget) gotypes.Importer {
	for _, declaration := range program.Imports {
		if !declaration.Go {
			continue
		}
		if lockedTarget != nil {
			return newModuleGoImporter(product.DependencyDirectory(lockedRoot), lockedTarget)
		}
		metadata, err := build.Default.Import(declaration.Path, "", build.FindOnly)
		if err != nil || !metadata.Goroot {
			directory := "."
			if len(rootPaths) != 0 {
				if absolute, absoluteErr := filepath.Abs(rootPaths[0]); absoluteErr == nil {
					directory = filepath.Dir(absolute)
				}
				if root, found, rootErr := project.FindRoot(rootPaths[0]); rootErr == nil && found {
					directory = product.DependencyDirectory(root)
				}
			}
			return newModuleGoImporter(directory, nil)
		}
	}
	return importer.Default()
}

type moduleLoader struct {
	states      map[string]int
	programs    map[string]*ast.Program
	paths       map[string]string
	merged      *ast.Program
	diagnostics []diagnostic.Diagnostic
	linkBase    string
	overlay     map[string]string
}

func (l *moduleLoader) load(path string, importedBy *ast.ImportDecl) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absolute = filepath.Clean(absolute)
	var input string
	if overlaid, exists := l.overlay[absolute]; exists {
		input = overlaid
	} else if contents, readErr := os.ReadFile(path); readErr == nil {
		input = string(contents)
	} else {
		if importedBy == nil {
			return readErr
		}
		l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{
			Message: fmt.Sprintf("cannot load imported module %q: %v", importedBy.Path, readErr), Span: importedBy.PathSpan,
		})
		return nil
	}
	return l.loadSource(absolute, path, input, importedBy, false)
}

func (l *moduleLoader) loadSource(key, path, input string, importedBy *ast.ImportDecl, embedded bool) error {
	switch l.states[key] {
	case 1:
		if importedBy != nil {
			l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{
				Message: fmt.Sprintf("import cycle detected through %q", importedBy.Path), Span: importedBy.PathSpan,
			})
		}
		return nil
	case 2:
		return nil
	}
	l.states[key] = 1
	tokens, lexerDiagnostics := lexer.Lex(path, input)
	program, parserDiagnostics := parser.Parse(tokens)
	l.diagnostics = append(l.diagnostics, lexerDiagnostics...)
	l.diagnostics = append(l.diagnostics, parserDiagnostics...)
	l.programs[key] = program
	l.paths[key] = path

	for i := range program.Imports {
		imported := &program.Imports[i]
		if imported.Go {
			continue
		}
		if !strings.HasPrefix(imported.Path, ".") {
			standardSource, found := stdlib.Lookup(imported.Path)
			if !found {
				message := "package imports are not supported in this compiler stage"
				if strings.HasPrefix(imported.Path, "ontama/") {
					message = fmt.Sprintf("standard package %q is not available", imported.Path)
				}
				l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: message, Span: imported.PathSpan})
				continue
			}
			standardKey := filepath.FromSlash(standardSource.VirtualPath)
			imported.ResolvedPath = standardKey
			if err := l.loadSource(standardKey, filepath.FromSlash(standardSource.VirtualPath), standardSource.Contents, imported, true); err != nil {
				return err
			}
			l.validateImport(*imported, l.programs[standardKey])
			continue
		}
		if embedded {
			l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{
				Message: fmt.Sprintf("compiler-managed module %q cannot use relative import %q", path, imported.Path), Span: imported.PathSpan,
			})
			continue
		}
		target := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(imported.Path)))
		if filepath.Ext(target) == "" {
			target += product.SourceExtension
		}
		targetAbsolute, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		imported.ResolvedPath = filepath.Clean(targetAbsolute)
		if err := l.load(target, imported); err != nil {
			return err
		}
		l.validateImport(*imported, l.programs[filepath.Clean(targetAbsolute)])
	}

	l.states[key] = 2
	l.merged.Imports = append(l.merged.Imports, program.Imports...)
	l.merged.Declarations = append(l.merged.Declarations, program.Declarations...)
	return nil
}

func (l *moduleLoader) validateImport(imported ast.ImportDecl, target *ast.Program) {
	if target == nil {
		return
	}
	available := map[string]bool{}
	for _, declaration := range target.Declarations {
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			available[declaration.Name] = true
		case *ast.VariableDecl:
			available[declaration.Name] = true
		case *ast.ClassDecl:
			available[declaration.Name] = true
		case *ast.StructDecl:
			available[declaration.Name] = true
		case *ast.TypeDecl:
			available[declaration.Name] = true
		case *ast.EnumDecl:
			available[declaration.Name] = true
		case *ast.InterfaceDecl:
			available[declaration.Name] = true
		}
	}
	seen := map[string]bool{}
	for _, name := range imported.Names {
		if seen[name] {
			l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("duplicate imported name %q", name), Span: imported.Span})
			continue
		}
		seen[name] = true
		if !available[name] {
			l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{
				Message: fmt.Sprintf("module %q does not declare %q", imported.Path, name), Span: imported.Span,
			})
		}
	}
}

func EmitGo(paths []string, packageName string) ([]byte, []diagnostic.Diagnostic, error) {
	result, err := CheckFiles(paths)
	if err != nil {
		return nil, nil, err
	}
	if len(result.Diagnostics) != 0 {
		return nil, result.Diagnostics, nil
	}
	generated, err := codegen.GenerateWithImporter(result.Program, packageName, result.goImporter)
	return generated, nil, err
}

// EmitCABI emits the ordinary Go implementation together with a cgo export
// gateway and stable C header for functions declared with export c(...).
func EmitCABI(paths []string) (CABIArtifacts, []diagnostic.Diagnostic, error) {
	result, err := CheckFiles(paths)
	if err != nil {
		return CABIArtifacts{}, nil, err
	}
	if len(result.Diagnostics) != 0 {
		return CABIArtifacts{}, result.Diagnostics, nil
	}
	generated, err := codegen.GenerateWithImporter(result.Program, "main", result.goImporter)
	if err != nil {
		return CABIArtifacts{}, nil, err
	}
	cabi, err := codegen.GenerateCABI(result.Program)
	if err != nil {
		return CABIArtifacts{}, nil, err
	}
	return CABIArtifacts{
		GoSource: generated, Gateway: cabi.Gateway, Header: cabi.Header, Manifest: cabi.Manifest, Fingerprint: cabi.Fingerprint,
	}, nil, nil
}

// WriteGeneratedModule emits the readable Go intermediate representation into
// the project's generated-state directory and returns that directory.
func WriteGeneratedModule(paths []string, packageName string) (string, []diagnostic.Diagnostic, error) {
	if len(paths) == 0 {
		return "", nil, fmt.Errorf("at least one source path is required")
	}
	generated, diagnostics, err := EmitGo(paths, packageName)
	if err != nil || len(diagnostics) != 0 {
		return "", diagnostics, err
	}
	root, err := findProjectRoot(paths[0])
	if err != nil {
		return "", nil, err
	}
	directory := product.GeneratedDirectory(root)
	if err = os.MkdirAll(directory, 0o755); err != nil {
		return "", nil, err
	}
	if err = os.WriteFile(filepath.Join(directory, "generated.go"), generated, 0o644); err != nil {
		return "", nil, err
	}
	generatedModule := filepath.Join(directory, "go.mod")
	if _, statErr := os.Stat(filepath.Join(root, product.ProjectFileName)); statErr == nil {
		for _, name := range []string{"go.mod", "go.sum"} {
			contents, readErr := os.ReadFile(filepath.Join(product.DependencyDirectory(root), name))
			if readErr != nil {
				return "", nil, fmt.Errorf("cannot read locked %s: %w", name, readErr)
			}
			if err = os.WriteFile(filepath.Join(directory, name), contents, 0o644); err != nil {
				return "", nil, err
			}
		}
	} else if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr == nil {
		if err = os.Remove(generatedModule); err != nil && !os.IsNotExist(err) {
			return "", nil, err
		}
	} else {
		module := []byte("module " + product.GeneratedModulePath + "\n\ngo 1.23\n")
		if err = os.WriteFile(generatedModule, module, 0o644); err != nil {
			return "", nil, err
		}
	}
	return directory, nil, nil
}

func findProjectRoot(sourcePath string) (string, error) {
	absolute, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", err
	}
	start := filepath.Dir(absolute)
	for directory := start; ; directory = filepath.Dir(directory) {
		if _, err := os.Stat(filepath.Join(directory, product.ProjectFileName)); err == nil {
			return directory, nil
		}
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
	}
	workingDirectory, err := os.Getwd()
	if err == nil {
		relative, relErr := filepath.Rel(workingDirectory, absolute)
		if relErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return workingDirectory, nil
		}
	}
	return start, nil
}
