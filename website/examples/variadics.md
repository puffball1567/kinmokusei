---
title: Variadics and slice spread
description: Accept zero or more typed arguments and expand one final slice explicitly.
---

# Variadics and slice spread

A rest parameter is a slice inside the function and a Go variadic at its call boundary.

## Source

<<< ../snippets/variadics.km{ts}

## Run

```sh
keika check variadics.km
keika run variadics.km
```

Expected output:

```text
10 13 22
empty tamago
```

## Call rules

- A rest parameter uses `...values: T[]` and must be last.
- Callers may pass zero or more individual `T` values.
- One final `T[]` may be expanded with `values...`.
- Fixed arguments still appear before the expanded slice.
- Generic inference includes both individual and expanded arguments.

The same form works for functions, methods, interfaces, arrows, function types, and constructors. A plain `T[]` parameter is not interchangeable with a variadic signature.

The documentation suite also rejects spreading `string[]` into `...values: int[]`, verifying that spread does not bypass element typing.

See [Functions and generics](../guide/functions-and-generics#rest-parameters).
