package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassHierarchyPublicGoAPIMatchesIndependentPackage(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "hierarchy.otm")
	input := `
class Animal {
  constructor(public name: string) {}
  public virtual function speak(): string { return "animal"; }
  public virtual function touch(count: *int): void { (*count)++; }
  public function describe(): string { return this.name + ":" + this.speak(); }
}
class Dog extends Animal {
  constructor(name: string) { super(name); }
  public override function speak(): string { return super.speak() + "/dog"; }
  public override function touch(count: *int): void { (*count) += 2; }
}
final class GuideDog extends Dog {
  constructor(name: string) { super(name); }
  public final override function speak(): string { return super.speak() + "/guide"; }
  public final override function touch(count: *int): void { (*count) += 10; }
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "hierarchy")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, expected := range []string{
		"func UpcastGuideDogToAnimal", "func DowncastAnimalToGuideDog", "func MustDowncastAnimalToGuideDog",
		"func (this *Animal) Speak() string", "return this.__ontamaAnimalSelf.__ontamaAnimalSpeak()",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated public Go API does not contain %q:\n%s", expected, generated)
		}
	}
	for _, forbidden := range []string{temporary, source, "ontama.local/ontama"} {
		if strings.Contains(string(generated), forbidden) {
			t.Errorf("publishable generated Go contains local/compiler-only path %q:\n%s", forbidden, generated)
		}
	}

	referenceSource := `package reference

type animalBehavior interface { speakImpl() string; touchImpl(*int) }
type Animal struct { self animalBehavior; Name string }
func initAnimal(value *Animal, self animalBehavior, name string) { value.self = self; value.Name = name }
func NewAnimal(name string) *Animal { value := &Animal{}; initAnimal(value, value, name); return value }
func (value *Animal) speakImpl() string { return "animal" }
func (value *Animal) touchImpl(count *int) { (*count)++ }
func (value *Animal) Speak() string { if value == nil || value.self == nil { return value.speakImpl() }; return value.self.speakImpl() }
func (value *Animal) Touch(count *int) { if value == nil || value.self == nil { value.touchImpl(count); return }; value.self.touchImpl(count) }
func (value *Animal) Describe() string { return value.Name + ":" + value.self.speakImpl() }
type Dog struct { Animal }
func NewDog(name string) *Dog { value := &Dog{}; initAnimal(&value.Animal, value, name); return value }
func (value *Dog) speakImpl() string { return value.Animal.speakImpl() + "/dog" }
func (value *Dog) touchImpl(count *int) { (*count) += 2 }
type GuideDog struct { Dog }
func NewGuideDog(name string) *GuideDog { value := &GuideDog{}; initAnimal(&value.Animal, value, name); return value }
func (value *GuideDog) speakImpl() string { return value.Dog.speakImpl() + "/guide" }
func (value *GuideDog) touchImpl(count *int) { (*count) += 10 }
func UpcastDogToAnimal(value *Dog) *Animal { if value == nil { return nil }; return &value.Animal }
func UpcastGuideDogToDog(value *GuideDog) *Dog { if value == nil { return nil }; return &value.Dog }
func UpcastGuideDogToAnimal(value *GuideDog) *Animal { if value == nil { return nil }; return &value.Animal }
func DowncastAnimalToDog(value *Animal) (*Dog, bool) {
  if value == nil { return nil, false }
  switch result := value.self.(type) { case *Dog: return result, true; case *GuideDog: return &result.Dog, true; default: return nil, false }
}
func DowncastAnimalToGuideDog(value *Animal) (*GuideDog, bool) { if value == nil { return nil, false }; result, ok := value.self.(*GuideDog); return result, ok }
func MustDowncastAnimalToGuideDog(value *Animal) *GuideDog { result, ok := DowncastAnimalToGuideDog(value); if !ok { panic("cannot downcast") }; return result }
`
	testSource := `package hierarchy_test
