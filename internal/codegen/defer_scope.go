package codegen

import (
	goast "go/ast"
	"go/token"
	"sort"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

const sourceDeferName = "__kinmokuseiSourceDefer"
const sourceCallableName = "__kinmokuseiSourceCallable"

// These transient markers distinguish source callables/defers from exception
// machinery. They are consumed on the fresh Go AST before formatting, including
// when emitting source maps; they never enter the checked source AST.
func sourceCallableMarker() goast.Stmt {
	return &goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent(sourceCallableName)}}
}

func markSourceDefer(source *ast.CallExpr, call *goast.CallExpr) goast.Stmt {
	mask := make([]byte, len(call.Args))
	offset := len(call.Args) - len(source.Arguments)
	for i := range mask {
		mask[i] = 'v'
		if index := i - offset; index >= 0 && index < len(source.Arguments) && isGoConstant(source.Arguments[index]) {
			mask[i] = 'c'
		}
	}
	// A generic method/tuple capture factory must itself run at registration.
	capture := !source.GenericCall || source.MultipleArgumentResult != nil
	if source.Builtin != ast.NotBuiltinCall {
		if _, builtin := call.Fun.(*goast.Ident); builtin {
			capture = false
		}
	}
	tuple := source.MultipleArgumentCount
	if source.MultipleArgumentResult != nil {
		tuple = 0 // The factory already captures the expanded result tuple.
	}
	context := goast.Expr(goast.NewIdent("nil"))
	if source.DeferSignature != nil {
		context = goType(*source.DeferSignature)
		if signature, ok := context.(*goast.FuncType); ok {
			for i := 0; i < offset; i++ {
				signature.Params.List = append([]*goast.Field{{Type: goast.NewIdent("any")}}, signature.Params.List...)
			}
		}
	}
	arguments := []goast.Expr{
		&goast.BasicLit{Kind: token.STRING, Value: strconv.Quote(string(mask))},
		goast.NewIdent(strconv.FormatBool(capture)),
		&goast.BasicLit{Kind: token.INT, Value: strconv.Itoa(tuple)}, context,
	}
	for i := range call.Args {
		context := goast.Expr(goast.NewIdent("nil"))
		if index := i - offset; index >= 0 && index < len(source.CaptureArgumentTypes) {
			ref := source.CaptureArgumentTypes[index]
			if ref.Name != "" || ref.IsArray() || ref.IsPointer() || ref.IsFunction() {
				context = goType(ref)
			}
		}
		arguments = append(arguments, context)
	}
	return &goast.BlockStmt{List: []goast.Stmt{
		&goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent(sourceDeferName), Args: arguments}},
		&goast.DeferStmt{Call: call},
	}}
}

func sourceMarker(stmt goast.Stmt, name string) *goast.CallExpr {
	expression, ok := stmt.(*goast.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := expression.X.(*goast.CallExpr)
	if !ok {
		return nil
	}
	identifier, ok := call.Fun.(*goast.Ident)
	if !ok || identifier.Name != name {
		return nil
	}
	return call
}

func markedSourceCallable(body *goast.BlockStmt) bool {
	return len(body.List) != 0 && sourceMarker(body.List[0], sourceCallableName) != nil
}

func lowerSourceDefers(file *goast.File) ([]goast.Decl, error) {
	needed := false
	contexts := map[string]*goast.FuncDecl{}
	goast.Inspect(file, func(node goast.Node) bool {
		switch fn := node.(type) {
		case *goast.FuncDecl:
			if fn.Body != nil && lowerCallableDefers(fn.Type, fn.Body, contexts) {
				needed = true
			}
			return false
		case *goast.FuncLit:
			if markedSourceCallable(fn.Body) {
				if lowerCallableDefers(fn.Type, fn.Body, contexts) {
					needed = true
				}
				return false
			}
		}
		return true
	})
	if !needed {
		return nil, nil
	}
	declarations, err := deferRuntimeDeclarations()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(contexts))
	for name := range contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		declarations = append(declarations, contexts[name])
	}
	return declarations, nil
}

func lowerCallableDefers(signature *goast.FuncType, body *goast.BlockStmt, contexts map[string]*goast.FuncDecl) bool {
	if markedSourceCallable(body) {
		body.List = body.List[1:]
	}
	var registrations []*goast.BlockStmt
	needed, crossesFrame := false, false
	var scan func(goast.Node, int)
	scan = func(root goast.Node, depth int) {
		goast.Inspect(root, func(node goast.Node) bool {
			if fn, ok := node.(*goast.FuncLit); ok {
				if markedSourceCallable(fn.Body) {
					if lowerCallableDefers(fn.Type, fn.Body, contexts) {
						needed = true
					}
				} else {
					scan(fn.Body, depth+1)
				}
				return false
			}
			if block, ok := node.(*goast.BlockStmt); ok && len(block.List) == 2 && sourceMarker(block.List[0], sourceDeferName) != nil {
				registrations = append(registrations, block)
				crossesFrame = crossesFrame || depth > 0
			}
			return true
		})
	}
	scan(body, 0)
	if !crossesFrame {
		// Leave ordinary defer-only functions as direct, idiomatic Go.
		plain := map[*goast.BlockStmt]goast.Stmt{}
		for _, block := range registrations {
			plain[block] = block.List[1]
		}
		unwrapSourceDefers(body, plain)
		return needed
	}
	for _, block := range registrations {
		captureSourceDefer(block, contexts)
	}
	wrapDeferredCallable(signature, body)
	return true
}

func unwrapSourceDefers(body *goast.BlockStmt, plain map[*goast.BlockStmt]goast.Stmt) {
	replace := func(stmt goast.Stmt) goast.Stmt {
		if block, ok := stmt.(*goast.BlockStmt); ok && plain[block] != nil {
			return plain[block]
		}
		return stmt
	}
	goast.Inspect(body, func(node goast.Node) bool {
		switch node := node.(type) {
		case *goast.BlockStmt:
			for i := range node.List {
				node.List[i] = replace(node.List[i])
			}
		case *goast.CaseClause:
			for i := range node.Body {
				node.Body[i] = replace(node.Body[i])
			}
		case *goast.CommClause:
			for i := range node.Body {
				node.Body[i] = replace(node.Body[i])
			}
		case *goast.LabeledStmt:
			node.Stmt = replace(node.Stmt)
		}
		return true
	})
}
