package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSplitStorageBoundariesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "split.km")
	input := `import { Timed } from "./models";
import go clock from "strings";
import go __kinmokuseiSelectPresent from "strconv";

export class Box {
  constructor(public n: int) {}
}
export interface Reader {
  function read(): int;
}
export alias Callback = () => int;
export let Hits = 0;
export let Supply: () => Result<Callback> = () => { return ok(() => 7); };

export function Map(values: Map<int, Box>): int {
  const [value, present] = values[0];
  Hits++;
  if (!present) { return 0; }
  return value.n;
}
export function Receive(ch: GoChannel<Callback>): int {
  let [value, present] = <-ch;
  Hits++;
  if (!present) { return 0; }
  return value();
}
export function Selected(ch: GoChannel<Callback>): int {
  select {
    case const [value, present] = <-ch {
      Hits++;
      if (!present) { return 0; }
      return value();
    }
  }
}
export function Interface(values: Map<int, Reader>): int {
  const [value, present] = values[0];
  Hits++;
  if (!present) { return 0; }
  return value.read();
}
export function ResultBinding(): int {
  const [value, failure] = Supply();
  Hits++;
  if (failure !== nil) { return 0; }
  return value();
}
export function DiscardError(): void {
  const [value, _] = Supply();
  Hits++;
}
export function Unused(values: Map<int, Box>): void {
  const [value, present] = values[0];
  Hits++;
}
export function DiscardFlag(values: Map<int, Box>): void {
  const [value, _] = values[0];
  Hits++;
}
export function DiscardPayload(values: Map<int, Box>): void {
  const [_, present] = values[0];
  Hits++;
}
export function UnusedSelected(ch: GoChannel<Callback>): void {
  select {
    case const [value, present] = <-ch { Hits++; }
  }
}
export function SelectDiscardFlag(ch: GoChannel<Callback>): void {
  select {
    case const [value, _] = <-ch { Hits++; }
  }
}
export function SelectDiscardPayload(ch: GoChannel<Callback>): void {
  select {
    case const [_, present] = <-ch { Hits++; }
  }
}
export function Unselected(ch: GoChannel<Callback>): void {
  select {
    case const [value, _] = <-ch { Hits++; }
    default { Hits += 10; }
  }
}
export function Loop(values: Map<int, Box>): int {
  let callbacks: Callback[] = [];
  for (let [value, present] = values[0]; present && value.n < 10; value = new Box(value.n + 1)) {
    const snapshot = value;
    callbacks = append(callbacks, () => snapshot.n);
  }
  Hits++;
  if (len(callbacks) === 0) { return 0; }
  return callbacks[0]() * 100 + callbacks[1]() * 10 + callbacks[2]();
}
export function FalseCondition(values: Map<int, Box>): void {
  for (const [value, _] = values[0]; false;) {}
  Hits++;
}
constraint Functions = () => int;
export function Generic<T extends Functions>(ch: GoChannel<T>, fallback: T): T {
  const [value, present] = <-ch;
  if (!present) { return fallback; }
  return value;
}
export function Assertion(input: interface {}): int {
  const [value, valid] = input as? Callback;
  Hits++;
  if (!valid) { return 0; }
  return value();
}
export function Optional(values: Map<int, Box | null>): int {
  const [value, present] = values[0];
  Hits++;
  if (!present || value === null) { return 0; }
  return value.n;
}
export type Handler = distinct () => int;
export function Defined(values: Map<int, Handler>): int {
  const [value, present] = values[0];
  Hits++;
  if (!present) { return 0; }
  return value();
}
export function Qualified(ch: GoChannel<Timed>): int {
  const unused = clock.ToUpper(b"x");
  const [value, present] = <-ch;
  Hits++;
  if (!present) { return 0; }
  return value(4);
}
export function QualifiedSelected(ch: GoChannel<Timed>): int {
  select {
    case const [value, present] = <-ch {
      Hits++;
      if (!present) { return 0; }
      return value(4);
    }
  }
}
export function Shadow(ch: GoChannel<Box>): int {
  select {
    case const [Box, present] = <-ch {
      Hits++;
      if (!present) { return 0; }
      return Box.n;
    }
  }
}
export function Namespace(ch: GoChannel<Callback>): int {
  select {
    case const [value, _] = <-ch {
      Hits++;
      return len(__kinmokuseiSelectPresent.Itoa(7));
    }
  }
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	models := `import go clock from "time";
export alias Timed = (duration: clock.Duration) => int;
`
	if err := os.WriteFile(filepath.Join(root, "models.km"), []byte(models), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "splitstorage")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{path}, "splitstorage")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	reference := `package reference

