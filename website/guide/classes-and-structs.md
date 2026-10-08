---
title: Classes and structs
description: Choose reference classes or Go-compatible value structs and define their methods explicitly.
---

# Classes and structs

Classes and structs may both have methods, but they make different promises. A class is a reference with identity. A struct is a value copied according to Go's struct rules.

<<< ../snippets/classes-and-structs.km{ts}

The original `Point` remains unchanged because `moved` works on a value-receiver copy. The `Counter` instance is shared by reference.

```text
2 3 5
```

## Classes

Constructor parameters may declare fields directly:

```ts
class User {
  constructor(public id: int, public name: string) {}
}
```

Classes support public, protected, and private members; static methods and fields; interfaces; single `extends`; `virtual`; explicit `override`; `final`; `super`; and abstract classes/methods. Typed instance fields may have initializers evaluated separately for each construction.

Base construction precedes derived defaults. Within each class, defaults run
in declaration order before constructor-field parameter assignments and the
body. An initializer may read earlier initialized fields and accessible base
fields, but not constructor locals, later fields, accessors or a captured
receiver. Constructor virtual calls are phase-local, not calls into uninitialized
descendants. See [initialization order](../book/structs-classes-interfaces#initialization-order-and-construction-dispatch)
for the trace and rejected example.

Static fields require an explicit type and initializer:

```ts
class Counter<T> {
  public static count: int = 0;
  constructor(public value: T) { Counter.count++; }
}
```

Use `Counter.count`, not `instance.count`. The storage is shared across generic
instantiations and descendants; it is initialized once in Go package dependency
order and excluded from instance JSON. Class type parameters and `this`/`super`
cannot be used in static initializers. Cyclic initialization is a compile error.
Public fields become Go package variables such as `CounterCount`. Concurrent
updates require explicit synchronization.

Use `public static const limit: int = 32;` for a typed class constant and read it
as `ClassName.limit`. Its initializer must be a known compile-time numeric,
string or boolean value; runtime calls and mutable values are rejected. Constants
are inherited with the same visibility rules and cannot be assigned or addressed.
Public constants become Go constants, not storage or accessor calls. On the
development branch, an accessible integer constant can also supply a Kinmokusei
array type length such as `[ClassName.limit]int`; v0.4.6 still requires integer
literals there. See the [array-length rules](../reference/types#fixed-array-lengths).

Instance properties use `public get value(): int { ... }` and
`private set value(next: int) { ... }`. Read with `instance.value` and assign
with `instance.value = next`; each accessor has its own visibility. Updates
such as `instance.value++` require both accessors. Inherited generic properties
are supported. Each accessor can be virtual or abstract, with explicit
`override` and optional `final` in descendants. Overriding one accessor retains
the other inherited accessor; its visibility and type must remain unchanged.
`super.value` uses the base implementation. Interfaces accept public accessor
signatures such as `get value(): int;` and `set value(next: int);`, including
generic inheritance and DI. Class implementations must supply the corresponding
public accessors. Static accessors use `public static get` / `set` and are
accessed as `ClassName.value`; they are inherited without virtual dispatch.
In generic classes, static property signatures and bodies cannot refer to the
class type parameters. Static properties do not declare backing storage.
Bind a nullable getter result locally before checking it: separate
reads are separate calls and may return different values.

## JSON

Public class and struct fields use their Kinmokusei names as JSON keys. Private
and protected class state is not serialized. Generic fields and inherited
public fields work with Go's `encoding/json` package:

```ts
import go json from "encoding/json";

class Box<T> {
  constructor(public value: T, private secret: string) {}
}

function encode(box: Box<string>): Result<byte[]> {
  const data = json.Marshal(box)?;
  return ok(data);
}
```

Decoding into an existing raw-data (`bstring`) constructor-created instance preserves its existing
private state and dispatch setup, but **does not validate domain invariants**.
`encoding/json` updates public Go fields directly, without setters or constructor
checks; missing fields, nulls and partial updates on an error retain ordinary
Go behavior. Newly allocated nested class values do not run Kinmokusei
constructors. A getter alone does not create a serialized field.

For application input, decode into a value DTO, validate it and then construct
the domain class:

<<< ../snippets/json-class-validation.km{ts}

This prints `Aki 2` and `true`. The wrapper validates the name, copies tags into
independent non-nil storage and calls the ordinary constructor only on success.
The DTO uses raw `bstring` fields: Go reflection cannot write into shared
verified `string` storage. Each name/tag is decoded explicitly before the domain
object is constructed. Native structs with raw fields and imported Go DTOs
are also valid decoder targets when their shape fits the intended API.
It retains Go's default unknown-field policy; stricter schemas need explicit
validation or a configured decoder. A custom JSON hook must be a public method
whose generated Go signature matches `UnmarshalJSON([]byte) error`; a private
lowercase method is not picked up automatically. See [JSON recipes](../examples/json).

## Struct receivers

A nested method is a value receiver by default. Add `pointer` when it should mutate shared storage:

```ts
struct Counter {
  public value: int;

  public function snapshot(): int { return this.value; }
  public pointer function increment(): void { this.value++; }
}
```

External receiver declarations are also available when separating data layout and behavior improves readability.
