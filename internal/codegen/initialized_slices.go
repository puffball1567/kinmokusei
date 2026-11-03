package codegen

import (
	"fmt"
	goast "go/ast"
	"path/filepath"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

const initializedSliceName = "__kinmokuseiSliceInitialized"

func generateInitializedSlice(source *ast.SliceExpr, object, low, high, max goast.Expr) goast.Expr {
	bound := func(source ast.Expression, value goast.Expr) goast.Expr {
		if value == nil {
			return goast.NewIdent("0")
		}
		// Integral untyped floats are valid indexes, but would default to float64
		// during generic inference. Constants were checked against the target int.
		if isGoConstant(source) {
			return &goast.CallExpr{Fun: goast.NewIdent("int"), Args: []goast.Expr{value}}
		}
		return value
	}
	message := fmt.Sprintf("slice would expose uninitialized elements at %s:%d:%d", filepath.Base(source.Span.Path), source.Span.Start.Line, source.Span.Start.Column)
	return &goast.CallExpr{Fun: goast.NewIdent(initializedSliceName), Args: []goast.Expr{
		object, bound(source.Low, low), bound(source.High, high), bound(source.Max, max),
		goast.NewIdent(strconv.FormatBool(source.High != nil)), goast.NewIdent(strconv.FormatBool(source.Full)), stringLiteral(message),
	}}
}

func initializedSliceDeclarations(file *goast.File) ([]goast.Decl, error) {
	needed := false
	goast.Inspect(file, func(node goast.Node) bool {
		if identifier, ok := node.(*goast.Ident); ok && identifier.Name == initializedSliceName {
			needed = true
		}
		return true
	})
	if !needed {
		return nil, nil
	}
	return parseDeferSupport(`package p
func __kinmokuseiSliceInitialized[S ~[]E,E any,L,H,M interface{~int|~int8|~int16|~int32|~int64|~uint|~uint8|~uint16|~uint32|~uint64|~uintptr}](value S,low L,high H,max M,hasHigh,full bool,message string) S {
 // Let Go validate the original integer bounds without narrowing or changing
 // its runtime.Error. The temporary header cannot escape before our check.
 var result S
 if full {result=value[low:high:max]} else if hasHigh {result=value[low:high]} else {result=value[low:]}
 if hasHigh&&uint64(high)>uint64(len(value)){panic(message)}
 return result
}
`)
}
