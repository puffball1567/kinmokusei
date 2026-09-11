package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/lexer"
)

func TestParseNamedGoImports(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`import go { Println, Sprint } from "fmt";`,
		"import go {\n Println, // comment\n Sprint,\n} from \"fmt\"\nfunction main():void{}",
	} {
		tokens, _ := lexer.Lex("imports.km", input)
		program, diagnostics := Parse(tokens)
		if len(diagnostics) != 0 || len(program.Imports) != 1 {
			t.Fatalf("%v: %v", input, diagnostics)
		}
		imported := program.Imports[0]
		if !imported.Go || imported.Alias != "" || imported.Path != "fmt" || len(imported.Names) != 2 {
			t.Fatalf("%+v", imported)
		}
		for i, span := range imported.NameSpans {
			if input[span.Start.Offset:span.End.Offset] != imported.Names[i] {
				t.Fatalf("bad name span %+v", span)
			}
		}
	}
}
