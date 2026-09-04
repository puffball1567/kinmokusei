package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoPackageFunctionSignatureMatrix(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		export        string
		wantNames     []string
		wantTypes     []string
		wantResult    string
		wantVariadic  bool
		wantAvailable bool
	}{
		{
			name: "ordinary function", path: "strings", export: "ReplaceAll",
			wantNames: []string{"s", "old", "new"}, wantTypes: []string{"string", "string", "string"},
			wantResult: "string", wantAvailable: true,
		},
		{
			name: "variadic function", path: "path", export: "Join",
			wantNames: []string{"elem"}, wantTypes: []string{"string"},
			wantResult: "string", wantVariadic: true, wantAvailable: true,
		},
		{
			name: "Kinmokusei collection spelling", path: "bytes", export: "Clone",
			wantNames: []string{"b"}, wantTypes: []string{"byte[]"},
			wantResult: "byte[]", wantAvailable: true,
		},
		{name: "type is not a function", path: "strings", export: "Reader"},
		{name: "missing export", path: "strings", export: "DoesNotExist"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			signature, available, err := (Result{}).GoPackageFunctionSignature(test.path, test.export)
			if err != nil {
				t.Fatal(err)
			}
			if available != test.wantAvailable {
				t.Fatalf("available = %v, want %v", available, test.wantAvailable)
			}
			if !available {
				return
			}
			if strings.Join(signature.ParameterNames, ",") != strings.Join(test.wantNames, ",") ||
				strings.Join(signature.ParameterTypes, ",") != strings.Join(test.wantTypes, ",") ||
				signature.Result != test.wantResult || signature.Variadic != test.wantVariadic {
				t.Fatalf("signature = %#v", signature)
			}
		})
	}
}

func TestGoTypeMembersFollowGoSelectorRules(t *testing.T) {
	result := Result{}
	valueMembers, found, err := result.GoTypeMembers("net/http", "Request", false, false)
	if err != nil || !found {
		t.Fatalf("Request value members: found=%v err=%v", found, err)
	}
	value := map[string]GoMember{}
	for _, member := range valueMembers {
		value[member.Name] = member
	}
	if value["Method"].Kind != "field" || value["URL"].Kind != "field" {
		t.Fatalf("Request fields = %#v", value)
	}
	if value["Clone"].Name != "" {
		t.Fatalf("non-addressable Request exposed pointer method Clone: %#v", value["Clone"])
	}

	addressableMembers, found, err := result.GoTypeMembers("net/http", "Request", false, true)
	if err != nil || !found {
		t.Fatalf("addressable Request members: found=%v err=%v", found, err)
	}
	addressable := map[string]GoMember{}
	for _, member := range addressableMembers {
		addressable[member.Name] = member
	}
	if addressable["Clone"].Kind != "method" || !strings.Contains(addressable["Clone"].Detail, "Clone") {
		t.Fatalf("addressable Request Clone = %#v", addressable["Clone"])
	}
	if addressable["ctx"].Name != "" {
		t.Fatalf("unexported Request field leaked: %#v", addressable["ctx"])
	}

	pointerMembers, found, err := result.GoTypeMembers("net/http", "Request", true, false)
	if err != nil || !found {
		t.Fatalf("*Request members: found=%v err=%v", found, err)
	}
	pointer := map[string]GoMember{}
	for _, member := range pointerMembers {
		pointer[member.Name] = member
	}
	if pointer["Clone"].Kind != "method" || pointer["Method"].Kind != "field" {
		t.Fatalf("*Request members = %#v", pointer)
	}

	if members, found, err := result.GoTypeMembers("net/http", "Missing", false, true); err != nil || found || len(members) != 0 {
		t.Fatalf("missing type members=%#v found=%v err=%v", members, found, err)
	}
}

func TestGoTypeMethodSignatureFollowsSelectorRules(t *testing.T) {
	result := Result{}
	clone, found, err := result.GoTypeMethodSignature("net/http", "Request", false, true, "Clone")
	if err != nil || !found {
		t.Fatalf("Request.Clone: found=%v err=%v", found, err)
	}
	if len(clone.ParameterNames) != 1 || clone.ParameterNames[0] != "ctx" || clone.ParameterTypes[0] != "context.Context" || clone.Result != "*http.Request" || clone.Variadic {
		t.Fatalf("Request.Clone signature = %#v", clone)
	}
	if _, found, err := result.GoTypeMethodSignature("net/http", "Request", false, false, "Clone"); err != nil || found {
		t.Fatalf("non-addressable Request.Clone found=%v err=%v", found, err)
	}
	do, found, err := result.GoTypeMethodSignature("net/http", "Client", true, false, "Do")
	if err != nil || !found || len(do.ParameterTypes) != 1 || do.ParameterTypes[0] != "*http.Request" || do.Result != "(*http.Response, error)" {
		t.Fatalf("Client.Do signature = %#v found=%v err=%v", do, found, err)
	}
	for _, name := range []string{"Timeout", "missing"} {
		if signature, found, err := result.GoTypeMethodSignature("net/http", "Client", true, true, name); err != nil || found {
			t.Fatalf("Client.%s signature=%#v found=%v err=%v", name, signature, found, err)
		}
	}
}

