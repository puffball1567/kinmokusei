package codegen

import (
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func iteratorControlName(kind ast.ForRangeKind) string {
	switch kind {
	case ast.IteratorZeroRange:
		return "__kinmokuseiIterator0"
	case ast.IteratorRange:
		return "__kinmokuseiIterator1"
	case ast.IteratorPairRange:
		return "__kinmokuseiIterator2"
	}
	return ""
}

// Preserve checked iterator classification until the whole program's exception
// use is known. This operates on fresh Go nodes, never the shared checked AST.
// Programs without language exceptions retain their original direct Go ranges.
func iteratorControlDeclarations(file *goast.File, enabled bool) ([]goast.Decl, error) {
	needed := false
	goast.Inspect(file, func(node goast.Node) bool {
		loop, ok := node.(*goast.RangeStmt)
		if !ok {
			return true
		}
		call, ok := loop.X.(*goast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		name, ok := call.Fun.(*goast.Ident)
		if !ok || (name.Name != "__kinmokuseiIterator0" && name.Name != "__kinmokuseiIterator1" && name.Name != "__kinmokuseiIterator2") {
			return true
		}
		if !enabled {
			loop.X = call.Args[0]
		} else {
			needed = true
		}
		return true
	})
	if !needed {
		return nil, nil
	}
	const runtimeSource = `package iteratorcontrol
// Each active range owns a distinct, non-zero-sized frame. Nested adapters must
// not unwrap a caller's signal before it leaves the iterator implementation.
type __kinmokuseiIteratorFrame struct { control any }
type __kinmokuseiIteratorPanic struct { frame *__kinmokuseiIteratorFrame }
func (frame *__kinmokuseiIteratorFrame) invoke(call func()bool) bool {
 var value any
 var result bool
 completed:=false
 func(){defer func(){value=recover()}();result=call();completed=true}()
 if completed{return result}
 // Goexit never resumes here. A recovered legacy panic(nil) does, and must
 // be rethrown even though recover returned nil.
 switch value.(type) {
 case interface { __kinmokuseiReturnControl() }, interface { KinmokuseiExceptionError() error }:
  frame.control=value
  panic(__kinmokuseiIteratorPanic{frame})
 default:
  panic(value)
 }
}
func (frame *__kinmokuseiIteratorFrame) run(call func()) {
 var value any
 completed:=false
 func(){defer func(){value=recover()}();call();completed=true}()
 if !completed {
  if wrapped,ok:=value.(__kinmokuseiIteratorPanic);ok&&wrapped.frame==frame {panic(frame.control)}
  panic(value)
 }
 // A return in the iterator's finally must not consume its caller's pending
 // completion. A Goexit, unlike a normal source return, keeps unwinding.
 if frame.control!=nil {panic(frame.control)}
}
func __kinmokuseiIterator0[Y ~func()bool,S ~func(Y)](source S)func(func()bool) {
 return func(yield func()bool) {
  frame:=&__kinmokuseiIteratorFrame{}
  frame.run(func(){source(Y(func()bool{return frame.invoke(yield)}))})
 }
}
func __kinmokuseiIterator1[T any,Y ~func(T)bool,S ~func(Y)](source S)func(func(T)bool) {
 return func(yield func(T)bool) {
  frame:=&__kinmokuseiIteratorFrame{}
  frame.run(func(){source(Y(func(value T)bool{return frame.invoke(func()bool{return yield(value)})}))})
 }
}
func __kinmokuseiIterator2[K,V any,Y ~func(K,V)bool,S ~func(Y)](source S)func(func(K,V)bool) {
 return func(yield func(K,V)bool) {
  frame:=&__kinmokuseiIteratorFrame{}
  frame.run(func(){source(Y(func(key K,value V)bool{return frame.invoke(func()bool{return yield(key,value)})}))})
 }
}
`
	parsed, err := parser.ParseFile(token.NewFileSet(), "iterator_control.go", runtimeSource, parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("parse internal iterator control runtime: %w", err)
	}
	return parsed.Decls, nil
}
