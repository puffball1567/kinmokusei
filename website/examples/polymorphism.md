---
title: Interface polymorphism
description: Implement a Kinmokusei interface with a reference class and dispatch through the interface.
---

# Interface polymorphism

This recipe declares a small interface, implements it explicitly, and calls a method through the interface value.

## Project tree

```text
polymorphism/
└── main.km
```

## Source

<<< ../snippets/polymorphism.km{ts}

## Run

```sh
keika check main.km
keika run main.km
```

Expected output:

```text
Welcome, Aki
```

## Contract demonstrated

- `Guest` explicitly names `Greeter` in `implements`.
- The compiler validates its public instance method signature.
- A `Guest` class value is a reference with identity.
- Passing it as `Greeter` uses generated Go interface dispatch.
- The private `name` field is not exposed as a public Go field.

Interfaces are intentionally explicit even when a method set happens to match. Classes can also implement imported Go interfaces when their generated public method set satisfies the original Go contract.

See [Classes and structs](../guide/classes-and-structs).
