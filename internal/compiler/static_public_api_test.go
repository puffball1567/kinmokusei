package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticMethodPublicGoAPIMatchesIndependentPackage(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "static_api.otm")
	input := `
class Meter {
  constructor(public value: int) {}

  private static function magnitude(value: int): int {
    if (value < 0) { return -value; }
    return value;
  }

  public static function create(value: int): Meter {
    return new Meter(Meter.magnitude(value));
  }

  public static function sum(left: int, right: int): Result<Meter> {
    return ok(new Meter(left + right));
  }

  public function current(): int { return this.value; }
}

function BuildMeter(value: int): Meter { return Meter.create(value); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "staticapi")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, expected := range []string{
		"func MeterCreate(value int) *Meter",
		"func MeterSum(left int, right int) (*Meter, error)",
		"func __ontamaStaticMetermagnitude(value int) int",
		"return MeterCreate(value)",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated public static API does not contain %q:\n%s", expected, generated)
		}
	}
	for _, forbidden := range []string{
		"func Meter_Create", "func Meter_Sum", "func Meter_magnitude", "func MeterMagnitude",
		temporary, source, "github.com/puffball1567/onsentamago",
	} {
		if strings.Contains(string(generated), forbidden) {
			t.Errorf("publishable generated Go contains forbidden public/static artifact %q:\n%s", forbidden, generated)
		}
	}

	referenceSource := `package reference

type Meter struct { Value int }
func NewMeter(value int) *Meter { return &Meter{Value: value} }
func magnitude(value int) int { if value < 0 { return -value }; return value }
func MeterCreate(value int) *Meter { return NewMeter(magnitude(value)) }
func MeterSum(left, right int) (*Meter, error) { return NewMeter(left + right), nil }
func (value *Meter) Current() int { return value.Value }
func BuildMeter(value int) *Meter { return MeterCreate(value) }
`
	testSource := `package staticapi_test

import (
  "testing"
  staticapi "example.com/staticapi"
  reference "example.com/staticapi/reference"
)

func TestPublicStaticAPI(t *testing.T) {
  for _, input := range []int{-9, -1, 0, 1, 12} {
    generated := staticapi.MeterCreate(input)
    expected := reference.MeterCreate(input)
    if generated.Value != expected.Value || generated.Current() != expected.Current() {
      t.Errorf("MeterCreate(%d) = (%d, %d), Go = (%d, %d)", input, generated.Value, generated.Current(), expected.Value, expected.Current())
    }
    generated = staticapi.BuildMeter(input)
    expected = reference.BuildMeter(input)
    if generated.Value != expected.Value || generated.Current() != expected.Current() {
      t.Errorf("BuildMeter(%d) = (%d, %d), Go = (%d, %d)", input, generated.Value, generated.Current(), expected.Value, expected.Current())
    }
  }
  for _, pair := range [][2]int{{0, 0}, {1, 2}, {-4, 9}, {12, -7}} {
    generated, generatedErr := staticapi.MeterSum(pair[0], pair[1])
    expected, expectedErr := reference.MeterSum(pair[0], pair[1])
    if generatedErr != nil || expectedErr != nil || generated.Value != expected.Value || generated.Current() != expected.Current() {
      t.Errorf("MeterSum(%v) = (%v, %v), Go = (%v, %v)", pair, generated, generatedErr, expected, expectedErr)
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "example.com/staticapi", generated, referenceSource, testSource)
}
