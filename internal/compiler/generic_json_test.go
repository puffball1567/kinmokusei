package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericClassAndStructJSONMatchesIndependentGo(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "generic_json.otm")
	input := `
import go json from "encoding/json";

struct Envelope<T> {
  public item: T;
}

class Base<T> {
  constructor(public value: T, private secret: string) {}
  public virtual function describe(): string { return "base"; }
}

class Child<T> extends Base<T> {
  constructor(value: T, secret: string, public label: string) {
    super(value, secret);
  }
  public override function describe(): string { return this.label; }
}

function EncodeEnvelope<T>(value: T): Result<byte[]> {
  const encoded = json.Marshal(Envelope<T> { item: value })?;
  return ok(encoded);
}

function EncodeChild(value: string, label: string): Result<byte[]> {
  const encoded = json.Marshal(new Child<string>(value, "not serialized", label))?;
  return ok(encoded);
}

function DecodeChild(data: byte[], child: Child<string>): error {
  return json.Unmarshal(data, child);
}

function Describe(child: Child<string>): string {
  return child.describe();
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "genericjson")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, expected := range []string{
		`Item T ` + "`json:\"item\"`",
		"`json:\"value\"`",
		`Label string ` + "`json:\"label\"`",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}
	if strings.Contains(string(generated), `json:"secret"`) {
		t.Fatalf("private constructor field acquired a JSON tag:\n%s", generated)
	}

	referenceSource := `package reference

import "encoding/json"

type Envelope[T any] struct { Item T ` + "`json:\"item\"`" + ` }
type Base[T any] struct {
  Value T ` + "`json:\"value\"`" + `
  secret string
}
type Child[T any] struct {
  Base[T]
  Label string ` + "`json:\"label\"`" + `
}
func NewChild[T any](value T, secret, label string) *Child[T] {
  return &Child[T]{Base: Base[T]{Value: value, secret: secret}, Label: label}
}
func (value *Child[T]) Describe() string { return value.Label }
func EncodeEnvelope[T any](value T) ([]byte, error) { return json.Marshal(Envelope[T]{Item: value}) }
func EncodeChild(value, label string) ([]byte, error) { return json.Marshal(NewChild(value, "not serialized", label)) }
func DecodeChild(data []byte, child *Child[string]) error { return json.Unmarshal(data, child) }
`
	testSource := `package genericjson_test

import (
  "bytes"
  "encoding/json"
  "testing"
  generated "genericjson.test"
  reference "genericjson.test/reference"
)

func TestGenericJSONBehavior(t *testing.T) {
  for _, value := range []string{"", "onsen", "温泉卵"} {
    got, gotErr := generated.EncodeEnvelope(value)
    want, wantErr := reference.EncodeEnvelope(value)
    if !bytes.Equal(got, want) || (gotErr == nil) != (wantErr == nil) {
      t.Errorf("EncodeEnvelope(%q) = (%s, %v), Go = (%s, %v)", value, got, gotErr, want, wantErr)
    }
    got, gotErr = generated.EncodeChild(value, "label")
    want, wantErr = reference.EncodeChild(value, "label")
    if !bytes.Equal(got, want) || (gotErr == nil) != (wantErr == nil) {
      t.Errorf("EncodeChild(%q) = (%s, %v), Go = (%s, %v)", value, got, gotErr, want, wantErr)
    }
    if bytes.Contains(got, []byte("not serialized")) { t.Errorf("private field leaked: %s", got) }
  }

  cases := []string{
    ` + "`{\"value\":\"decoded\",\"label\":\"child\",\"unknown\":1}`" + `,
    ` + "`{\"label\":\"only\"}`" + `,
    ` + "`{\"value\":null,\"label\":\"null\"}`" + `,
    ` + "`{\"value\":1,\"label\":\"after-error\"}`" + `,
    ` + "`{\"value\":\"broken\",\"label\":}`" + `,
  }
  for _, encoded := range cases {
    got := generated.NewChild("before", "generated secret", "before")
    want := reference.NewChild("before", "reference secret", "before")
    gotErr := generated.DecodeChild([]byte(encoded), got)
    wantErr := reference.DecodeChild([]byte(encoded), want)
    if (gotErr == nil) != (wantErr == nil) || got.Value != want.Value || got.Label != want.Label {
      t.Errorf("DecodeChild(%s) = (%q, %q, %v), Go = (%q, %q, %v)", encoded, got.Value, got.Label, gotErr, want.Value, want.Label, wantErr)
    }
    if gotErr == nil && generated.Describe(got) != want.Describe() {
      t.Errorf("Describe after decode = %q, Go = %q", generated.Describe(got), want.Describe())
    }
  }

  encoded := []byte(` + "`{\"value\":\"external\",\"label\":\"restored\"}`" + `)
  got := generated.NewChild("before", "generated secret", "before")
  want := reference.NewChild("before", "reference secret", "before")
  gotErr := json.Unmarshal(encoded, got)
  wantErr := json.Unmarshal(encoded, want)
  if gotErr != nil || wantErr != nil {
    t.Fatalf("external unmarshal = (%v, %v)", gotErr, wantErr)
  }
  if got.Value != want.Value || got.Label != want.Label || generated.Describe(got) != want.Describe() {
    t.Errorf("decoded class = (%q, %q, %q), Go = (%q, %q, %q)", got.Value, got.Label, generated.Describe(got), want.Value, want.Label, want.Describe())
  }
  base := generated.UpcastChildToBase(got)
  restored, ok := generated.DowncastBaseToChild(base)
  if !ok || restored != got { t.Errorf("JSON decode did not preserve constructed class identity") }
}
`
	runGeneratedGoDifferentialTest(t, root, "genericjson.test", generated, referenceSource, testSource)
}
