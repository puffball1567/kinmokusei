package sema

import (
	"math/big"
	"strings"
	"testing"
)

func TestLengthComparisonProvesNonEmptyMatrix(t *testing.T) {
	for _, test := range []struct {
		operator            string
		constant            int64
		wantTrue, wantFalse bool
	}{
		{">", 0, true, false},
		{">", -1, false, false},
		{">=", 1, true, false},
		{">=", 0, false, false},
		{"<", 1, false, true},
		{"<", 0, false, false},
		{"<=", 0, false, true},
		{"<=", -1, false, false},
		{"==", 1, true, false},
		{"===", 0, false, true},
		{"!=", 0, true, false},
		{"!==", 1, false, true},
	} {
		gotTrue, gotFalse := lengthComparisonProvesNonEmpty(test.operator, big.NewInt(test.constant))
		if gotTrue != test.wantTrue || gotFalse != test.wantFalse {
			t.Errorf("len %s %d proof = (%v, %v), want (%v, %v)", test.operator, test.constant, gotTrue, gotFalse, test.wantTrue, test.wantFalse)
		}
	}
}

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
class GuardedSliceRangeInitialized {
  private user: User;
  constructor(values: int[]) {
    if (len(values) > 0) {
      for (const value of values) { this.user = new User("nonempty-slice"); }
    } else {
      this.user = new User("empty-slice");
    }
  }
}
class ReversedLengthGuardInitialized {
  private user: User;
  constructor(values: string) {
    if (0 < len(values)) {
      for (const rune of values) { this.user = new User("nonempty-string"); }
    } else {
      this.user = new User("empty-string");
    }
  }
}
class ZeroLengthElseInitialized {
  private user: User;
  constructor(values: int[]) {
    if (len(values) === 0) {
      this.user = new User("empty");
    } else {
      for (const value of values) { this.user = new User("nonempty"); }
    }
  }
}
class NegatedLengthGuardInitialized {
  private user: User;
  constructor(values: int[]) {
    if (!(len(values) <= 0)) {
      for (const value of values) { this.user = new User("nonempty"); }
    } else {
      this.user = new User("empty");
    }
  }
}
class ThrowingEmptyLengthBranchInitialized {
  private user: User;
  constructor(values: int[]) {
    if (len(values) !== 0) {
      for (const value of values) { this.user = new User("nonempty"); }
    } else {
      throw new Exception("values must not be empty");
    }
  }
}
class EmptyGuardClauseInitialized {
  private user: User;
  constructor(values: int[]) {
    if (len(values) === 0) { throw new Exception("empty"); }
    for (const value of values) { this.user = new User("nonempty"); }
  }
}
class NegatedEmptyGuardClauseInitialized {
  private user: User;
  constructor(values: string) {
    if (!(len(values) > 0)) { throw new Exception("empty"); }
    for (const rune of values) { this.user = new User("nonempty"); }
  }
}
class CompoundAndGuardInitialized {
  private user: User;
  constructor(values: int[], enabled: boolean) {
    if (enabled && len(values) > 0) {
      for (const value of values) { this.user = new User("nonempty"); }
    } else {
      this.user = new User("fallback");
    }
  }
}
class CompoundOrElseInitialized {
  private user: User;
  constructor(values: int[], fallback: boolean) {
    if (len(values) === 0 || fallback) {
      this.user = new User("fallback");
    } else {
      for (const value of values) { this.user = new User("nonempty"); }
    }
  }
}
class MultipleLengthGuardsInitialized {
  private user: User;
  constructor(left: int[], right: int[]) {
    if (len(left) > 0 && len(right) > 0) {
      for (const value of right) { this.user = new User("both-nonempty"); }
    } else {
      this.user = new User("fallback");
    }
  }
}
class CompoundEmptyGuardClauseInitialized {
  private user: User;
  constructor(values: int[], blocked: boolean) {
    if (blocked || len(values) === 0) { throw new Exception("unavailable"); }
    for (const value of values) { this.user = new User("available"); }
  }
}
class RedundantOrGuardInitialized {
  private user: User;
  constructor(values: int[]) {
    if (len(values) > 0 || len(values) > 2) {
      for (const value of values) { this.user = new User("nonempty"); }
    } else {
      this.user = new User("empty");
    }
  }
}
class LengthSwitchDefaultInitialized {
  private user: User;
  constructor(values: int[]) {
    switch (len(values)) {
      case 0 { this.user = new User("empty"); }
      default {
        for (const value of values) { this.user = new User("nonempty"); }
      }
    }
  }
}
class LengthSwitchPositiveCasesInitialized {
  private user: User;
  constructor(values: int[]) {
    switch (len(values)) {
      case 1, 1 + 1 {
        for (const value of values) { this.user = new User("short"); }
      }
      default { this.user = new User("other"); }
    }
  }
}
class StringLengthSwitchInitialized {
  private user: User;
  constructor(value: string) {
    switch (len(value)) {
      default {
        for (const rune of value) { this.user = new User("text"); }
      }
      case 1 - 1 { this.user = new User("empty"); }
    }
  }
}
class NestedLengthGuardInitialized {
  private user: User;
  constructor(values: int[], enabled: boolean) {
    if (len(values) > 0) {
      if (enabled) {
        for (const value of values) { this.user = new User("enabled"); }
      } else {
        for (const value of values) { this.user = new User("disabled"); }
      }
    } else {
      this.user = new User("empty");
    }
  }
}
class NestedMultipleLengthGuardsInitialized {
  private user: User;
  constructor(left: int[], right: int[]) {
    if (len(left) > 0) {
      if (len(right) > 0) {
        for (const value of right) { this.user = new User("right"); }
      } else {
        for (const value of left) { this.user = new User("left"); }
      }
    } else {
      this.user = new User("empty-left");
    }
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
		{"length guard without empty path", `class User {} class Holder { private user: User; constructor(values: int[]) { if (len(values) > 0) { for (const value of values) { this.user = new User(); } } } }`, `every constructor path`},
		{"length guard for different range", `class User {} class Holder { private user: User; constructor(values: int[], other: int[]) { if (len(values) > 0) { for (const value of other) { this.user = new User(); } } else { this.user = new User(); } } }`, `every constructor path`},
		{"non-strict zero length guard", `class User {} class Holder { private user: User; constructor(values: int[]) { if (len(values) >= 0) { for (const value of values) { this.user = new User(); } } else { this.user = new User(); } } }`, `every constructor path`},
		{"intervening collection assignment", `class User {} class Holder { private user: User; constructor(values: int[]) { if (len(values) > 0) { values = []; for (const value of values) { this.user = new User(); } } else { this.user = new User(); } } }`, `every constructor path`},
		{"channel length is not a range proof", `class User {} class Holder { private user: User; constructor(values: GoChannel<int>) { if (len(values) > 0) { for (const value of values) { this.user = new User(); } } else { this.user = new User(); } } }`, `every constructor path`},
		{"shadowed len is not a proof", `function len(values: int[]): int { return 1; } class User {} class Holder { private user: User; constructor(values: int[]) { if (len(values) > 0) { for (const value of values) { this.user = new User(); } } else { this.user = new User(); } } }`, `every constructor path`},
		{"nonterminal empty guard", `class User {} class Holder { private user: User; constructor(values: int[]) { if (len(values) === 0) {} for (const value of values) { this.user = new User(); } } }`, `every constructor path`},
		{"guard clause for different range", `class User {} class Holder { private user: User; constructor(values: int[], other: int[]) { if (len(values) === 0) { throw new Exception("empty"); } for (const value of other) { this.user = new User(); } } }`, `every constructor path`},
		{"intervening assignment after guard clause", `class User {} class Holder { private user: User; constructor(values: int[]) { if (len(values) === 0) { throw new Exception("empty"); } const count = len(values); values = []; for (const value of values) { this.user = new User(); } } }`, `every constructor path`},
		{"channel guard clause is not a range proof", `class User {} class Holder { private user: User; constructor(values: GoChannel<int>) { if (len(values) === 0) { throw new Exception("empty"); } for (const value of values) { this.user = new User(); } } }`, `every constructor path`},
		{"disjunction does not prove selected range", `class User {} class Holder { private user: User; constructor(values: int[], fallback: boolean) { if (len(values) > 0 || fallback) { for (const value of values) { this.user = new User(); } } else { this.user = new User(); } } }`, `every constructor path`},
		{"conjunction false path may still be empty", `class User {} class Holder { private user: User; constructor(values: int[], enabled: boolean) { if (len(values) === 0 && enabled) { this.user = new User(); } else { for (const value of values) { this.user = new User(); } } } }`, `every constructor path`},
		{"different collection disjunction", `class User {} class Holder { private user: User; constructor(left: int[], right: int[]) { if (len(left) > 0 || len(right) > 0) { for (const value of left) { this.user = new User(); } } else { this.user = new User(); } } }`, `every constructor path`},
		{"effectful compound guard", `function clearAndKeep(values: int[]): boolean { clear(values); return true; } class User {} class Holder { private user: User; constructor(values: int[]) { if (len(values) > 0 && clearAndKeep(values)) { for (const value of values) { this.user = new User(); } } else { this.user = new User(); } } }`, `every constructor path`},
		{"effectful nested guard", `function clearAndKeep(values: int[]): boolean { clear(values); return true; } class User {} class Holder { private user: User; constructor(values: int[]) { if (len(values) > 0) { if (clearAndKeep(values)) { for (const value of values) { this.user = new User(); } } else { this.user = new User(); } } else { this.user = new User(); } } }`, `every constructor path`},
		{"nested guard intervening assignment", `class User {} class Holder { private user: User; constructor(values: int[], enabled: boolean) { if (len(values) > 0) { if (enabled) { const count = len(values); values = []; for (const value of values) { this.user = new User(); } } else { this.user = new User(); } } else { this.user = new User(); } } }`, `every constructor path`},
		{"nested guard different collection", `class User {} class Holder { private user: User; constructor(values: int[], other: int[], enabled: boolean) { if (len(values) > 0) { if (enabled) { for (const value of other) { this.user = new User(); } } else { this.user = new User(); } } else { this.user = new User(); } } }`, `every constructor path`},
		{"shadowed false constant", `const enabled = true; class User {} class Holder { private user: User; constructor() { const enabled = false; while (enabled) { this.user = new User(); break; } } }`, `every constructor path`},
		{"bound empty string", `class User {} class Holder { private user: User; constructor() { const left = ""; const text = left + ""; for (const rune of text) { this.user = new User(); } } }`, `every constructor path`},
		{"value switch missing default", `class User {} class Holder { private user: User; constructor(mode: int) { switch (mode) { case 0 { this.user = new User(); } } } }`, `every constructor path`},
		{"value switch one case misses field", `class User {} class Holder { private user: User; constructor(mode: int) { switch (mode) { case 0, 1 { this.user = new User(); } default {} } } }`, `every constructor path`},
		{"length switch default without zero case", `class User {} class Holder { private user: User; constructor(values: int[]) { switch (len(values)) { case 2 { this.user = new User(); } default { for (const value of values) { this.user = new User(); } } } } }`, `every constructor path`},
		{"length switch mixed zero case", `class User {} class Holder { private user: User; constructor(values: int[]) { switch (len(values)) { case 0, 1 { for (const value of values) { this.user = new User(); } } default { this.user = new User(); } } } }`, `every constructor path`},
		{"length switch different range", `class User {} class Holder { private user: User; constructor(values: int[], other: int[]) { switch (len(values)) { case 0 { this.user = new User(); } default { for (const value of other) { this.user = new User(); } } } } }`, `every constructor path`},
		{"length switch expression source is not tracked", `class User {} class Holder { private user: User; constructor(values: int[]) { switch (len(append(values))) { case 0 { this.user = new User(); } default { for (const value of values) { this.user = new User(); } } } } }`, `every constructor path`},
		{"length switch intervening assignment", `class User {} class Holder { private user: User; constructor(values: int[]) { switch (len(values)) { case 0 { this.user = new User(); } default { const count = len(values); values = []; for (const value of values) { this.user = new User(); } } } } }`, `every constructor path`},
		{"length switch channel range", `class User {} class Holder { private user: User; constructor(values: GoChannel<int>) { switch (len(values)) { case 0 { this.user = new User(); } default { for (const value of values) { this.user = new User(); } } } } }`, `every constructor path`},
		{"shadowed len switch", `function len(values: int[]): int { return 1; } class User {} class Holder { private user: User; constructor(values: int[]) { switch (len(values)) { case 0 { this.user = new User(); } default { for (const value of values) { this.user = new User(); } } } } }`, `every constructor path`},
		{"effectful length switch case", `function empty(values: int[]): int { clear(values); return -1; } class User {} class Holder { private user: User; constructor(values: int[]) { switch (len(values)) { case empty(values) { this.user = new User(); } case 0 { this.user = new User(); } default { for (const value of values) { this.user = new User(); } } } } }`, `every constructor path`},
		{"effectful case before positive length case", `function empty(values: int[]): int { clear(values); return -1; } class User {} class Holder { private user: User; constructor(values: int[]) { switch (len(values)) { case empty(values) { this.user = new User(); } case 1 { for (const value of values) { this.user = new User(); } } default { this.user = new User(); } } } }`, `every constructor path`},
		{"length switch fallthrough bypasses positive case", `class User {} class Holder { private user: User; constructor(values: int[]) { switch (len(values)) { case 0 { fallthrough; } case 1 { for (const value of values) { this.user = new User(); } } default { this.user = new User(); } } } }`, `every constructor path`},
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
