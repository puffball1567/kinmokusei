package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckedMapLookupMatchesIndependentGo(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "checked_map_lookup.km")
	if err := os.WriteFile(source, []byte(`
import go http from "net/http";
type Scores = distinct Map<string, int>;

function Lookup(values: Map<string, int>, key: string): int {
  const [value, present] = values[key];
  if (present) { return value * 10 + 1; }
  return -1;
}
function Reassign(first: Map<string, int>, second: Map<string, int>, key: string): int {
  let [value, present] = first[key];
  [value, present] = second[key];
  if (present) { return value * 10 + 1; }
  return -1;
}
function Defined(values: Scores, key: string): int {
  const [value, present] = values[key];
  if (present) { return value * 10 + 1; }
  return -1;
}
function Header(values: http.Header, key: string): int {
  const [items, present] = values[key];
  if (present) { return len(items) * 10 + 1; }
  return -1;
}
function source(counter: *int, values: Map<string, int>): Map<string, int> {
  *counter += 1;
  return values;
}
function key(counter: *int, value: string): string {
  *counter += 10;
  return value;
}
function EvaluationCount(values: Map<string, int>, name: string): int {
  let counter = 0;
  const [value, present] = source(&counter, values)[key(&counter, name)];
  if (present) { return counter * 100 + value; }
  return counter * 100 - 1;
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "checkedmap")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"var value, present = values[key]",
		"value, present = second[key]",
		"var value, present = source(&counter, values)[key(&counter, name)]",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference
import "net/http"
type Scores map[string]int
func Lookup(values map[string]int, key string) int { value, present := values[key]; if present { return value*10+1 }; return -1 }
func Reassign(first, second map[string]int, key string) int { value, present := first[key]; value, present = second[key]; if present { return value*10+1 }; return -1 }
func Defined(values Scores, key string) int { value, present := values[key]; if present { return value*10+1 }; return -1 }
func Header(values http.Header, key string) int { items, present := values[key]; if present { return len(items)*10+1 }; return -1 }
func source(counter *int, values map[string]int) map[string]int { *counter++; return values }
func key(counter *int, value string) string { *counter += 10; return value }
func EvaluationCount(values map[string]int, name string) int { counter := 0; value, present := source(&counter, values)[key(&counter, name)]; if present { return counter*100+value }; return counter*100-1 }
`
	testSource := `package checkedmap_test
import (
  "net/http"
  "testing"
  generated "checkedmap.test"
  reference "checkedmap.test/reference"
)
func TestBehavior(t *testing.T) {
  cases := []struct { values map[string]int; key string }{
    {nil, "missing"},
    {map[string]int{}, "missing"},
    {map[string]int{"zero": 0}, "zero"},
    {map[string]int{"温泉": 7, "other": -2}, "温泉"},
    {map[string]int{"present": -1}, "missing"},
  }
  for _, item := range cases {
    if got, want := generated.Lookup(item.values, item.key), reference.Lookup(item.values, item.key); got != want { t.Errorf("Lookup(%v, %q) = %d, Go = %d", item.values, item.key, got, want) }
    if got, want := generated.Defined(generated.Scores(item.values), item.key), reference.Defined(reference.Scores(item.values), item.key); got != want { t.Errorf("Defined(%v, %q) = %d, Go = %d", item.values, item.key, got, want) }
    if got, want := generated.EvaluationCount(item.values, item.key), reference.EvaluationCount(item.values, item.key); got != want { t.Errorf("EvaluationCount(%v, %q) = %d, Go = %d", item.values, item.key, got, want) }
    replacement := map[string]int{item.key: 3}
    if got, want := generated.Reassign(item.values, replacement, item.key), reference.Reassign(item.values, replacement, item.key); got != want { t.Errorf("Reassign(%v, %q) = %d, Go = %d", item.values, item.key, got, want) }
  }
  headers := http.Header{"Empty": {}, "Many": {"one", "two"}}
  for _, key := range []string{"Missing", "Empty", "Many"} {
    if got, want := generated.Header(headers, key), reference.Header(headers, key); got != want { t.Errorf("Header(%q) = %d, Go = %d", key, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "checkedmap.test", generated, referenceSource, testSource)
}
