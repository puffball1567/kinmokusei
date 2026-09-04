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
onsen
guest
```

`snapshot` has declared type `User | null`. After the early return, the remaining path proves that particular binding contains `User`. Updating `profile.user` later does not mutate the earlier snapshot.

## Proof invalidation

The compiler rejects this sequence:

```ts
if (user !== null) {
  user = null;
  return user.name;
}
```

The diagnostic explains that the previous non-null proof was invalidated by assignment. A new `!== null` check or a non-null assignment establishes a new proof. Flow joins remain conservative across branches, loops, switches, selects, captures, aliases, and address-taking.

See [Errors and nullability](../guide/errors-and-nullability) for nil-capable types and operations that are safe without narrowing.
