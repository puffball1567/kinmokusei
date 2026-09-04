package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstructorBoundConstantProofsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "constructor_bound_constants.km")
	input := `
const globalEnabled = 2 * 3 === 6;
const globalPrefix = "温";
const globalText = globalPrefix + "泉";

class User { constructor(public name: string) {} }
class LocalBooleanHolder {
  private user: User;
  private visits: int[];
  constructor() {
    const compared = 1 + 2 === 3;
    const enabled = !false && compared;
    while (enabled) {
      this.user = new User("local-boolean");
      this.visits = [1, 2];
      break;
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.visits); }
}
class ForInitializerHolder {
  private user: User;
  constructor() {
    for (const enabled = "a" < "b"; enabled; ) {
      this.user = new User("for-initializer");
      break;
    }
  }
  public function name(): string { return this.user.name; }
}
class BoundMakeSliceHolder {
  private user: User;
  private values: int[];
  constructor() {
    this.values = [];
    const count = 1 + 1;
    const source = makeSlice[int](count);
    for (const value of source) {
      this.user = new User("bound-make-slice");
      this.values = append(this.values, value);
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.values); }
}
class BoundAppendHolder {
  private user: User;
  private values: int[];
  constructor() {
    this.values = [];
    const source = append(makeSlice[int](0), 7);
    for (const value of source) {
      this.user = new User("bound-append");
      this.values = append(this.values, value);
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.values); }
}
class BoundStringHolder {
  private user: User;
  private runes: int32[];
  constructor() {
    this.runes = [];
    const prefix = "on";
    const text = prefix + "sen";
    for (const rune of text) {
      this.user = new User("bound-string");
      this.runes = append(this.runes, rune);
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.runes); }
}
class GlobalHolder {
  private booleanUser: User;
  private stringUser: User;
  private runes: int32[];
  constructor() {
    this.runes = [];
    while (globalEnabled) {
      this.booleanUser = new User("global-boolean");
      break;
    }
    for (const rune of globalText) {
      this.stringUser = new User("global-string");
      this.runes = append(this.runes, rune);
    }
  }
  public function names(): string { return this.booleanUser.name + ":" + this.stringUser.name; }
  public function count(): int { return len(this.runes); }
}

function localBooleanName(): string { return new LocalBooleanHolder().name(); }
function localBooleanCount(): int { return new LocalBooleanHolder().count(); }
function forInitializerName(): string { return new ForInitializerHolder().name(); }
function boundMakeSliceName(): string { return new BoundMakeSliceHolder().name(); }
function boundMakeSliceCount(): int { return new BoundMakeSliceHolder().count(); }
function boundAppendName(): string { return new BoundAppendHolder().name(); }
function boundAppendCount(): int { return new BoundAppendHolder().count(); }
function boundStringName(): string { return new BoundStringHolder().name(); }
function boundStringCount(): int { return new BoundStringHolder().count(); }
function globalNames(): string { return new GlobalHolder().names(); }
function globalCount(): int { return new GlobalHolder().count(); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "constructorboundconstants")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{
		"globalEnabled = 2*3 == 6",
		"var enabled = !false && compared",
		"for enabled",
		"var source = make([]int, count)",
		`var globalText = globalPrefix + "泉"`,
	} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated Go does not preserve %q:\n%s", want, generated)
		}
	}

	referenceSource := `package reference

const globalEnabled = 2*3 == 6
const globalPrefix = "温"
const globalText = globalPrefix + "泉"

func LocalBoolean() (string, int) {
  name, count := "", 0
  const compared = 1+2 == 3
  const enabled = !false && compared
  for enabled { name = "local-boolean"; count = 2; break }
  return name, count
}
func ForInitializer() string {
  name := ""
  for enabled := "a" < "b"; enabled; { name = "for-initializer"; break }
  return name
}
func BoundMakeSlice() (string, int) {
  name, visits := "", 0
  const count = 1+1
  source := make([]int, count)
  for range source { name = "bound-make-slice"; visits++ }
  return name, visits
}
func BoundAppend() (string, int) {
  name, visits := "", 0
  source := append(make([]int, 0), 7)
  for range source { name = "bound-append"; visits++ }
  return name, visits
}
func BoundString() (string, int) {
  name, visits := "", 0
  const prefix = "on"
  const text = prefix + "sen"
  for range text { name = "bound-string"; visits++ }
  return name, visits
}
func Global() (string, int) {
  booleanName, stringName, visits := "", "", 0
  for globalEnabled { booleanName = "global-boolean"; break }
  for range globalText { stringName = "global-string"; visits++ }
  return booleanName + ":" + stringName, visits
}
`
	testSource := `package constructorboundconstants

import (
  "testing"
  reference "constructor-bound-constants.test/reference"
)

func TestConstructorBoundConstants(t *testing.T) {
  wantName, wantCount := reference.LocalBoolean()
  if got := localBooleanName(); got != wantName { t.Fatalf("localBooleanName = %q, equivalent Go = %q", got, wantName) }
  if got := localBooleanCount(); got != wantCount { t.Fatalf("localBooleanCount = %d, equivalent Go = %d", got, wantCount) }
  if got, want := forInitializerName(), reference.ForInitializer(); got != want { t.Fatalf("forInitializerName = %q, equivalent Go = %q", got, want) }
  wantName, wantCount = reference.BoundMakeSlice()
  if got := boundMakeSliceName(); got != wantName { t.Fatalf("boundMakeSliceName = %q, equivalent Go = %q", got, wantName) }
  if got := boundMakeSliceCount(); got != wantCount { t.Fatalf("boundMakeSliceCount = %d, equivalent Go = %d", got, wantCount) }
  wantName, wantCount = reference.BoundAppend()
  if got := boundAppendName(); got != wantName { t.Fatalf("boundAppendName = %q, equivalent Go = %q", got, wantName) }
  if got := boundAppendCount(); got != wantCount { t.Fatalf("boundAppendCount = %d, equivalent Go = %d", got, wantCount) }
  wantName, wantCount = reference.BoundString()
  if got := boundStringName(); got != wantName { t.Fatalf("boundStringName = %q, equivalent Go = %q", got, wantName) }
  if got := boundStringCount(); got != wantCount { t.Fatalf("boundStringCount = %d, equivalent Go = %d", got, wantCount) }
  wantName, wantCount = reference.Global()
  if got := globalNames(); got != wantName { t.Fatalf("globalNames = %q, equivalent Go = %q", got, wantName) }
  if got := globalCount(); got != wantCount { t.Fatalf("globalCount = %d, equivalent Go = %d", got, wantCount) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "constructor-bound-constants.test", generated, referenceSource, testSource)
}
