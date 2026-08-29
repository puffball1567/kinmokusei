package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNullableStableMemberFlowCompilesAndMatchesGo(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "nullable_member.otm")
	input := `
class User { constructor(public name: string) {} }
class Profile { constructor(public user: User | null) {} }
class Holder {
  constructor(public user: User | null, public profile: Profile) {}
  public function current(): string {
    if (this.user === null) { return "missing"; }
    return this.user.name;
  }
}
function describe(present: boolean): string {
  let user: User | null = null;
  if (present) { user = new User("onsen"); }
  const holder = new Holder(user, new Profile(user));
  if (holder.user === null) { return "missing"; }
  return holder.user.name;
}
function nested(present: boolean): string {
  let user: User | null = null;
  if (present) { user = new User("nested"); }
  const holder = new Holder(null, new Profile(user));
  if (null === holder.profile.user) { return "missing"; }
  return holder.profile.user.name;
}
function assigned(left: boolean): string {
  const holder = new Holder(null, new Profile(null));
  if (left) { holder.user = new User("left"); }
  else { holder.user = new User("right"); }
  return holder.user.name;
}
function switchAssigned(mode: int): string {
  const holder = new Holder(null, new Profile(null));
  switch (mode) {
    case 0, 1 { holder.user = new User("case"); }
    default { holder.user = new User("default"); }
  }
  return holder.user.name;
}
function aliasRead(present: boolean): string {
  let user: User | null = null;
  if (present) { user = new User("alias"); }
  const holder = new Holder(user, new Profile(null));
  const alias = holder;
  if (alias.user === null) { return "missing"; }
  return alias.user.name;
}
function iterations(present: boolean, rounds: int): string {
  let user: User | null = null;
  if (present) { user = new User("initial"); }
  const holder = new Holder(user, new Profile(null));
  let result = "";
  for (let index = 0; index < rounds; index = index + 1) {
    if (holder.user === null) { holder.user = new User("restored"); }
    result = result + holder.user.name;
    holder.user = null;
  }
  return result;
}
function methodCurrent(present: boolean): string {
  let user: User | null = null;
  if (present) { user = new User("method"); }
  return new Holder(user, new Profile(null)).current();
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "nullablemember")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}

	referenceSource := `package reference
type User struct { name string }
type Profile struct { user *User }
type Holder struct { user *User; profile *Profile }
func newUser(name string) *User { return &User{name: name} }
func newHolder(user *User, profile *Profile) *Holder { return &Holder{user: user, profile: profile} }
func (holder *Holder) current() string { if holder.user == nil { return "missing" }; return holder.user.name }
func maybeUser(present bool, name string) *User { if present { return newUser(name) }; return nil }
func Describe(present bool) string { user := maybeUser(present, "onsen"); holder := newHolder(user, &Profile{user: user}); if holder.user == nil { return "missing" }; return holder.user.name }
func Nested(present bool) string { holder := newHolder(nil, &Profile{user: maybeUser(present, "nested")}); if holder.profile.user == nil { return "missing" }; return holder.profile.user.name }
func Assigned(left bool) string { holder := newHolder(nil, &Profile{}); if left { holder.user = newUser("left") } else { holder.user = newUser("right") }; return holder.user.name }
func SwitchAssigned(mode int) string { holder := newHolder(nil, &Profile{}); switch mode { case 0, 1: holder.user = newUser("case"); default: holder.user = newUser("default") }; return holder.user.name }
func AliasRead(present bool) string { holder := newHolder(maybeUser(present, "alias"), &Profile{}); alias := holder; if alias.user == nil { return "missing" }; return alias.user.name }
func Iterations(present bool, rounds int) string { holder := newHolder(maybeUser(present, "initial"), &Profile{}); result := ""; for index := 0; index < rounds; index++ { if holder.user == nil { holder.user = newUser("restored") }; result += holder.user.name; holder.user = nil }; return result }
func MethodCurrent(present bool) string { return newHolder(maybeUser(present, "method"), &Profile{}).current() }
`
	testSource := `package nullablemember
import (
  "testing"
  reference "nullable-members.test/reference"
)
func TestNullableMemberRuntimeMatrix(t *testing.T) {
  for _, present := range []bool{false, true} {
    if got, want := describe(present), reference.Describe(present); got != want { t.Fatalf("describe(%v) = %q, equivalent Go = %q", present, got, want) }
    if got, want := nested(present), reference.Nested(present); got != want { t.Fatalf("nested(%v) = %q, equivalent Go = %q", present, got, want) }
    if got, want := aliasRead(present), reference.AliasRead(present); got != want { t.Fatalf("aliasRead(%v) = %q, equivalent Go = %q", present, got, want) }
    if got, want := methodCurrent(present), reference.MethodCurrent(present); got != want { t.Fatalf("methodCurrent(%v) = %q, equivalent Go = %q", present, got, want) }
    for _, rounds := range []int{0, 1, 3} {
      if got, want := iterations(present, rounds), reference.Iterations(present, rounds); got != want { t.Fatalf("iterations(%v, %d) = %q, equivalent Go = %q", present, rounds, got, want) }
    }
  }
  for _, left := range []bool{false, true} {
    if got, want := assigned(left), reference.Assigned(left); got != want { t.Fatalf("assigned(%v) = %q, equivalent Go = %q", left, got, want) }
  }
  for _, mode := range []int{-1, 0, 1, 2} {
    if got, want := switchAssigned(mode), reference.SwitchAssigned(mode); got != want { t.Fatalf("switchAssigned(%d) = %q, equivalent Go = %q", mode, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "nullable-members.test", generated, referenceSource, testSource)
}
