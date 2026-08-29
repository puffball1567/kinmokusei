package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResultPropagationAndSplitCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "result.otm")
	input := `
import go strconv from "strconv";
import go errors from "errors";

function parse(text: string): Result<int> {
	const value: int = strconv.Atoi(text)?;
  return ok(value);
}
function double(text: string): Result<int> {
  const value = parse(text)?;
  return ok(value * 2);
}
function forward(text: string): Result<int> { return double(text); }
function reject(): Result<int> { return fail(errors.New("rejected")); }
function ensure(ready: boolean): Result<void> {
  if (!ready) { return fail(errors.New("not ready")); }
  return ok();
}
function sequence(ready: boolean): Result<void> {
  ensure(ready)?;
  return ok();
}
function split(text: string): int {
  const [value, err] = parse(text);
  if (err != nil) { return -100; }
  return value;
}
function splitVoid(ready: boolean): boolean {
  const [err] = ensure(ready);
  return err == nil;
}
function assignSplit(text: string): int {
  let value = 0;
  let err: error = nil;
  [value, err] = parse(text);
  if (err != nil) { return -200; }
  return value;
}
function assignSplitVoid(ready: boolean): boolean {
  let err: error = nil;
  [err] = ensure(ready);
  return err == nil;
}
interface Loader { function load(text: string): Result<int>; }
class Parser implements Loader {
  public function load(text: string): Result<int> { return parse(text); }
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "resultmatrix")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, want := range []string{
		"func parse(text string) (int, error)",
		"var value, __ontama_result_error_",
		"var __ontama_result_value_",
		"var value int = __ontama_result_value_",
		"if __ontama_result_error_",
		"return value * 2, nil",
		"return *new(int), errors.New(\"rejected\")",
		"func ensure(ready bool) error",
		"var value, err = parse(text)",
		"var err = ensure(ready)",
		"Load(text string) (int, error)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "errors"
  "strconv"
)
func Parse(text string) (int, error) {
  value, err := strconv.Atoi(text)
  if err != nil { return 0, err }
  return value, nil
}
func Double(text string) (int, error) {
  value, err := Parse(text)
  if err != nil { return 0, err }
  return value * 2, nil
}
func Forward(text string) (int, error) { return Double(text) }
func Reject() (int, error) { return 0, errors.New("rejected") }
func Ensure(ready bool) error {
  if !ready { return errors.New("not ready") }
  return nil
}
func Sequence(ready bool) error {
  if err := Ensure(ready); err != nil { return err }
  return nil
}
func Split(text string) int { value, err := Parse(text); if err != nil { return -100 }; return value }
func SplitVoid(ready bool) bool { err := Ensure(ready); return err == nil }
func AssignSplit(text string) int { value := 0; var err error; value, err = Parse(text); if err != nil { return -200 }; return value }
func AssignSplitVoid(ready bool) bool { var err error; err = Ensure(ready); return err == nil }
type Loader interface { Load(string) (int, error) }
type Parser struct{}
func (Parser) Load(text string) (int, error) { return Parse(text) }
func NewParser() Loader { return Parser{} }
`
	testSource := `package resultmatrix
import (
  "testing"
  reference "result.test/reference"
)
func errorText(err error) string {
  if err == nil { return "" }
  return err.Error()
}
func compareResult(t *testing.T, name string, got int, gotErr error, want int, wantErr error) {
  t.Helper()
  if got != want || errorText(gotErr) != errorText(wantErr) || (gotErr == nil) != (wantErr == nil) {
    t.Errorf("%s = (%d, %v), Go = (%d, %v)", name, got, gotErr, want, wantErr)
  }
}
func TestResultRuntimeMatrix(t *testing.T) {
  for _, input := range []string{"21", "-7", "0", "bad", "", " 1"} {
    got, gotErr := parse(input)
    want, wantErr := reference.Parse(input)
    compareResult(t, "parse(" + input + ")", got, gotErr, want, wantErr)
    got, gotErr = double(input)
    want, wantErr = reference.Double(input)
    compareResult(t, "double(" + input + ")", got, gotErr, want, wantErr)
    got, gotErr = forward(input)
    want, wantErr = reference.Forward(input)
    compareResult(t, "forward(" + input + ")", got, gotErr, want, wantErr)
  }
  got, gotErr := reject()
  want, wantErr := reference.Reject()
  compareResult(t, "reject", got, gotErr, want, wantErr)
  for _, ready := range []bool{false, true} {
    if gotErr, wantErr := ensure(ready), reference.Ensure(ready); errorText(gotErr) != errorText(wantErr) || (gotErr == nil) != (wantErr == nil) { t.Errorf("ensure(%v) = %v, Go = %v", ready, gotErr, wantErr) }
    if gotErr, wantErr := sequence(ready), reference.Sequence(ready); errorText(gotErr) != errorText(wantErr) || (gotErr == nil) != (wantErr == nil) { t.Errorf("sequence(%v) = %v, Go = %v", ready, gotErr, wantErr) }
  }
  for _, input := range []string{"21", "-7", "bad", ""} {
    if got, want := split(input), reference.Split(input); got != want { t.Errorf("split(%q) = %d, Go = %d", input, got, want) }
    if got, want := assignSplit(input), reference.AssignSplit(input); got != want { t.Errorf("assignSplit(%q) = %d, Go = %d", input, got, want) }
  }
  for _, ready := range []bool{false, true} {
    if got, want := splitVoid(ready), reference.SplitVoid(ready); got != want { t.Errorf("splitVoid(%v) = %v, Go = %v", ready, got, want) }
    if got, want := assignSplitVoid(ready), reference.AssignSplitVoid(ready); got != want { t.Errorf("assignSplitVoid(%v) = %v, Go = %v", ready, got, want) }
  }
  var gotLoader Loader = NewParser()
  wantLoader := reference.NewParser()
  for _, input := range []string{"7", "bad"} {
    got, gotErr := gotLoader.Load(input)
    want, wantErr := wantLoader.Load(input)
    compareResult(t, "loader(" + input + ")", got, gotErr, want, wantErr)
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "result.test", generated, referenceSource, testSource)
}
