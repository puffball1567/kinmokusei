package codegen

import (
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"
)

func parseDeferSupport(source string) ([]goast.Decl, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "defer_scope.go", source, parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("parse internal defer support: %w", err)
	}
	return parsed.Decls, nil
}

func deferRuntimeDeclarations() ([]goast.Decl, error) {
	return parseDeferSupport(`package p
type __kinmokuseiDeferFrame struct {
 calls []func(func())
 panicked bool
 value any
}
func __kinmokuseiDeferMapKey[K comparable,V any](m map[K]V,key K) K { return key }
func (frame *__kinmokuseiDeferFrame) run() {
 defer func() { if frame.panicked { panic(frame.value) } }()
 // Chunks bound recursive depth even when a loop registers many defers.
 // Native deferred chunks unwind newest first, like the calls inside them.
 for start := 0; start < len(frame.calls); start += 64 {
  end := start + 64
  if end > len(frame.calls) { end = len(frame.calls) }
  defer frame.runChunk(start,end)
 }
}
func (frame *__kinmokuseiDeferFrame) runChunk(start,end int) {
 resumed := false
 defer func() { if !resumed { frame.panicked = false } }()
 func() {
  completed := false
  defer func() {
   frame.value = recover()
   frame.panicked = !completed
  }()
  var next func(int)
  next = func(index int) {
   if index == end {
    if frame.panicked { panic(frame.value) }
    return
   }
   frame.calls[index](func() { next(index+1) })
  }
  next(start)
  completed = true
 }()
 resumed = true
}
`)
}
