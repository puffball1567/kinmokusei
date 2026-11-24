package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProductMatchMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "product.km")
	input := `export struct Config { public enabled: boolean; public fallback: *int | null; public unused: int }
export struct Payload<T> { public flags: [2]T; public unused: string }

export function Record(config: Config): int {
  return match(config) {
    case { enabled: true, fallback: null } => 1
    case { fallback: const value!, enabled: true } => *value
    case { enabled: false } => 0
  }
}
export function ResultPayload(load: () => Result<Payload<boolean>>): int {
  return match(load()) {
    case ok({ flags: [true, ..._] }) => 1
    case ok({ flags: [..._, true] }) => 2
    case ok({ flags: [false, false] }) => 3
    case fail(_) => -1
  }
}
export function Last(values: [3]boolean): boolean {
  return match(values) { case [..._, true] => true, case [_, _, false] => false }
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "productmatch")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
func Record(enabled bool, fallback *int) int {
  if !enabled { return 0 }
  if fallback == nil { return 1 }
  return *fallback
}
func ResultPayload(load func() ([2]bool, error)) int {
  flags, err := load()
  if err != nil { return -1 }
  if flags[0] { return 1 }
  if flags[1] { return 2 }
  return 3
}
func Last(values [3]bool) bool { return values[2] }
`
	comparison := `package productmatch_test
import (
  "errors"
  "testing"
  g "product-match.test"
  r "product-match.test/reference"
)
func TestMatch(t *testing.T) {
  n := 42
  for _, enabled := range []bool{true, false} {
    for _, fallback := range []*int{nil, &n} {
      got := g.Record(g.Config{Enabled: enabled, Fallback: fallback, Unused: 7})
      if want := r.Record(enabled, fallback); got != want { t.Fatal(got, want) }
    }
  }
  for _, a := range []bool{true, false} {
    for _, b := range []bool{true, false} {
      for _, c := range []bool{true, false} {
        values := [3]bool{a, b, c}
        if got, want := g.Last(values), r.Last(values); got != want { t.Fatal(values, got, want) }
      }
      for _, err := range []error{nil, errors.New("failed")} {
        flags := [2]bool{a, b}
        calls := 0
        got := g.ResultPayload(func() (g.Payload[bool], error) {
          calls++
          return g.Payload[bool]{Flags: flags}, err
        })
        want := r.ResultPayload(func() ([2]bool, error) { return flags, err })
        if got != want || calls != 1 { t.Fatal(flags, err, got, want, calls) }
      }
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "product-match.test", generated, reference, comparison)
}