import (
  "testing"
  hierarchy "example.com/hierarchy"
  reference "example.com/hierarchy/reference"
)
func TestPublicHierarchyAPI(t *testing.T) {
  generatedGuide, goGuide := hierarchy.NewGuideDog("Nana"), reference.NewGuideDog("Nana")
  if got, want := generatedGuide.Speak(), goGuide.Speak(); got != want { t.Errorf("derived direct dispatch = %q, Go = %q", got, want) }
  generatedAnimal, goAnimal := hierarchy.UpcastGuideDogToAnimal(generatedGuide), reference.UpcastGuideDogToAnimal(goGuide)
  if got, want := generatedAnimal.Speak(), goAnimal.Speak(); got != want { t.Errorf("base direct dispatch = %q, Go = %q", got, want) }
  if got, want := generatedAnimal.Describe(), goAnimal.Describe(); got != want { t.Errorf("base implementation dispatch = %q, Go = %q", got, want) }
  generatedMethod, goMethod := generatedAnimal.Speak, goAnimal.Speak
  if got, want := generatedMethod(), goMethod(); got != want { t.Errorf("bound public method = %q, Go = %q", got, want) }
  generatedCount, goCount := 0, 0
  generatedAnimal.Touch(&generatedCount); goAnimal.Touch(&goCount)
  if generatedCount != goCount { t.Errorf("void virtual dispatch = %d, Go = %d", generatedCount, goCount) }
  generatedDog, generatedDogOK := hierarchy.DowncastAnimalToDog(generatedAnimal)
  goDog, goDogOK := reference.DowncastAnimalToDog(goAnimal)
  if generatedDogOK != goDogOK || generatedDog != hierarchy.UpcastGuideDogToDog(generatedGuide) || goDog != reference.UpcastGuideDogToDog(goGuide) { t.Errorf("intermediate downcast identity mismatch") }
  if got, want := generatedDog.Speak(), goDog.Speak(); got != want { t.Errorf("intermediate direct dispatch = %q, Go = %q", got, want) }
  generatedRestored, generatedOK := hierarchy.DowncastAnimalToGuideDog(generatedAnimal)
  goRestored, goOK := reference.DowncastAnimalToGuideDog(goAnimal)
  if generatedOK != goOK || generatedRestored != generatedGuide || goRestored != goGuide { t.Errorf("exact downcast identity mismatch") }
  if got, want := hierarchy.MustDowncastAnimalToGuideDog(generatedAnimal).Speak(), reference.MustDowncastAnimalToGuideDog(goAnimal).Speak(); got != want { t.Errorf("must downcast = %q, Go = %q", got, want) }
  generatedPlain, goPlain := hierarchy.NewAnimal("plain"), reference.NewAnimal("plain")
  _, generatedOK = hierarchy.DowncastAnimalToGuideDog(generatedPlain); _, goOK = reference.DowncastAnimalToGuideDog(goPlain)
  if generatedOK != goOK || generatedOK { t.Errorf("failed downcast = %v, Go = %v", generatedOK, goOK) }
  if got, want := didPanic(func() { hierarchy.MustDowncastAnimalToGuideDog(generatedPlain) }), didPanic(func() { reference.MustDowncastAnimalToGuideDog(goPlain) }); got != want || !got { t.Errorf("must failure panic = %v, Go = %v", got, want) }
  if hierarchy.UpcastGuideDogToAnimal(nil) != nil || reference.UpcastGuideDogToAnimal(nil) != nil { t.Error("nil upcast was not preserved") }
  generatedZero, goZero := new(hierarchy.Animal), new(reference.Animal)
  if got, want := generatedZero.Speak(), goZero.Speak(); got != want { t.Errorf("zero-value fallback = %q, Go = %q", got, want) }
  var generatedNil *hierarchy.Animal; var goNil *reference.Animal
  if got, want := generatedNil.Speak(), goNil.Speak(); got != want { t.Errorf("nil receiver fallback = %q, Go = %q", got, want) }
}
func didPanic(call func()) (panicked bool) { defer func() { panicked = recover() != nil }(); call(); return false }
`
	runGeneratedGoDifferentialTest(t, temporary, "example.com/hierarchy", generated, referenceSource, testSource)
}
