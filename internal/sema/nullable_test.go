package sema

import (
	"strings"
	"testing"
)

func TestNullableSemanticAndFlowMatrix(t *testing.T) {
	diagnostics := checkSource(t, `
class User { constructor(public name: string) {} }

function maybe(present: boolean): User | null {
  if (present) { return new User("onsen"); }
  return null;
}
function branch(present: boolean): string {
  const user = maybe(present);
  if (user !== null) { return user.name; }
  return "missing";
}
function guard(present: boolean): string {
  const user: User | null = maybe(present);
  if (user === null) { return "missing"; }
  return user.name;
}
function inverse(present: boolean): string {
  const user = maybe(present);
  if (null === user) { return "missing"; } else { return user.name; }
}
function stableParameter(user: User | null): string {
  if (user !== null) { return user.name; }
  return "missing";
}
function stableMutable(present: boolean): string {
  let user = maybe(present);
  if (user === null) { return "missing"; }
  return user.name;
}
function writeAfterUse(present: boolean): string {
  let user = maybe(present);
  let name = "missing";
  if (user !== null) { name = user.name; }
  user = null;
  return name;
}
function nonNullAssignment(): string {
  let user: User | null = null;
  user = new User("assigned");
  return user.name;
}
function recheckAfterWrite(present: boolean): string {
  let user = maybe(present);
  if (user !== null) { user = null; }
  if (user === null) { return "missing"; }
  return user.name;
}
function joinedAssignment(flag: boolean): string {
  let user: User | null = null;
  if (flag) { user = new User("left"); }
  else { user = new User("right"); }
  return user.name;
}
function loopNarrowing(present: boolean): string {
  let user = maybe(present);
  let name = "missing";
  while (user !== null) {
    name = user.name;
    user = null;
  }
  return name;
}
function switchJoin(flag: boolean): string {
  let user: User | null = null;
  switch (flag) {
    case true { user = new User("case"); }
    default { user = new User("default"); }
  }
  return user.name;
}
function survivingBranch(present: boolean): string {
  const user = maybe(present);
  if (user !== null) {
    const user = new User("shadow");
    return user.name;
  } else { return "missing"; }
}
function nullableResult(present: boolean): Result<User | null> {
  return ok(maybe(present));
}
function maybeValues(present: boolean): int[] | null {
  if (present) { return [1, 2]; }
  return null;
}
function safeLength(present: boolean): int {
  const values = maybeValues(present);
  return len(values);
}
function accessBeforeTakingAddress(present: boolean): string {
  let user = maybe(present);
  let name = "missing";
  if (user !== null) { name = user.name; }
  const pointer = &user;
  *pointer = null;
  return name;
}
function accessBeforeCapturingWrite(present: boolean): string {
  let user = maybe(present);
  let name = "missing";
  if (user !== null) { name = user.name; }
  const clear = (): void => { user = null; };
  clear();
  return name;
}
function shadowedCaptureDoesNotEscape(present: boolean): string {
  let user = maybe(present);
  if (user === null) { return "missing"; }
  const clear = (): void => {
    let user: User | null = new User("shadow");
    user = null;
  };
  clear();
  return user.name;
}
function shadowedAddressDoesNotEscape(present: boolean): string {
  let user = maybe(present);
  if (user === null) { return "missing"; }
  {
    let user: User | null = new User("shadow");
    const pointer = &user;
    *pointer = null;
  }
  return user.name;
}
function checkedCapturedRead(present: boolean): string {
  let user = maybe(present);
  const read = (): string => {
    if (user === null) { return "missing"; }
    return user.name;
  };
  return read();
}
function readOnlyClosurePreservesOuterFact(present: boolean): string {
  let user = maybe(present);
  if (user === null) { return "missing"; }
  const read = (): string => {
    if (user === null) { return "missing"; }
    return user.name;
  };
  read();
  return user.name;
}
function guardedWhileIterations(present: boolean, repeat: boolean): string {
  let user = maybe(present);
  let name = "missing";
  while (repeat) {
    if (user === null) { user = new User("restored"); }
    name = user.name;
    user = null;
    repeat = false;
  }
  return name;
}
function guardedForIterations(present: boolean): string {
  let user = maybe(present);
  let name = "missing";
  for (let index = 0; index < 2; index = index + 1) {
    if (user === null) { user = new User("restored"); }
    name = user.name;
    user = null;
  }
  return name;
}
function guardedRangeIterations(present: boolean): string {
  let user = maybe(present);
  let name = "missing";
  for (const ignored of [1, 2]) {
    if (user === null) { user = new User("restored"); }
    name = user.name;
    user = null;
  }
  return name;
}
function nonNullBackedge(): string {
  let user: User | null = new User("initial");
  let repeat = true;
  while (repeat) {
    const name = user.name;
    user = new User(name);
    repeat = false;
  }
  return user.name;
}
function terminatingLoopPaths(flag: boolean): string {
  let user: User | null = new User("initial");
  let name = "missing";
  while (flag) {
    name = user.name;
    user = null;
    break;
    user = new User("unreachable");
  }
  return name;
}
function conditionalContinueIsRechecked(flag: boolean): string {
  let user: User | null = new User("initial");
  let name = "missing";
  while (flag) {
    if (user === null) { user = new User("restored"); }
    name = user.name;
    if (flag) {
      user = null;
      continue;
    }
    user = new User("next");
  }
  return name;
}
function unreachableEscapesDoNotAffectFlow(flag: boolean): string {
  let user: User | null = new User("initial");
  while (flag) {
    break;
    const clear = (): void => { user = null; };
    const pointer = &user;
    *pointer = null;
    continue;
  }
  return user.name;
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestNullableStableMemberFlowMatrix(t *testing.T) {
	diagnostics := checkSource(t, `
class User { constructor(public name: string) {} }
class Profile { constructor(public user: User | null) {} }
class Holder {
  constructor(public user: User | null, public profile: Profile) {}
  public function current(): string {
    if (this.user === null) { return "missing"; }
    return this.user.name;
  }
}
function branch(holder: Holder): string {
  if (holder.user !== null) { return holder.user.name; }
  return "missing";
}
function inverse(holder: Holder): string {
  if (null === holder.user) { return "missing"; }
  return holder.user.name;
}
function nested(holder: Holder): string {
  if (holder.profile.user === null) { return "missing"; }
  return holder.profile.user.name;
}
function assignmentEstablishes(holder: Holder): string {
  holder.user = new User("assigned");
  return holder.user.name;
}
function exhaustiveAssignment(holder: Holder, left: boolean): string {
  if (left) { holder.user = new User("left"); }
  else { holder.user = new User("right"); }
  return holder.user.name;
}
function switchAssignment(holder: Holder, mode: int): string {
  switch (mode) {
    case 0, 1 { holder.user = new User("case"); }
    default { holder.user = new User("default"); }
  }
  return holder.user.name;
}
function shadowedReceiver(holder: Holder): string {
  if (holder.user === null) { return "missing"; }
  {
    const holder = new Holder(null, new Profile(null));
    if (holder.user !== null) { return holder.user.name; }
  }
  return holder.user.name;
}
function recheckAfterWrite(holder: Holder): string {
  if (holder.user !== null) { holder.user = null; }
  if (holder.user === null) { return "missing"; }
  return holder.user.name;
}
function builtinDoesNotInvalidate(holder: Holder): string {
  if (holder.user === null) { return "missing"; }
  const ignored = len([1, 2]);
  return holder.user.name;
}
function guardedIterations(holder: Holder, rounds: int): string {
  let result = "";
  for (let index = 0; index < rounds; index = index + 1) {
    if (holder.user === null) { holder.user = new User("restored"); }
    result = result + holder.user.name;
    holder.user = null;
  }
  return result;
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestRejectsInvalidNullableMemberFlow(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"unchecked field", `class User { public name: string; } class Holder { constructor(public user: User | null) {} } function bad(holder: Holder): string { return holder.user.name; }`, "must be checked against null"},
		{"direct field write", `class User { public name: string; } class Holder { constructor(public user: User | null) {} } function bad(holder: Holder): string { if (holder.user === null) { return ""; } holder.user = null; return holder.user.name; }`, "possibly aliased field assignment"},
		{"alias field write", `class User { public name: string; } class Holder { constructor(public user: User | null) {} } function bad(holder: Holder): string { const alias = holder; if (holder.user === null) { return ""; } alias.user = null; return holder.user.name; }`, "possibly aliased field assignment"},
		{"receiver reassignment", `class User { public name: string; } class Holder { constructor(public user: User | null) {} } function bad(input: Holder): string { let holder = input; if (holder.user === null) { return ""; } holder = new Holder(null); return holder.user.name; }`, "assignment to its receiver"},
		{"unknown call", `class User { public name: string; } class Holder { constructor(public user: User | null) {} } function clear(holder: Holder): void { holder.user = null; } function bad(holder: Holder): string { if (holder.user === null) { return ""; } clear(holder); return holder.user.name; }`, "call with unknown mutation effects"},
		{"member address", `class User { public name: string; } class Holder { constructor(public user: User | null) {} } function bad(holder: Holder): string { if (holder.user === null) { return ""; } const pointer = &holder.user; *pointer = null; return holder.user.name; }`, "address of possibly aliased member storage"},
		{"mutable closure", `class User { public name: string; } class Holder { constructor(public user: User | null) {} } function bad(holder: Holder): string { if (holder.user === null) { return ""; } const clear = (): void => { holder.user = null; }; return holder.user.name; }`, "closure with possible member mutation"},
		{"closure receiver reassignment", `class User { public name: string; } class Holder { constructor(public user: User | null) {} } function bad(input: Holder): string { let holder = input; if (holder.user === null) { return ""; } const replace = (): void => { holder = new Holder(null); }; return holder.user.name; }`, "closure with possible member mutation"},
		{"one branch assignment", `class User { public name: string; } class Holder { constructor(public user: User | null) {} } function bad(holder: Holder, set: boolean): string { if (set) { holder.user = new User(); } return holder.user.name; }`, "must be checked against null"},
		{"loop backedge write", `class User { public name: string; } class Holder { constructor(public user: User | null) {} } function bad(holder: Holder, repeat: boolean): void { if (holder.user === null) { return; } while (repeat) { const name = holder.user.name; holder.user = null; } }`, "must be checked against null"},
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

func TestRejectsInvalidNullableUses(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"value type", `function bad(): int | null { return null; }`, "has no nil representation"},
		{"fixed array", `function bad(): [2]int | null { return null; }`, "has no nil representation"},
		{"null to nonnullable", `class User {} function bad(): User { return null; }`, "cannot use null as User"},
		{"nil is raw", `class User {} function bad(): User | null { return nil; }`, "cannot use nil as User | null"},
		{"null inference", `function bad(): void { const value = null; }`, "cannot infer a variable type"},
		{"unsafe member", `class User { public name: string; } function bad(user: User | null): string { return user.name; }`, "must be checked against null"},
		{"write invalidates narrowing", `class User { public name: string; } function bad(user: User | null): string { if (user !== null) { user = null; return user.name; } return ""; }`, "previous non-null proof was invalidated by an assignment"},
		{"join restores nullable", `class User { public name: string; } function bad(flag: boolean): string { let user: User | null = null; if (flag) { user = new User(); } return user.name; }`, "must be checked against null"},
		{"loop write reaches exit", `class User { public name: string; } function bad(flag: boolean): string { let user: User | null = new User(); while (flag) { user = null; } return user.name; }`, "must be checked against null"},
		{"switch join restores nullable", `class User { public name: string; } function bad(flag: boolean): string { let user: User | null = null; switch (flag) { case true { user = new User(); } default {} } return user.name; }`, "must be checked against null"},
		{"addressed mutable not narrowed", `class User { public name: string; } function bad(user: User | null): string { let copy = user; const pointer = &copy; if (copy !== null) { return copy.name; } return ""; }`, "must be checked against null"},
		{"captured mutation not narrowed", `class User { public name: string; } function bad(user: User | null): string { let copy = user; const clear = (): void => { copy = null; }; if (copy !== null) { clear(); return copy.name; } return ""; }`, "must be checked against null"},
		{"address invalidates at its program point", `class User { public name: string; } function bad(user: User | null): string { let copy = user; if (copy === null) { return ""; } const pointer = &copy; return copy.name; }`, "invalidated by taking its address"},
		{"closure invalidates at its program point", `class User { public name: string; } function bad(user: User | null): string { let copy = user; if (copy === null) { return ""; } const clear = (): void => { copy = null; }; return copy.name; }`, "invalidated by a closure that can mutate it"},
		{"closure does not inherit outer narrowing", `class User { public name: string; } function bad(user: User | null): string { let copy = user; if (copy === null) { return ""; } const read = (): string => { return copy.name; }; return read(); }`, "must be checked against null"},
		{"capture in one branch escapes after join", `class User { public name: string; } function bad(user: User | null, flag: boolean): string { let copy = user; if (flag) { const clear = (): void => { copy = null; }; clear(); } if (copy !== null) { return copy.name; } return ""; }`, "must be checked against null"},
		{"nested closure write escapes declaration", `class User { public name: string; } function bad(user: User | null): string { let copy = user; const outer = (): void => { const inner = (): void => { copy = null; }; inner(); }; if (copy !== null) { return copy.name; } return ""; }`, "must be checked against null"},
		{"while backedge invalidates next iteration", `class User { public name: string; } function maybe(flag: boolean): User | null { if (flag) { return new User(); } return null; } function bad(flag: boolean): void { let user: User | null = new User(); while (flag) { const name = user.name; user = maybe(flag); } }`, "must be checked against null"},
		{"for backedge invalidates next iteration", `class User { public name: string; } function maybe(flag: boolean): User | null { if (flag) { return new User(); } return null; } function bad(flag: boolean): void { let user: User | null = new User(); for (let index = 0; index < 2; index = index + 1) { const name = user.name; user = maybe(flag); } }`, "must be checked against null"},
		{"range backedge invalidates next iteration", `class User { public name: string; } function maybe(flag: boolean): User | null { if (flag) { return new User(); } return null; } function bad(flag: boolean): void { let user: User | null = new User(); for (const ignored of [1, 2]) { const name = user.name; user = maybe(flag); } }`, "must be checked against null"},
		{"closure escape reaches next iteration", `class User { public name: string; } function bad(flag: boolean): void { let user: User | null = new User(); while (flag) { const name = user.name; const clear = (): void => { user = null; }; clear(); } }`, "must be checked against null"},
		{"address escape reaches next iteration", `class User { public name: string; } function bad(flag: boolean): void { let user: User | null = new User(); while (flag) { const name = user.name; const pointer = &user; *pointer = null; } }`, "must be checked against null"},
		{"continue backedge ignores unreachable restoration", `class User { public name: string; } function bad(flag: boolean): void { let user: User | null = new User(); while (flag) { const name = user.name; user = null; continue; user = new User(); } }`, "must be checked against null"},
		{"conditional continue contributes its backedge", `class User { public name: string; } function bad(flag: boolean, skip: boolean): void { let user: User | null = new User(); while (flag) { const name = user.name; if (skip) { user = null; continue; } user = new User(); } }`, "must be checked against null"},
		{"break state reaches loop exit", `class User { public name: string; } function bad(flag: boolean): string { let user: User | null = new User(); while (flag) { user = null; break; user = new User(); } return user.name; }`, "must be checked against null"},
		{"switch break path remains reachable", `class User { public name: string; } function bad(flag: boolean, stop: boolean): string { let user: User | null = new User(); switch (flag) { case true { if (stop) { user = null; break; } user = new User(); } default { user = new User(); } } return user.name; }`, "must be checked against null"},
		{"select break path remains reachable", `class User { public name: string; } function bad(stop: boolean): string { let user: User | null = new User(); select { default { if (stop) { user = null; break; } user = new User(); } } return user.name; }`, "must be checked against null"},
		{"nullable to base", `class User {} function bad(user: User | null): User { return user; }`, "cannot use User | null as User"},
		{"null compared nil", `class User {} function bad(user: User | null): boolean { return user === nil; }`, "cannot compare User | null and nil"},
		{"null with null", `function bad(): boolean { return null === null; }`, "cannot compare null with null"},
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

func TestNullableLoopFixedPointReportsFinalIterationOnce(t *testing.T) {
	diagnostics := checkSource(t, `
class User { public name: string; }
function maybe(flag: boolean): User | null {
  if (flag) { return new User(); }
  return null;
}
function bad(flag: boolean): void {
  let user: User | null = new User();
  while (flag) {
    const name = user.name;
    user = maybe(flag);
  }
}
`)
	count := 0
	for _, item := range diagnostics {
		if strings.Contains(item, "previous non-null proof was invalidated by an assignment") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("assignment-invalidation diagnostic count = %d, diagnostics = %v", count, diagnostics)
	}
}
