package compiler

import (
	"path/filepath"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/codegen"
)

const benchmarkProgram = `
constraint Numeric = ~int | ~int64 | ~float64;
constraint Text = ~string;
function first<T extends Numeric>(values: T[]): T { return values[0]; }
interface Named { function name(): string; }
class Entity<T extends comparable> implements Named {
  constructor(public id: T, private label: string) {}
  public function name(): string { return this.label; }
  public function rename<U extends Text>(value: U): string {
    this.label = string(value);
    return this.label;
  }
}
struct Point {
  public x: int;
  public y: int;
  pointer function move(dx: int, dy: int): void {
    this.x += dx;
    this.y += dy;
  }
}
function execute(values: int[]): int {
  if (len(values) === 0) { return 0; }
  const entity = new Entity<int>(values[0], "entry");
  let point = Point{x: values[0], y: len(values)};
  point.move(1, 2);
  entity.rename("updated");
  return first(values) + point.x + point.y + len(entity.name());
}
`

func benchmarkCheckedProgram(b *testing.B) Result {
	b.Helper()
	path := filepath.Join(b.TempDir(), "benchmark.km")
	result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: benchmarkProgram})
	if err != nil {
		b.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		b.Fatalf("diagnostics = %v", result.Diagnostics)
	}
	return result
}

func BenchmarkKinmokuseiCheck(b *testing.B) {
	path := filepath.Join(b.TempDir(), "benchmark.km")
	overlay := map[string]string{path: benchmarkProgram}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result, err := CheckFilesWithOverlay([]string{path}, overlay)
		if err != nil || len(result.Diagnostics) != 0 {
			b.Fatalf("err=%v diagnostics=%v", err, result.Diagnostics)
		}
	}
}

func BenchmarkKinmokuseiCodegen(b *testing.B) {
	result := benchmarkCheckedProgram(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		generated, err := codegen.Generate(result.Program, "benchmark")
		if err != nil || len(generated) == 0 {
			b.Fatalf("generated=%d bytes err=%v", len(generated), err)
		}
	}
}
