package sema

import (
	gotypes "go/types"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/source"
)

func TestGoInterfaceTextDiagnostics(t *testing.T) {
	tests := []struct {
		name, declarations, parameter, want string
	}{
		{"verified field", `struct Config { public name:string; }`, "*Config", "verified UTF-8 string storage"},
		{"nested field", `struct Item { public name:string; } struct Config { public items:Item[]; }`, "*Config", "verified UTF-8 string storage"},
		{"generic instance", `struct Config<T> { public item:T; }`, "*Config<string>", "verified UTF-8 string storage"},
		{"open pointer", "", "*T", "generic storage may hide verified UTF-8 strings"},
		{"open value", "", "T", "generic storage may hide verified UTF-8 strings"},
		{"hidden interface storage", `interface Label { function label():bstring; } class Row implements Label { public text:string=""; public function label():bstring{return "";} }`, "Label", "verified UTF-8 string storage"},
		{"shared slice", "", "string[]", "verified UTF-8 string storage"},
		{"callback", "", "(text:string)=>void", "verified UTF-8 string storage"},
		{"raw pointer", `struct Config { public name:bstring; }`, "*Config", ""},
		{"numeric pointer", `struct Config { public count:int; }`, "*Config", ""},
		{"scalar text", "", "string", ""},
		{"copied text field", `struct Config { public name:string; }`, "Config", ""},
		{"raw generic instance", `struct Config<T> { public item:T; }`, "*Config<bstring>", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parameters := ""
			if test.parameter == "T" || test.parameter == "*T" {
				parameters = "<T>"
			}
			diagnostics := checkSource(t, `import go reflect from "reflect";`+test.declarations+`function inspect`+parameters+`(value:`+test.parameter+`):void{reflect.ValueOf(value);}`)
			if test.want == "" {
				if len(diagnostics) != 0 {
					t.Fatalf("unexpected diagnostics: %v", diagnostics)
				}
				return
			}
			if len(diagnostics) != 1 || !strings.HasPrefix(diagnostics[0], "cannot use ") || !strings.Contains(diagnostics[0], " as any: "+test.want) || !strings.Contains(diagnostics[0], "bstring/raw input storage") || !strings.Contains(diagnostics[0], "string(raw)") {
				t.Fatalf("diagnostics = %v; want reason %q and checked input guidance", diagnostics, test.want)
			}
		})
	}
}

func TestJSONDecoderVerifiedTextDiagnostic(t *testing.T) {
	diagnostics := checkSource(t, `import go json from "encoding/json";
import go strings from "strings";
struct Config { public name:string; }
function decode():Result<void>{
  let config=Config{name:""};
  const decoder=json.NewDecoder(strings.NewReader("{}"));
  decoder.Decode(&config)?;
  return ok();
}`)
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "cannot use *Config as any: verified UTF-8 string storage") {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestUnrelatedAssignmentDiagnosticHasNoTextAdvice(t *testing.T) {
	diagnostics := checkSource(t, `function f():int{return "text";}`)
	if len(diagnostics) != 1 || strings.Contains(diagnostics[0], "UTF-8") || !strings.Contains(diagnostics[0], "cannot use string as int") {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestForwardedTextResultDiagnosticKeepsLocation(t *testing.T) {
	text := builtins["string"]
	value := Type{Kind: GoPointer, Element: &text, GoType: gotypes.NewPointer(gotypes.Typ[gotypes.String])}
	target := Type{Kind: GoInterface, Name: "any", GoType: gotypes.NewInterfaceType(nil, nil).Complete()}
	span := source.Span{Path: "results.km", Start: source.Position{Line: 4, Column: 10}}
	checker := &Checker{}
	checker.requireAssignable(Type{Kind: MultiValue, Results: []Type{target, builtins["int"]}}, Type{Kind: MultiValue, Results: []Type{value, builtins["int"]}}, span)
	if len(checker.diagnostics) != 1 {
		t.Fatalf("diagnostics = %v", checker.diagnostics)
	}
	diagnostic := checker.diagnostics[0]
	if diagnostic.Span != span || !strings.HasPrefix(diagnostic.Message, "cannot use result 1 of ") || !strings.Contains(diagnostic.Message, "verified UTF-8 string storage") {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}
