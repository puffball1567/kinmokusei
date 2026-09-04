---
title: Structs, classes, and interfaces
description: Choose value or reference semantics, write constructors and methods, enforce visibility, implement interfaces, and use inheritance.
---

# Structs, classes, and interfaces

The surface syntax is intentionally familiar, but the semantic choice is explicit: a `struct` is a nominal copied value, while a `class` is a reference with identity.

## Declaring a struct

```ts
struct Point {
  public x: int;
  public y: int;
}
```

Construct it with a complete field literal:

```ts
const point = Point { x: 10, y: 20 };
```

Missing, unknown, duplicate, and mistyped fields are diagnostics. Structs cannot contain themselves directly or only through fixed arrays; pointer/slice/map/function/channel indirection makes recursive shapes finite.

## Struct methods

A nested method is a value receiver by default:

```ts
struct Point {
  public x: int;
  public function moved(delta: int): Point {
    this.x += delta;
    return this;
  }
}
```

The receiver is a copy. Use `pointer function` to mutate addressable shared storage:

```ts
struct Counter {
  public value: int;
  public pointer function increment(): void { this.value++; }
}
```

Go-like automatic address/dereference method selection is supported when the value is addressable. Pointer methods on a temporary literal are rejected.

## Declaring a class

```ts
class User {
  private token: string;

  constructor(public name: string, token: string) {
    this.token = token;
  }

  public function authenticated(): boolean {
    return this.token !== "";
  }
}
```

Construct with `new User(...)`. Constructor parameters may declare fields directly using visibility. Every non-null class field must be initialized on every completing path.

Class assignment preserves identity:

```ts
const first = new User("Aki", "secret");
const second = first;
second.name = "Hana"; // first.name is also Hana
```

## Visibility

| Modifier | Access |
| --- | --- |
| `public` | Public member boundary |
| `private` | Declaring type only |
| `protected` | Declaring class and descendants |

Visibility is checked before Go name generation. A generated capital letter does not grant Kinmokusei access, and a generated private helper does not weaken a public source contract.

## Static methods

```ts
class Meter {
  constructor(public value: int) {}
  public static function create(value: int): Meter {
    return new Meter(value);
  }
}
```

Call through the class: `Meter.create(1)`. Calling a static method on an instance is rejected. Public static methods lower to idiomatic package functions because Go has no type-level methods; generated name collisions are checked.

## Interfaces

```ts
interface Greeter {
  function greet(name: string): string;
}

class Welcome implements Greeter {
  public function greet(name: string): string {
    return "Welcome, " + name;
  }
}
```

Implementation is explicit. The compiler checks the public instance method set, including parameter/result types and variadic status. Static/private methods do not satisfy interface methods.

Imported Go interfaces can also appear after `implements` when the generated method set connects exactly.

## Single inheritance

```ts
class Animal {
  constructor(public name: string) {}
  public virtual function speak(): string { return "animal"; }
}

final class Dog extends Animal {
  constructor(name: string) { super(name); }
  public final override function speak(): string {
    return super.speak() + "/woof";
  }
}
```

`extends` reuses one base class. A derived constructor calls `super(...)`. Only `virtual` slots dispatch dynamically; replacement requires `override`. `final override` closes a slot, and `final class` prevents further derivation.

`super.method()` statically invokes the immediate base implementation. Calling another virtual method through `this` keeps dynamic dispatch.

## Upcast and downcast

A derived class upcasts to its base implicitly while preserving nil and identity:

```ts
const dog = new Dog("Hana");
const animal: Animal = dog;
```

Recover a derived reference with a checked downcast:

```ts
const [restored, ok] = animal as? Dog;
```

Or force it when failure is a programming error:

```ts
const required = animal as! Dog;
```

Checked failure returns nil/false; forced failure panics. Downcasts are only meaningful within one declared inheritance chain and evaluate their source once.

A struct communicates copied domain state; a class communicates shared identity and constructor invariants; an interface communicates behavior independently of either representation. [Failures, results, and exceptions](./errors-results-exceptions) adds the completion paths those APIs can expose.
