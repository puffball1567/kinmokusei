---
title: Value and pointer receivers
description: Compare copied value-receiver behavior with explicit pointer mutation on a native struct.
---

# Value and pointer receivers

A native struct makes copying visible. This recipe performs one update on a receiver copy and one through a shared pointer.

## Project tree

```text
struct-receivers/
└── main.km
```

## Source

<<< ../snippets/struct-receivers.km{ts}

## Run

```sh
keika check main.km
keika run main.km
```

Expected output:

```text
12 15
```

`added(5)` changes and returns a copy whose value is `15`; it does not change `counter`. `add(2)` is a pointer receiver, so selecting it from the addressable local mutates `counter` to `12`.

## Method values

Forming a method value follows Go behavior: a value receiver is copied when the method value is formed, while a pointer receiver retains shared storage. A nil pointer receiver can be called, but dereferencing it retains ordinary Go panic behavior.

The same receiver methods can be organized outside the struct with `function name(this: Counter, ...)` or `function name(this: *Counter, ...)` when the receiver type is declared in the same module.
