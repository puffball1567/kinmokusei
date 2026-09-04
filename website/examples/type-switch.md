---
title: Inspect a Go interface with a type switch
description: Match concrete Go pointer types, handle nil, and keep each narrowed binding case-local.
---

# Inspect a Go interface with a type switch

Type switches are for imported Go interface values. They preserve Go's dynamic type behavior while giving every case a statically narrowed binding.

## Source

<<< ../snippets/type-switch.km{ts}

## Run

```sh
keika check type-switch.km
keika run type-switch.km
```

Expected output:

```text
string:5
buffer:6
nil
```

## Contract demonstrated

- The switch subject evaluates once.
- `case const name as Type` creates a case-local typed binding.
- A `nil` interface case is distinct from `default`.
- Types must be possible implementations of the subject interface.
- Duplicate types, duplicate `nil`, and duplicate `default` are rejected.
- Type switches never use `fallthrough`.

Use `let` instead of `const` when the narrowed binding itself must be reassigned, or `_` when only the dynamic-type match matters.

For one assertion outside a switch, use `const [reader, ok] = value as? *strings.Reader;`. Use `as!` only when assertion failure should panic.

See [Go interoperability](../guide/go-interop) and [Language syntax](../reference/language#control-flow).
