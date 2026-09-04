---
title: Use Go standard-library values
description: Work with an imported Go struct, pointer-receiver methods, multiple results, raw errors, package functions, and callbacks.
---

# Use Go standard-library values

This recipe stays on the direct Go boundary. It uses an imported `bytes.Buffer` value, calls its pointer-receiver methods through addressable local storage, keeps both results from Go APIs visible, and passes a typed arrow back into Go.

## Project tree

```text
go-standard-library/
└── main.km
```

## Source

<<< ../snippets/go-standard-library.km{ts}

## Run

```sh
keika check main.km
keika run main.km
```

Expected output:

```text
5 true ONSEN 42 true [1 2 3]
```

## Read the boundary

1. `bytes.Buffer` keeps the identity and method set of the real Go type.
2. `buffer` is a named, addressable value. The compiler can therefore call `WriteString` and `String`, whose Go method set includes pointer-receiver behavior.
3. The nested `strings.ToUpper` call finishes before `WriteString` starts; each target and argument evaluates once in source order.
4. `WriteString` and `strconv.Atoi` expose their Go results directly. `[written, writeErr]` and `[parsedNumber, parseErr]` are compiler-known result lists, not tuple objects.
5. Raw Go `error` values compare with `nil`. Nothing converts them to exceptions or hides them behind a result wrapper.
6. `sort.Slice` receives a Kinmokusei arrow with the exact Go callback shape. The arrow captures `values` lexically and Go calls it during sorting.

Use `Result<T>` and postfix `?` when propagation is part of your own function contract. Keep explicit results when the current scope must inspect, combine, or report them separately.

Continue with the [Go interoperability guide](../guide/go-interop), the [interop support matrix](../reference/go-interop), or [Result parsing](./result-parsing).
