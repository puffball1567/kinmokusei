---
title: Safe input decoding and binding
description: Stage raw input, construct checked native values, and publish only after validation succeeds.
---

# Safe input decoding and binding

::: info Available since v0.4.10
`kinmokusei/input` requires v0.4.10 or later.
:::

Reusable JSON, query, URI and form binders cannot send an arbitrary writable
`*T` to Go. A generic destination might contain verified `string` fields, and
a decoder can partially modify it before reporting an error. Checking the
destination only after a successful Go return is therefore insufficient.

The supported adapter separates three steps:

1. Read into **fresh raw DTO storage** at a concretely typed Go call site.
2. Validate the raw fields and construct a new native value.
3. Publish that value only after both steps succeed.

<<< ../snippets/input-adapters.km{ts}

The decoder sees `*RawInput`, never `User` or an unconstrained `*T`. Its normal
boundary checks still apply. `string(raw.name)` validates UTF-8; the `User`
constructor only receives the checked text. No reflection-based assertion or
special compiler exception is needed for each user DTO.

## Reusable library APIs

```ts
import { decodeInput, bindInput } from "kinmokusei/input";

function decodeForLibrary<Raw, Value>(
  read: () => Result<Raw>,
  construct: (raw: Raw) => Result<Value>,
): Result<Value> {
  return decodeInput(read, construct);
}
```

`decodeInput` returns the newly constructed value. `bindInput` additionally
accepts a `(value: Value) => void` publication callback, such as an assignment
to a local variable, object field or cache entry. Both infer the raw and native
types from typed callbacks. A Result-returning arrow needs a block body with an
explicit `return`, as in the executable example above.

The concrete read callback is part of the adapter. Moving `json.Unmarshal` or
`ShouldBindJSON` into a generic body taking `*Raw` does not make that body safe;
it remains rejected. A framework can take the typed read/construction callbacks
as parameters or store them in a user-supplied adapter. The application defines
its DTO adapter once and reuses it; the framework does not enumerate DTO types.

## Framework policies stay in the read stage

A read callback can capture the framework context and call a concrete Go
binder. Keep unknown-field checks, trailing-value checks, request/body limits,
content-type selection and framework validation there. For example, a strict
JSON reader uses `Decoder.DisallowUnknownFields`, decodes into its raw DTO, and
requires the next decode to return `io.EOF` before returning `ok(raw)`.

Query/URI/form readers likewise produce raw DTOs with `bstring` fields. The
construct stage checks them even if the binder itself succeeded. The adapter
does not change the selected decoder's normalization rules or impose a separate
UTF-8 check on the complete wire payload; it validates the native values being
constructed. JSON decoding and raw query decoding can have different input
normalization behavior.

## Nested fields and destination types

- Construct classes with `new`, so constructor checks, virtual dispatch and
  hierarchy identity are retained. Publication replaces the reference; it does
  not mutate the previous instance in place.
- Construct structs with checked literals. Publication assigns the resulting
  struct value, retaining ordinary shallow-copy semantics for reference fields.
- Check optional raw pointers before dereferencing them; map absence explicitly
  to the destination's `T | null` contract or report an error.
- Build verified collections by validating every relevant key, element and
  nested field. Validate `string(rawText)` conversions before storing their
  results in verified fields. Raw `bstring` collections do not become
  `string` collections through a Go type assertion or shared backing storage.

The compiler checks the construct callback normally, including generic source
contracts. Its result may be a scalar, struct, class, nullable value, collection
or another ordinary native value. There is no automatic DTO traversal, cloning,
field-name policy or unchecked construction of an arbitrary native `T`.

## Failure, aliases and callback effects

If reading fails, construction and publication are not invoked. If construction
fails, publication is not invoked. Successful binding invokes publication once.
The original destination and aliases to its previous instance stay untouched
when the adapter uses fresh raw storage and only the final callback assigns the
destination. Partial decoder writes, including writes made by a custom decoder
before returning an error, remain confined to the raw staging object.

Do not reuse the destination, its reference-bearing fields or a shared cached
raw object as staging storage. A struct copy alone does not isolate slices,
maps or pointers. Allocate the raw graph for each operation and construct new
verified storage where appropriate.

Callbacks are ordinary code, **not transactions or purity-enforced functions**.
The helper does not roll back explicit mutations through captured aliases,
global state, custom decoder/validator side effects, a throwing constructor or
a partially executed publication callback. Ordinary panics propagate. In
particular, a user callback that modifies the destination early defeats the
staging protocol; use a final assignment callback if preserving it on failure
is required. Synchronize concurrent access as usual.

Opaque Go writes into verified or unknown storage and unchecked verified-text
assertions remain rejected, on both success and error paths. This adapter does
not grant unsafe permission or relax those rules.

See the [standard-module signatures](../reference/standard-library#kinmokuseiinput-api)
and [Result handling](../book/errors-results-exceptions).
