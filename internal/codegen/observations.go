package codegen

import (
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func observationRuntimeDeclarations(alias string) ([]goast.Decl, error) {
	// No view, type object, address, callback or original value escapes these
	// operations. Equal/Float/String retain the corresponding reflect semantics.
	source := fmt.Sprintf(`package runtime
func __kinmokuseiInspectTypeName(value any) string {
 t := %[1]s.TypeOf(value); if t == nil { return "<nil>" }; return t.String()
}
func __kinmokuseiInspectSameType(a, b any) bool { return %[1]s.TypeOf(a) == %[1]s.TypeOf(b) }
func __kinmokuseiInspectKind(value any) string { return %[1]s.ValueOf(value).Kind().String() }
func __kinmokuseiInspectNil(value any) bool {
 v := %[1]s.ValueOf(value)
 if !v.IsValid() { return true }
 switch v.Kind() {
 case %[1]s.Chan, %[1]s.Func, %[1]s.Interface, %[1]s.Map, %[1]s.Pointer, %[1]s.Slice, %[1]s.UnsafePointer:
  return v.IsNil()
 }
 return false
}
func __kinmokuseiInspectComparable(value any) bool { return %[1]s.ValueOf(value).Comparable() }
func __kinmokuseiInspectEqual(a,b any) bool { return %[1]s.ValueOf(a).Equal(%[1]s.ValueOf(b)) }
func __kinmokuseiInspectFloat(value any) float64 { return %[1]s.ValueOf(value).Float() }
func __kinmokuseiInspectString(value any) string { return %[1]s.ValueOf(value).String() }
`, alias)
	file, err := parser.ParseFile(token.NewFileSet(), "observation-runtime.go", source, 0)
	if err != nil {
		return nil, err
	}
	return file.Decls, nil
}

func jsonSnapshotRuntimeDeclarations(alias string) ([]goast.Decl, error) {
	// In particular, json.UnsupportedValueError.Value is a writable reflective
	// alias to the input. Returning it (or an unwrap-able custom error) would
	// defeat the otherwise read-only encoder contract. Return only its text.
	source := fmt.Sprintf(`package runtime
type __kinmokuseiJSONSnapshotError string
func (e __kinmokuseiJSONSnapshotError) Error() string { return string(e) }
func __kinmokuseiJSONSnapshot(value any, prefix, indent string, pretty bool) ([]byte, error) {
 var encoded []byte
 var err error
 if pretty { encoded, err = %[1]s.MarshalIndent(value, prefix, indent) } else { encoded, err = %[1]s.Marshal(value) }
 if err != nil { return nil, __kinmokuseiJSONSnapshotError(err.Error()) }
 return encoded, nil
}
`, alias)
	file, err := parser.ParseFile(token.NewFileSet(), "json-snapshot-runtime.go", source, 0)
	if err != nil {
		return nil, err
	}
	return file.Decls, nil
}

func generateObservationCall(expr *ast.CallExpr) (goast.Expr, bool, error) {
	name := ""
	for _, definition := range ast.ObservationBuiltins() {
		if expr.Builtin == definition.Kind {
			name = "__kinmokusei" + memberName(definition.Name, ast.Public)
			break
		}
	}
	if expr.Builtin == ast.JSONSnapshotCall {
		name = "__kinmokuseiJSONSnapshot"
	}
	if name == "" {
		return nil, false, nil
	}
	arguments := make([]goast.Expr, 0, len(expr.Arguments))
	for _, argument := range expr.Arguments {
		value, err := generateExpression(argument)
		if err != nil {
			return nil, true, err
		}
		arguments = append(arguments, value)
	}
	if expr.Builtin == ast.JSONSnapshotCall && len(arguments) == 1 {
		arguments = append(arguments, stringLiteral(""), stringLiteral(""), goast.NewIdent("false"))
	} else if expr.Builtin == ast.JSONSnapshotCall {
		arguments = append(arguments, goast.NewIdent("true"))
	}
	return &goast.CallExpr{Fun: goast.NewIdent(name), Args: arguments}, true, nil
}