import "time"

type Box struct{ N int }
type Reader interface{ Read() int }

var Hits int

func Check[T any](value T, present bool, nilValue func(T) bool) T {
	if present && nilValue(value) {
		panic("nil payload")
	}
	return value
}
func Map(values map[int]*Box) int {
	value, present := values[0]
	Check(value, present, func(v *Box) bool { return v == nil })
	Hits++
	if !present {
		return 0
	}
	return value.N
}
func Receive(ch chan func() int) int {
	value, present := <-ch
	Check(value, present, func(v func() int) bool { return v == nil })
	Hits++
	if !present {
		return 0
	}
	return value()
}
func Timed(ch chan func(time.Duration) int) int {
	value, present := <-ch
	Check(value, present, func(v func(time.Duration) int) bool { return v == nil })
	Hits++
	if !present {
		return 0
	}
	return value(4)
}
func Interface(values map[int]Reader) int {
	value, present := values[0]
	Check(value, present, func(v Reader) bool { return v == nil })
	Hits++
	if !present {
		return 0
	}
	return value.Read()
}
func Result(producer func() (func() int, error)) int {
	value, failure := producer()
	Check(value, failure == nil, func(v func() int) bool { return v == nil })
	Hits++
	if failure != nil {
		return 0
	}
	return value()
}
func DiscardError(producer func() (func() int, error)) {
	value, failure := producer()
	Check(value, failure == nil, func(v func() int) bool { return v == nil })
	Hits++
}
func Unused(values map[int]*Box, discardPayload bool) {
	value, present := values[0]
	if !discardPayload {
		Check(value, present, func(v *Box) bool { return v == nil })
	}
	Hits++
}
func UnusedReceive(ch chan func() int, discardPayload bool) {
	value, present := <-ch
	if !discardPayload {
		Check(value, present, func(v func() int) bool { return v == nil })
	}
	Hits++
}
func Loop(values map[int]*Box) int {
	value, present := values[0]
	Check(value, present, func(v *Box) bool { return v == nil })
	var callbacks []func() int
	for ; present && value.N < 10; value = (&Box{value.N + 1}) {
		snapshot := value
		callbacks = append(callbacks, func() int { return snapshot.N })
	}
	Hits++
	if len(callbacks) == 0 {
		return 0
	}
	return callbacks[0]()*100 + callbacks[1]()*10 + callbacks[2]()
}
func Assertion(input any) int {
	value, valid := input.(func() int)
	valid = valid && value != nil
	Hits++
	if !valid {
		return 0
	}
	return value()
}
`
	comparison := `package splitstorage_test

import (
	"errors"
	"fmt"
	g "split-storage.test"
	r "split-storage.test/reference"
	"strings"
	"testing"
	"time"
)

