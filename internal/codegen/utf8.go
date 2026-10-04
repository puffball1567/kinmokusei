package codegen

import (
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"
)

func utf8RuntimeDeclarations(alias string) ([]goast.Decl, error) {
	// Check wide bounds before narrowing so large unsigned indices cannot wrap.
	source := fmt.Sprintf(`package runtime
type __kinmokuseiUTF8Error struct{}
func (__kinmokuseiUTF8Error) Error() string { return "invalid UTF-8" }
func __kinmokuseiDecodeUTF8[T ~string](value string) (T, error) {
  if !%s.ValidString(value) { return T(""), __kinmokuseiUTF8Error{} }
  return T(value), nil
}
func __kinmokuseiSliceUTF8[T ~string](value T, low, high uint64, hasHigh bool) T {
  if !hasHigh { high = uint64(len(value)) }
  if low > high || high > uint64(len(value)) { panic("string slice bounds out of range") }
  if low != uint64(len(value)) && value[low]&0xc0 == 0x80 || high != uint64(len(value)) && value[high]&0xc0 == 0x80 {
    panic("string slice is not on a UTF-8 boundary; slice bstring(value) for raw bytes")
  }
  return value[low:high]
}
`, alias)
	file, err := parser.ParseFile(token.NewFileSet(), "utf8-runtime.go", source, 0)
	if err != nil {
		return nil, err
	}
	return file.Decls, nil
}
