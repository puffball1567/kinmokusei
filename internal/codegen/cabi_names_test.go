package codegen

import (
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestGenerateCABIRejectsUnsafeSymbolDefensively(t *testing.T) {
	for _, symbol := range []string{"double", "GoString", "malloc", "nil", "KINMOKUSEI_ABI_OK", "Exception", "NewException"} {
		program := &ast.Program{UsesExceptions: true, CABIExports: []ast.CABIExport{{Symbol: symbol}}}
		output, err := GenerateCABI(program)
		if err == nil || !strings.Contains(err.Error(), symbol) || len(output.Gateway) != 0 || len(output.Header) != 0 || len(output.Manifest) != 0 {
			t.Fatalf("symbol=%s err=%v output=%#v", symbol, err, output)
		}
	}
}
