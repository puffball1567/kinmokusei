package codegen

import (
	"bytes"
	"fmt"
	goast "go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"runtime"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/product"
)

func Generate(program *kinmokuseiAST.Program, packageName string) ([]byte, error) {
	return GenerateWithImporter(program, packageName, importer.Default())
}

func GenerateWithImporter(program *kinmokuseiAST.Program, packageName string, goImporter types.Importer) ([]byte, error) {
	return GenerateWithTarget(program, packageName, goImporter, nil)
}

// GenerateWithTarget validates against the same target sizes as semantic checking.
func GenerateWithTarget(program *kinmokuseiAST.Program, packageName string, goImporter types.Importer, sizes types.Sizes) ([]byte, error) {
	generated, _, err := generateWithTarget(program, packageName, goImporter, sizes, false, nil)
	return generated, err
}

// GenerateMappedWithTarget emits physical Go ranges without changing Go's line
// numbering. The checked input is not mutated and can be shared by callers.
func GenerateMappedWithTarget(program *kinmokuseiAST.Program, packageName string, goImporter types.Importer, sizes types.Sizes) ([]byte, []SourceMapping, error) {
	return generateWithTarget(program, packageName, goImporter, sizes, true, nil)
}

// CheckInstantiationCycles applies Go's monomorphization rules to the actual
// lowering, including erased aliases and generated generic method helpers.
// Mapping uses a clone of the checked program; no artifacts are written.
func CheckInstantiationCycles(program *kinmokuseiAST.Program, goImporter types.Importer, sizes types.Sizes) ([]diagnostic.Diagnostic, error) {
	var diagnostics []diagnostic.Diagnostic
	_, _, err := generateWithTarget(program, "kinmokusei_check", goImporter, sizes, false, &diagnostics)
	if err == nil && len(diagnostics) != 0 {
		// Most programs need no mapping clone/token pass. Recheck a detected
		// cycle with statement origins to report its recursive source use.
		_, _, err = generateWithTarget(program, "kinmokusei_check", goImporter, sizes, true, &diagnostics)
	}
	return diagnostics, err
}

