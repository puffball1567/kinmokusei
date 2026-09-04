---
title: Inheritance and checked downcasts
description: Override virtual methods, preserve identity through an upcast, and recover a derived class safely.
---

# Inheritance and checked downcasts

This example uses single inheritance for shared implementation and an interface for the public behavior contract.

## Source

<<< ../snippets/inheritance.km{ts}

## Run

```sh
keika check inheritance.km
keika run inheritance.km
```

Expected output:

```text
Hana:animal/woof/guide
animal/woof/guide true
false true
```

## What the example proves

- `GuideDog` initializes its base classes through `super(...)`.
- Calls made through `Animal` still dispatch to the most-derived override.
- An upcast preserves reference identity.
- `value as? GuideDog` returns `[value, boolean]` and never panics.
- A failed checked downcast returns the nil zero value plus `false`.

Use `as!` only when failure is a programmer error; a failed forced downcast panics with the corresponding Go assertion behavior. Downcasts are valid only within one class inheritance chain. Interface assertions use the same surface syntax but follow Go interface method-set rules.

See [Classes and structs](../guide/classes-and-structs) for initialization, visibility, `virtual`, `override`, and `final` rules.
