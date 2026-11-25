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
export struct Request { public status: State; public enabled: boolean }
export struct Event<T> { public states: [2]T }
export function Record(request: Request, urgent: () => boolean): int {
  return match(request) {
    case { status: Status.Ready, enabled: true } if (urgent()) => 10
    case { status: Status.Waiting } => 1
    case { status: Status.Running, enabled: false } => 2
    case { status: Status.Ready, enabled: true } => 3
    case { status: Status.Failed } => 4
  }
}
export function ResultArray(load: () => Result<Event<Status>>): int {
  return match(load()) {
    case ok({ states: [..._, Status.Waiting] }) => 1
    case ok({ states: [_, Status.Ready] }) => 2
    case ok({ states: [..._, Status.Failed] }) => 3
    case fail(_) => -1
  }
}
export function ResultEnum(load: () => Result<State>): int {
  return match(load()) {
    case ok(Status.Waiting) => 1
    case ok(Status.Running) => 2
    case ok(Status.Failed) => 3
    case fail(_) => -1
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
func Record(status Status, enabled bool, urgent func() bool) int {
  switch status {
  case Waiting: return 1
  case Ready:
    if !enabled { return 2 }
    if urgent() { return 10 }
    return 3
  case Failed: return 4
  default: panic("undeclared enum")
  }
}
func ResultArray(load func() ([2]Status, error)) int {
  states, err := load()
  return labelResult(states[1], err)
}
func ResultEnum(load func() (Status, error)) int {
  return labelResult(load())
}
func labelResult(status Status, err error) int {
  if err != nil { return -1 }
  switch status {
  case Waiting: return 1
  case Ready: return 2
  case Failed: return 3
  default: panic("undeclared enum")
  }
}
`
	comparison := `package enummatch_test
import (
  "errors"
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
func TestProducts(t *testing.T) {
  for _, status := range []g.Status{g.StatusWaiting, g.StatusReady, g.StatusRunning, g.StatusFailed, 42} {
    for _, enabled := range []bool{false, true} {
      for _, urgent := range []bool{false, true} {
        actualCalls, expectedCalls := 0, 0
        got, actualPanic := capture(func() int {
          return g.Record(g.Request{Status: status, Enabled: enabled}, func() bool { actualCalls++; return urgent })
        })
        want, expectedPanic := capture(func() int {
          return r.Record(r.Status(status), enabled, func() bool { expectedCalls++; return urgent })
        })
        compare(t, got, want, actualPanic, expectedPanic, actualCalls, expectedCalls)
      }
    }
    for _, failure := range []error{nil, errors.New("load failed")} {
      actualCalls, expectedCalls := 0, 0
      got, actualPanic := capture(func() int {
        return g.ResultArray(func() (g.Event[g.Status], error) {
          actualCalls++
          return g.Event[g.Status]{States: [2]g.Status{99, status}}, failure
        })
      })
      want, expectedPanic := capture(func() int {
        return r.ResultArray(func() ([2]r.Status, error) {
          expectedCalls++
          return [2]r.Status{99, r.Status(status)}, failure
        })
      })
      compare(t, got, want, actualPanic, expectedPanic, actualCalls, expectedCalls)
      actualCalls, expectedCalls = 0, 0
      got, actualPanic = capture(func() int {
        return g.ResultEnum(func() (g.Status, error) { actualCalls++; return status, failure })
      })
      want, expectedPanic = capture(func() int {
        return r.ResultEnum(func() (r.Status, error) { expectedCalls++; return r.Status(status), failure })
      })
      compare(t, got, want, actualPanic, expectedPanic, actualCalls, expectedCalls)
    }
  }
}
func capture(run func() int) (value int, panicked any) {
  defer func() { panicked = recover() }()
  return run(), nil
}
func compare(t *testing.T, got, want int, actualPanic, expectedPanic any, actualCalls, expectedCalls int) {
  t.Helper()
  if got != want || (actualPanic != nil) != (expectedPanic != nil) || actualCalls != expectedCalls {
    t.Fatalf("got %d, %v, %d calls, want %d, %v, %d calls", got, actualPanic, actualCalls, want, expectedPanic, expectedCalls)
  }
  if actualPanic != nil && !strings.Contains(fmt.Sprint(actualPanic), "undeclared enum value at main.km:") {
    t.Fatalf("panic missing the original source location: %v", actualPanic)
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "enum-match.test", generated, reference, comparison)
}
