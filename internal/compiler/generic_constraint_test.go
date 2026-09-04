package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComparableTypeParameterConstraintsMatchIndependentGo(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "generic_constraint.km")
	if err := os.WriteFile(source, []byte(`
function Equal<T extends comparable>(left: T, right: T): boolean { return left === right; }
function Contains<T extends comparable>(values: Map<T, string>, key: T): boolean {
  return len(values[key]) > 0;
}
struct Keyed<T extends comparable> {
  public key: T;
  public values: Map<T, string>;
  public function present(): boolean { return Contains(this.values, this.key); }
}
interface Matcher<T extends comparable> { function matches(left: T, right: T): boolean; }
class StringMatcher implements Matcher<string> {
  public function matches(left: string, right: string): boolean { return Equal(left, right); }
}
type Lookup<T extends comparable> = distinct Map<T, string>;
function KeyedPresent(key: string, value: string): boolean {
  const values = makeMap[string, string]();
  values[key] = value;
  const keyed = Keyed<string> { key: key, values: values };
  return keyed.present();
}
function MatchStrings(left: string, right: string): boolean {
  const matcher: Matcher<string> = new StringMatcher();
  return matcher.matches(left, right);
}
function LookupSize(values: Map<int, string>): int { return len(Lookup<int>(values)); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "genericconstraint")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"func Equal[T comparable]",
		"func Contains[T comparable]",
		"type Keyed[T comparable] struct",
		"type Matcher[T comparable] interface",
		"type Lookup[T comparable] map[T]string",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference
func Equal[T comparable](left, right T) bool { return left == right }
func Contains[T comparable](values map[T]string, key T) bool { return len(values[key]) > 0 }
type Keyed[T comparable] struct { Key T; Values map[T]string }
func (value Keyed[T]) Present() bool { return Contains(value.Values, value.Key) }
type Matcher[T comparable] interface { Matches(T, T) bool }
type StringMatcher struct{}
func (*StringMatcher) Matches(left, right string) bool { return Equal(left, right) }
type Lookup[T comparable] map[T]string
func KeyedPresent(key, value string) bool { return Keyed[string]{Key: key, Values: map[string]string{key: value}}.Present() }
func MatchStrings(left, right string) bool { var matcher Matcher[string] = &StringMatcher{}; return matcher.Matches(left, right) }
func LookupSize(values map[int]string) int { return len(Lookup[int](values)) }
`
	testSource := `package genericconstraint_test
import (
  "testing"
  generated "genericconstraint.test"
  reference "genericconstraint.test/reference"
)
func TestBehavior(t *testing.T) {
  for _, item := range []struct{ left, right string }{{"", ""}, {"onsen", "tamago"}, {"温泉", "温泉"}} {
    if got, want := generated.Equal(item.left, item.right), reference.Equal(item.left, item.right); got != want { t.Errorf("Equal(%q, %q) = %v, Go = %v", item.left, item.right, got, want) }
    if got, want := generated.MatchStrings(item.left, item.right), reference.MatchStrings(item.left, item.right); got != want { t.Errorf("MatchStrings(%q, %q) = %v, Go = %v", item.left, item.right, got, want) }
    if got, want := generated.KeyedPresent(item.left, item.right), reference.KeyedPresent(item.left, item.right); got != want { t.Errorf("KeyedPresent(%q, %q) = %v, Go = %v", item.left, item.right, got, want) }
  }
  for _, values := range []map[int]string{nil, {}, {1: ""}, {1: "one", 2: "two"}} {
    if got, want := generated.LookupSize(values), reference.LookupSize(values); got != want { t.Errorf("LookupSize(%v) = %d, Go = %d", values, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "genericconstraint.test", generated, referenceSource, testSource)
}
