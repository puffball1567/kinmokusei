package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNullableCompilesAndRuns(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "nullable.otm")
	input := `
class User { constructor(public name: string) {} }

function maybe(present: boolean): User | null {
  if (present) { return new User("onsen"); }
  return null;
}
function describe(present: boolean): string {
  const user = maybe(present);
  if (user === null) { return "missing"; }
  return user.name;
}
function describeMutable(present: boolean): string {
  let user = maybe(present);
  if (user === null) { return "missing"; }
  return user.name;
}
function reassignedName(present: boolean): string {
  let user = maybe(present);
  let first = "missing";
  if (user !== null) { first = user.name; }
  user = null;
  user = new User("again");
  return first + ":" + user.name;
}
function joinedName(left: boolean): string {
  let user: User | null = null;
  if (left) { user = new User("left"); }
  else { user = new User("right"); }
  return user.name;
}
function loopName(present: boolean): string {
  let user = maybe(present);
  let name = "missing";
  while (user !== null) {
    name = user.name;
    user = null;
  }
  return name;
}
function load(present: boolean): Result<User | null> {
  return ok(maybe(present));
}
function loadedName(present: boolean): Result<string> {
  const user = load(present)?;
  if (user === null) { return ok("missing"); }
  return ok(user.name);
}
function maybeValues(present: boolean): int[] | null {
  if (present) { return [1, 2]; }
  return null;
}
function safeLength(present: boolean): int { return len(maybeValues(present)); }
function closureMutationName(present: boolean, invoke: boolean): string {
  let user = maybe(present);
  const clear = (): void => { user = null; };
  if (invoke) { clear(); }
  const snapshot = user;
  if (snapshot === null) { return "missing"; }
  return snapshot.name;
}
function pointerMutationName(present: boolean, clear: boolean): string {
  let user = maybe(present);
  const pointer = &user;
  if (clear) { *pointer = null; }
  const snapshot = *pointer;
  if (snapshot === null) { return "missing"; }
  return snapshot.name;
}
function accessBeforeCapture(present: boolean): string {
  let user = maybe(present);
  let observed = "missing";
  if (user !== null) { observed = user.name; }
  const clear = (): void => { user = null; };
  clear();
  return observed;
}
function guardedLoopName(present: boolean, rounds: int): string {
  let user = maybe(present);
  let names = "";
  for (let index = 0; index < rounds; index = index + 1) {
    if (user === null) { user = new User("again"); }
    names = names + user.name;
    user = null;
  }
  return names;
}
function continueLoopName(rounds: int): string {
  let user: User | null = new User("first");
  let names = "";
  for (let index = 0; index < rounds; index = index + 1) {
    if (user === null) { user = new User("again"); }
    names = names + user.name;
    user = null;
    continue;
    user = new User("unreachable");
  }
  return names;
}
function breakLoopName(present: boolean): string {
  let user = maybe(present);
  if (user === null) { return "missing"; }
  let observed = "";
  let repeat = true;
  while (repeat) {
    observed = user.name;
    user = null;
    break;
    user = new User("unreachable");
  }
  return observed;
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "nullablematrix")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, want := range []string{
		"func maybe(present bool) *User",
		"return nil",
		"if user == nil",
		"func load(present bool) (*User, error)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	if err := os.WriteFile(filepath.Join(temp, "generated.go"), generated, 0o644); err != nil {
		t.Fatal(err)
	}
	referenceSource := `package reference
