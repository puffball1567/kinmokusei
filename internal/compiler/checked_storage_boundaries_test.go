package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckedStorageBoundariesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module checked-storage.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native

import "time"

type Raw func() int
type hidden func() int

func value(mode int) Raw {
	if mode == 2 {
		return func() int { return 7 }
	}
	return nil
}
func Map(mode int) map[int]Raw {
	m := map[int]Raw{}
	if mode != 0 {
		m[0] = value(mode)
	}
	return m
}
func Channel(mode int) chan Raw {
	ch := make(chan Raw, 1)
	if mode != 0 {
		ch <- value(mode)
	}
	close(ch)
	return ch
}
func HiddenChannel(mode int) chan hidden {
	ch := make(chan hidden, 1)
	if mode != 0 {
		ch <- hidden(value(mode))
	}
	close(ch)
	return ch
}
func Box(mode int) any {
	if mode == 0 {
		return 9
	}
	return value(mode)
}
func PlainBox(mode int) any {
	if mode == 0 {
		return 9
	}
	return (func() int)(value(mode))
}

type Failure struct{}

func (*Failure) Error() string { return "failure" }
func PointerMap(p bool) map[int]*Failure {
	m := map[int]*Failure{}
	if p {
		m[0] = nil
	}
	return m
}

type privateTimed func(time.Duration) int

func TimedChannel(mode int) chan privateTimed {
	ch := make(chan privateTimed, 1)
	if mode != 0 {
		var f privateTimed
		if mode == 2 {
			f = func(v time.Duration) int { return int(v) }
		}
		ch <- f
	}
	close(ch)
	return ch
}
`,
		"models.km": `import go clock from "time";
export alias Callback=()=>int;
export alias View=interface {
  Error():bstring;
};
export alias Timed=(value:clock.Duration)=>int;
`,
		"entry.km": `import go native from "example.com/native";
