---
title: Generic functions and structs
description: Instantiate a generic value struct and call a generic function with explicit type arguments.
---

# Generic functions and structs

This recipe combines a generic nominal value type with a generic function.

## Project tree

```text
generics/
└── main.km
```

## Source

<<< ../snippets/generics.km{ts}

## Run

```sh
keika check main.km
keika run main.km
```

Expected output:

```text
onsen 42
```

## Contract demonstrated

- `Box<string>` is explicitly instantiated in the type and literal.
- The value receiver method substitutes `T` with `string`.
- `choose<int>` supplies an explicit function argument; `choose(21, 42, true)` could infer `int` instead.
- Different instantiations such as `Box<string>` and `Box<int>` are distinct types.
- A generic struct remains a value: assignment copies the outer Go struct.

Use `T extends comparable` when equality or map-key use requires the constraint. Native type sets beyond `comparable`, generic aliases, and method-local type parameters are not implemented.

See [Functions and generics](../guide/functions-and-generics).
