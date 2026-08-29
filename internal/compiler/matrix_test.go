package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ontama.local/ontama/internal/product"
	"ontama.local/ontama/internal/project"
)

func TestImportFailureMatrix(t *testing.T) {
	tests := []struct {
		name       string
		dependency string
		entry      string
		want       string
	}{
		{"missing module", "", `import { value } from "./missing";`, "cannot load imported module"},
		{"package import", "", `import { value } from "external/package";`, "package imports are not supported"},
		{"unknown standard package", "", `import { value } from "ontama/missing";`, `standard package "ontama/missing" is not available`},
		{"noncanonical standard package", "", `import { value } from "ontama/../http";`, `standard package "ontama/../http" is not available`},
		{"missing standard package declaration", "", `import { missing } from "ontama/http";`, `module "ontama/http" does not declare "missing"`},
		{"missing exported name", `function present(): int { return 1; }`, `import { absent } from "./dependency";`, "does not declare"},
		{"duplicate imported name", `function present(): int { return 1; }`, `import { present, present } from "./dependency";`, "duplicate imported name"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			temp := t.TempDir()
			if test.dependency != "" {
				if err := os.WriteFile(filepath.Join(temp, "dependency.otm"), []byte(test.dependency), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			entry := filepath.Join(temp, "entry.otm")
			if err := os.WriteFile(entry, []byte(test.entry), 0o644); err != nil {
				t.Fatal(err)
			}
			result, err := CheckFiles([]string{entry})
			if err != nil {
				t.Fatal(err)
			}
			messages := make([]string, len(result.Diagnostics))
			for i, item := range result.Diagnostics {
				messages[i] = item.Message
			}
			if !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", messages, test.want)
			}
		})
	}
}