func TestEmitGoMultipleFilesAndCompile(t *testing.T) {
	temp := t.TempDir()
	first := filepath.Join(temp, "first.km")
	second := filepath.Join(temp, "second.km")
	if err := os.WriteFile(first, []byte(`function add(left: int, right: int): int { return left + right; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte(`
import { add } from "./first";
function answer(): int {
  const increment = (value: int) => add(value, 1);
  let result = 0;
  for (let index = 0; index < 2; index = index + 1) {
    result = increment(result);
  }
  return result;
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{second}, "sample")
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", diagnostics)
	}
	if strings.Index(string(generated), "func add") > strings.Index(string(generated), "func answer") {
		t.Fatalf("declarations are not emitted in stable path order:\n%s", generated)
	}
	referenceSource := `package reference
func Add(left, right int) int { return left + right }
func Answer() int {
  increment := func(value int) int { return Add(value, 1) }
  result := 0
  for index := 0; index < 2; index++ { result = increment(result) }
  return result
}
`
	testSource := `package sample
import (
  "testing"
  reference "generated.test/reference"
)
func TestMultipleFiles(t *testing.T) {
  if got, want := answer(), reference.Answer(); got != want { t.Errorf("answer = %d, Go = %d", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "generated.test", generated, referenceSource, testSource)
}

func TestExtensionlessRelativeImportSupportsLegacyMigrationSources(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "main.otm")
	dependency := filepath.Join(root, "value.otm")
	if err := os.WriteFile(entry, []byte(`import { value } from "./value"; function answer(): int { return value(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dependency, []byte(`function value(): int { return 42; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := CheckFiles([]string{entry})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("legacy relative import diagnostics: %v", result.Diagnostics)
	}
}

func TestCheckFilesReportsImportCycle(t *testing.T) {
	temp := t.TempDir()
	first := filepath.Join(temp, "first.km")
	second := filepath.Join(temp, "second.km")
	if err := os.WriteFile(first, []byte(`import { second } from "./second"; function first(): int { return second(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte(`import { first } from "./first"; function second(): int { return first(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := CheckFiles([]string{first})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, "import cycle") {
		t.Fatalf("diagnostics = %v", result.Diagnostics)
	}
}

func TestCheckFilesValidatesImportedNames(t *testing.T) {
	temp := t.TempDir()
	dependency := filepath.Join(temp, "dependency.km")
	entry := filepath.Join(temp, "entry.km")
	if err := os.WriteFile(dependency, []byte(`function available(): int { return 42; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`import { missing } from "./dependency"; function value(): int { return 42; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := CheckFiles([]string{entry})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, "does not declare") {
		t.Fatalf("diagnostics = %v", result.Diagnostics)
	}
}

func TestCheckFilesReportsSourcePath(t *testing.T) {
	temp := t.TempDir()
	path := filepath.Join(temp, "broken.km")
	if err := os.WriteFile(path, []byte(`function bad(): int { return false; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := CheckFiles([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("diagnostics: %v", result.Diagnostics)
	}
	if result.Diagnostics[0].Span.Path != path {
		t.Fatalf("diagnostic path = %q", result.Diagnostics[0].Span.Path)
	}
}

func TestCheckFilesWithOverlayUsesUnsavedRootAndImportedDocuments(t *testing.T) {
	temp := t.TempDir()
	entry := filepath.Join(temp, "entry.km")
	dependency := filepath.Join(temp, "dependency.km")
	overlay := map[string]string{
		entry:      `import { value } from "./dependency"; function answer(): int { return value(); }`,
		dependency: `function value(): int { return 42; }`,
	}
	result, err := CheckFilesWithOverlay([]string{entry}, overlay)
	if err != nil || len(result.Diagnostics) != 0 || len(result.Program.Declarations) != 2 {
		t.Fatalf("result=%#v err=%v", result, err)
	}

	overlay[dependency] = `function value(): int { return "wrong"; }`
	result, err = CheckFilesWithOverlay([]string{entry}, overlay)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0].Span.Path != dependency || !strings.Contains(result.Diagnostics[0].Message, "cannot use string as int") {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
}

func TestGeneratedClassAndCollectionProgramCompiles(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "main.km")
	input := `
class Counter {
  constructor(private value: int) {}
  public static function create(value: int): Counter { return new Counter(value); }
  public function increment(): void { this.value = this.value + 1; }
  public function current(): int { return this.value; }
}
function lookup(values: Map<string, int>, key: string): int { return values[key]; }
function counterValue(start: int, steps: int): int {
  const counter = Counter.create(start);
  for (let index = 0; index < steps; index = index + 1) { counter.increment(); }
  return counter.current();
}
function compute(): int {
  const counter = Counter.create(1);
  counter.increment();
  let values: int[] = [];
  values = [counter.current(), 2];
  const dto = { count: values[0], label: "ok" };
  return dto.count;
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "sample")
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", diagnostics)
	}
	referenceSource := `package reference
type Counter struct { value int }
func NewCounter(value int) *Counter { return &Counter{value: value} }
func CounterCreate(value int) *Counter { return NewCounter(value) }
func (counter *Counter) Increment() { counter.value++ }
func (counter *Counter) Current() int { return counter.value }
func Lookup(values map[string]int, key string) int { return values[key] }
func CounterValue(start, steps int) int {
  counter := CounterCreate(start)
  for index := 0; index < steps; index++ { counter.Increment() }
  return counter.Current()
}
func Compute() int {
  counter := CounterCreate(1)
  counter.Increment()
  values := []int{counter.Current(), 2}
  dto := struct { count int; label string }{count: values[0], label: "ok"}
  return dto.count
}
`
	testSource := `package sample
import (
  "testing"
  reference "generated.class.test/reference"
)
func TestClassAndCollectionBehavior(t *testing.T) {
  if got, want := compute(), reference.Compute(); got != want { t.Errorf("compute = %d, Go = %d", got, want) }
  for _, item := range [][2]int{{0, 0}, {1, 3}, {-2, 4}, {10, -1}} {
    if got, want := counterValue(item[0], item[1]), reference.CounterValue(item[0], item[1]); got != want { t.Errorf("counterValue(%v) = %d, Go = %d", item, got, want) }
  }
  for _, values := range []map[string]int{{"x": 1}, {"other": 2}, {"x": -3}} {
    for _, key := range []string{"x", "missing"} {
      if got, want := lookup(values, key), reference.Lookup(values, key); got != want { t.Errorf("lookup(%v, %q) = %d, Go = %d", values, key, got, want) }
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "generated.class.test", generated, referenceSource, testSource)
}

func TestInterfacePolymorphismAcrossImportsCompilesAndRuns(t *testing.T) {
	temp := t.TempDir()
	contract := filepath.Join(temp, "contract.km")
	implementation := filepath.Join(temp, "implementation.km")
	entry := filepath.Join(temp, "entry.km")
	if err := os.WriteFile(contract, []byte(`interface Formatter { function format(value: string): string; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(implementation, []byte(`
import { Formatter } from "./contract";
class PrefixFormatter implements Formatter {
  constructor(private prefix: string) {}
  public function format(value: string): string { return this.prefix + value; }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`
import { Formatter } from "./contract";
import { PrefixFormatter } from "./implementation";
function render(formatter: Formatter, value: string): string { return formatter.format(value); }
function exampleWith(prefix: string, value: string): string { return render(new PrefixFormatter(prefix), value); }
function example(): string { return render(new PrefixFormatter("x:"), "ok"); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "sample")
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", diagnostics)
	}
	referenceSource := `package reference
type Formatter interface { Format(string) string }
type PrefixFormatter struct { prefix string }
func NewPrefixFormatter(prefix string) *PrefixFormatter { return &PrefixFormatter{prefix: prefix} }
func (formatter *PrefixFormatter) Format(value string) string { return formatter.prefix + value }
func Render(formatter Formatter, value string) string { return formatter.Format(value) }
func ExampleWith(prefix, value string) string { return Render(NewPrefixFormatter(prefix), value) }
func Example() string { return Render(NewPrefixFormatter("x:"), "ok") }
`
	testSource := `package sample
import (
  "testing"
  reference "generated.interface.test/reference"
)
func TestPolymorphism(t *testing.T) {
  if got, want := example(), reference.Example(); got != want { t.Errorf("example = %q, Go = %q", got, want) }
  for _, item := range [][2]string{{"", ""}, {"x:", "ok"}, {"温:", "泉"}} {
    if got, want := exampleWith(item[0], item[1]), reference.ExampleWith(item[0], item[1]); got != want { t.Errorf("exampleWith(%q, %q) = %q, Go = %q", item[0], item[1], got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "generated.interface.test", generated, referenceSource, testSource)
}
