package codegen

import (
	goast "go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"runtime"
	"strings"

	kmast "github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func checkInstantiationCycles(generated []byte, mappings []SourceMapping, origins map[int]kmast.Declaration, goImporter types.Importer, sizes types.Sizes) []diagnostic.Diagnostic {
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, "generated.go", generated, 0)
	if err != nil {
		return nil // Ordinary emission retains its existing validation errors.
	}
	if goImporter == nil {
		goImporter = importer.Default()
	}
	if sizes == nil {
		sizes = types.SizesFor("gc", runtime.GOARCH)
	}
	var errors []types.Error
	configuration := types.Config{Importer: goImporter, Sizes: sizes, Error: func(err error) {
		if typed, ok := err.(types.Error); ok {
			errors = append(errors, typed)
		}
	}}
	_, _ = configuration.Check(file.Name.Name, files, []*goast.File{file}, nil)
	var result []diagnostic.Diagnostic
	seen := map[source.Span]bool{}
	for index, err := range errors {
		if !strings.HasPrefix(err.Msg, "instantiation cycle") {
			continue
		}
		span := instantiationOrigin(err.Pos, files, file, mappings, origins)
		// Go reports a primary parameter declaration followed by cycle edges.
		// Prefer an edge's source use over its parameter's declaration.
		for next := index + 1; next < len(errors) && strings.HasPrefix(errors[next].Msg, "\t"); next++ {
			if origin := instantiationOrigin(errors[next].Pos, files, file, mappings, origins); origin.Path != "" {
				fallback := false
				for _, declaration := range origins {
					if origin == declaration.GetSpan() {
						fallback = true
						break
					}
				}
				if !fallback {
					span = origin
					break
				}
				if span.Path == "" {
					span = origin
				}
			}
		}
		if span.Path != "" && !seen[span] {
			seen[span] = true
			result = append(result, diagnostic.Diagnostic{
				Message: "generic instantiation cycle: recursive instantiations grow type arguments without bound",
				Span:    span,
			})
		}
	}
	return result
}

func instantiationOrigin(pos token.Pos, files *token.FileSet, file *goast.File, mappings []SourceMapping, origins map[int]kmast.Declaration) source.Span {
	if !pos.IsValid() {
		return source.Span{}
	}
	offset := files.PositionFor(pos, false).Offset
	for _, mapping := range mappings {
		if offset >= mapping.Generated.Start.Offset && offset < mapping.Generated.End.Offset {
			return mapping.Source
		}
	}
	for index, declaration := range file.Decls {
		if pos < declaration.Pos() || pos >= declaration.End() {
			continue
		}
		original := origins[index]
		if original == nil {
			return source.Span{}
		}
		return instantiationDeclarationOrigin(declaration, original, pos)
	}
	return source.Span{}
}

// Declaration signatures have no executable coverage origin. Match their
// checked Go names without adding non-executable ranges to coverage maps.
func instantiationDeclarationOrigin(declaration goast.Decl, original kmast.Declaration, pos token.Pos) source.Span {
	var fields []kmast.FieldDecl
	var methods []*kmast.MethodDecl
	var contract *kmast.InterfaceDecl
	switch original := original.(type) {
	case *kmast.ClassDecl:
		fields, methods = original.Fields, original.Methods
	case *kmast.StructDecl:
		fields, methods = original.Fields, original.Methods
	case *kmast.InterfaceDecl:
		contract = original
	case *kmast.TypeDecl:
		return original.Underlying.Span
	}
	var fieldOrigin source.Span
	goast.Inspect(declaration, func(node goast.Node) bool {
		spec, ok := node.(*goast.TypeSpec)
		if !ok {
			return true
		}
		var generatedFields []*goast.Field
		switch shape := spec.Type.(type) {
		case *goast.StructType:
			generatedFields = shape.Fields.List
		case *goast.InterfaceType:
			generatedFields = shape.Methods.List
		}
		for _, field := range generatedFields {
			if pos < field.Pos() || pos >= field.End() {
				continue
			}
			for _, name := range field.Names {
				for _, candidate := range fields {
					goName := candidate.GoName
					if goName == "" {
						goName = memberName(candidate.Name, candidate.Visibility)
					}
					if name.Name == goName {
						fieldOrigin = candidate.Type.Span
					}
				}
				if contract != nil {
					for _, method := range contract.Methods {
						checkedName := method.GoName
						if checkedName == "" {
							checkedName = memberName(method.Name, kmast.Public)
						}
						if checkedName == name.Name {
							fieldOrigin = method.Span
						}
					}
				}
			}
		}
		return false
	})
	if fieldOrigin.Path != "" {
		return fieldOrigin
	}
	if function, ok := declaration.(*goast.FuncDecl); ok {
		if sourceFunction, ok := original.(*kmast.FunctionDecl); ok {
			return instantiationSignatureOrigin(function.Type, sourceFunction.Parameters, sourceFunction.ReturnType, sourceFunction.Span, pos)
		}
		if sourceMethod, ok := original.(*kmast.MethodDecl); ok {
			return instantiationSignatureOrigin(function.Type, sourceMethod.Parameters, sourceMethod.ReturnType, sourceMethod.Span, pos)
		}
		for _, method := range methods {
			name := method.GoName
			if name == "" {
				name = memberName(method.Name, method.Visibility)
			}
			owner := ""
			switch original := original.(type) {
			case *kmast.ClassDecl:
				owner = original.Name
			case *kmast.StructDecl:
				owner = original.Name
			}
			if function.Name.Name == name || function.Name.Name == staticMethodName(owner, name, method.Visibility) {
				return instantiationSignatureOrigin(function.Type, method.Parameters, method.ReturnType, method.Span, pos)
			}
		}
	}
	return original.GetSpan()
}

func instantiationSignatureOrigin(signature *goast.FuncType, parameters []kmast.Parameter, result kmast.TypeRef, fallback source.Span, pos token.Pos) source.Span {
	if signature.Results != nil && pos >= signature.Results.Pos() && pos < signature.Results.End() && result.Span.Path != "" {
		return result.Span
	}
	if signature.Params != nil {
		for _, field := range signature.Params.List {
			if pos < field.Pos() || pos >= field.End() {
				continue
			}
			for _, name := range field.Names {
				for _, parameter := range parameters {
					if name.Name == goName(parameter.Name) {
						return parameter.Type.Span
					}
				}
			}
		}
	}
	return fallback
}
