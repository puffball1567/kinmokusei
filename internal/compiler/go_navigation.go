package compiler

import (
	goast "go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type positionedGoImporter struct {
	types.Importer
	fileSet *token.FileSet
}

func newPositionedGoImporter() types.Importer {
	files := token.NewFileSet()
	return &positionedGoImporter{Importer: importer.ForCompiler(files, "gc", nil), fileSet: files}
}

func (i *positionedGoImporter) GoSourcePosition(pos int) token.Position {
	return goSourcePosition(i.fileSet, pos)
}
func (i *moduleGoImporter) GoSourcePosition(pos int) token.Position {
	return goSourcePosition(i.fileSet, pos)
}

func goSourcePosition(files *token.FileSet, pos int) token.Position {
	if files == nil || pos <= 0 {
		return token.Position{}
	}
	position := files.Position(token.Pos(pos))
	if strings.HasPrefix(position.Filename, "$GOROOT/") {
		// Export data refers to the selected Go toolchain, which can differ from
		// the toolchain used to build keika. Do not resolve through runtime.GOROOT.
		output, err := exec.Command("go", "env", "GOROOT").Output()
		if err != nil {
			return token.Position{}
		}
		root := strings.TrimSpace(string(output))
		if root == "" {
			return token.Position{}
		}
		position.Filename = filepath.Join(root, strings.TrimPrefix(position.Filename, "$GOROOT/"))
	}
	return position
}

// GoSymbol provides hover text even if the selected package's sources are
// unavailable. A declaration is returned only after verifying its actual AST
// identifier; compiler export positions do not retain precise byte offsets.
type GoSymbol struct {
	Detail        string
	Documentation string
	Declaration   source.Span
}

func (r Result) GoSymbol(reference ast.GoReference) GoSymbol {
	result := GoSymbol{Detail: reference.Detail}
	positions, ok := r.goImporter.(interface{ GoSourcePosition(int) token.Position })
	if !ok {
		return result
	}
	position := positions.GoSourcePosition(reference.Position)
	if !position.IsValid() || position.Filename == "" {
		return result
	}
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, position.Filename, nil, parser.ParseComments)
	if err != nil {
		return result
	}
	span, documentation := goDeclarationAt(files, file, reference.Name, position.Line)
	result.Declaration, result.Documentation = span, documentation
	return result
}

func goDeclarationAt(files *token.FileSet, file *goast.File, name string, line int) (source.Span, string) {
	var selected *goast.Ident
	var documentation string
	matches := 0
	selectName := func(id *goast.Ident, doc, trailing *goast.CommentGroup) {
		if id == nil || id.Name != name || files.Position(id.Pos()).Line != line {
			return
		}
		matches++
		selected = id
		documentation = strings.TrimSpace(doc.Text())
		if documentation == "" {
			documentation = strings.TrimSpace(trailing.Text())
		}
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *goast.FuncDecl:
			selectName(declaration.Name, declaration.Doc, nil)
		case *goast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *goast.TypeSpec:
					doc := spec.Doc
					if doc == nil {
						doc = declaration.Doc
					}
					selectName(spec.Name, doc, spec.Comment)
					// Named struct fields and interface methods retain their own comments.
					goast.Inspect(spec.Type, func(node goast.Node) bool {
						field, ok := node.(*goast.Field)
						if !ok {
							return true
						}
						fieldDoc := field.Doc
						if _, isInterface := spec.Type.(*goast.InterfaceType); isInterface && fieldDoc == nil {
							fieldDoc = doc
						}
						for _, id := range field.Names {
							selectName(id, fieldDoc, field.Comment)
						}
						if len(field.Names) == 0 {
							selectName(goEmbeddedIdentifier(field.Type), field.Doc, field.Comment)
						}
						return true
					})
				case *goast.ValueSpec:
					doc := spec.Doc
					if doc == nil {
						doc = declaration.Doc
					}
					for _, id := range spec.Names {
						selectName(id, doc, spec.Comment)
					}
				}
			}
		}
	}
	if matches != 1 {
		return source.Span{}, ""
	}
	start, end := files.Position(selected.Pos()), files.Position(selected.End())
	return source.Span{Path: start.Filename, Start: source.Position{Offset: start.Offset, Line: start.Line, Column: start.Column}, End: source.Position{Offset: end.Offset, Line: end.Line, Column: end.Column}}, documentation
}

func goEmbeddedIdentifier(expression goast.Expr) *goast.Ident {
	switch expression := expression.(type) {
	case *goast.Ident:
		return expression
	case *goast.SelectorExpr:
		return expression.Sel
	case *goast.StarExpr:
		return goEmbeddedIdentifier(expression.X)
	case *goast.IndexExpr:
		return goEmbeddedIdentifier(expression.X)
	case *goast.IndexListExpr:
		return goEmbeddedIdentifier(expression.X)
	}
	return nil
}
