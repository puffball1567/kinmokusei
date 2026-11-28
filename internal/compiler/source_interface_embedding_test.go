package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceInterfaceEmbeddingMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"readers.km": `import go errors from "errors"
export interface Reader<T> { function read(): Result<T>; }
export interface NamedReader extends Reader<string> { function name(): string; get title(): string; }
export class Text implements NamedReader {
  constructor(private text: string) {}
  public function read(): Result<string> {
    if (this.text === "") { return fail(errors.New("empty")); }
    return ok(this.text)
  }
  public function name(): string { return this.text; }
  public get title(): string { return this.text; }
}
`,
		"entry.km": `import { Reader, NamedReader, Text } from "./readers"
export struct Forward<T> { Reader<T>; }
export struct Layer { Forward<string>; }
struct IntLayer { Forward<int>; }
struct Diamond {
  IntLayer;
  Layer;
  public function read(): Result<string> { return ok("override both"); }
}
export alias ReaderAlias = NamedReader
export struct AliasLayer { ReaderAlias; }
export struct Override {
  ReaderAlias;
  public function read(): Result<string> { return ok("override"); }
}
function value<T extends Reader<string>>(reader: T): string {
  const [text, err] = reader.read()
  if (err != nil) { return "failed"; }
  return text
}
export function ReadLayers(before: string, after: string): string[] {
  let layer = Layer{Forward: Forward<string>{Reader: new Text(before)}}
  const bound = layer.read
  const read = Layer.read
  const snapshot: Reader<string> = layer
  layer.Forward.Reader = new Text(after)
  const [original, first] = bound()
  const [current, second] = read(layer)
  if (first != nil || second != nil) { return ["failed", value(layer), value(snapshot)]; }
  const alias = AliasLayer{ReaderAlias: new Text(before)}
  const overridden: NamedReader = Override{ReaderAlias: new Text(after)}
  return [original, current, value(layer), value(snapshot), value(alias), alias.name(), alias.ReaderAlias.title, value(overridden), overridden.title]
}
export function ReadPointer(layer: *Layer): string {
  const reader: Reader<string> = layer
  return value(reader)
}
export function AsReader(layer: *Layer): Reader<string> { return layer; }
function diamond(value: Diamond): Reader<string> { return value; }
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "sourceembedding")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference

import "errors"

type reader[T any] interface{ Read() (T, error) }
type namedReader interface{ reader[string]; Name() string; GetTitle() string }
type text string
func (t text) Read() (string, error) {
  if t == "" { return "", errors.New("empty") }
  return string(t), nil
}
func (t text) Name() string { return string(t) }
func (t text) GetTitle() string { return string(t) }

type forward[T any] struct{ reader[T] }
type layer struct{ forward[string] }
type readerAlias = namedReader
type aliasLayer struct{ readerAlias }
type override struct{ readerAlias }
func (override) Read() (string, error) { return "override", nil }

func value(r reader[string]) string {
  v, err := r.Read()
  if err != nil { return "failed" }
  return v
}

func ReadLayers(before, after string) []string {
  wrap := layer{forward[string]{text(before)}}
  bound, read := wrap.Read, layer.Read
  var snapshot reader[string] = wrap
  wrap.forward.reader = text(after)
  original, first := bound()
  current, second := read(wrap)
  if first != nil || second != nil { return []string{"failed", value(wrap), value(snapshot)} }
  a := aliasLayer{text(before)}
  var o namedReader = override{text(after)}
  return []string{original, current, value(wrap), value(snapshot), value(a), a.Name(), a.readerAlias.GetTitle(), value(o), o.GetTitle()}
}
`
	comparison := `package sourceembedding_test

import (
  "slices"
  "testing"
  g "source-interface-embedding.test"
  r "source-interface-embedding.test/reference"
)

func TestEmbedding(t *testing.T) {
  for _, input := range [][2]string{{"雪", "春"}, {"", "春"}, {"雪", ""}} {
    if got, want := g.ReadLayers(input[0], input[1]), r.ReadLayers(input[0], input[1]); !slices.Equal(got, want) {
      t.Fatalf("%q: got %q, want %q", input, got, want)
    }
  }
  layer := g.Layer{Forward: g.Forward[string]{Reader: g.NewText("Go caller")}}
  var reader g.Reader[string] = layer
  if got, err := reader.Read(); err != nil || got != "Go caller" { t.Fatal(got, err) }
  if got := g.ReadPointer(&layer); got != "Go caller" { t.Fatal(got) }
}
func TestTypedNilPointer(t *testing.T) {
  var ptr *g.Layer
  var want g.Reader[string] = ptr
  if got := g.AsReader(ptr); got == nil || got != want { t.Fatal(got) }
}
`
	runGeneratedGoDifferentialTest(t, root, "source-interface-embedding.test", generated, reference, comparison)
}

func TestSourceInterfaceEmbeddingDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"pointer to interface", `struct Box{*Reader;}`, "cannot be used as a Go pointer target"},
		{"nullable interface", `struct Box{Reader|null;}`, "embedded field must name"},
		{"non-null field", `struct Box{Reader;} function make():Box{return Box{Reader:nil};}`, "cannot use nil as Reader"},
		{"missing delegate", `struct Box{Reader;} function make():Box{return Box{};}`, "missing field"},
		{"zero slice storage", `struct Box{Reader;} function make():Box[]{return makeSlice<Box>(1);}`, "cannot initialize elements"},
		{"incompatible shadowing", `struct Box{Reader;public function read():bstring{return b"raw";}} function use(v:Box):Reader{return v;}`, "cannot use Box as Reader"},
		{"ambiguous paths", `struct L{Reader;}struct R{Reader;}struct Box{L;R;} function use(v:Box):string{return v.read();}`, "ambiguous promoted"},
		{"unrelated contract", `interface Other{function read():string;} struct Box{Other;} function use(v:Box):Reader{return v;}`, "cannot use Box as Reader"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.km")
			input := `interface Reader{function read():string;}` + test.input
			result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
			if err != nil {
				t.Fatal(err)
			}
			for _, diagnostic := range result.Diagnostics {
				if strings.Contains(diagnostic.Message, test.want) {
					return
				}
			}
			t.Fatalf("expected %q, got %v", test.want, result.Diagnostics)
		})
	}
}
