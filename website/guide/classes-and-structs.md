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

Classes support public, protected, and private members; static members; interfaces; single `extends`; `virtual`; explicit `override`; `final`; and `super`.

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

Decode into a class instance created by its constructor. Because JSON only
updates public fields, its private invariants, internal identity, and virtual
dispatch remain intact. Automatic allocation by `encoding/json` does not run a
class constructor; define `unmarshalJSON(data: byte[]): error` when a class
needs custom allocation-time initialization or a different wire format.

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