type User struct { name string }
func newUser(name string) *User { return &User{name: name} }
func maybe(present bool) *User { if present { return newUser("onsen") }; return nil }
func Describe(present bool) string { user := maybe(present); if user == nil { return "missing" }; return user.name }
func ReassignedName(present bool) string { user := maybe(present); first := "missing"; if user != nil { first = user.name }; user = nil; user = newUser("again"); return first+":"+user.name }
func JoinedName(left bool) string { var user *User; if left { user = newUser("left") } else { user = newUser("right") }; return user.name }
func LoopName(present bool) string { user := maybe(present); name := "missing"; for user != nil { name = user.name; user = nil }; return name }
func LoadedName(present bool) (string, error) { return Describe(present), nil }
func SafeLength(present bool) int { var values []int; if present { values = []int{1, 2} }; return len(values) }
func ClosureMutationName(present, invoke bool) string { user := maybe(present); clear := func() { user = nil }; if invoke { clear() }; snapshot := user; if snapshot == nil { return "missing" }; return snapshot.name }
func PointerMutationName(present, clear bool) string { user := maybe(present); pointer := &user; if clear { *pointer = nil }; snapshot := *pointer; if snapshot == nil { return "missing" }; return snapshot.name }
func AccessBeforeCapture(present bool) string { user := maybe(present); observed := "missing"; if user != nil { observed = user.name }; clear := func() { user = nil }; clear(); return observed }
func GuardedLoopName(present bool, rounds int) string { user := maybe(present); names := ""; for index := 0; index < rounds; index++ { if user == nil { user = newUser("again") }; names += user.name; user = nil }; return names }
func ContinueLoopName(rounds int) string { user := newUser("first"); names := ""; for index := 0; index < rounds; index++ { if user == nil { user = newUser("again") }; names += user.name; user = nil; continue }; return names }
func BreakLoopName(present bool) string { user := maybe(present); if user == nil { return "missing" }; observed := ""; repeat := true; for repeat { observed = user.name; user = nil; break }; return observed }
`
	testSource := `package nullablematrix
import (
  "testing"
  reference "nullable.test/reference"
)
func TestNullableRuntimeMatrix(t *testing.T) {
  for _, present := range []bool{false, true} {
    if got, want := describe(present), reference.Describe(present); got != want { t.Fatalf("describe(%v) = %q, equivalent Go = %q", present, got, want) }
    if got, want := describeMutable(present), reference.Describe(present); got != want { t.Fatalf("describeMutable(%v) = %q, equivalent Go = %q", present, got, want) }
    if got, want := reassignedName(present), reference.ReassignedName(present); got != want { t.Fatalf("reassignedName(%v) = %q, equivalent Go = %q", present, got, want) }
    if got, want := loopName(present), reference.LoopName(present); got != want { t.Fatalf("loopName(%v) = %q, equivalent Go = %q", present, got, want) }
    gotLoaded, gotErr := loadedName(present)
    wantLoaded, wantErr := reference.LoadedName(present)
    if gotLoaded != wantLoaded || (gotErr == nil) != (wantErr == nil) { t.Fatalf("loadedName(%v) = (%q, %v), equivalent Go = (%q, %v)", present, gotLoaded, gotErr, wantLoaded, wantErr) }
    if got, want := safeLength(present), reference.SafeLength(present); got != want { t.Fatalf("safeLength(%v) = %d, equivalent Go = %d", present, got, want) }
    for _, mutate := range []bool{false, true} {
      if got, want := closureMutationName(present, mutate), reference.ClosureMutationName(present, mutate); got != want {
        t.Fatalf("closureMutationName(%v, %v) = %q, equivalent Go = %q", present, mutate, got, want)
      }
      if got, want := pointerMutationName(present, mutate), reference.PointerMutationName(present, mutate); got != want {
        t.Fatalf("pointerMutationName(%v, %v) = %q, equivalent Go = %q", present, mutate, got, want)
      }
    }
    if got, want := accessBeforeCapture(present), reference.AccessBeforeCapture(present); got != want {
      t.Fatalf("accessBeforeCapture(%v) = %q, equivalent Go = %q", present, got, want)
    }
    for _, rounds := range []int{0, 1, 3} {
      if got, want := guardedLoopName(present, rounds), reference.GuardedLoopName(present, rounds); got != want {
        t.Fatalf("guardedLoopName(%v, %d) = %q, equivalent Go = %q", present, rounds, got, want)
      }
    }
    if got, want := breakLoopName(present), reference.BreakLoopName(present); got != want {
      t.Fatalf("breakLoopName(%v) = %q, equivalent Go = %q", present, got, want)
    }
  }
  for _, left := range []bool{false, true} {
    if got, want := joinedName(left), reference.JoinedName(left); got != want { t.Fatalf("joinedName(%v) = %q, equivalent Go = %q", left, got, want) }
  }
  for _, rounds := range []int{0, 1, 3} {
    if got, want := continueLoopName(rounds), reference.ContinueLoopName(rounds); got != want {
      t.Fatalf("continueLoopName(%d) = %q, equivalent Go = %q", rounds, got, want)
    }
  }
}

