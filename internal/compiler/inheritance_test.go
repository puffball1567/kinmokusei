package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSingleInheritanceVirtualDispatchMatchesIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "inheritance.otm")
	input := `
interface Speaker { function speak(): string; }

class Animal implements Speaker {
  constructor(public name: string) {}
  public virtual function speak(): string { return "animal"; }
  public function describe(): string { return this.name + ":" + this.speak(); }
}

class Dog extends Animal {
  constructor(name: string, public sound: string) { super(name); }
  public override function speak(): string { return super.speak() + "/" + this.sound; }
  public virtual function role(): string { return "dog"; }
  public function roleText(): string { return "role:" + this.role(); }
  public function rename(name: string): void { this.name = name; }
}

final class GuideDog extends Dog {
  constructor(name: string) { super(name, "woof"); }
  public override function speak(): string { return super.speak() + "/guide"; }
  public final override function role(): string { return "guide"; }
}

function inheritedBehavior(): string {
  const dog = new Dog("Mugi", "bow");
  dog.rename("Komugi");
  return dog.describe() + ";" + dog.speak() + ";" + dog.roleText();
}
class PhaseBase {
  public baseSeen: string;
  public baseDowncastSeen: boolean;
  constructor() {
    const [child, ok] = this as? PhaseChild;
    this.baseDowncastSeen = ok;
    this.baseSeen = this.phase();
  }
  public virtual function phase(): string { return "base"; }
  public function seesChild(): boolean {
    const [child, ok] = this as? PhaseChild;
    return ok;
  }
}
class PhaseChild extends PhaseBase {
  public childSeen: string;
  public childDowncastSeen: boolean;
  constructor() {
    super();
    this.childDowncastSeen = this.seesChild();
    this.childSeen = this.phase();
  }
  public override function phase(): string { return "child"; }
}
function constructorPhase(): string {
  const value = new PhaseChild();
  return value.baseSeen + "/" + value.childSeen;
}
function constructorDowncastPhase(): boolean {
  const value = new PhaseChild();
  return !value.baseDowncastSeen && value.childDowncastSeen;
}
function multiLevelBehavior(): string {
  const dog = new GuideDog("Hana");
  return dog.describe() + ";" + dog.speak() + ";" + dog.roleText();
}
function inheritedInterface(): string {
  const speaker: Speaker = new GuideDog("Sora");
  return speaker.speak();
}
function describeAnimal(value: Animal): string { return value.describe(); }
function argumentUpcast(): string { return describeAnimal(new GuideDog("Yuki")); }
function returnUpcast(): Animal { return new GuideDog("Kuu"); }
function returnUpcastBehavior(): string { return returnUpcast().describe(); }
function identityAfterUpcast(): boolean {
  const dog = new GuideDog("Rin");
  const first: Animal = dog;
  const second: Animal = dog;
  return first === second;
}
function nilAfterUpcast(): boolean {
  const dog: GuideDog = nil;
  const animal: Animal = dog;
  return animal === nil;
}
function nullableAfterUpcast(dog: GuideDog | null): boolean {
  const animal: Animal | null = dog;
  return animal === null;
}
function animalValue(): Animal { return new Animal("plain"); }
function dogValue(): Animal { return new Dog("Pochi", "wan"); }
function guideValue(): Animal { return new GuideDog("Nana"); }
function nilAnimalValue(): Animal { const value: Animal = nil; return value; }
function checkedGuide(value: Animal): string {
  const [guide, ok] = value as? GuideDog;
  if (!ok) { return "none"; }
  return guide.speak();
}
function checkedDog(value: Animal): string {
  const [dog, ok] = value as? Dog;
  if (!ok) { return "none"; }
  return dog.roleText();
}
function forcedGuide(value: Animal): string {
  const guide = value as! GuideDog;
  return guide.roleText();
}
function downcastIdentity(): boolean {
  const guide = new GuideDog("Momo");
  const animal: Animal = guide;
  const [restored, ok] = animal as? GuideDog;
  return ok && restored === guide;
}
function countedAnimal(count: *int): Animal {
  (*count)++;
  return new GuideDog("Ichi");
}
function downcastEvaluationCount(): int {
  let count = 0;
  const [guide, ok] = countedAnimal(&count) as? GuideDog;
  if (ok) { return count; }
  return -1;
}
class ProtectedBase {
  constructor(protected value: int) {}
  protected virtual function adjust(delta: int): int { return this.value + delta; }
  protected static function label(): string { return "base"; }
  public function run(delta: int): int { return this.adjust(delta); }
  public function labelText(): string { return ProtectedBase.label(); }
}
class ProtectedChild extends ProtectedBase {
  constructor(value: int) { super(value); }
  protected override function adjust(delta: int): int { return super.adjust(delta) * 2; }
  public function inspect(other: ProtectedBase): int { return this.adjust(1) + other.value; }
}
function protectedBehavior(): int {
  const child = new ProtectedChild(3);
  const base: ProtectedBase = child;
  return base.run(1) + child.inspect(base);
}
function protectedLabel(): string { return new ProtectedChild(3).labelText(); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "inheritance")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, expected := range []string{
		"type Dog struct", "Animal", "__ontamaAnimalSelf", "__ontamaInitAnimal", "__ontamaInitDog", "this.Animal.__ontamaAnimalSpeak()", "__ontamaUpcastGuideDogToAnimal",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

type speaker interface { speak() string }
type animal struct { self speaker; root any; name string }
func initAnimal(value *animal, self speaker, root any, name string) { value.self = self; value.root = root; value.name = name }
func newAnimal(name string) *animal { value := &animal{}; initAnimal(value, value, value, name); return value }
func (value *animal) speak() string { return "animal" }
func (value *animal) describe() string { return value.name + ":" + value.self.speak() }
type dogRole interface { role() string }
type dog struct { animal; sound string; roleSelf dogRole }
func newDog(name, sound string) *dog { value := &dog{sound: sound}; initAnimal(&value.animal, value, value, name); value.roleSelf = value; return value }
func (value *dog) speak() string { return value.animal.speak() + "/" + value.sound }
func (value *dog) role() string { return "dog" }
func (value *dog) roleText() string { return "role:" + value.roleSelf.role() }
func (value *dog) rename(name string) { value.name = name }
type guideDog struct { dog }
func newGuideDog(name string) *guideDog { value := &guideDog{}; initAnimal(&value.animal, value, value, name); value.sound = "woof"; value.roleSelf = value; return value }
func (value *guideDog) speak() string { return value.dog.speak() + "/guide" }
func (value *guideDog) role() string { return "guide" }
func InheritedBehavior() string { value := newDog("Mugi", "bow"); value.rename("Komugi"); return value.describe() + ";" + value.speak() + ";" + value.roleText() }
func MultiLevelBehavior() string { value := newGuideDog("Hana"); return value.describe() + ";" + value.speak() + ";" + value.roleText() }
func InheritedInterface() string { var value speaker = newGuideDog("Sora"); return value.speak() }
func describeAnimal(value *animal) string { return value.describe() }
func ArgumentUpcast() string { return describeAnimal(&newGuideDog("Yuki").animal) }
func ReturnUpcast() *animal { return &newGuideDog("Kuu").animal }
func ReturnUpcastBehavior() string { return ReturnUpcast().describe() }
func IdentityAfterUpcast() bool { value := newGuideDog("Rin"); first := &value.animal; second := &value.animal; return first == second }
func NilAfterUpcast() bool { var value *guideDog; var animalValue *animal; if value != nil { animalValue = &value.animal }; return animalValue == nil }
func NullableAfterUpcast(value *guideDog) bool { var animalValue *animal; if value != nil { animalValue = &value.animal }; return animalValue == nil }
func NullablePresent() bool { return NullableAfterUpcast(newGuideDog("Nagi")) }
type phaseContract interface { phase() string }
type phaseBase struct { root any; self phaseContract; baseSeen string; baseDowncastSeen bool }
func initPhaseBase(value *phaseBase) { value.root = value; value.self = value; _, value.baseDowncastSeen = value.root.(*phaseChild); value.baseSeen = value.self.phase() }
func (value *phaseBase) phase() string { return "base" }
func (value *phaseBase) seesChild() bool { _, ok := value.root.(*phaseChild); return ok }
type phaseChild struct { phaseBase; childSeen string; childDowncastSeen bool }
func newPhaseChild() *phaseChild { value := &phaseChild{}; initPhaseBase(&value.phaseBase); value.self = value; value.root = value; value.childDowncastSeen = value.seesChild(); value.childSeen = value.self.phase(); return value }
func (value *phaseChild) phase() string { return "child" }
func ConstructorPhase() string { value := newPhaseChild(); return value.baseSeen + "/" + value.childSeen }
func ConstructorDowncastPhase() bool { value := newPhaseChild(); return !value.baseDowncastSeen && value.childDowncastSeen }
func AnimalValue() *animal { return newAnimal("plain") }
func DogValue() *animal { return &newDog("Pochi", "wan").animal }
func GuideValue() *animal { return &newGuideDog("Nana").animal }
func NilAnimalValue() *animal { return nil }
func CheckedGuide(value *animal) string { if value == nil { return "none" }; guide, ok := value.root.(*guideDog); if !ok { return "none" }; return guide.speak() }
func CheckedDog(value *animal) string {
  if value == nil { return "none" }
  switch root := value.root.(type) { case *dog: return root.roleText(); case *guideDog: return root.roleText(); default: return "none" }
}
func ForcedGuide(value *animal) string { return value.root.(*guideDog).roleText() }
func DowncastIdentity() bool { guide := newGuideDog("Momo"); animalValue := &guide.animal; restored, ok := animalValue.root.(*guideDog); return ok && restored == guide }
func DowncastEvaluationCount() int { count := 0; makeValue := func() *animal { count++; return GuideValue() }; _, ok := makeValue().root.(*guideDog); if ok { return count }; return -1 }
type protectedContract interface { adjust(int) int }
type protectedBase struct { self protectedContract; value int }
func initProtectedBase(value *protectedBase, self protectedContract, initial int) { value.self = self; value.value = initial }
func (value *protectedBase) adjust(delta int) int { return value.value + delta }
func protectedBaseLabel() string { return "base" }
func (value *protectedBase) run(delta int) int { return value.self.adjust(delta) }
func (value *protectedBase) labelText() string { return protectedBaseLabel() }
type protectedChild struct { protectedBase }
func newProtectedChild(initial int) *protectedChild { value := &protectedChild{}; initProtectedBase(&value.protectedBase, value, initial); return value }
func (value *protectedChild) adjust(delta int) int { return value.protectedBase.adjust(delta) * 2 }
func (value *protectedChild) inspect(other *protectedBase) int { return value.self.adjust(1) + other.value }
func ProtectedBehavior() int { child := newProtectedChild(3); base := &child.protectedBase; return base.run(1) + child.inspect(base) }
func ProtectedLabel() string { return newProtectedChild(3).labelText() }
`
	testSource := `package inheritance
import (
  "testing"
  reference "inheritance.test/reference"
)
func TestInheritanceRuntimeContract(t *testing.T) {
  if got, want := inheritedBehavior(), reference.InheritedBehavior(); got != want { t.Errorf("inherited = %q, Go = %q", got, want) }
  if got, want := multiLevelBehavior(), reference.MultiLevelBehavior(); got != want { t.Errorf("multi-level = %q, Go = %q", got, want) }
  if got, want := inheritedInterface(), reference.InheritedInterface(); got != want { t.Errorf("interface = %q, Go = %q", got, want) }
  if got, want := argumentUpcast(), reference.ArgumentUpcast(); got != want { t.Errorf("argument upcast = %q, Go = %q", got, want) }
  if got, want := returnUpcastBehavior(), reference.ReturnUpcastBehavior(); got != want { t.Errorf("return upcast = %q, Go = %q", got, want) }
  if got, want := identityAfterUpcast(), reference.IdentityAfterUpcast(); got != want { t.Errorf("identity = %v, Go = %v", got, want) }
  if got, want := nilAfterUpcast(), reference.NilAfterUpcast(); got != want { t.Errorf("nil = %v, Go = %v", got, want) }
  if got, want := nullableAfterUpcast(nil), reference.NullableAfterUpcast(nil); got != want { t.Errorf("nullable nil = %v, Go = %v", got, want) }
  if got, want := nullableAfterUpcast(NewGuideDog("Nagi")), reference.NullablePresent(); got != want { t.Errorf("nullable present = %v, Go = %v", got, want) }
  if got, want := constructorPhase(), reference.ConstructorPhase(); got != want { t.Errorf("constructor phase = %q, Go = %q", got, want) }
  if got, want := constructorDowncastPhase(), reference.ConstructorDowncastPhase(); got != want || !got { t.Errorf("constructor downcast phase = %v, Go = %v", got, want) }
  for _, test := range []struct { name string; got, want string }{
    {"guide/guide", checkedGuide(guideValue()), reference.CheckedGuide(reference.GuideValue())},
    {"dog/guide", checkedGuide(dogValue()), reference.CheckedGuide(reference.DogValue())},
    {"animal/guide", checkedGuide(animalValue()), reference.CheckedGuide(reference.AnimalValue())},
    {"nil/guide", checkedGuide(nilAnimalValue()), reference.CheckedGuide(reference.NilAnimalValue())},
    {"guide/intermediate dog", checkedDog(guideValue()), reference.CheckedDog(reference.GuideValue())},
    {"dog/intermediate dog", checkedDog(dogValue()), reference.CheckedDog(reference.DogValue())},
  } { if test.got != test.want { t.Errorf("%s = %q, Go = %q", test.name, test.got, test.want) } }
  if got, want := forcedGuide(guideValue()), reference.ForcedGuide(reference.GuideValue()); got != want { t.Errorf("forced = %q, Go = %q", got, want) }
  if got, want := didPanic(func() { forcedGuide(dogValue()) }), didPanic(func() { reference.ForcedGuide(reference.DogValue()) }); got != want || !got { t.Errorf("forced failure panic = %v, Go = %v", got, want) }
  if got, want := didPanic(func() { forcedGuide(nilAnimalValue()) }), didPanic(func() { reference.ForcedGuide(reference.NilAnimalValue()) }); got != want || !got { t.Errorf("forced nil panic = %v, Go = %v", got, want) }
  if got, want := downcastIdentity(), reference.DowncastIdentity(); got != want { t.Errorf("downcast identity = %v, Go = %v", got, want) }
  if got, want := downcastEvaluationCount(), reference.DowncastEvaluationCount(); got != want { t.Errorf("downcast evaluation = %d, Go = %d", got, want) }
  if got, want := protectedBehavior(), reference.ProtectedBehavior(); got != want { t.Errorf("protected behavior = %d, Go = %d", got, want) }
  if got, want := protectedLabel(), reference.ProtectedLabel(); got != want { t.Errorf("protected label = %q, Go = %q", got, want) }
}
func didPanic(call func()) (panicked bool) {
  defer func() { panicked = recover() != nil }()
  call()
  return false
}
`
	runGeneratedGoDifferentialTest(t, temporary, "inheritance.test", generated, referenceSource, testSource)
}
