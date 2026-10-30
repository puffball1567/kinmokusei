package parser

import (
	"reflect"
	"sync"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type syntaxShape struct {
	children []int
	span     int
}

var syntaxShapes sync.Map // reflect.Type -> syntaxShape; immutable after publication
var syntaxPackage = reflect.TypeOf(ast.Program{}).PkgPath()
var spanType = reflect.TypeOf(source.Span{})

// Only inspect raw parser output, before semantic metadata/links can form
// cycles. Walking AST-owned records automatically covers expression, type,
// statement and declaration children, including new syntax fields. Source
// spans and scalar/Go metadata are not syntax children. Cache field layouts so
// reflection inspects each record type only once across concurrent parses.
func shapeOfSyntax(t reflect.Type) syntaxShape {
	if cached, ok := syntaxShapes.Load(t); ok {
		return cached.(syntaxShape)
	}
	shape := syntaxShape{span: -1}
	for index := 0; index < t.NumField(); index++ {
		field := t.Field(index)
		if field.Name == "Span" && field.Type == spanType {
			shape.span = index
		}
		child := field.Type
		for child.Kind() == reflect.Pointer || child.Kind() == reflect.Slice || child.Kind() == reflect.Array {
			child = child.Elem()
		}
		if child.PkgPath() == syntaxPackage && (child.Kind() == reflect.Struct || child.Kind() == reflect.Interface) {
			shape.children = append(shape.children, index)
		}
	}
	actual, _ := syntaxShapes.LoadOrStore(t, shape)
	return actual.(syntaxShape)
}

// Use an explicit DFS stack, with one child processed at a time. Stack space
// depends on depth, not the width of a block/argument list; no recursive
// visitor touches an over-deep tree, even if it contains a million operators.
func excessiveSyntaxDepth(program *ast.Program) (source.Span, bool) {
	type frame struct {
		value       reflect.Value
		depth, next int
		shape       syntaxShape
	}
	stack := []frame{{value: reflect.ValueOf(program)}}
	for len(stack) != 0 {
		current := &stack[len(stack)-1]
		value := current.value
		for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
			if value.IsNil() {
				value = reflect.Value{}
				break
			}
			value = value.Elem()
		}
		if !value.IsValid() {
			stack = stack[:len(stack)-1]
			continue
		}
		switch value.Kind() {
		case reflect.Struct:
			if current.next == 0 {
				current.shape = shapeOfSyntax(value.Type())
				if current.depth > maxSyntaxDepth {
					// Some syntax containers have no span. Use the nearest
					// enclosing source record, without boxing spans on the
					// normal (non-error) traversal path.
					for index := len(stack) - 1; index >= 0; index-- {
						ancestor := stack[index].value
						for ancestor.Kind() == reflect.Pointer || ancestor.Kind() == reflect.Interface {
							ancestor = ancestor.Elem()
						}
						if ancestor.Kind() != reflect.Struct {
							continue
						}
						if shape := shapeOfSyntax(ancestor.Type()); shape.span >= 0 {
							if span := ancestor.Field(shape.span).Interface().(source.Span); span.Path != "" {
								return span, true
							}
						}
					}
					return source.Span{}, true
				}
			}
			if current.next == len(current.shape.children) {
				stack = stack[:len(stack)-1]
				continue
			}
			child := value.Field(current.shape.children[current.next])
			current.next++
			stack = append(stack, frame{value: child, depth: current.depth + 1})
		case reflect.Slice, reflect.Array:
			if current.next == value.Len() {
				stack = stack[:len(stack)-1]
				continue
			}
			child := value.Index(current.next)
			current.next++
			stack = append(stack, frame{value: child, depth: current.depth})
		default:
			stack = stack[:len(stack)-1]
		}
	}
	return source.Span{}, false
}
