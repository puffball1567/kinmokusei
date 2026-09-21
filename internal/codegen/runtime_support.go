package codegen

import (
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func taskRuntimeDeclarations() ([]goast.Decl, error) {
	const runtimeSource = `package taskruntime
type __kinmokuseiTask[T any] struct { done chan struct{}; value T; panicValue any }
type __kinmokuseiVoidTask struct { done chan struct{}; panicValue any }
type __kinmokuseiResultTask[T any] struct { done chan struct{}; value T; err error; panicValue any }
type __kinmokuseiVoidResultTask struct { done chan struct{}; err error; panicValue any }
`
	parsed, err := parser.ParseFile(token.NewFileSet(), "task_runtime.go", runtimeSource, parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("parse internal task runtime: %w", err)
	}
	return parsed.Decls, nil
}

func exceptionRuntimeDeclarations() ([]goast.Decl, error) {
	const runtimeSource = `package exceptionruntime
type __kinmokuseiException interface { KinmokuseiExceptionError() error }
type __kinmokuseiThrown struct { err error }
func (value __kinmokuseiThrown) KinmokuseiExceptionError() error { return value.err }
type __kinmokuseiReturn struct { value any; err error }
func __kinmokuseiReturnValue[T any](value any) T {
	if value == nil { var zero T; return zero }
	return value.(T)
}
type Exception struct { __kinmokuseiRoot any; Message string }
func __kinmokuseiInitException(this *Exception, message string) { this.Message = message; this.__kinmokuseiRoot = this }
func NewException(message string) *Exception { this := &Exception{}; __kinmokuseiInitException(this, message); return this }
func (this *Exception) Error() string { return this.Message }
func __kinmokuseiExceptionFromError(err error) *Exception {
	if value, ok := err.(*Exception); ok { return value }
	if err == nil { return NewException("") }
	return NewException(err.Error())
}
`
	parsed, err := parser.ParseFile(token.NewFileSet(), "exception_runtime.go", runtimeSource, parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("parse internal exception runtime: %w", err)
	}
	return parsed.Decls, nil
}

func taskTypeFromAnnotation(value kinmokuseiAST.TypeRef, resultTask, void bool) goast.Expr {
	if void {
		name := "__kinmokuseiVoidTask"
		if resultTask {
			name = "__kinmokuseiVoidResultTask"
		}
		return &goast.StarExpr{X: goast.NewIdent(name)}
	}
	name := "__kinmokuseiTask"
	if resultTask {
		name = "__kinmokuseiResultTask"
	}
	return &goast.StarExpr{X: &goast.IndexExpr{X: goast.NewIdent(name), Index: goType(value)}}
}

func generateTaskStart(expr *kinmokuseiAST.TaskStartExpr) (goast.Expr, error) {
	body, call, err := captureTaskCall(expr.Call)
	if err != nil {
		return nil, err
	}
	taskType := taskTypeFromAnnotation(expr.ValueType, expr.ResultTask, expr.Void)
	used := map[string]bool{}
	goast.Inspect(call, func(node goast.Node) bool {
		if name, ok := node.(*goast.Ident); ok {
			used[name.Name] = true
		}
		return true
	})
	taskName := "task"
	for i := 0; used[taskName]; i++ {
		taskName = fmt.Sprintf("__task%d", i)
	}
	task := goast.NewIdent(taskName)
	concreteTaskType := taskType.(*goast.StarExpr).X
	doneChannelType := &goast.ChanType{Dir: goast.SEND | goast.RECV, Value: &goast.StructType{Fields: &goast.FieldList{}}}
	taskLiteral := &goast.CompositeLit{Type: concreteTaskType, Elts: []goast.Expr{
		&goast.KeyValueExpr{Key: goast.NewIdent("done"), Value: &goast.CallExpr{Fun: goast.NewIdent("make"), Args: []goast.Expr{doneChannelType}}},
	}}
	body.List = append(body.List, &goast.AssignStmt{
		Lhs: []goast.Expr{task}, Tok: token.DEFINE,
		Rhs: []goast.Expr{&goast.UnaryExpr{Op: token.AND, X: taskLiteral}},
	})
	recoverBody := &goast.BlockStmt{List: []goast.Stmt{&goast.AssignStmt{
		Lhs: []goast.Expr{&goast.SelectorExpr{X: task, Sel: goast.NewIdent("panicValue")}},
		Tok: token.ASSIGN, Rhs: []goast.Expr{&goast.CallExpr{Fun: goast.NewIdent("recover")}},
	}}}
	workerBody := &goast.BlockStmt{List: []goast.Stmt{
		&goast.DeferStmt{Call: &goast.CallExpr{Fun: goast.NewIdent("close"), Args: []goast.Expr{&goast.SelectorExpr{X: task, Sel: goast.NewIdent("done")}}}},
		&goast.DeferStmt{Call: &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: recoverBody}}},
	}}
	switch {
	case expr.ResultTask && expr.Void:
		workerBody.List = append(workerBody.List, &goast.AssignStmt{Lhs: []goast.Expr{&goast.SelectorExpr{X: task, Sel: goast.NewIdent("err")}}, Tok: token.ASSIGN, Rhs: []goast.Expr{call}})
	case expr.ResultTask:
		workerBody.List = append(workerBody.List, &goast.AssignStmt{Lhs: []goast.Expr{&goast.SelectorExpr{X: task, Sel: goast.NewIdent("value")}, &goast.SelectorExpr{X: task, Sel: goast.NewIdent("err")}}, Tok: token.ASSIGN, Rhs: []goast.Expr{call}})
	case expr.Void:
		workerBody.List = append(workerBody.List, &goast.ExprStmt{X: call})
	default:
		workerBody.List = append(workerBody.List, &goast.AssignStmt{Lhs: []goast.Expr{&goast.SelectorExpr{X: task, Sel: goast.NewIdent("value")}}, Tok: token.ASSIGN, Rhs: []goast.Expr{call}})
	}
	body.List = append(body.List,
		&goast.GoStmt{Call: &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: workerBody}}},
		&goast.ReturnStmt{Results: []goast.Expr{task}},
	)
	return &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}, Results: &goast.FieldList{List: []*goast.Field{{Type: taskType}}}}, Body: body}}, nil
}

