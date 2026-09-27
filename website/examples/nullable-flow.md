---
title: Stable nullable flow
description: Snapshot mutable nullable storage and re-establish a proof after every write.
---

# Stable nullable flow

Nullable checks are flow-sensitive, but a mutable member may be written through another path. Snapshot the value into an immutable local before using it repeatedly.

## Source

<<< ../snippets/nullable-flow.km{ts}

## Run

```sh
keika check nullable-flow.km
keika run nullable-flow.km
```

Expected output:

```text
guest
hello
guest
```

`snapshot` has declared type `User | null`. After the early return, the remaining path proves that particular binding contains `User`. Updating `profile.user` later does not mutate the earlier snapshot.

## Proof invalidation

Checks also flow through short-circuit boolean operators:

```ts
function hasName(user: User | null): boolean {
  return user !== null && user.name !== "";
}

function greeting(user: User | null, enabled: boolean): string {
  if (user === null || !enabled) { return "guest"; }
  return user.name;
}
```

`&&` checks its right operand on the left operand's true path; `||` uses its
false path, and `!` reverses the paths. The same rules apply to `if`, `while`
and `for` conditions. Storing the comparison in a separate boolean binding
does not store its non-null proof.

An intervening function call may mutate a member: checking
`profile.user !== null` before a call does not prove it is still non-null
after that call. Check again afterwards or use an immutable snapshot.

The compiler rejects this sequence:

```ts
if (user !== null) {
  user = null;
  return user.name;
}
```

The diagnostic explains that the previous non-null proof was invalidated by assignment. A new `!== null` check or a non-null assignment establishes a new proof. Flow joins remain conservative across branches, loops, switches, selects, captures, aliases, and address-taking.

See [Errors and nullability](../guide/errors-and-nullability) for nil-capable types and operations that are safe without narrowing.