import go clock from "strings";
import  {
  Callback,View,Timed
}
from "./models";
export function Map(mode:int):int {
  let value:Callback=()=>3;
  let present=false;
  [value,present]=native.Map(mode)[0];
  if(!present) {
    return 0;
  }
  return value();
}
export function Receive(mode:int):int {
  let value:Callback=()=>3;
  let open=false;
  [value,open]=<-native.Channel(mode);
  if(!open) {
    return 0;
  }
  return value();
}
export function Assertion(mode:int):int {
  let value:Callback=()=>3;
  let valid=false;
  [value,valid]=native.Box(mode) as? native.Raw;
  if(!valid) {
    return 0;
  }
  return value();
}
export function NativeAssertion(mode:int):int {
  let value:Callback=()=>3;
  let valid=false;
  [value,valid]=native.PlainBox(mode) as? Callback;
  if(!valid) {
    return 0;
  }
  return value();
}
export function Selected(mode:int):int {
  let value:Callback=()=>3;
  let open=false;
  select {
    case [value,open]=<-native.Channel(mode) {
      if(!open) {
        return 0;
      }
      return value();
    }
  }
}
export function Hidden(mode:int):int {
  let value:Callback=()=>3;
  let open=false;
  select {
    case [value,open]=<-native.HiddenChannel(mode) {
      if(!open) {
        return 0;
      }
      return value();
    }
  }
}
export function Pointer(p:boolean):bstring {
  let value:View=fallbackView();
  let present=false;
  [value,present]=native.PointerMap(p)[0];
  if(!present) {
    return b"absent";
  }
  return value.Error();
}
function fallbackView():View {
  const [value,ok]=native.PointerMap(true)[0];
  if(!ok) {
    throw new Exception("missing");
  }
  return value;
}
export function Raw(mode:int):boolean {
  let value:native.Raw=nil;
  let present=false;
  [value,present]=native.Map(mode)[0];
  if(!present) {
    return false;
  }
  return value===nil;
}
export function Nullable(mode:int):boolean {
  let value:Callback|null=null;
  let present=false;
  [value,present]=native.Map(mode)[0];
  if(!present) {
    return false;
  }
  return value===null;
}
export function DiscardFlag(mode:int):void {
  let value:Callback=()=>3;
  [value,_]=native.Map(mode)[0];
}
export function SelectDiscardFlag(mode:int):void {
  let value:Callback=()=>3;
  select {
    case [value,_]=<-native.HiddenChannel(mode) {
    }
  }
}
export function SelectDiscardPayload(mode:int):boolean {
  let open=false;
  select {
    case [_,open]=<-native.Channel(mode) {
      return open;
    }
  }
}
export function Unselected(mark:()=>void):void {
  let value:Callback=()=>3;
  let open=false;
  const ch:GoChannel<native.Raw>=nil;
  select {
    case [value,open]=<-ch {
      mark();
    }
    default {
    }
  }
}
export function SelectedTargets(mode:int,flags:boolean[],index:()=>int):void {
  let value:Callback=()=>3;
  select {
    case [value,flags[index()]]=<-native.Channel(mode) {
    }
  }
}
export function Qualified(mode:int):int {
  const unused=clock.ToUpper(b"x");
  let value:Timed=(v)=>int(v);
  let open=false;
  select {
    case [value,open]=<-native.TimedChannel(mode) {
      if(!open) {
        return 0;
      }
      return value(4);
    }
  }
}
constraint Functions=()=>int;
export function Generic<T extends Functions>(ch:GoChannel<T>,fallback:T):T {
  let value:T=fallback;
  let open=false;
  select {
    case [value,open]=<-ch {
      if(!open) {
        return fallback;
      }
      return value;
    }
  }
}
export alias Provider=()=>Result<Callback>;
export let Supply:Provider=():Result<Callback>=> {
  return ok(()=>7);
};
export function ResultAssign(observe:(n:int,failed:boolean)=>void):int {
  let value:Callback=()=>3;
  let failure:error=nil;
  const report=():void=> {
    let n=0;
    if(value!==nil) {
      n=value();
    }
    observe(n,failure!==nil);
  };
  defer report();
  [value,failure]=Supply();
  if(failure!==nil) {
    return 0;
  }
  return value();
}
export function ResultDiscardError():void {
  let value:Callback=()=>3;
  [value,_]=Supply();
}
export class Item {
  public n: int = 7;
}
export function ClassMap(values: Map<int, Item>): int {
  let item: Item = new Item();
  let present = false;
  [item, present] = values[0];
  if (!present) { return 0; }
  return item.n;
}
export interface Reader {
  function read(): int;
}
class DefaultReader implements Reader {
  public function read(): int { return 3; }
}
export function InterfaceReceive(ch: GoChannel<Reader>): int {
  let value: Reader = new DefaultReader();
  let open = false;
  select {
    case [value, open] = <-ch {
      if (!open) { return 0; }
      return value.read();
    }
  }
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "checkedstorage")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "checkedstorage")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	if bytes.Contains(generated, []byte("native.hidden")) || bytes.Contains(generated, []byte("native.privateTimed")) {
		t.Fatalf("private Go type leaked:\n%s", generated)
	}
	reference := `package reference

func value(mode int) func() int {
	if mode == 2 {
		return func() int { return 7 }
	}
	return nil
}
func check(value func() int, success bool) func() int {
	if success && value == nil {
		panic("nil storage")
	}
	return value
}
func Map(mode int) int {
	m := map[int]func() int{}
	if mode != 0 {
		m[0] = value(mode)
	}
	v, ok := m[0]
	v = check(v, ok)
	if !ok {
		return 0
	}
	return v()
}
func Receive(mode int) int {
	ch := make(chan func() int, 1)
	if mode != 0 {
		ch <- value(mode)
	}
	close(ch)
	v, open := <-ch
	v = check(v, open)
	if !open {
		return 0
	}
	return v()
}
func Assertion(mode int) int {
	var input any = 9
	if mode != 0 {
		input = value(mode)
	}
	v, ok := input.(func() int)
	v = check(v, ok)
	if !ok {
		return 0
	}
	return v()
}
func NativeAssertion(mode int) int {
	if mode != 2 {
		return 0
	}
	return 7
}
func Selected(mode int) int {
	ch := make(chan func() int, 1)
	if mode != 0 {
		ch <- value(mode)
	}
	close(ch)
	var v func() int
	var open bool
	select {
	case v, open = <-ch:
		v = check(v, open)
		if !open {
			return 0
		}
		return v()
	}
}
func Generic(mode int) func() int {
	if mode == 0 {
		return func() int { return 3 }
	}
	return check(value(mode), true)
}
func Qualified(mode int) int {
	if mode == 1 {
		panic("nil timed")
	}
	if mode == 0 {
		return 0
	}
	return 4
}
func DiscardFlag(mode int) { var v func() int; ok := mode != 0; v = check(value(mode), ok); _ = v }
func Raw(mode int) bool {
	if mode == 0 {
		return false
	}
	return value(mode) == nil
}
func DiscardPayload(mode int) bool { return mode != 0 }
func SelectedTargets(mode int, flags []bool, index func() int) {
	ch := make(chan func() int, 1)
	if mode != 0 {
		ch <- value(mode)
	}
	close(ch)
	var old func() int = func() int { return 3 }
	select {
	case v, open := <-ch:
		first := &old
		second := &flags[index()]
		checked := check(v, open)
		*first = checked
		*second = open
	}
}
func ResultAssign(producer func() (func() int, error), observe func(int, bool)) int {
	v := func() int { return 3 }
	var failure error
	defer func() {
		n := 0
		if v != nil {
			n = v()
		}
		observe(n, failure != nil)
	}()
	nv, err := producer()
	checked := check(nv, err == nil)
	v, failure = checked, err
	if failure != nil {
		return 0
	}
	return v()
}
func ResultDiscardError(producer func() (func() int, error)) {
	v, err := producer()
	_ = check(v, err == nil)
}
`
	comparison := `package checkedstorage_test

import (
	g "checked-storage.test"
	r "checked-storage.test/reference"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func capture(f func()) (failure any) { defer func() { failure = recover() }(); f(); return nil }

type failure struct{}

func (*failure) Error() string { return "failure" }

type reader struct{}

func (*reader) Read() int { return 8 }

func TestNativeReferencesFromGo(t *testing.T) {
	if got := g.ClassMap(nil); got != 0 {
		t.Fatal("missing class", got)
	}
	if got := g.ClassMap(map[int]*g.Item{0: g.NewItem()}); got != 7 {
		t.Fatal("present class", got)
	}
	gp := capture(func() { g.ClassMap(map[int]*g.Item{0: nil}) })
	if !strings.Contains(fmt.Sprint(gp), "non-null storage boundary failed at entry.km:") {
		t.Fatal("nil class", gp)
	}

	ch := make(chan g.Reader, 2)
	ch <- nil
	ch <- (*reader)(nil)
	close(ch)
	gp = capture(func() { g.InterfaceReceive(ch) })
	if !strings.Contains(fmt.Sprint(gp), "non-null storage boundary failed at entry.km:") {
		t.Fatal("nil interface", gp)
	}
	if got := g.InterfaceReceive(ch); got != 8 {
		t.Fatal("typed nil interface", got)
	}
	if got := g.InterfaceReceive(ch); got != 0 {
		t.Fatal("closed channel", got)
	}
}

func TestChecked(t *testing.T) {
	for mode := 0; mode < 3; mode++ {
		for name, pair := range map[string][2]func(int) int{
			"map":              {g.Map, r.Map},
			"receive":          {g.Receive, r.Receive},
			"assertion":        {g.Assertion, r.Assertion},
			"native assertion": {g.NativeAssertion, r.NativeAssertion},
			"select":           {g.Selected, r.Selected},
			"hidden":           {g.Hidden, r.Selected},
			"qualified":        {g.Qualified, r.Qualified},
		} {
			t.Run(fmt.Sprint(name, mode), func(t *testing.T) {
				var got, want int
				gp := capture(func() { got = pair[0](mode) })
				rp := capture(func() { want = pair[1](mode) })
				if (gp == nil) != (rp == nil) || got != want {
					t.Fatal(gp, rp, got, want)
				}
				if gp != nil && !strings.Contains(fmt.Sprint(gp), "non-null storage boundary failed at entry.km:") {
					t.Fatal(gp)
				}
			})
		}
	}
}
func TestPreservedCases(t *testing.T) {
	for mode := 0; mode < 3; mode++ {
		if g.Raw(mode) != r.Raw(mode) || g.Nullable(mode) != r.Raw(mode) || g.SelectDiscardPayload(mode) != r.DiscardPayload(mode) {
			t.Fatal(mode)
		}
		for _, call := range []func(int){g.DiscardFlag, g.SelectDiscardFlag} {
			gp := capture(func() { call(mode) })
			rp := capture(func() { r.DiscardFlag(mode) })
			if (gp == nil) != (rp == nil) {
				t.Fatal(gp, rp)
			}
		}
	}
	if g.Pointer(false) != "absent" || g.Pointer(true) != "failure" {
		t.Fatal("typed nil boxing changed")
	}
	called := false
	g.Unselected(func() { called = true })
	if called {
		t.Fatal("unselected body ran")
	}
}
func TestSelectedTargets(t *testing.T) {
	for mode := 0; mode < 3; mode++ {
		initial := mode == 0
		gf, rf := []bool{initial}, []bool{initial}
		gc, rc := 0, 0
		gp := capture(func() { g.SelectedTargets(mode, gf, func() int { gc++; return 0 }) })
		rp := capture(func() { r.SelectedTargets(mode, rf, func() int { rc++; return 0 }) })
		if (gp == nil) != (rp == nil) || !reflect.DeepEqual(gf, rf) || gc != rc || gc != 1 {
			t.Fatal(gp, rp, gf, rf, gc, rc)
		}
	}
	gp := capture(func() { g.SelectedTargets(1, []bool{}, func() int { return 0 }) })
	rp := capture(func() { r.SelectedTargets(1, []bool{}, func() int { return 0 }) })
	if !strings.Contains(fmt.Sprint(gp), "index out of range") || !strings.Contains(fmt.Sprint(rp), "index out of range") {
		t.Fatal("LHS order", gp, rp)
	}
}
func TestGeneric(t *testing.T) {
	for mode := 0; mode < 3; mode++ {
		ch := make(chan func() int, 1)
		if mode != 0 {
			var fn func() int
			if mode == 2 {
				fn = func() int { return 7 }
			}
			ch <- fn
		}
		close(ch)
		var got, want int
		gp := capture(func() { got = g.Generic(ch, func() int { return 3 })() })
		rp := capture(func() { want = r.Generic(mode)() })
		if (gp == nil) != (rp == nil) || got != want {
			t.Fatal(gp, rp, got, want)
		}
	}
}
func TestResult(t *testing.T) {
	for mode := 0; mode < 5; mode++ {
		var callback func() int = func() int { return 7 }
		var err error
		if mode == 1 || mode == 2 || mode == 3 {
			callback = nil
		}
		if mode == 2 || mode == 4 {
			err = errors.New("failed")
		}
		if mode == 3 {
			err = (*failure)(nil)
		}
		producer := func() (func() int, error) { return callback, err }
		g.Supply = producer
		var got, want int
		var gs, rs []int
		gp := capture(func() {
			got = g.ResultAssign(func(n int, failed bool) {
				flag := 0
				if failed {
					flag = 1
				}
				gs = []int{n, flag}
			})
		})
		rp := capture(func() {
			want = r.ResultAssign(producer, func(n int, failed bool) {
				flag := 0
				if failed {
					flag = 1
				}
				rs = []int{n, flag}
			})
		})
		if (gp == nil) != (rp == nil) || got != want || !reflect.DeepEqual(gs, rs) {
			t.Fatal(mode, gp, rp, got, want, gs, rs)
		}
		gp = capture(g.ResultDiscardError)
		rp = capture(func() { r.ResultDiscardError(producer) })
		if (gp == nil) != (rp == nil) {
			t.Fatal(mode, gp, rp)
		}
	}
}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "checked-storage.test", generated, reference, comparison, nil, nil)
}