func TestImportSuccessAndOrderingMatrix(t *testing.T) {
	temp := t.TempDir()
	base := filepath.Join(temp, "base.otm")
	middle := filepath.Join(temp, "middle.otm")
	entry := filepath.Join(temp, "entry.otm")
	if err := os.WriteFile(base, []byte(`interface Reader { function read(): int; } function base(): int { return 1; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(middle, []byte(`import { Reader, base } from "./base.otm"; class Value implements Reader { public function read(): int { return base(); } }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`import { Value } from "./middle"; function entry(): int { return new Value().read(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	first, diagnostics, err := EmitGo([]string{entry, base}, "stable")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err = %v, diagnostics = %v", err, diagnostics)
	}
	second, diagnostics, err := EmitGo([]string{base, entry, entry}, "stable")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err = %v, diagnostics = %v", err, diagnostics)
	}
	if string(first) != string(second) {
		t.Fatalf("output depends on root ordering or duplicate roots:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	text := string(first)
	if !(strings.Index(text, "type Reader interface") < strings.Index(text, "type Value struct") && strings.Index(text, "type Value struct") < strings.Index(text, "func entry")) {
		t.Fatalf("dependency order is incorrect:\n%s", text)
	}
}

func TestWriteGeneratedModuleUsesProjectMarker(t *testing.T) {
	temp := t.TempDir()
	sourceDirectory := filepath.Join(temp, "src", "nested")
	if err := os.MkdirAll(sourceDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "marker"
version = "0.1.0"
go-module = "example.com/marker"
go-version = "1.23"
`
	if err := os.WriteFile(filepath.Join(temp, product.ProjectFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := project.LockDependencies(temp, true); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceDirectory, "main.otm")
	if err := os.WriteFile(source, []byte(`function main(): void {}`), 0o644); err != nil {
		t.Fatal(err)
	}
	directory, diagnostics, err := WriteGeneratedModule([]string{source}, "main")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err = %v, diagnostics = %v", err, diagnostics)
	}
	want := product.GeneratedDirectory(temp)
	if directory != want {
		t.Fatalf("directory = %q, want %q", directory, want)
	}
	for _, name := range []string{"generated.go", "go.mod", "go.sum"} {
		if info, statErr := os.Stat(filepath.Join(directory, name)); statErr != nil || info.IsDir() {
			t.Fatalf("%s missing: info=%v err=%v", name, info, statErr)
		}
	}
}

func TestWriteGeneratedModuleRejectsEmptySources(t *testing.T) {
	directory, diagnostics, err := WriteGeneratedModule(nil, "main")
	if err == nil || directory != "" || len(diagnostics) != 0 || !strings.Contains(err.Error(), "at least one source") {
		t.Fatalf("directory=%q diagnostics=%v err=%v", directory, diagnostics, err)
	}
}

func TestGeneratedBehaviorMatrix(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "behavior.otm")
	input := `
const fixed: int = 7;
let changing = 2;
interface Transformer { function transform(value: int): int; }
class Scale implements Transformer {
  constructor(private factor: int) {}
  public function transform(value: int): int { return value * this.factor; }
  public static function create(factor: int): Scale { return new Scale(factor); }
}
class Pipeline {
  constructor(private transformer: Transformer) {}
  public function run(value: int): int { return this.transformer.transform(value); }
}
function arithmetic(): int { return (20 + 4) / 2 - 2 * 3 + 5 % 2; }
function flow(limit: int): int {
  let total = 0;
  for (let index = 0; index < limit; index = index + 1) {
    if (index === 2) { continue; }
    total = total + index;
  }
  while (total < 10) { total = total + 1; }
  return total;
}
function arrows(): int {
  const double = (value: int) => value * 2;
  const addOne = (value: int): int => { return value + 1; };
  return addOne(double(20));
}
function first(values: int[]): int { return values[0]; }
function lookup(values: Map<string, int>): int { return values["answer"]; }
function objectValue(): int { const dto = { value: 42, label: "ok" }; return dto.value; }
function firstByte(value: string): byte { return value[0]; }
function use(transformer: Transformer, value: int): int { return transformer.transform(value); }
function make(factor: int): Transformer { return new Scale(factor); }
function firstTransformer(values: Transformer[]): Transformer { return values[0]; }
function mappedTransformer(values: Map<string, Transformer>): Transformer { return values["item"]; }
function arrowInterface(): int {
  const factory = (value: int): Transformer => { return new Scale(value); };
  return factory(2).transform(3);
}
function globals(): int { changing = changing + 1; return fixed + changing; }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "behavior")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err = %v, diagnostics = %v", err, diagnostics)
	}
	referenceSource := `package reference
const fixed int = 7
var changing = 2
type Transformer interface { Transform(int) int }
type Scale struct { factor int }
func NewScale(factor int) *Scale { return &Scale{factor: factor} }
func (scale *Scale) Transform(value int) int { return value * scale.factor }
func ScaleCreate(factor int) *Scale { return NewScale(factor) }
type Pipeline struct { transformer Transformer }
func NewPipeline(transformer Transformer) *Pipeline { return &Pipeline{transformer: transformer} }
func (pipeline *Pipeline) Run(value int) int { return pipeline.transformer.Transform(value) }
func Arithmetic() int { return (20+4)/2 - 2*3 + 5%2 }
func Flow(limit int) int {
  total := 0
  for index := 0; index < limit; index++ { if index == 2 { continue }; total += index }
  for total < 10 { total++ }
  return total
}
func Arrows() int { double := func(value int) int { return value*2 }; addOne := func(value int) int { return value+1 }; return addOne(double(20)) }
func First(values []int) int { return values[0] }
func Lookup(values map[string]int) int { return values["answer"] }
func ObjectValue() int { dto := struct { value int; label string }{value: 42, label: "ok"}; return dto.value }
func FirstByte(value string) byte { return value[0] }
func Use(transformer Transformer, value int) int { return transformer.Transform(value) }
func Make(factor int) Transformer { return NewScale(factor) }
func FirstTransformer(values []Transformer) Transformer { return values[0] }
func MappedTransformer(values map[string]Transformer) Transformer { return values["item"] }
func ArrowInterface() int { factory := func(value int) Transformer { return NewScale(value) }; return factory(2).Transform(3) }
func Globals() int { changing++; return fixed+changing }
`
	testSource := `package behavior
import (
  "testing"
  reference "behavior.test/reference"
)
func TestBehavior(t *testing.T) {
  if got, want := arithmetic(), reference.Arithmetic(); got != want { t.Errorf("arithmetic = %d, Go = %d", got, want) }
  if got, want := arrows(), reference.Arrows(); got != want { t.Errorf("arrows = %d, Go = %d", got, want) }
  if got, want := objectValue(), reference.ObjectValue(); got != want { t.Errorf("objectValue = %d, Go = %d", got, want) }
  if got, want := arrowInterface(), reference.ArrowInterface(); got != want { t.Errorf("arrowInterface = %d, Go = %d", got, want) }
  for _, limit := range []int{-3, 0, 2, 5, 8} {
    if got, want := flow(limit), reference.Flow(limit); got != want { t.Errorf("flow(%d) = %d, Go = %d", limit, got, want) }
  }
  for _, values := range [][]int{{3}, {-1, 9}, {0}} {
    if got, want := first(values), reference.First(values); got != want { t.Errorf("first(%v) = %d, Go = %d", values, got, want) }
  }
  for _, values := range []map[string]int{{"answer": 42}, {"other": 7}, {"answer": -1}} {
    if got, want := lookup(values), reference.Lookup(values); got != want { t.Errorf("lookup(%v) = %d, Go = %d", values, got, want) }
  }
  for _, value := range []string{"abc", "温泉", "x"} {
    if got, want := firstByte(value), reference.FirstByte(value); got != want { t.Errorf("firstByte(%q) = %d, Go = %d", value, got, want) }
  }
  for _, item := range [][2]int{{3, 4}, {-2, 5}, {0, 99}} {
    if got, want := use(make_(item[0]), item[1]), reference.Use(reference.Make(item[0]), item[1]); got != want { t.Errorf("interface(%v) = %d, Go = %d", item, got, want) }
    if got, want := NewPipeline(make_(item[0])).Run(item[1]), reference.NewPipeline(reference.Make(item[0])).Run(item[1]); got != want { t.Errorf("pipeline(%v) = %d, Go = %d", item, got, want) }
    if got, want := firstTransformer([]Transformer{make_(item[0])}).Transform(item[1]), reference.FirstTransformer([]reference.Transformer{reference.Make(item[0])}).Transform(item[1]); got != want { t.Errorf("interface slice(%v) = %d, Go = %d", item, got, want) }
    if got, want := mappedTransformer(map[string]Transformer{"item": make_(item[0])}).Transform(item[1]), reference.MappedTransformer(map[string]reference.Transformer{"item": reference.Make(item[0])}).Transform(item[1]); got != want { t.Errorf("interface map(%v) = %d, Go = %d", item, got, want) }
    if got, want := ScaleCreate(item[0]).Transform(item[1]), reference.ScaleCreate(item[0]).Transform(item[1]); got != want { t.Errorf("static method(%v) = %d, Go = %d", item, got, want) }
  }
  for range 3 {
    if got, want := globals(), reference.Globals(); got != want { t.Errorf("globals = %d, Go = %d", got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "behavior.test", generated, referenceSource, testSource)
}
