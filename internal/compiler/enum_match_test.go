package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnumMatchMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"enum.km": `enum Status: int8 { Waiting = -3, Ready = 7, Running = 7, Failed = 9 }
alias State = Status
`,
		"main.km": `import { Status, State } from "./enum"

export function Label(status: State, urgent: boolean): string {
  return match(status) {
    case Status.Ready if (urgent) => "urgent"
    case Status.Ready => "ready"
    case -3 => "waiting"
    case Status.Failed => "failed"
  }
}
export function WithFallback(status: Status): string {
  return match(status) {
    case Status.Waiting => "waiting"
    case Status.Running => "ready"
    case Status.Failed => "failed"
    default => "unknown"
  }
}
export function Nested(statuses: Status[]): boolean {
  const entry = { statuses: statuses }
  return match(entry) {
    case { statuses: [Status.Waiting, Status.Ready] } => true
    default => false
  }
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "main.km")}, "enummatch")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
type Status int8
const ( Waiting Status = -3; Ready Status = 7; Failed Status = 9 )
func WithFallback(status Status) string {
  switch status {
  case Waiting: return "waiting"
  case Ready: return "ready"
  case Failed: return "failed"
  default: return "unknown"
  }
}
func Label(status Status, urgent bool) string {
  if status == Ready && urgent { return "urgent" }
  return WithFallback(status)
}
func Nested(statuses []Status) bool {
  return len(statuses) == 2 && statuses[0] == Waiting && statuses[1] == Ready
}
`
	comparison := `package enummatch_test
import (
  "fmt"
  "strings"
  "testing"
  g "enum-match.test"
  r "enum-match.test/reference"
)
func TestMatch(t *testing.T) {
  for _, status := range []g.Status{g.StatusWaiting, g.StatusReady, g.StatusRunning, g.StatusFailed, 42} {
    if got, want := g.WithFallback(status), r.WithFallback(r.Status(status)); got != want {
      t.Errorf("fallback for %d = %q, want %q", status, got, want)
    }
    if status != 42 {
      for _, urgent := range []bool{false, true} {
        if got, want := g.Label(status, urgent), r.Label(r.Status(status), urgent); got != want {
          t.Errorf("label for %d (%t) = %q, want %q", status, urgent, got, want)
        }
      }
    }
    if got, want := g.Nested([]g.Status{g.StatusWaiting, status}), r.Nested([]r.Status{r.Waiting, r.Status(status)}); got != want {
      t.Errorf("nested match for %d = %t, want %t", status, got, want)
    }
  }
  if g.Nested(nil) { t.Fatal("empty sequence matched") }
  defer func() {
    if got := fmt.Sprint(recover()); !strings.Contains(got, "undeclared enum value at main.km:4:10") {
      t.Errorf("unknown enum panic = %q", got)
    }
  }()
  g.Label(42, false)
}
`
	runGeneratedGoDifferentialTest(t, root, "enum-match.test", generated, reference, comparison)
}
