package stdlib

import (
	"strings"
	"testing"
)

func TestLookupUsesExactCanonicalPaths(t *testing.T) {
	console, found := Lookup(ConsoleImportPath)
	if !found || console.ImportPath != ConsoleImportPath || console.VirtualPath == "" || !strings.Contains(console.Contents, "export { Console as console }") {
		t.Fatalf("invalid embedded console source: %#v", console)
	}
	input, found := Lookup(InputImportPath)
	if !found || input.ImportPath != InputImportPath || input.VirtualPath == "" || !strings.Contains(input.Contents, "function decodeInput<") || !strings.Contains(input.Contents, "function bindInput<") {
		t.Fatalf("invalid embedded input source: %#v", input)
	}
	source, ok := Lookup(HTTPImportPath)
	if !ok {
		t.Fatal("kinmokusei/http source is not embedded")
	}
	if source.ImportPath != HTTPImportPath || source.VirtualPath == "" ||
		!strings.Contains(source.Contents, "function fetch(") ||
		!strings.Contains(source.Contents, "class Context") ||
		!strings.Contains(source.Contents, "class App implements http.Handler") {
		t.Fatalf("invalid embedded HTTP source: %#v", source)
	}
	for _, invalid := range []string{"", "kinmokusei", "kinmokusei/HTTP", "kinmokusei/http/", "kinmokusei/../http", "./kinmokusei/http", "kinmokusei/Input", "kinmokusei/input/", "./kinmokusei/input", "kinmokusei/../input", "kinmokusei/Console", "kinmokusei/console/", "./kinmokusei/console", "kinmokusei/../console"} {
		if _, found := Lookup(invalid); found {
			t.Errorf("Lookup(%q) unexpectedly resolved", invalid)
		}
	}
}
