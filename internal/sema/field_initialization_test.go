package sema

import (
	"strings"
	"testing"
)

func TestChecksDefiniteNonNullFieldInitialization(t *testing.T) {
	diagnostics := checkSource(t, `
const globalConstructorEnabled = 2 * 3 === 6;
const globalConstructorPrefix = "温";
const globalConstructorText = globalConstructorPrefix + "泉";
class User { constructor(public name: string) {} }
class Direct {
  private user: User;
  constructor(user: User) { this.user = user; }
  public function name(): string { return this.user.name; }
}
class Branched {
  private user: User;
  constructor(flag: boolean) {
    if (flag) { this.user = new User("left"); }
    else { this.user = new User("right"); }
  }
}
class Nested {
  private user: User;
  constructor(user: User) { { this.user = user; } }
}
class Optional { public user: User | null; }
class Values {
  private items: int[];
  constructor() { this.items = []; }
}
class ValueSwitched {
  private user: User;
  private items: int[];
  constructor(mode: int) {
    switch (mode) {
      case 0, 1 {
        this.user = new User("case");
        this.items = [mode];
        break;
      }
      default {
        if (mode < 0) { this.user = new User("negative"); }
        else { this.user = new User("other"); }
        { this.items = []; }
      }
    }
  }
}
class PreinitializedSwitch {
  private user: User;
  constructor(mode: int) {
    this.user = new User("before");
    switch (mode) { case 0 {} }
  }
}
class NestedSwitch {
  private user: User;
  constructor(left: boolean, mode: int) {
    switch (left) {
      case true {
        switch (mode) {
          case 0 { this.user = new User("nested"); }
          default { this.user = new User("nested-default"); break; }
        }
      }
      default { this.user = new User("outer-default"); }
    }
  }
}
class TypeSwitched {
  private user: User;
  constructor(value: error) {
    switch (value) {
      case nil { this.user = new User("nil"); }
      case const typed as error { this.user = new User(typed.Error()); break; }
      default { this.user = new User("default"); }
    }
  }
}
class Selected {
  private user: User;
  private items: int[];
  constructor(input: GoReceiveChannel<int>, output: GoSendChannel<int>) {
    select {
      case <-input { this.user = new User("receive"); this.items = [1]; }
      case output <- 1 { this.user = new User("send"); this.items = [2]; break; }
      default { { this.user = new User("default"); } this.items = []; }
    }
  }
}
class BlockingSelected {
  private user: User;
  constructor(input: GoReceiveChannel<int>, output: GoSendChannel<int>) {
    select {
      case <-input { this.user = new User("receive"); }
      case output <- 1 { this.user = new User("send"); }
    }
  }
}
class ConditionalBreakAfterAssignment {
  private user: User;
  constructor(mode: int, stop: boolean) {
    switch (mode) {
      case 0 {
        this.user = new User("before-break");
        if (stop) { break; }
        this.user = new User("after-break-check");
      }
      default { this.user = new User("default"); }
    }
  }
}
class SequentialSwitches {
  private user: User;
  private items: int[];
  constructor(mode: int) {
    switch (mode) {
      case 0 { this.user = new User("zero"); }
      default { this.user = new User("other"); }
    }
    switch (mode) {
      case 0, 1 { this.items = [mode]; }
      default { this.items = []; }
    }
  }
}
class WhileTrueInitialized {
  private user: User;
  private items: int[];
  constructor(flag: boolean) {
    while (true) {
      if (flag) { this.user = new User("left"); }
      else { this.user = new User("right"); }
      this.items = [];
      break;
    }
  }
}
class ForeverForInitialized {
  private user: User;
  constructor() {
    for (;;) { { this.user = new User("forever"); } break; }
  }
}
class TrueForInitialized {
  private user: User;
  constructor(stop: boolean) {
    for (; true; ) {
      this.user = new User("true");
      if (stop) { break; }
      continue;
    }
  }
}
class InitializerInitialized {
  private user: User;
  constructor(run: boolean) {
    for (this.user = new User("initializer"); run; ) { break; }
  }
}
class NonEmptyRangeInitialized {
  private user: User;
  private items: int[];
  constructor(stop: boolean) {
    for (const value of [1, 2]) {
      this.user = new User("array");
      this.items = [value];
      if (stop) { break; }
    }
  }
}
class NonEmptyStringRangeInitialized {
  private user: User;
  constructor(skip: boolean) {
    for (const rune of "x") {
      this.user = new User("string");
      if (skip) { continue; }
    }
  }
}
class FixedArrayRangeInitialized {
  private user: User;
  constructor(values: [2]int) {
    for (const value of values) { this.user = new User("fixed"); }
  }
}
class FixedArrayPointerRangeInitialized {
  private user: User;
  constructor(values: *[2]int) {
    for (const value of values) { this.user = new User("fixed-pointer"); }
  }
}
class NegatedBooleanInitialized {
  private user: User;
  constructor() {
    while (!false) { this.user = new User("negated"); break; }
  }
}
class ComparedIntegerInitialized {
  private user: User;
  constructor() {
    for (; (1 + 2) * 3 === 9 && int32(4) < int32(5); ) {
      this.user = new User("integer-comparison");
      break;
    }
  }
}
class ComparedStringInitialized {
  private user: User;
  constructor() {
    while (("a" + "b") < "b" && "same" === "same") {
      this.user = new User("string-comparison");
      break;
    }
  }
}
class ConcatenatedStringRangeInitialized {
  private user: User;
  constructor() {
    for (const rune of "温" + "泉") { this.user = new User("string-concat"); }
  }
}
class AppendedRangeInitialized {
  private user: User;
  constructor() {
    for (const value of append(makeSlice[int](0), 1)) { this.user = new User("append-item"); }
  }
}
class AppendedSpreadRangeInitialized {
  private user: User;
  constructor() {
    for (const value of append(makeSlice[int](0), [1, 2]...)) { this.user = new User("append-spread"); }
  }
}
class PreservedAppendRangeInitialized {
  private user: User;
  constructor() {
    for (const value of append([1])) { this.user = new User("append-preserved"); }
  }
}
class MadeSliceRangeInitialized {
  private user: User;
  constructor() {
    for (const value of makeSlice[int](1 + 1)) { this.user = new User("make-slice"); }
  }
}
class BoundBooleanInitialized {
  private user: User;
  constructor() {
    const compared = 1 + 2 === 3;
    const enabled = !false && compared;
    while (enabled) { this.user = new User("bound-boolean"); break; }
  }
}
class ForInitializerConstantInitialized {
  private user: User;
  constructor() {
    for (const enabled = "a" < "b"; enabled; ) {
      this.user = new User("for-constant");
      break;
    }
  }
}
class BoundCardinalityInitialized {
  private user: User;
  constructor() {
    const count = 1 + 1;
    const values = makeSlice[int](count);
    for (const value of values) { this.user = new User("bound-cardinality"); }
  }
}
class BoundAppendInitialized {
  private user: User;
  constructor() {
    const values = append(makeSlice[int](0), 1);
    for (const value of values) { this.user = new User("bound-append"); }
  }
}
class BoundStringInitialized {
  private user: User;
  constructor() {
    const prefix = "on";
    const text = prefix + "sen";
    for (const rune of text) { this.user = new User("bound-string"); }
  }
}
class GlobalConstantsInitialized {
  private user: User;
  private items: int[];
  constructor() {
    while (globalConstructorEnabled) { this.user = new User("global-boolean"); break; }
    for (const rune of globalConstructorText) { this.items = [int(rune)]; }
  }
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestRejectsIncompleteNonNullFieldInitialization(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"missing constructor", `class User {} class Holder { private user: User; }`, `non-null field "user"`},
		{"missing assignment", `class User {} class Holder { private user: User; constructor() {} }`, `assign this.user`},
		{"one branch", `class User {} class Holder { private user: User; constructor(flag: boolean) { if (flag) { this.user = new User(); } } }`, `every constructor path`},
		{"loop is not definite", `class User {} class Holder { private user: User; constructor(flag: boolean) { while (flag) { this.user = new User(); } } }`, `every constructor path`},
		{"while true break before assignment", `class User {} class Holder { private user: User; constructor(stop: boolean) { while (true) { if (stop) { break; } this.user = new User(); break; } } }`, `every constructor path`},
		{"while true without completing break", `class User {} class Holder { private user: User; constructor() { while (true) { this.user = new User(); } } }`, `every constructor path`},
		{"for true break before assignment", `class User {} class Holder { private user: User; constructor(stop: boolean) { for (; true; ) { if (stop) { break; } this.user = new User(); break; } } }`, `every constructor path`},
		{"dynamic for may execute zero times", `class User {} class Holder { private user: User; constructor(run: boolean) { for (; run; ) { this.user = new User(); break; } } }`, `every constructor path`},
		{"for post does not run before break", `class User {} class Holder { private user: User; constructor() { for (; true; this.user = new User()) { break; } } }`, `every constructor path`},
		{"empty array range", `class User {} class Holder { private user: User; constructor() { for (const value of []) { this.user = new User(); } } }`, `every constructor path`},
		{"empty string range", `class User {} class Holder { private user: User; constructor() { for (const rune of "") { this.user = new User(); } } }`, `every constructor path`},
		{"unknown slice range", `class User {} class Holder { private user: User; constructor(values: int[]) { for (const value of values) { this.user = new User(); } } }`, `every constructor path`},
		{"zero fixed array range", `class User {} class Holder { private user: User; constructor(values: [0]int) { for (const value of values) { this.user = new User(); } } }`, `every constructor path`},
		{"range continue before assignment", `class User {} class Holder { private user: User; constructor(skip: boolean) { for (const value of [1]) { if (skip) { continue; } this.user = new User(); } } }`, `every constructor path`},
		{"range break before assignment", `class User {} class Holder { private user: User; constructor(stop: boolean) { for (const value of [1, 2]) { if (stop) { break; } this.user = new User(); } } }`, `every constructor path`},
		{"negated dynamic condition", `class User {} class Holder { private user: User; constructor(flag: boolean) { while (!flag) { this.user = new User(); break; } } }`, `every constructor path`},
		{"constant false comparison", `class User {} class Holder { private user: User; constructor() { while (1 + 1 > 3) { this.user = new User(); break; } } }`, `every constructor path`},
		{"constant false boolean expression", `class User {} class Holder { private user: User; constructor() { for (; true && false; ) { this.user = new User(); break; } } }`, `every constructor path`},
		{"empty concatenated string range", `class User {} class Holder { private user: User; constructor() { for (const rune of "" + "") { this.user = new User(); } } }`, `every constructor path`},
		{"append preserves empty range", `class User {} class Holder { private user: User; constructor() { for (const value of append(makeSlice[int](0))) { this.user = new User(); } } }`, `every constructor path`},
		{"append empty spread range", `class User {} class Holder { private user: User; constructor() { for (const value of append(makeSlice[int](0), makeSlice[int](0)...)) { this.user = new User(); } } }`, `every constructor path`},
		{"zero make slice range", `class User {} class Holder { private user: User; constructor() { for (const value of makeSlice[int](0)) { this.user = new User(); } } }`, `every constructor path`},
		{"dynamic make slice range", `class User {} class Holder { private user: User; constructor(length: int) { for (const value of makeSlice[int](length)) { this.user = new User(); } } }`, `every constructor path`},
		{"let boolean is not a proof", `class User {} class Holder { private user: User; constructor() { let enabled = true; while (enabled) { this.user = new User(); break; } } }`, `every constructor path`},
		{"const from parameter is dynamic", `class User {} class Holder { private user: User; constructor(flag: boolean) { const enabled = flag; while (enabled) { this.user = new User(); break; } } }`, `every constructor path`},
		{"const dynamic slice is not a proof", `class User {} class Holder { private user: User; constructor(values: int[]) { const snapshot = values; for (const value of snapshot) { this.user = new User(); } } }`, `every constructor path`},
		{"shadowed false constant", `const enabled = true; class User {} class Holder { private user: User; constructor() { const enabled = false; while (enabled) { this.user = new User(); break; } } }`, `every constructor path`},
		{"bound empty string", `class User {} class Holder { private user: User; constructor() { const left = ""; const text = left + ""; for (const rune of text) { this.user = new User(); } } }`, `every constructor path`},
		{"value switch missing default", `class User {} class Holder { private user: User; constructor(mode: int) { switch (mode) { case 0 { this.user = new User(); } } } }`, `every constructor path`},
		{"value switch one case misses field", `class User {} class Holder { private user: User; constructor(mode: int) { switch (mode) { case 0, 1 { this.user = new User(); } default {} } } }`, `every constructor path`},
		{"break before assignment", `class User {} class Holder { private user: User; constructor(mode: int) { switch (mode) { case 0 { break; this.user = new User(); } default { this.user = new User(); } } } }`, `every constructor path`},
		{"conditional break before assignment", `class User {} class Holder { private user: User; constructor(mode: int, stop: boolean) { switch (mode) { case 0 { if (stop) { break; } this.user = new User(); } default { this.user = new User(); } } } }`, `every constructor path`},
		{"nested switch missing inner default", `class User {} class Holder { private user: User; constructor(mode: int, inner: int) { switch (mode) { case 0 { switch (inner) { case 1 { this.user = new User(); } } } default { this.user = new User(); } } } }`, `every constructor path`},
		{"type switch missing default", `class User {} class Holder { private user: User; constructor(value: error) { switch (value) { case nil { this.user = new User(); } case const typed as error { this.user = new User(); } } } }`, `every constructor path`},
		{"select case misses assignment", `class User {} class Holder { private user: User; constructor(input: GoReceiveChannel<int>) { select { case <-input { this.user = new User(); } default {} } } }`, `every constructor path`},
		{"select conditional break", `class User {} class Holder { private user: User; constructor(input: GoReceiveChannel<int>, stop: boolean) { select { case <-input { if (stop) { break; } this.user = new User(); } default { this.user = new User(); } } } }`, `every constructor path`},
		{"empty select", `class User {} class Holder { private user: User; constructor() { select {} } }`, `every constructor path`},
		{"fields split across cases", `class User {} class Holder { private user: User; private items: int[]; constructor(mode: int) { switch (mode) { case 0 { this.user = new User(); } default { this.items = []; } } } }`, `every constructor path`},
		{"constructor return", `class Holder { constructor() { return; } }`, `constructors cannot return early`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, test.want) {
				t.Fatalf("diagnostics = %v, want substring %q", diagnostics, test.want)
			}
		})
	}
}
