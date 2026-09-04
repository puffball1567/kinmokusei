---
title: Model domain values with defined types
description: Give strings and maps nominal identities, attach methods, and convert explicitly at boundaries.
---

# Model domain values with defined types

Use `type Name = distinct T` when two values share a runtime representation but should not be mixed accidentally. Use `alias` only when a second spelling should remain fully interchangeable.

## Source

<<< ../snippets/defined-types.km{ts}

## Run

```sh
keika check defined-types.km
keika run defined-types.km
```

Expected output:

```text
user:42 Aki 100
-1
```

## Defined type versus alias

`UserID` and `Scores` are new nominal types. Their compatible underlying values cross the boundary with explicit conversions such as `UserID(raw)` and `string(id)`. `UserID` keeps string operations between two `UserID` values and is comparable, so it is a valid map key.

`DisplayName` is a transparent alias for `string`; no conversion is needed. It communicates intent but does not prevent mixing.

The `label` receiver is declared in the same module as `UserID`, giving the defined type its own method set. Imported types cannot be extended this way.

The documentation suite also verifies that returning a raw `string` as `UserID` is rejected. This protects the domain boundary rather than silently converting it.

See [Type-system reference](../reference/types#native-named-types).
