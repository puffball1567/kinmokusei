package codegen

import (
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"
)

func nonNullKindsRuntimeDeclarations(alias string) ([]goast.Decl, error) {
	const placeholder = "__kinmokuseiNonNullReflectImport"
	file, err := parser.ParseFile(token.NewFileSet(), "non-null-kinds.go", fmt.Sprintf(`package runtime
func __kinmokuseiIsNonNullKinds(value any, kinds uint8) bool {
 v := %[1]s.ValueOf(value)
 if !v.IsValid() { return kinds & 4 == 0 }
 var kind uint8
 switch v.Kind() {
 case %[1]s.Pointer: kind=1
 case %[1]s.Func: kind=2
 case %[1]s.Interface: kind=4
 case %[1]s.Map: kind=8
 case %[1]s.Slice: kind=16
 case %[1]s.Chan: kind=32
 }
 return kinds & kind == 0 || !v.IsNil()
}
func __kinmokuseiCheckNonNullKinds(value any, kinds uint8, message string) {
 if !__kinmokuseiIsNonNullKinds(value,kinds) { panic(message) }
}
func __kinmokuseiValidateNonNullKinds[T any](value T, kinds uint8, message string, success bool) T {
 if success { __kinmokuseiCheckNonNullKinds(value,kinds,message) }; return value
}
`, placeholder), 0)
	if err != nil {
		return nil, err
	}
	// User-selected Go import aliases may be ordinary helper-local names.
	// Rename locals before installing the actual reflect qualifier.
	locals := map[string]bool{"value": true, "v": true, "kind": true, "kinds": true, "message": true, "success": true, "T": true}
	goast.Inspect(file, func(node goast.Node) bool {
		if name, ok := node.(*goast.Ident); ok {
			if name.Name == placeholder {
				name.Name = alias
			} else if name.Name == alias && locals[alias] {
				name.Name += "_local"
			}
		}
		return true
	})
	return file.Decls, nil
}