type goReferenceUser struct { name string }

func newGoReferenceUser(name string) *goReferenceUser { return &goReferenceUser{name: name} }

func goMaybe(present bool) *goReferenceUser {
  if present { return newGoReferenceUser("onsen") }
  return nil
}

func goDescribe(present bool) string {
  user := goMaybe(present)
  if user == nil { return "missing" }
  return user.name
}

func goReassignedName(present bool) string {
  user := goMaybe(present)
  first := "missing"
  if user != nil { first = user.name }
  user = nil
  user = newGoReferenceUser("again")
  return first + ":" + user.name
}

func goJoinedName(left bool) string {
  var user *goReferenceUser
  if left { user = newGoReferenceUser("left") } else { user = newGoReferenceUser("right") }
  return user.name
}

func goLoopName(present bool) string {
  user := goMaybe(present)
  name := "missing"
  for user != nil { name = user.name; user = nil }
  return name
}

func goLoadedName(present bool) (string, error) { return goDescribe(present), nil }

func goSafeLength(present bool) int {
  var values []int
  if present { values = []int{1, 2} }
  return len(values)
}

func goClosureMutationName(present, invoke bool) string {
  user := goMaybe(present)
  clear := func() { user = nil }
  if invoke { clear() }
  snapshot := user
  if snapshot == nil { return "missing" }
  return snapshot.name
}

func goPointerMutationName(present, clear bool) string {
  user := goMaybe(present)
  pointer := &user
  if clear { *pointer = nil }
  snapshot := *pointer
  if snapshot == nil { return "missing" }
  return snapshot.name
}

func goAccessBeforeCapture(present bool) string {
  user := goMaybe(present)
  observed := "missing"
  if user != nil { observed = user.name }
  clear := func() { user = nil }
  clear()
  return observed
}

func goGuardedLoopName(present bool, rounds int) string {
  user := goMaybe(present)
  names := ""
  for index := 0; index < rounds; index++ {
    if user == nil { user = newGoReferenceUser("again") }
    names += user.name
    user = nil
  }
  return names
}

func goContinueLoopName(rounds int) string {
  user := newGoReferenceUser("first")
  names := ""
  for index := 0; index < rounds; index++ {
    if user == nil { user = newGoReferenceUser("again") }
    names += user.name
    user = nil
    continue
  }
  return names
}

func goBreakLoopName(present bool) string {
  user := goMaybe(present)
  if user == nil { return "missing" }
  observed := ""
  repeat := true
  for repeat {
    observed = user.name
    user = nil
    break
  }
  return observed
}
`
	runGeneratedGoDifferentialTest(t, temp, "nullable.test", generated, referenceSource, testSource)
}

func TestDefiniteNonNullFieldInitializationCompilesAndRuns(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "field_initialization.otm")
	input := `
