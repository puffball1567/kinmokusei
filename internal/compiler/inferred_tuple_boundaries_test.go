package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInferredTupleBoundariesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module inferred-tuples.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native
type hidden struct { Value int }
func Pair[T any](value T) (T, hidden) { return value, hidden{9} }
`,
		"tuple.km": `import go native from "example.com/native";
import go context from "context";
export class Box {
  constructor(public n: int) {}
}
export alias Callback = () => int;
export let Supply: () => (Box, int) = (): (Box, int) => { return new Box(7), 0; };
export let Callbacks: () => (Callback, Callback, boolean) = (): (Callback, Callback, boolean) => {
  return () => 7, () => 8, false;
};
export let Hits = 0;

export function Read(): int {
  const [value, n] = Supply();
  Hits++;
  return value.n + n;
}
export function Unused(): void {
  const [value, n] = Supply();
  Hits++;
}
export function Discard(): void {
  const [_, n] = Supply();
  Hits++;
}
export function Flag(): int {
  const [first, second, flag] = Callbacks();
  Hits++;
  return first() + second();
}
export function DiscardFirst(): int {
  const [_, second, flag] = Callbacks();
  Hits++;
  return second();
}
export function FalseCondition(): void {
  for (const [value, n] = Supply(); false;) {}
  Hits++;
}
export function Fresh(): int {
  let snapshots: Callback[] = [];
  for (let [value, n] = Supply(); n < 3; n++) {
    snapshots = append(snapshots, () => value.n + n);
  }
  Hits++;
  return snapshots[0]() * 100 + snapshots[1]() * 10 + snapshots[2]();
}
export function Labels(): int {
  let total = 0;
  outer: for (let [value, n] = Supply(); n < 3; n++) {
    if (n === 1) { continue outer; }
    total += value.n + n;
    if (n === 2) { break outer; }
  }
  Hits++;
  return total;
}
export function Restart(): int {
  let attempts = 0;
  outer: for (let [value, n] = Supply(); n < 3; n++) {
    attempts++;
    if (attempts === 1) { goto outer; }
    if (n === 0) { continue outer; }
    Hits++;
    return value.n + n;
  }
  return -1;
}
export function Infinite(): int {
  for (const [value, n] = Supply();;) {
    Hits++;
    return value.n + n;
  }
  return -1;
}
export function Opaque(value: int): int {
  const [item, token] = native.Pair(value);
  Hits++;
  return item + token.Value;
}
export function OpaqueLoop(value: int): int {
  for (let [item, token] = native.Pair(value);;) {
    Hits++;
    return item + token.Value;
  }
  return -1;
}
export function Shadow(): int {
  const [Box, n] = Supply();
  Hits++;
  return Box.n + n;
}
export function Finally(): int {
  try {
    const [value, n] = Supply();
    Hits++;
    return value.n + n;
  } finally {
    Hits += 10;
  }
}
constraint Functions = () => int;
export function Generic<T extends Functions>(producer: () => (T, int)): T {
  const [value, n] = producer();
  Hits++;
  return value;
}
function optional(): (Box | null, int) { return null, 4; }
export function Nullable(): int {
  const [value, n] = optional();
  Hits++;
  if (value === null) { return n; }
  return value.n;
}
function raw(): (context.CancelFunc, int) { return nil, 4; }
export function Raw(): int {
  const [value, n] = raw();
  Hits++;
  if (value === nil) { return n; }
  value();
  return 0;
}
export interface Reader { function read(): int; }
class DefaultReader implements Reader {
  public function read(): int { return 8; }
}
export let Readers: () => (Reader, int) = (): (Reader, int) => { return new DefaultReader(), 0; };
export function Interface(): int {
  const [value, n] = Readers();
  Hits++;
  return value.read();
}
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "tuple.km")}
	generated, diagnostics, err := EmitGo(paths, "inferredtuples")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "inferredtuples")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	if bytes.Contains(generated, []byte("native.hidden")) {
		t.Fatalf("private companion result type escaped inference:\n%s", generated)
	}
	reference := `package reference

type Box struct{ N int }

var Hits int

func check(value *Box) {
	if value == nil {
		panic("nil box")
	}
}
func Read(producer func() (*Box, int)) int {
	value, n := producer()
	check(value)
	Hits++
	return value.N + n
}
func Unused(producer func() (*Box, int), discard bool) {
	value, _ := producer()
	if !discard {
		check(value)
	}
	Hits++
}
func Flag(producer func() (func() int, func() int, bool), discardFirst bool) int {
	first, second, _ := producer()
	if !discardFirst && first == nil {
		panic("nil first")
	}
	if second == nil {
		panic("nil second")
	}
	Hits++
	if discardFirst {
		return second()
	}
	return first() + second()
}
func Fresh(producer func() (*Box, int)) int {
	value, start := producer()
	check(value)
	var snapshots []func() int
	for n := start; n < 3; n++ {
		snapshots = append(snapshots, func() int { return value.N + n })
	}
	Hits++
	return snapshots[0]()*100 + snapshots[1]()*10 + snapshots[2]()
}
func Labels(producer func() (*Box, int)) int {
	value, start := producer()
	check(value)
	total := 0
outer:
	for n := start; n < 3; n++ {
		if n == 1 {
			continue outer
		}
		total += value.N + n
		if n == 2 {
			break outer
		}
	}
	Hits++
	return total
}
func Restart(producer func() (*Box, int)) int {
	attempts := 0
outer:
	for value, n := producer(); n < 3; n++ {
		check(value)
		attempts++
		if attempts == 1 {
			goto outer
		}
		if n == 0 {
			continue outer
		}
		Hits++
		return value.N + n
	}
	return -1
}
func Finally(producer func() (*Box, int)) int {
	defer func() { Hits += 10 }()
	return Read(producer)
}
`
	comparison := `package inferredtuples_test

import (
	"fmt"
	g "inferred-tuples.test"
	r "inferred-tuples.test/reference"
	"strings"
	"testing"
)

func capture(call func()) (failure any) {
	defer func() { failure = recover() }()
	call()
	return nil
}
func TestTransfers(t *testing.T) {
	for _, valid := range []bool{false, true} {
		for name, pair := range map[string]struct {
			generated func() int
			reference func(func() (*r.Box, int)) int
		}{
			"read":            {g.Read, r.Read},
			"shadow":          {g.Shadow, r.Read},
			"unused":          {func() int { g.Unused(); return 0 }, func(p func() (*r.Box, int)) int { r.Unused(p, false); return 0 }},
			"discard":         {func() int { g.Discard(); return 0 }, func(p func() (*r.Box, int)) int { r.Unused(p, true); return 0 }},
			"false condition": {func() int { g.FalseCondition(); return 0 }, func(p func() (*r.Box, int)) int { r.Unused(p, false); return 0 }},
			"fresh":           {g.Fresh, r.Fresh},
			"labels":          {g.Labels, r.Labels},
			"restart":         {g.Restart, r.Restart},
			"infinite":        {g.Infinite, r.Read},
			"finally":         {g.Finally, r.Finally},
		} {
			t.Run(fmt.Sprint(name, valid), func(t *testing.T) {
				g.Hits, r.Hits = 0, 0
				gc, rc := 0, 0
				g.Supply = func() (*g.Box, int) {
					gc++
					if !valid {
						return nil, 0
					}
					return g.NewBox(7), 0
				}
				rp := func() (*r.Box, int) {
					rc++
					if !valid {
						return nil, 0
					}
					return &r.Box{7}, 0
				}
				var got, want int
				gp := capture(func() { got = pair.generated() })
				failure := capture(func() { want = pair.reference(rp) })
				if (gp == nil) != (failure == nil) || got != want || g.Hits != r.Hits || gc != rc {
					t.Fatal(gp, failure, got, want, g.Hits, r.Hits, gc, rc)
				}
				if gp != nil && !strings.Contains(fmt.Sprint(gp), "non-null storage boundary failed at tuple.km:") {
					t.Fatal("missing boundary location", gp)
				}
			})
		}
	}
}
func TestSlots(t *testing.T) {
	for mode := 0; mode < 4; mode++ {
		first, second := (func() int)(nil), (func() int)(nil)
		if mode&1 != 0 {
			first = func() int { return 7 }
		}
		if mode&2 != 0 {
			second = func() int { return 8 }
		}
		calls := 0
		producer := func() (func() int, func() int, bool) { calls++; return first, second, false }
		g.Callbacks = producer
		for _, discard := range []bool{false, true} {
			g.Hits, r.Hits = 0, 0
			var got, want int
			gp := capture(func() {
				if discard {
					got = g.DiscardFirst()
				} else {
					got = g.Flag()
				}
			})
			failure := capture(func() { want = r.Flag(producer, discard) })
			if (gp == nil) != (failure == nil) || got != want || g.Hits != r.Hits {
				t.Fatal(mode, discard, gp, failure, got, want, g.Hits, r.Hits)
			}
		}
		if calls != 4 {
			t.Fatal("producer evaluated repeatedly", calls)
		}
	}
}
func TestPrivateCompanion(t *testing.T) {
	if got, want := g.Opaque(7), 16; got != want {
		t.Fatal(got, want)
	}
	if got, want := g.OpaqueLoop(7), 16; got != want {
		t.Fatal(got, want)
	}
}
func TestRestartValidation(t *testing.T) {
	g.Hits, r.Hits = 0, 0
	gc, rc := 0, 0
	g.Supply = func() (*g.Box, int) {
		gc++
		if gc > 1 {
			return nil, 0
		}
		return g.NewBox(7), 0
	}
	rp := func() (*r.Box, int) {
		rc++
		if rc > 1 {
			return nil, 0
		}
		return &r.Box{7}, 0
	}
	gp := capture(func() { g.Restart() })
	failure := capture(func() { r.Restart(rp) })
	if gp == nil || failure == nil || gc != rc || gc != 2 || g.Hits != r.Hits || g.Hits != 0 {
		t.Fatal(gp, failure, gc, rc, g.Hits, r.Hits)
	}
	if !strings.Contains(fmt.Sprint(gp), "non-null storage boundary failed at tuple.km:") {
		t.Fatal(gp)
	}
}
func TestGenericAndNullable(t *testing.T) {
	for _, valid := range []bool{false, true} {
		g.Hits = 0
		producer := func() (func() int, int) {
			if !valid {
				return nil, 0
			}
			return func() int { return 7 }, 0
		}
		var got int
		gp := capture(func() { got = g.Generic(producer)() })
		if (gp == nil) != valid || (valid && got != 7) || (!valid && g.Hits != 0) {
			t.Fatal(valid, gp, got, g.Hits)
		}
	}
	if got, want := g.Nullable(), 4; got != want {
		t.Fatal(got, want)
	}
	if got, want := g.Raw(), 4; got != want {
		t.Fatal(got, want)
	}
}

type reader struct{}

func (*reader) Read() int { return 8 }
func TestInterfaces(t *testing.T) {
	for _, value := range []g.Reader{nil, (*reader)(nil), &reader{}} {
		g.Readers = func() (g.Reader, int) { return value, 0 }
		g.Hits = 0
		var got int
		gp := capture(func() { got = g.Interface() })
		if (gp == nil) != (value != nil) || (value != nil && got != 8) || (value == nil && g.Hits != 0) {
			t.Fatal(value, gp, got, g.Hits)
		}
	}
}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "inferred-tuples.test", generated, reference, comparison, nil, nil)
}