func generateWithTarget(program *kinmokuseiAST.Program, packageName string, goImporter types.Importer, sizes types.Sizes, mapped bool, instantiations *[]diagnostic.Diagnostic) ([]byte, []SourceMapping, error) {
	if mapped {
		program = sourceMappedProgram(program)
	}
	if packageName == "" {
		packageName = "main"
	}
	file := &goast.File{
		Name: goast.NewIdent(packageName),
	}
	seenImports := map[string]bool{}
	utf8Alias := "__kinmokuseiUTF8"
	var imports []goast.Spec
	for _, imported := range program.Imports {
		if !imported.Go || !imported.Used || seenImports[imported.Path] {
			continue
		}
		seenImports[imported.Path] = true
		alias := imported.Alias
		if imported.ResolvedAlias != "" {
			alias = imported.ResolvedAlias
		}
		if imported.Path == "unicode/utf8" {
			utf8Alias = goName(alias)
		}
		imports = append(imports, &goast.ImportSpec{
			Name: goast.NewIdent(goName(alias)),
			Path: &goast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", imported.Path)},
		})
	}
	if program.UsesUTF8 && !seenImports["unicode/utf8"] {
		imports = append(imports, &goast.ImportSpec{Name: goast.NewIdent(utf8Alias), Path: &goast.BasicLit{Kind: token.STRING, Value: `"unicode/utf8"`}})
		seenImports["unicode/utf8"] = true
	}
	// Reuse checked import aliases rather than introducing a second import of
	// the same package. Runtime aliases use the reserved compiler namespace.
	runtimeImport := func(path, fallback string) string {
		for _, spec := range imports {
			imported := spec.(*goast.ImportSpec)
			if imported.Path.Value == fmt.Sprintf("%q", path) {
				return imported.Name.Name
			}
		}
		imports = append(imports, &goast.ImportSpec{Name: goast.NewIdent(fallback), Path: &goast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", path)}})
		seenImports[path] = true
		return fallback
	}
	observationAlias, snapshotAlias := "", ""
	if program.UsesObservation {
		observationAlias = runtimeImport("reflect", "__kinmokuseiReflect")
	}
	if program.UsesJSONSnapshot {
		snapshotAlias = runtimeImport("encoding/json", "__kinmokuseiJSON")
	}
	// Constant length expressions no longer occur in the generated types.
	// Retain their packages' initialization without an unused namespace alias.
	for _, imported := range program.Imports {
		if !imported.Go || !imported.UsedByArrayLength || seenImports[imported.Path] {
			continue
		}
		seenImports[imported.Path] = true
		imports = append(imports, &goast.ImportSpec{
			Name: goast.NewIdent("_"),
			Path: &goast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", imported.Path)},
		})
	}
	if len(imports) != 0 {
		file.Decls = append(file.Decls, &goast.GenDecl{Tok: token.IMPORT, Specs: imports})
	}
	if program.UsesTasks {
		runtimeDeclarations, err := taskRuntimeDeclarations()
		if err != nil {
			return nil, nil, err
		}
		file.Decls = append(file.Decls, runtimeDeclarations...)
	}
	if program.UsesUTF8 {
		declarations, err := utf8RuntimeDeclarations(utf8Alias)
		if err != nil {
			return nil, nil, err
		}
		file.Decls = append(file.Decls, declarations...)
	}
	if program.UsesObservation {
		declarations, err := observationRuntimeDeclarations(observationAlias)
		if err != nil {
			return nil, nil, err
		}
		file.Decls = append(file.Decls, declarations...)
	}
	if program.UsesJSONSnapshot {
		declarations, err := jsonSnapshotRuntimeDeclarations(snapshotAlias)
		if err != nil {
			return nil, nil, err
		}
		file.Decls = append(file.Decls, declarations...)
	}
	if program.UsesExceptions {
		runtimeDeclarations, err := exceptionRuntimeDeclarations()
		if err != nil {
			return nil, nil, err
		}
		file.Decls = append(file.Decls, runtimeDeclarations...)
	}
	if program.UsesDecoratorContext || len(program.Decorators) != 0 {
		file.Decls = append(file.Decls, decoratorRuntimeDeclarations()...)
	}
	var origins map[int]kinmokuseiAST.Declaration
	if instantiations != nil {
		origins = map[int]kinmokuseiAST.Declaration{}
	}
	for _, decl := range program.Declarations {
		generated, err := generateDeclaration(decl)
		if err != nil {
			return nil, nil, err
		}
		if instantiations != nil {
			for index := range generated {
				origins[len(file.Decls)+index] = decl
			}
		}
		file.Decls = append(file.Decls, generated...)
	}
	decoratorInit, err := generateDecoratorInit(program)
	if err != nil {
		return nil, nil, err
	}
	if decoratorInit != nil {
		file.Decls = append(file.Decls, decoratorInit)
	}
	iteratorDeclarations, err := iteratorControlDeclarations(file, program.UsesExceptions)
	if err != nil {
		return nil, nil, err
	}
	file.Decls = append(file.Decls, iteratorDeclarations...)
	needsContracts := false
	goast.Inspect(file, func(node goast.Node) bool {
		if name, ok := node.(*goast.Ident); ok && (name.Name == "__kinmokuseiSourceContract" || name.Name == "__kinmokuseiKnownContracts") {
			needsContracts = true
		}
		return true
	})
	if needsContracts {
		file.Decls = append(file.Decls, nativeContractRuntimeDeclarations()...)
	}
	deferDeclarations, err := lowerSourceDefers(file)
	if err != nil {
		return nil, nil, err
	}
	file.Decls = append(file.Decls, deferDeclarations...)
	evaluationDeclarations, err := evaluationRuntimeDeclarations(file)
	if err != nil {
		return nil, nil, err
	}
	file.Decls = append(file.Decls, evaluationDeclarations...)
	sliceDeclarations, err := initializedSliceDeclarations(file)
	if err != nil {
		return nil, nil, err
	}
	file.Decls = append(file.Decls, sliceDeclarations...)
	coalescing, err := coalesceDeclarations(file)
	if err != nil {
		return nil, nil, err
	}
	file.Decls = append(file.Decls, coalescing...)
	set := token.NewFileSet()
	var output bytes.Buffer
	if err := format.Node(&output, set, file); err != nil {
		return nil, nil, err
	}
	generated := append([]byte("// Code generated by "+product.DisplayName+". DO NOT EDIT.\n"), output.Bytes()...)
	var mappings []SourceMapping
	if mapped {
		var err error
		generated, mappings, err = extractSourceMappings(generated)
		if err != nil {
			return nil, nil, err
		}
	}
	if instantiations != nil {
		*instantiations = checkInstantiationCycles(generated, mappings, origins, goImporter, sizes)
		return generated, mappings, nil
	}
	if err := validateGeneratedGoWithTarget(generated, goImporter, sizes); err != nil {
		return nil, nil, fmt.Errorf("generated Go failed validation: %w", err)
	}
	return generated, mappings, nil
}

func validateGeneratedGo(generated []byte) error {
	return validateGeneratedGoWithImporter(generated, importer.Default())
}

func validateGeneratedGoWithImporter(generated []byte, goImporter types.Importer) error {
	return validateGeneratedGoWithTarget(generated, goImporter, nil)
}

func validateGeneratedGoWithTarget(generated []byte, goImporter types.Importer, sizes types.Sizes) error {
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, "generated.go", generated, parser.AllErrors)
	if err != nil {
		return err
	}
	if goImporter == nil {
		goImporter = importer.Default()
	}
	if sizes == nil {
		sizes = types.SizesFor("gc", runtime.GOARCH)
	}
	configuration := types.Config{Importer: goImporter, Sizes: sizes}
	_, err = configuration.Check(file.Name.Name, files, []*goast.File{file}, nil)
	return err
}