import go errors from "errors";
class User { constructor(public name: string) {} }
class Holder {
  private user: User;
  constructor(primary: boolean) {
    if (primary) { this.user = new User("onsen"); }
    else { this.user = new User("tamago"); }
  }
  public function name(): string { return this.user.name; }
}
function heldName(primary: boolean): string { return new Holder(primary).name(); }
class SwitchHolder {
  private user: User;
  private items: int[];
  constructor(mode: int) {
    switch (mode) {
      case 0, 1 { this.user = new User("case"); this.items = [mode]; break; }
      default {
        if (mode < 0) { this.user = new User("negative"); }
        else { this.user = new User("other"); }
        this.items = [];
      }
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.items); }
}
function switchHeldName(mode: int): string { return new SwitchHolder(mode).name(); }
function switchHeldCount(mode: int): int { return new SwitchHolder(mode).count(); }
class NestedSwitchHolder {
  private user: User;
  constructor(outer: boolean, inner: int) {
    switch (outer) {
      case true {
        switch (inner) {
          case 0 { this.user = new User("nested-zero"); }
          default { this.user = new User("nested-default"); break; }
        }
      }
      default { this.user = new User("outer-default"); }
    }
  }
  public function name(): string { return this.user.name; }
}
function nestedHeldName(outer: boolean, inner: int): string { return new NestedSwitchHolder(outer, inner).name(); }
class TypeSwitchHolder {
  private user: User;
  constructor(value: error) {
    switch (value) {
      case nil { this.user = new User("nil"); }
      case const typed as error { this.user = new User(typed.Error()); break; }
      default { this.user = new User("default"); }
    }
  }
  public function name(): string { return this.user.name; }
}
function typeHeldName(failed: boolean): string {
  if (failed) { return new TypeSwitchHolder(errors.New("boom")).name(); }
  return new TypeSwitchHolder(nil).name();
}
class ReceiveSelectHolder {
  private user: User;
  constructor(input: GoReceiveChannel<int>) {
    select {
      case <-input { this.user = new User("receive"); }
      default { this.user = new User("default"); }
    }
  }
  public function name(): string { return this.user.name; }
}
function receiveHeldName(ready: boolean): string {
  const channel = goChannel[int](1);
  if (ready) { channel <- 1; }
  return new ReceiveSelectHolder(channel).name();
}
class SendSelectHolder {
  private user: User;
  constructor(output: GoSendChannel<int>) {
    select {
      case output <- 1 { this.user = new User("send"); break; }
      default { this.user = new User("default"); }
    }
  }
  public function name(): string { return this.user.name; }
}
function sendHeldName(ready: boolean): string {
  const channel = goChannel[int](1);
  if (!ready) { channel <- 0; }
  return new SendSelectHolder(channel).name();
}
class WhileLoopHolder {
  private user: User;
  constructor(left: boolean) {
    while (true) {
      if (left) { this.user = new User("while-left"); }
      else { this.user = new User("while-right"); }
      break;
    }
  }
  public function name(): string { return this.user.name; }
}
function whileLoopHeldName(left: boolean): string { return new WhileLoopHolder(left).name(); }
class ForeverForHolder {
  private user: User;
  constructor() { for (;;) { this.user = new User("for-ever"); break; } }
  public function name(): string { return this.user.name; }
}
function foreverForHeldName(): string { return new ForeverForHolder().name(); }
class InitializerHolder {
  private user: User;
  constructor(run: boolean) {
    for (this.user = new User("initializer"); run; ) { break; }
  }
  public function name(): string { return this.user.name; }
}
function initializerHeldName(run: boolean): string { return new InitializerHolder(run).name(); }
class RangeHolder {
  private user: User;
  private items: int[];
  constructor(stop: boolean) {
    for (const value of [1, 2]) {
      this.user = new User("range");
      this.items = [value];
      if (stop) { break; }
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.items); }
}
function rangeHeldName(stop: boolean): string { return new RangeHolder(stop).name(); }
function rangeHeldCount(stop: boolean): int { return new RangeHolder(stop).count(); }
class StringRangeHolder {
  private user: User;
  constructor(skip: boolean) {
    for (const rune of "温") {
      this.user = new User("string-range");
      if (skip) { continue; }
    }
  }
  public function name(): string { return this.user.name; }
}
function stringRangeHeldName(skip: boolean): string { return new StringRangeHolder(skip).name(); }
class FixedArrayRangeHolder {
  private user: User;
  private items: int[];
  constructor(values: [2]int) {
    for (const value of values) {
      this.user = new User("fixed-range");
      this.items = [value];
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.items); }
}
function fixedArrayRangeHeldName(values: [2]int): string { return new FixedArrayRangeHolder(values).name(); }
function fixedArrayRangeHeldCount(values: [2]int): int { return new FixedArrayRangeHolder(values).count(); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "fieldinitialization")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if !strings.Contains(string(generated), "this.user = NewUser") {
		t.Fatalf("generated Go does not initialize the field:\n%s", generated)
	}
	if err := os.WriteFile(filepath.Join(temp, "generated.go"), generated, 0o644); err != nil {
		t.Fatal(err)
	}
	referenceSource := `package reference
import "errors"
func HeldName(primary bool) string { if primary { return "onsen" }; return "tamago" }
func SwitchHeld(mode int) (string, int) { switch mode { case 0, 1: return "case", 1; default: if mode < 0 { return "negative", 0 }; return "other", 0 } }
func NestedHeldName(outer bool, inner int) string { switch outer { case true: switch inner { case 0: return "nested-zero"; default: return "nested-default" }; default: return "outer-default" } }
func TypeHeldName(failed bool) string { var value error; if failed { value = errors.New("boom") }; switch typed := value.(type) { case nil: return "nil"; case error: return typed.Error(); default: return "default" } }
func ReceiveHeldName(ready bool) string { channel := make(chan int, 1); if ready { channel <- 1 }; select { case <-channel: return "receive"; default: return "default" } }
func SendHeldName(ready bool) string { channel := make(chan int, 1); if !ready { channel <- 0 }; select { case channel <- 1: return "send"; default: return "default" } }
func WhileLoopHeldName(left bool) string { for { if left { return "while-left" }; return "while-right" } }
func ForeverForHeldName() string { for { return "for-ever" } }
func InitializerHeldName(run bool) string { name := "initializer"; for ; run; { break }; return name }
func RangeHeldName(stop bool) string { name := ""; for range []int{1, 2} { name = "range"; if stop { break } }; return name }
func RangeHeldCount(stop bool) int { var items []int; for _, value := range []int{1, 2} { items = []int{value}; if stop { break } }; return len(items) }
func StringRangeHeldName(skip bool) string { name := ""; for range "温" { name = "string-range"; if skip { continue } }; return name }
func FixedArrayRangeHeldName(values [2]int) string { name := ""; for range values { name = "fixed-range" }; return name }
func FixedArrayRangeHeldCount(values [2]int) int { var items []int; for _, value := range values { items = []int{value} }; return len(items) }
`
	testSource := `package fieldinitialization
import (
  "errors"
  "testing"
  reference "field-initialization.test/reference"
)
func TestFieldInitializationRuntime(t *testing.T) {
  for _, primary := range []bool{false, true} {
    if got, want := heldName(primary), reference.HeldName(primary); got != want { t.Fatalf("heldName(%v) = %q, equivalent Go = %q", primary, got, want) }
  }
  for _, mode := range []int{-1, 0, 1, 2} {
    wantName, wantCount := reference.SwitchHeld(mode)
    if got := switchHeldName(mode); got != wantName { t.Fatalf("switchHeldName(%d) = %q, equivalent Go = %q", mode, got, wantName) }
    if got := switchHeldCount(mode); got != wantCount { t.Fatalf("switchHeldCount(%d) = %d, equivalent Go = %d", mode, got, wantCount) }
  }
  for _, test := range []struct{ outer bool; inner int }{{false, 0}, {true, 0}, {true, 2}} {
    if got, want := nestedHeldName(test.outer, test.inner), reference.NestedHeldName(test.outer, test.inner); got != want {
      t.Fatalf("nestedHeldName(%v, %d) = %q, equivalent Go = %q", test.outer, test.inner, got, want)
    }
  }
  for _, failed := range []bool{false, true} {
    if got, want := typeHeldName(failed), reference.TypeHeldName(failed); got != want { t.Fatalf("typeHeldName(%v) = %q, equivalent Go = %q", failed, got, want) }
  }
  for _, ready := range []bool{false, true} {
    if got, want := receiveHeldName(ready), reference.ReceiveHeldName(ready); got != want { t.Fatalf("receiveHeldName(%v) = %q, equivalent Go = %q", ready, got, want) }
    if got, want := sendHeldName(ready), reference.SendHeldName(ready); got != want { t.Fatalf("sendHeldName(%v) = %q, equivalent Go = %q", ready, got, want) }
  }
  for _, value := range []bool{false, true} {
    if got, want := whileLoopHeldName(value), reference.WhileLoopHeldName(value); got != want { t.Fatalf("whileLoopHeldName(%v) = %q, equivalent Go = %q", value, got, want) }
    if got, want := initializerHeldName(value), reference.InitializerHeldName(value); got != want { t.Fatalf("initializerHeldName(%v) = %q, equivalent Go = %q", value, got, want) }
    if got, want := rangeHeldName(value), reference.RangeHeldName(value); got != want { t.Fatalf("rangeHeldName(%v) = %q, equivalent Go = %q", value, got, want) }
    if got, want := rangeHeldCount(value), reference.RangeHeldCount(value); got != want { t.Fatalf("rangeHeldCount(%v) = %d, equivalent Go = %d", value, got, want) }
    if got, want := stringRangeHeldName(value), reference.StringRangeHeldName(value); got != want { t.Fatalf("stringRangeHeldName(%v) = %q, equivalent Go = %q", value, got, want) }
  }
  if got, want := foreverForHeldName(), reference.ForeverForHeldName(); got != want { t.Fatalf("foreverForHeldName() = %q, equivalent Go = %q", got, want) }
  for _, values := range [][2]int{{1, 2}, {0, -1}} {
    if got, want := fixedArrayRangeHeldName(values), reference.FixedArrayRangeHeldName(values); got != want { t.Fatalf("fixedArrayRangeHeldName(%v) = %q, equivalent Go = %q", values, got, want) }
    if got, want := fixedArrayRangeHeldCount(values), reference.FixedArrayRangeHeldCount(values); got != want { t.Fatalf("fixedArrayRangeHeldCount(%v) = %d, equivalent Go = %d", values, got, want) }
  }
}

func goHeldName(primary bool) string {
  if primary { return "onsen" }
  return "tamago"
}

func goSwitchHeld(mode int) (string, int) {
  switch mode {
  case 0, 1:
    return "case", 1
  default:
    if mode < 0 { return "negative", 0 }
    return "other", 0
  }
}

func goNestedHeldName(outer bool, inner int) string {
  switch outer {
  case true:
    switch inner {
    case 0: return "nested-zero"
    default: return "nested-default"
    }
  default:
    return "outer-default"
  }
}

func goTypeHeldName(failed bool) string {
  var value error
  if failed { value = errors.New("boom") }
  switch typed := value.(type) {
  case nil: return "nil"
  case error: return typed.Error()
  default: return "default"
  }
}

func goReceiveHeldName(ready bool) string {
  channel := make(chan int, 1)
  if ready { channel <- 1 }
  select {
  case <-channel: return "receive"
  default: return "default"
  }
}

func goSendHeldName(ready bool) string {
  channel := make(chan int, 1)
  if !ready { channel <- 0 }
  select {
  case channel <- 1: return "send"
  default: return "default"
  }
}

func goWhileLoopHeldName(left bool) string {
  for {
    if left { return "while-left" }
    return "while-right"
  }
}

func goForeverForHeldName() string {
  for { return "for-ever" }
}

func goInitializerHeldName(run bool) string {
  name := "initializer"
  for ; run; { break }
  return name
}

func goRangeHeldName(stop bool) string {
  name := ""
  for range []int{1, 2} {
    name = "range"
    if stop { break }
  }
  return name
}

func goRangeHeldCount(stop bool) int {
  items := []int(nil)
  for _, value := range []int{1, 2} {
    items = []int{value}
    if stop { break }
  }
  return len(items)
}

func goStringRangeHeldName(skip bool) string {
  name := ""
  for range "温" {
    name = "string-range"
    if skip { continue }
  }
  return name
}

func goFixedArrayRangeHeldName(values [2]int) string {
  name := ""
  for range values { name = "fixed-range" }
  return name
}

func goFixedArrayRangeHeldCount(values [2]int) int {
  items := []int(nil)
  for _, value := range values { items = []int{value} }
  return len(items)
}
`
	runGeneratedGoDifferentialTest(t, temp, "field-initialization.test", generated, referenceSource, testSource)
}
