# Classes and structs

Classes and structs may both have methods, but they make different promises. A class is a reference with identity. A struct is a value copied according to Go's struct rules.

<<< ../snippets/classes-and-structs.otm{ts}

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