func capture(call func()) (failure any) {
	defer func() { failure = recover() }()
	call()
	return nil
}
func compare(t *testing.T, generated, reference func() int, checked bool) {
	t.Helper()
	g.Hits, r.Hits = 0, 0
	var got, want int
	gp := capture(func() { got = generated() })
	rp := capture(func() { want = reference() })
	if (gp == nil) != (rp == nil) || got != want || g.Hits != r.Hits {
		t.Fatal(gp, rp, got, want, g.Hits, r.Hits)
	}
	if checked && gp != nil && (!strings.Contains(fmt.Sprint(gp), "non-null") || !strings.Contains(fmt.Sprint(gp), "split.km:")) {
		t.Fatal("missing source location", gp)
	}
}
func TestMap(t *testing.T) {
	for mode := 0; mode < 3; mode++ {
		gm, rm := map[int]*g.Box{}, map[int]*r.Box{}
		if mode != 0 {
			gm[0], rm[0] = nil, nil
		}
		if mode == 2 {
			gm[0], rm[0] = g.NewBox(7), &r.Box{7}
		}
		compare(t, func() int { return g.Map(gm) }, func() int { return r.Map(rm) }, true)
		compare(t, func() int {
			ch := make(chan *g.Box, 1)
			if mode != 0 {
				ch <- gm[0]
			}
			close(ch)
			return g.Shadow(ch)
		}, func() int { return r.Map(rm) }, true)
		compare(t, func() int { return g.Loop(gm) }, func() int { return r.Loop(rm) }, true)
		for _, test := range []struct {
			call    func(map[int]*g.Box)
			discard bool
		}{
			{g.Unused, false}, {g.DiscardFlag, false}, {g.DiscardPayload, true},
		} {
			compare(t, func() int { test.call(gm); return 0 }, func() int { r.Unused(rm, test.discard); return 0 }, true)
		}
		compare(t, func() int { g.FalseCondition(gm); return 0 }, func() int { r.Unused(rm, false); return 0 }, true)
		want := 0
		if mode == 2 {
			want = 7
		}
		if got := g.Optional(gm); got != want {
			t.Fatal("nullable payload", mode, got, want)
		}
	}
}
func channel(mode int) chan func() int {
	ch := make(chan func() int, 1)
	if mode == 1 {
		ch <- nil
	}
	if mode == 2 {
		ch <- func() int { return 7 }
	}
	close(ch)
	return ch
}
func TestReceive(t *testing.T) {
	for mode := 0; mode < 3; mode++ {
		for _, call := range []func(chan func() int) int{g.Receive, g.Selected} {
			compare(t, func() int { return call(channel(mode)) }, func() int { return r.Receive(channel(mode)) }, true)
		}
		for _, test := range []struct {
			call    func(chan func() int)
			discard bool
		}{
			{g.UnusedSelected, false}, {g.SelectDiscardFlag, false}, {g.SelectDiscardPayload, true},
		} {
			compare(t, func() int { test.call(channel(mode)); return 0 }, func() int { r.UnusedReceive(channel(mode), test.discard); return 0 }, true)
		}
		compare(t, func() int { return g.Namespace(channel(mode)) }, func() int { r.UnusedReceive(channel(mode), false); return 1 }, true)
		var got int
		gp := capture(func() { got = g.Generic(channel(mode), func() int { return 3 })() })
		want := 3
		if mode == 2 {
			want = 7
		}
		if (gp != nil) != (mode == 1) || (gp == nil && got != want) {
			t.Fatal(mode, gp, got)
		}
	}
	g.Hits = 0
	g.Unselected(nil)
	if g.Hits != 10 {
		t.Fatal("unselected case", g.Hits)
	}
}

type reader struct{}

func (*reader) Read() int { return 8 }

type failure struct{}

func (*failure) Error() string { return "failed" }
func TestInterface(t *testing.T) {
	for _, value := range []any{nil, (*reader)(nil), &reader{}} {
		gm, rm := map[int]g.Reader{}, map[int]r.Reader{}
		if value == nil {
			gm[0], rm[0] = nil, nil
		} else {
			gm[0], rm[0] = value.(*reader), value.(*reader)
		}
		compare(t, func() int { return g.Interface(gm) }, func() int { return r.Interface(rm) }, true)
	}
}
func TestResult(t *testing.T) {
	for mode := 0; mode < 5; mode++ {
		var value func() int
		var failureValue error
		if mode == 0 || mode == 4 {
			value = func() int { return 7 }
		}
		if mode == 2 || mode == 4 {
			failureValue = errors.New("failed")
		}
		if mode == 3 {
			failureValue = (*failure)(nil)
		}
		calls := 0
		producer := func() (func() int, error) { calls++; return value, failureValue }
		g.Supply = producer
		compare(t, g.ResultBinding, func() int { return r.Result(producer) }, true)
		compare(t, func() int { g.DiscardError(); return 0 }, func() int { r.DiscardError(producer); return 0 }, true)
		if calls != 4 {
			t.Fatal("producer evaluation count", calls)
		}
	}
}
func TestAssertion(t *testing.T) {
	for _, value := range []any{9, (func() int)(nil), func() int { return 7 }} {
		compare(t, func() int { return g.Assertion(value) }, func() int { return r.Assertion(value) }, true)
	}
}
func TestNamedAndQualified(t *testing.T) {
	for mode := 0; mode < 3; mode++ {
		values := map[int]g.Handler{}
		if mode != 0 {
			values[0] = nil
		}
		if mode == 2 {
			values[0] = func() int { return 7 }
		}
		compare(t, func() int { return g.Defined(values) }, func() int { return r.Receive(channel(mode)) }, true)
		timed := func() chan func(time.Duration) int {
			ch := make(chan func(time.Duration) int, 1)
			if mode == 1 {
				ch <- nil
			}
			if mode == 2 {
				ch <- func(v time.Duration) int { return int(v) }
			}
			close(ch)
			return ch
		}
		for _, call := range []func(chan func(time.Duration) int) int{g.Qualified, g.QualifiedSelected} {
			compare(t, func() int { return call(timed()) }, func() int { return r.Timed(timed()) }, true)
		}
	}
}
`
	runGeneratedGoDifferentialTest(t, root, "split-storage.test", generated, reference, comparison)
}
