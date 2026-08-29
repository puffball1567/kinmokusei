package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectionRangeCompileAndRunMatrix(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "range.otm")
	input := `
import go url from "net/url";

function sliceValues(items: int[]): int {
  let total = 0;
  for (const value of items) { total = total * 10 + value; }
  return total;
}
function sliceIndexes(items: int[]): int {
  let total = 0;
  for (const [index, value] of items) { total = total + index * value; }
  return total;
}
function mutateByIndex(items: int[]): int {
  for (const [index, value] of items) { items[index] = value * 2; }
  return items[0] * 100 + items[1] * 10 + items[2];
}
function rangeValuesAreCopies(items: int[]): int {
  for (let value of items) { value = 0; }
  return items[0];
}
function fixedAndPointer(items: [3]int): int {
  let total = 0;
  for (const value of items) { total = total * 10 + value; }
  for (const [index, value] of &items) { total = total + index + value; }
  return total;
}
function mapValues(table: Map<string, int>): int {
  let total = 0;
  for (const value of table) { total = total + value; }
  return total;
}
function mapPairs(table: Map<string, int>): int {
  let total = 0;
  for (const [key, value] of table) { total = total + len(key) + value; }
  return total;
}
function unicodeRange(text: string): int {
  let offsets = 0;
  let runes = 0;
  for (const [offset, rune] of text) { offsets = offsets * 10 + offset; runes = runes + int(rune); }
  return offsets + runes;
}
function observed(counter: *int): int[] { *counter = *counter + 1; return [4, 5]; }
function sourceEvaluatedOnce(): int {
  let count = 0;
  let total = 0;
  for (const value of observed(&count)) { total = total + value; }
  return count * 100 + total;
}
function emptyAndNil(): int {
  let empty: int[] = [];
  let absent: Map<string, int> = nil;
  let count = 0;
  for (const value of empty) { count = count + value; }
  for (const [key, value] of absent) { count = count + len(key) + value; }
  return count;
}
function namedGoMap(query: url.Values): int {
  let total = 0;
  for (const [key, values] of query) { total = total + len(key) + len(values); }
  return total;
}
function blankBindings(items: int[]): int {
  let count = 0;
  for (const _ of items) { count = count + 1; }
  for (const [_, _] of items) { count = count + 1; }
  for (const [index, _] of items) { count = count + index; }
  for (const [_, value] of items) { count = count + value; }
  return count;
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "rangematrix")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, want := range []string{
		"for _, value := range items", "for index, value := range items", "for _, value := range table",
		"for key, value := range table", "for offset, rune := range text", "for _, value := range observed(&count)",
		"for range items", "for index := range items",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import "net/url"
func SliceValues(items []int) int {
  total := 0
  for _, value := range items { total = total * 10 + value }
  return total
}
func SliceIndexes(items []int) int {
  total := 0
  for index, value := range items { total += index * value }
  return total
}
func MutateByIndex(items []int) int {
  for index, value := range items { items[index] = value * 2 }
  return items[0] * 100 + items[1] * 10 + items[2]
}
func RangeValuesAreCopies(items []int) int {
  for _, value := range items { value = 0; _ = value }
  return items[0]
}
func FixedAndPointer(items [3]int) int {
  total := 0
  for _, value := range items { total = total * 10 + value }
  for index, value := range &items { total += index + value }
  return total
}
func MapValues(table map[string]int) int {
  total := 0
  for _, value := range table { total += value }
  return total
}
func MapPairs(table map[string]int) int {
  total := 0
  for key, value := range table { total += len(key) + value }
  return total
}
func UnicodeRange(text string) int {
  offsets, runes := 0, 0
  for offset, runeValue := range text { offsets = offsets * 10 + offset; runes += int(runeValue) }
  return offsets + runes
}
func observed(counter *int) []int { *counter++; return []int{4, 5} }
func SourceEvaluatedOnce() int {
  count, total := 0, 0
  for _, value := range observed(&count) { total += value }
  return count * 100 + total
}
func EmptyAndNil() int {
  empty := []int{}
  var absent map[string]int
  count := 0
  for _, value := range empty { count += value }
  for key, value := range absent { count += len(key) + value }
  return count
}
func NamedGoMap(query url.Values) int {
  total := 0
  for key, values := range query { total += len(key) + len(values) }
  return total
}
func BlankBindings(items []int) int {
  count := 0
  for range items { count++ }
  for range items { count++ }
  for index := range items { count += index }
  for _, value := range items { count += value }
  return count
}
`
	testSource := `package rangematrix
import (
	"net/url"
	"slices"
	"testing"
	reference "rangematrix.test/reference"
)
func TestRangeMatrix(t *testing.T) {
	for _, input := range [][]int{{1, 2, 3}, {}, nil, {-1, 0, 2}} {
		if got, want := sliceValues(input), reference.SliceValues(input); got != want { t.Errorf("sliceValues(%v) = %d, Go = %d", input, got, want) }
		if got, want := sliceIndexes(input), reference.SliceIndexes(input); got != want { t.Errorf("sliceIndexes(%v) = %d, Go = %d", input, got, want) }
		if got, want := blankBindings(input), reference.BlankBindings(input); got != want { t.Errorf("blankBindings(%v) = %d, Go = %d", input, got, want) }
	}
	gotItems, wantItems := []int{1, 2, 3}, []int{1, 2, 3}
	if got, want := mutateByIndex(gotItems), reference.MutateByIndex(wantItems); got != want || !slices.Equal(gotItems, wantItems) { t.Errorf("mutateByIndex = (%d, %v), Go = (%d, %v)", got, gotItems, want, wantItems) }
	gotCopies, wantCopies := []int{7, 8}, []int{7, 8}
	if got, want := rangeValuesAreCopies(gotCopies), reference.RangeValuesAreCopies(wantCopies); got != want || !slices.Equal(gotCopies, wantCopies) { t.Errorf("range copies = (%d, %v), Go = (%d, %v)", got, gotCopies, want, wantCopies) }
	for _, input := range [][3]int{{1, 2, 3}, {0, -1, 4}} {
		if got, want := fixedAndPointer(input), reference.FixedAndPointer(input); got != want { t.Errorf("fixedAndPointer(%v) = %d, Go = %d", input, got, want) }
	}
	for _, table := range []map[string]int{{"a": 2, "bbb": 4}, {}, nil} {
		if got, want := mapValues(table), reference.MapValues(table); got != want { t.Errorf("mapValues(%v) = %d, Go = %d", table, got, want) }
		if got, want := mapPairs(table), reference.MapPairs(table); got != want { t.Errorf("mapPairs(%v) = %d, Go = %d", table, got, want) }
	}
	for _, input := range []string{"a世🙂", string([]byte{0xff, 'a'}), "", "温泉卵"} {
		if got, want := unicodeRange(input), reference.UnicodeRange(input); got != want { t.Errorf("unicodeRange(%q) = %d, Go = %d", input, got, want) }
	}
	if got, want := sourceEvaluatedOnce(), reference.SourceEvaluatedOnce(); got != want { t.Errorf("sourceEvaluatedOnce = %d, Go = %d", got, want) }
	if got, want := emptyAndNil(), reference.EmptyAndNil(); got != want { t.Errorf("emptyAndNil = %d, Go = %d", got, want) }
	query := url.Values{"a": {"1", "2"}, "bbb": {"3"}}
	if got, want := namedGoMap(query), reference.NamedGoMap(query); got != want { t.Errorf("namedGoMap = %d, Go = %d", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "rangematrix.test", generated, referenceSource, testSource)
}
