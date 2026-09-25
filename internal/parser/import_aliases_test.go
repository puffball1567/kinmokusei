package parser

import (
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/lexer"
)

func TestParseImportAliases(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"import", "import go"} {
		input := prefix + " {\n First as Local, // binding\n Second, Third as Third,\n} from \"library\"\n"
		tokens, _ := lexer.Lex("imports.km", input)
		program, diagnostics := Parse(tokens)
		if len(diagnostics) != 0 || len(program.Imports) != 1 {
			t.Fatalf("%s: %v", prefix, diagnostics)
		}
		imported := program.Imports[0]
		if imported.Go != (prefix == "import go") || len(imported.Names) != 3 {
			t.Fatalf("%+v", imported)
		}
		for i, want := range []string{"Local", "Second", "Third"} {
			span := imported.BindingSpan(i)
			selected := imported.NameSpans[i]
			if imported.BindingName(i) != want || input[span.Start.Offset:span.End.Offset] != want || input[selected.Start.Offset:selected.End.Offset] != imported.Names[i] || imported.HasNameAlias(i) != (i != 1) {
				t.Fatalf("binding %d: %+v", i, imported)
			}
		}
	}
}

func TestParseInvalidImportAliases(t *testing.T) {
	t.Parallel()
	for _, selection := range []string{"First as", "First as _", "First as class", "First as Local as Other"} {
		for _, prefix := range []string{"import", "import go"} {
			input := prefix + " {" + selection + "} from \"library\";"
			tokens, _ := lexer.Lex("imports.km", input)
			_, diagnostics := Parse(tokens)
			if len(diagnostics) == 0 {
				t.Fatalf("accepted %s", input)
			}
			if strings.Contains(selection, "_") && !strings.Contains(diagnostics[0].Message, "cannot be '_'") {
				t.Fatalf("%s: %v", input, diagnostics)
			}
		}
	}
}
