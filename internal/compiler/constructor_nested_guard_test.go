package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConstructorNestedLengthGuardProofsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "constructor_nested_guard.km")
	input := `
class User { constructor(public name: string) {} }

class NestedHolder {
  private user: User;
  private visits: int[];
  constructor(values: int[], enabled: boolean) {
    this.visits = [];
    if (len(values) > 0) {
      if (enabled) {
        for (const value of values) {
          this.user = new User("enabled");
          this.visits = append(this.visits, value);
        }
      } else {
        for (const value of values) {
          this.user = new User("disabled");
          this.visits = append(this.visits, value);
        }
      }
    } else {
      this.user = new User("empty");
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.visits); }
}

class NestedCollectionsHolder {
  private user: User;
  private visits: int[];
  constructor(left: int[], right: int[]) {
    this.visits = [];
    if (len(left) > 0) {
      if (len(right) > 0) {
        for (const value of right) {
          this.user = new User("right");
          this.visits = append(this.visits, value);
        }
      } else {
        for (const value of left) {
          this.user = new User("left");
          this.visits = append(this.visits, value);
        }
      }
    } else {
      this.user = new User("empty-left");
    }
  }
  public function name(): string { return this.user.name; }
  public function count(): int { return len(this.visits); }
}

function nestedName(values: int[], enabled: boolean): string { return new NestedHolder(values, enabled).name(); }
function nestedCount(values: int[], enabled: boolean): int { return new NestedHolder(values, enabled).count(); }
function collectionsName(left: int[], right: int[]): string { return new NestedCollectionsHolder(left, right).name(); }
function collectionsCount(left: int[], right: int[]): int { return new NestedCollectionsHolder(left, right).count(); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "constructornestedguard")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}

	referenceSource := `package reference
func Nested(values []int, enabled bool) (string, int) {
  name := "empty"
  visits := 0
  if len(values) > 0 {
    if enabled {
      for range values { name = "enabled"; visits++ }
    } else {
      for range values { name = "disabled"; visits++ }
    }
  }
  return name, visits
}

func Collections(left, right []int) (string, int) {
  name := "empty-left"
  visits := 0
  if len(left) > 0 {
    if len(right) > 0 {
      for range right { name = "right"; visits++ }
    } else {
      for range left { name = "left"; visits++ }
    }
  }
  return name, visits
}
`
	testSource := `package constructornestedguard
import (
  "testing"
  reference "constructor-nested-guard.test/reference"
)

func TestConstructorNestedGuards(t *testing.T) {
  valuesMatrix := [][]int{nil, {}, {1}, {2, 3}, {-1, 0, 4}}
  for _, values := range valuesMatrix {
    for _, enabled := range []bool{false, true} {
      wantName, wantCount := reference.Nested(values, enabled)
      if got := nestedName(values, enabled); got != wantName {
        t.Errorf("nestedName(%v, %v) = %q, equivalent Go = %q", values, enabled, got, wantName)
      }
      if got := nestedCount(values, enabled); got != wantCount {
        t.Errorf("nestedCount(%v, %v) = %d, equivalent Go = %d", values, enabled, got, wantCount)
      }
    }
  }
  for _, left := range valuesMatrix {
    for _, right := range valuesMatrix {
      wantName, wantCount := reference.Collections(left, right)
      if got := collectionsName(left, right); got != wantName {
        t.Errorf("collectionsName(%v, %v) = %q, equivalent Go = %q", left, right, got, wantName)
      }
      if got := collectionsCount(left, right); got != wantCount {
        t.Errorf("collectionsCount(%v, %v) = %d, equivalent Go = %d", left, right, got, wantCount)
      }
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "constructor-nested-guard.test", generated, referenceSource, testSource)
}
