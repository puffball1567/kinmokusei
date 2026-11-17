package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestResultPresenceMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "presence.km")
	input := `import go context from "context";
export class Item { constructor(public n: int) {} }
export let Source: (mode: int) => Result<Item> = (mode): Result<Item> => { return ok(new Item(7)); };
export function If(mode: int): int {
  const [value, err] = Source(mode);
  if (err === nil) { return value.n; }
  return 0;
}
export function Exit(mode: int): int {
  const [value, err] = Source(mode);
  if (err !== nil) { return 0; }
  return value.n;
}
export function Wrapped(mode: int): int {
  const [value, err] = Source(mode);
  if ((nil === err) !== false) { return value.n; }
  return 0;
}
export function Switch(mode: int): int {
  const [value, err] = Source(mode);
  switch (err) {
    default { return 0; }
    case nil { return value.n; }
  }
}
export function BooleanSwitch(mode: int): int {
  const [value, err] = Source(mode);
  switch (err === nil) {
    case false { return 0; }
    default { return value.n; }
  }
}
export function ShortCircuit(mode: int): boolean {
  const [value, err] = Source(mode);
  return err !== nil || value.n > 0;
}
export function Assignment(mode: int): int {
  let value: Item = new Item(3);
  let err: error = nil;
  [value, err] = Source(mode);
  if (err !== nil) { return 0; }
  return value.n;
}
export function NullableError(mode: int): int {
  let value: Item = new Item(3);
  let err: error | null = null;
  [value, err] = Source(mode);
  if (err === null) { return value.n; }
  return 0;
}
export function Closure(mode: int): int {
  const [value, err] = Source(mode);
  const read = (): int => {
    if (err !== nil) { return 0; }
    return value.n;
  };
  return read();
}
export function Loop(mode: int): int {
  const [value, err] = Source(mode);
  while (err === nil) { return value.n; }
  return 0;
}
export function Reassigned(mode: int): int {
  let [value, err] = Source(0);
  if (err !== nil) { return 0; }
  [value, err] = Source(mode);
  if (err !== nil) { return 0; }
  return value.n;
}
export function Generic<T>(source: () => Result<T>, fallback: T): T {
  const [value, err] = source();
  if (err !== nil) { return fallback; }
  return value;
}
export class Host {
  public function read(mode: int): int {
    const [value, err] = Source(mode);
    if (err !== nil) { return 0; }
    return value.n;
  }
}
function raw(): Result<context.CancelFunc> { return ok(nil); }
export function Raw(): boolean {
  const [value, err] = raw();
  if (err !== nil) { return false; }
  return value === nil;
}
function optional(): Result<Item | null> { return ok(null); }
export function Nullable(): boolean {
  const [value, _] = optional();
  return value === null;
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "resultpresence")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{path}, "resultpresence")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	reference := `package reference

import "errors"

type Item struct{ N int }
type failure struct{}

func (*failure) Error() string { return "failed" }
func source(mode int) (*Item, error) {
	var err error
	if mode == 1 || mode == 2 {
		err = errors.New("failed")
	}
	if mode == 3 || mode == 4 {
		err = (*failure)(nil)
	}
	if mode == 0 || mode == 2 || mode == 4 {
		return &Item{7}, err
	}
	return nil, err
}
func Read(mode int) int {
	value, err := source(mode)
	if err != nil {
		return 0
	}
	return value.N
}
func ShortCircuit(mode int) bool {
	value, err := source(mode)
	return err != nil || value.N > 0
}
func Generic[T any](source func() (T, error), fallback T) T {
	value, err := source()
	if err != nil {
		return fallback
	}
	return value
}
`
	comparison := `package resultpresence_test

import (
	"errors"
	"fmt"
	g "result-presence.test"
	r "result-presence.test/reference"
	"testing"
)

type failure struct{}

func (*failure) Error() string { return "failed" }
func TestGuards(t *testing.T) {
	g.Source = func(mode int) (*g.Item, error) {
		var err error
		if mode == 1 || mode == 2 {
			err = errors.New("failed")
		}
		if mode == 3 || mode == 4 {
			err = (*failure)(nil)
		}
		if mode == 0 || mode == 2 || mode == 4 {
			return g.NewItem(7), err
		}
		return nil, err
	}
	for mode := 0; mode < 5; mode++ {
		for name, call := range map[string]func(int) int{
			"if": g.If, "exit": g.Exit, "wrapped": g.Wrapped,
			"switch": g.Switch, "boolean switch": g.BooleanSwitch,
			"assignment": g.Assignment, "nullable error": g.NullableError,
			"closure": g.Closure, "loop": g.Loop, "reassigned": g.Reassigned,
			"method": g.NewHost().Read,
		} {
			t.Run(fmt.Sprint(name, mode), func(t *testing.T) {
				if got, want := call(mode), r.Read(mode); got != want {
					t.Fatal(got, want)
				}
			})
		}
		if got, want := g.ShortCircuit(mode), r.ShortCircuit(mode); got != want {
			t.Fatal(mode, got, want)
		}
		producer := func() (int, error) {
			if mode != 0 {
				return 0, errors.New("failed")
			}
			return 7, nil
		}
		if got, want := g.Generic(producer, 9), r.Generic(producer, 9); got != want {
			t.Fatal(got, want)
		}
	}
	if !g.Raw() || !g.Nullable() {
		t.Fatal("nullable/raw contracts")
	}
}
`
	runGeneratedGoDifferentialTest(t, root, "result-presence.test", generated, reference, comparison)
}
