package sema

import (
	"strings"
	"testing"
)

func TestNativeVariadicDeclarationsAndCalls(t *testing.T) {
	diagnostics := checkSource(t, `
function sum(prefix: int, ...values: int[]): int {
  let total = prefix;
  for (const value of values) { total += value; }
  return total;
}
function last<T>(fallback: T, ...values: T[]): T {
  if (len(values) === 0) { return fallback; }
  return values[len(values) - 1];
}
interface Joiner { function join(...parts: string[]): string; }
class Words implements Joiner {
  public function join(...parts: string[]): string { return parts[0]; }
}
class Batch {
  constructor(public ...values: int[]) {}
  public function size(): int { return len(this.values); }
}
struct Counter {
  public base: int;
  public function total(...values: int[]): int { return sum(this.base, values...); }
}
type Scores = distinct int[];
public function total(this: Scores, ...values: int[]): int { return sum(len(this), values...); }
function exercise(values: int[]): int {
  const rest: (...items: int[]) => int = (...items: int[]): int => sum(0, items...);
  const counter = Counter{ base: 10 };
  const scores = Scores(values);
  const batch = new Batch(values...);
  const empty = sum(1);
  const individual = sum(1, 2, 3);
  const spread = sum(1, values...);
  return empty + individual + spread + last(0, values...) + rest(4, 5) + counter.total(6, 7) + scores.total(8) + batch.size();
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestNativeVariadicFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"too few fixed arguments", `function sum(prefix: int, ...values: int[]): int { return prefix; } function bad(): int { return sum(); }`, "expects at least 1 arguments"},
		{"individual element mismatch", `function sum(...values: int[]): int { return 0; } function bad(): int { return sum(1, "x"); }`, "cannot use string as int"},
		{"spread element mismatch", `function sum(...values: int[]): int { return 0; } function bad(values: string[]): int { return sum(values...); }`, "cannot use string[] as int[]"},
		{"spread missing fixed argument", `function sum(prefix: int, ...values: int[]): int { return prefix; } function bad(values: int[]): int { return sum(values...); }`, "expects 2 arguments"},
		{"spread non-variadic constructor", `class Value { constructor(value: int) {} } function bad(values: int[]): Value { return new Value(values...); }`, "is not variadic"},
		{"constructor element mismatch", `class Value { constructor(...values: int[]) {} } function bad(): Value { return new Value("x"); }`, "cannot use string as int"},
		{"interface variadic mismatch", `interface Values { function add(...values: int[]): void; } class Bad implements Values { public function add(values: int[]): void {} }`, "incompatible signature"},
		{"function type variadic mismatch", `function plain(values: int[]): int { return len(values); } function bad(): void { const rest: (...values: int[]) => int = plain; }`, "cannot use"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
