package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGotoAndLabelsMatchIndependentGo(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "labels.km")
	if err := os.WriteFile(source, []byte(`
function StateMachine(limit: int): int {
  let total = 0;
  goto dispatch;
  increment: total += 2;
  dispatch: if (total < limit) { goto increment; }
  return total;
}
function LabeledLoops(stopRow: int): int {
  let total = 0;
  outer: for (let row = 0; row < 4; row++) {
    for (let column = 0; column < 4; column++) {
      if (column === 2) { continue outer; }
      total += row * 10 + column;
      if (row === stopRow) { break outer; }
    }
  }
  return total;
}
function LabeledSwitch(value: int): int {
  let result = 0;
  choice: switch (value) {
    case 1 { result = 10; break choice; }
    default { result = 20; }
  }
  return result + 1;
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "labels")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{"goto dispatch", "increment:", "outer:", "continue outer", "break outer", "choice:", "break choice"} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference
func StateMachine(limit int) int { total := 0; goto dispatch; increment: total += 2; dispatch: if total < limit { goto increment }; return total }
func LabeledLoops(stopRow int) int { total := 0; outer: for row := 0; row < 4; row++ { for column := 0; column < 4; column++ { if column == 2 { continue outer }; total += row*10 + column; if row == stopRow { break outer } } }; return total }
func LabeledSwitch(value int) int { result := 0; choice: switch value { case 1: result = 10; break choice; default: result = 20 }; return result + 1 }
`
	testSource := `package labels_test
import (
  "testing"
  generated "labels.test"
  reference "labels.test/reference"
)
func TestLabels(t *testing.T) {
  for _, limit := range []int{-3, 0, 1, 2, 3, 8, 9} {
    if got, want := generated.StateMachine(limit), reference.StateMachine(limit); got != want { t.Errorf("StateMachine(%d) = %d, Go = %d", limit, got, want) }
  }
  for _, stop := range []int{-1, 0, 1, 3, 9} {
    if got, want := generated.LabeledLoops(stop), reference.LabeledLoops(stop); got != want { t.Errorf("LabeledLoops(%d) = %d, Go = %d", stop, got, want) }
  }
  for _, value := range []int{-1, 0, 1, 2} {
    if got, want := generated.LabeledSwitch(value), reference.LabeledSwitch(value); got != want { t.Errorf("LabeledSwitch(%d) = %d, Go = %d", value, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "labels.test", generated, referenceSource, testSource)
}