func generateAwait(expr *kinmokuseiAST.AwaitExpr) (goast.Expr, error) {
	value, err := generateExpression(expr.Value)
	if err != nil {
		return nil, err
	}
	body := &goast.BlockStmt{List: []goast.Stmt{
		&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent("task")}, Tok: token.DEFINE, Rhs: []goast.Expr{value}},
		&goast.ExprStmt{X: &goast.UnaryExpr{Op: token.ARROW, X: &goast.SelectorExpr{X: goast.NewIdent("task"), Sel: goast.NewIdent("done")}}},
		&goast.IfStmt{Cond: &goast.BinaryExpr{X: &goast.SelectorExpr{X: goast.NewIdent("task"), Sel: goast.NewIdent("panicValue")}, Op: token.NEQ, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: []goast.Stmt{
			&goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{&goast.SelectorExpr{X: goast.NewIdent("task"), Sel: goast.NewIdent("panicValue")}}}},
		}}},
	}}
	results := &goast.FieldList{}
	switch {
	case expr.ResultTask && expr.Void:
		results.List = []*goast.Field{{Type: goast.NewIdent("error")}}
		body.List = append(body.List, &goast.ReturnStmt{Results: []goast.Expr{&goast.SelectorExpr{X: goast.NewIdent("task"), Sel: goast.NewIdent("err")}}})
	case expr.ResultTask:
		results.List = []*goast.Field{{Type: goType(expr.ValueType)}, {Type: goast.NewIdent("error")}}
		body.List = append(body.List, &goast.ReturnStmt{Results: []goast.Expr{&goast.SelectorExpr{X: goast.NewIdent("task"), Sel: goast.NewIdent("value")}, &goast.SelectorExpr{X: goast.NewIdent("task"), Sel: goast.NewIdent("err")}}})
	case expr.Void:
		results = nil
	default:
		results.List = []*goast.Field{{Type: goType(expr.ValueType)}}
		body.List = append(body.List, &goast.ReturnStmt{Results: []goast.Expr{&goast.SelectorExpr{X: goast.NewIdent("task"), Sel: goast.NewIdent("value")}}})
	}
	return &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}, Results: results}, Body: body}}, nil
}

func orderedSliceMake(sliceType, length, capacity goast.Expr) goast.Expr {
	lengthName := goast.NewIdent("__kinmokusei_make_length")
	capacityName := goast.NewIdent("__kinmokusei_make_capacity")
	returnType := &goast.FieldList{List: []*goast.Field{{Type: sliceType}}}
	body := &goast.BlockStmt{List: []goast.Stmt{
		&goast.AssignStmt{
			Lhs: []goast.Expr{lengthName}, Tok: token.DEFINE, Rhs: []goast.Expr{length},
		},
		&goast.AssignStmt{
			Lhs: []goast.Expr{capacityName}, Tok: token.DEFINE, Rhs: []goast.Expr{capacity},
		},
		&goast.ReturnStmt{Results: []goast.Expr{&goast.CallExpr{
			Fun: goast.NewIdent("make"), Args: []goast.Expr{sliceType, lengthName, capacityName},
		}}},
	}}
	return &goast.CallExpr{Fun: &goast.FuncLit{
		Type: &goast.FuncType{Params: &goast.FieldList{}, Results: returnType}, Body: body,
	}}
}
