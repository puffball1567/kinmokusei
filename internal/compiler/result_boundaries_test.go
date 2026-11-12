package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestResultBoundariesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module result-boundaries.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native
import "errors"
type hidden func() int
type failure struct{}
func (*failure) Error() string { return "failed" }
func Load(mode int) (hidden, error) {
  var value hidden
  if mode == 0 || mode == 3 || mode == 5 { value = func() int { return 7 } }
  if mode == 2 || mode == 3 { return value, errors.New("failed") }
  if mode == 4 || mode == 5 { return value, (*failure)(nil) }
  return value, nil
}
type reader struct{}
func (*reader) Read() int { return 8 }
func Pointer() (*reader, error) { return nil, nil }
`,
		"models.km": `import go clock from "time";
export alias Callback = () => int;
export alias Timed = (n: clock.Duration) => int;
export alias View = interface { Read(): int; };
`,
		"entry.km": `import { Callback, Timed, View } from "./models";
import go native from "example.com/native";
import go clock from "strings";
import go context from "context";
export class Box { constructor(public n: int) {} }
export let Hits = 0;
export let Source: () => Result<Box> = (): Result<Box> => { return ok(new Box(7)); };
export let Timer: () => Result<Timed> = (): Result<Timed> => { return ok((n) => int(n)); };
export function Forward(): Result<Box> { return Source(); }
export function ShadowForward(): Result<Box> { const Box = 4; return Source(); }
export const Arrow = (): Result<Box> => { return Source(); };
export class Host {
  public function load(): Result<Box> { return Source(); }
}
export function FinallyForward(): Result<Box> {
  try { return Source(); } finally { Hits += 10; }
}
export function Override(): Result<Box> {
  try { return Source(); } finally { Hits += 10; return ok(new Box(9)); }
}
export function AsyncForward(): Result<Box> {
  const task = go Source();
  return await task;
}
export function Propagate(): Result<int> {
  const value = Source()?;
  Hits++;
  return ok(value.n);
}
export function Typed(): Result<int> {
  const Box: Box = Source()?;
  Hits++;
  return ok(Box.n);
}
export function Unused(): Result<int> {
  const value = Source()?;
  Hits++;
  return ok(42);
}
export function Discard(): Result<int> {
  const _ = Source()?;
  Hits++;
  return ok(42);
}
export function FinallyPropagate(): Result<int> {
  try {
    const value = Source()?;
    Hits++;
    return ok(value.n);
  } finally { Hits += 10; }
}
export function AsyncPropagate(): Result<int> {
  const task = go Source();
  const value = await task?;
  Hits++;
  return ok(value.n);
}
export function Private(mode: int): Result<int> {
  const value: Callback = native.Load(mode)?;
  Hits++;
  return ok(value());
}
export function Boxed(): Result<int> {
  const value: View = native.Pointer()?;
  Hits++;
  return ok(value.Read());
}
export function Qualified(): Result<Timed> {
  const unused = clock.ToUpper(b"x");
  return Timer();
}
constraint Functions = () => int;
export function Generic<T extends Functions>(provider: () => Result<T>): Result<T> {
  return provider();
}
export function GenericPropagate<T extends Functions>(provider: () => Result<T>): Result<T> {
  const value = provider()?;
  return ok(value);
}
function optional(): Result<Box | null> { return ok(null); }
export function Nullable(): Result<Box | null> { return optional(); }
export function NullablePropagate(): Result<Box | null> {
  const value = optional()?;
  return ok(value);
}
function rawSource(): Result<context.CancelFunc> { return ok(nil); }
export function RawForward(): Result<context.CancelFunc> { return rawSource(); }
export function RawPropagate(): Result<context.CancelFunc> {
  const value = rawSource()?;
  return ok(value);
}
export let Reader: () => Result<View> = (): Result<View> => {
  const value: View = native.Pointer()?;
  return ok(value);
};
export function InterfaceForward(): Result<View> { return Reader(); }
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "resultboundaries")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "resultboundaries")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	if bytes.Contains(generated, []byte("native.hidden")) || bytes.Contains(generated, []byte("native.reader")) {
		t.Fatalf("private result type leaked:\n%s", generated)
	}
	reference := `package reference

import "errors"

type Box struct{ N int }

var Hits int

func Forward(producer func() (*Box, error)) (*Box, error) {
	value, err := producer()
	if err == nil && value == nil {
		panic("nil result")
	}
	return value, err
}
func FinallyForward(producer func() (*Box, error)) (*Box, error) {
	defer func() { Hits += 10 }()
	return Forward(producer)
}
func Override(producer func() (*Box, error)) (value *Box, err error) {
	defer func() {
		_ = recover()
		Hits += 10
		value, err = &Box{9}, nil
	}()
	return Forward(producer)
}
func Propagate(producer func() (*Box, error)) (int, error) {
	value, err := producer()
	if err != nil {
		return 0, err
	}
	if value == nil {
		panic("nil storage")
	}
	Hits++
	return value.N, nil
}
func Unused(producer func() (*Box, error), discard bool) (int, error) {
	value, err := producer()
	if err != nil {
		return 0, err
	}
	if !discard && value == nil {
		panic("nil storage")
	}
	Hits++
	return 42, nil
}
func FinallyPropagate(producer func() (*Box, error)) (int, error) {
	defer func() { Hits += 10 }()
	return Propagate(producer)
}

type failure struct{}

func (*failure) Error() string { return "failed" }
func Private(mode int) (int, error) {
	var value func() int
	var err error
	if mode == 0 || mode == 3 || mode == 5 {
		value = func() int { return 7 }
	}
	if mode == 2 || mode == 3 {
		err = errors.New("failed")
	}
	if mode == 4 || mode == 5 {
		err = (*failure)(nil)
	}
	if err != nil {
		return 0, err
	}
	if value == nil {
		panic("nil storage")
	}
	Hits++
	return value(), nil
}
`
	comparison := `package resultboundaries_test

import (
	"context"
	"errors"
	"fmt"
	g "result-boundaries.test"
	r "result-boundaries.test/reference"
	"strings"
	"testing"
	"time"
)

func capture(call func()) (failure any) {
	defer func() { failure = recover() }()
	call()
	return nil
}

type failure struct{}

func (*failure) Error() string { return "failed" }
func modeError(mode int) error {
	if mode == 2 || mode == 3 {
		return errors.New("failed")
	}
	if mode == 4 || mode == 5 {
		return (*failure)(nil)
	}
	return nil
}
func TestForwarding(t *testing.T) {
	for mode := 0; mode < 6; mode++ {
		for name, pair := range map[string]struct {
			generated func() (*g.Box, error)
			reference func(func() (*r.Box, error)) (*r.Box, error)
		}{
			"direct":   {g.Forward, r.Forward},
			"shadow":   {g.ShadowForward, r.Forward},
			"arrow":    {g.Arrow, r.Forward},
			"method":   {g.NewHost().Load, r.Forward},
			"finally":  {g.FinallyForward, r.FinallyForward},
			"override": {g.Override, r.Override},
			"task":     {g.AsyncForward, r.Forward},
		} {
			t.Run(fmt.Sprint(name, mode), func(t *testing.T) {
				g.Hits, r.Hits = 0, 0
				gc, rc := 0, 0
				err := modeError(mode)
				g.Source = func() (*g.Box, error) {
					gc++
					if mode == 0 || mode == 3 || mode == 5 {
						return g.NewBox(7), err
					}
					return nil, err
				}
				rp := func() (*r.Box, error) {
					rc++
					if mode == 0 || mode == 3 || mode == 5 {
						return &r.Box{7}, err
					}
					return nil, err
				}
				var gv *g.Box
				var rv *r.Box
				var ge, re error
				gp := capture(func() { gv, ge = pair.generated() })
				expected := capture(func() { rv, re = pair.reference(rp) })
				if (gp == nil) != (expected == nil) || (gv == nil) != (rv == nil) || ge != re || gc != rc || gc != 1 || g.Hits != r.Hits {
					t.Fatal(gp, expected, gv, rv, ge, re, gc, rc, g.Hits, r.Hits)
				}
				if gv != nil && gv.N != rv.N {
					t.Fatal(gv.N, rv.N)
				}
				if gp != nil && (!strings.Contains(fmt.Sprint(gp), "non-null result") || !strings.Contains(fmt.Sprint(gp), "entry.km:")) {
					t.Fatal(gp)
				}
			})
		}
	}
}
func TestPropagation(t *testing.T) {
	for mode := 0; mode < 6; mode++ {
		for name, pair := range map[string]struct {
			generated func() (int, error)
			reference func(func() (*r.Box, error)) (int, error)
		}{
			"inferred":     {g.Propagate, r.Propagate},
			"typed shadow": {g.Typed, r.Propagate},
			"unused":       {g.Unused, func(p func() (*r.Box, error)) (int, error) { return r.Unused(p, false) }},
			"discard":      {g.Discard, func(p func() (*r.Box, error)) (int, error) { return r.Unused(p, true) }},
			"finally":      {g.FinallyPropagate, r.FinallyPropagate},
			"task":         {g.AsyncPropagate, r.Propagate},
		} {
			t.Run(fmt.Sprint(name, mode), func(t *testing.T) {
				g.Hits, r.Hits = 0, 0
				gc, rc := 0, 0
				err := modeError(mode)
				g.Source = func() (*g.Box, error) {
					gc++
					if mode == 0 || mode == 3 || mode == 5 {
						return g.NewBox(7), err
					}
					return nil, err
				}
				rp := func() (*r.Box, error) {
					rc++
					if mode == 0 || mode == 3 || mode == 5 {
						return &r.Box{7}, err
					}
					return nil, err
				}
				var got, want int
				var ge, re error
				gp := capture(func() { got, ge = pair.generated() })
				expected := capture(func() { want, re = pair.reference(rp) })
				if (gp == nil) != (expected == nil) || got != want || ge != re || gc != rc || gc != 1 || g.Hits != r.Hits {
					t.Fatal(gp, expected, got, want, ge, re, gc, rc, g.Hits, r.Hits)
				}
				if gp != nil && !strings.Contains(fmt.Sprint(gp), "non-null storage boundary failed at entry.km:") {
					t.Fatal(gp)
				}
			})
		}
		g.Hits, r.Hits = 0, 0
		var got, want int
		var ge, re error
		gp := capture(func() { got, ge = g.Private(mode) })
		expected := capture(func() { want, re = r.Private(mode) })
		if (gp == nil) != (expected == nil) || got != want || (ge == nil) != (re == nil) || fmt.Sprint(ge) != fmt.Sprint(re) || g.Hits != r.Hits {
			t.Fatal(mode, gp, expected, got, want, ge, re, g.Hits, r.Hits)
		}
	}
}
func TestOtherReferences(t *testing.T) {
	for _, valid := range []bool{false, true} {
		producer := func() (func() int, error) {
			if !valid {
				return nil, nil
			}
			return func() int { return 7 }, nil
		}
		for _, call := range []func(func() (func() int, error)) (func() int, error){g.Generic[func() int], g.GenericPropagate[func() int]} {
			var value func() int
			var err error
			gp := capture(func() { value, err = call(producer) })
			if (gp == nil) != valid || (valid && (err != nil || value() != 7)) {
				t.Fatal(valid, gp, err)
			}
		}
		g.Timer = func() (func(time.Duration) int, error) {
			if !valid {
				return nil, nil
			}
			return func(n time.Duration) int { return int(n) }, nil
		}
		var timed func(time.Duration) int
		gp := capture(func() { timed, _ = g.Qualified() })
		if (gp == nil) != valid || (valid && timed(4) != 4) {
			t.Fatal(valid, gp)
		}
	}
	for _, call := range []func() (*g.Box, error){g.Nullable, g.NullablePropagate} {
		value, err := call()
		if value != nil || err != nil {
			t.Fatal(value, err)
		}
	}
	value, err := g.Boxed()
	if value != 8 || err != nil {
		t.Fatal(value, err)
	}
	for _, call := range []func() (context.CancelFunc, error){g.RawForward, g.RawPropagate} {
		value, err := call()
		if value != nil || err != nil {
			t.Fatal(value, err)
		}
	}
	for _, value := range []g.View{nil, (*reader)(nil), &reader{}} {
		g.Reader = func() (g.View, error) { return value, nil }
		var got g.View
		gp := capture(func() { got, _ = g.InterfaceForward() })
		if (gp == nil) != (value != nil) || (got != nil && got.Read() != 8) {
			t.Fatal(value, got, gp)
		}
	}
}

type reader struct{}

func (*reader) Read() int { return 8 }
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "result-boundaries.test", generated, reference, comparison, nil, nil)
}
