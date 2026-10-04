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

Typed instance fields can declare defaults, such as `private items: int[] = []`.
Each construction evaluates its own initializers, after base construction and
before constructor-field parameter assignments and the constructor body.
Initializers use module scope and class type parameters. They may read earlier
initialized fields and accessible inherited state through `this.field`, but
cannot read later fields, capture the receiver, or use `super` or constructor-local
parameters. An explicit constructor is optional when the base does not require
arguments. Struct field defaults are not supported. Static class fields require
an explicit type and initializer; their storage is shared across instantiations
and descendants. See [static fields and constants](../guide/classes-and-structs#classes).

Class assignment preserves identity:

```ts
const first = new User("Aki", "secret");
const second = first;
second.name = "Hana"; // first.name is also Hana
```

## Initialization order and construction dispatch

Base construction completes before the derived class's field initializers run.
Within each class, field initializers execute in declaration order, then
constructor-field parameter assignments, then the constructor body:

<<< ../snippets/class-initialization.km{ts}

This prints `125346 4 7 6 1 0`. The trace records the base initializer (`1`),
base body (`2`) and base-phase virtual call (`5`), followed by the child's
initializer (`3`), body (`4`) and child-phase virtual call (`6`). A base
constructor does not dispatch into a not-yet-initialized descendant. After
construction, calls through a base reference use the most-derived override.

`total` reads an earlier field and inherited state; `inheritedSeed` can read
the completed base constructor's parameter field. Initializers cannot read
their own class's later/uninitialized fields, getters, methods or constructor
parameters, or capture `this` in a closure. For example, this is rejected:

<<< ../snippets-invalid/field-initializer-order.km{ts}

Each `Bucket` construction evaluates its own `[]`, so appending to the first
does not mutate the second. Initializers that explicitly reuse a module-level
slice, map or class still share that reference; defaults do not imply a deep
copy. If an initializer throws or panics, later initializers and the constructor
body are not executed. Constructor analysis checks non-null initialization,
not arbitrary domain invariants such as a positive numeric field.

## Visibility

| Modifier | Access |
| --- | --- |
| `public` | Public member boundary |
| `private` | Declaring type only |
| `protected` | Declaring class and descendants |

Visibility is checked before Go name generation. A generated capital letter does not grant Kinmokusei access, and a generated private helper does not weaken a public source contract.

## Properties and shared class storage

Getter/setter properties use field-like syntax at the call site, but execute
checked accessor calls. Static fields and constants belong to the class rather
than an instance:

<<< ../snippets/class-properties.km{ts}

This prints `5 0 2 10`: each instance has its own `stored` field, while both
generic instantiations share `Counter.count`. Use the uninstantiated class name
for static access, not `Counter<string>.count` or `first.count`.

A getter takes no arguments and declares a non-void result; a setter takes one
value and has no result annotation. Paired types must match exactly. Each
accessor has its own visibility: a public getter and private setter expose a
read-only property to callers. Simple assignment needs only the setter;
compound assignment and `++`/`--` need both. No backing field is created
automatically. Bind a nullable getter result to a local before narrowing it,
because a second property read is a second call.

Instance accessors can be virtual or abstract. Descendants use `override` with
the same visibility and type, and can close a slot with `final`. Overriding one
accessor preserves the other inherited accessor. `super.property` calls the
base accessor instead of virtual dispatch.

Static fields require explicit types and initializers. Initialization runs once
in Go package dependency order; cycles are diagnosed. Static initializers
cannot use `this`, `super`, or class type parameters. Inherited static members
reuse the declaring class's storage. Concurrent mutation requires ordinary
synchronization. A `static const` must have a compile-time numeric, string or
boolean initializer and cannot be assigned or addressed.

Public instance accessors become Go methods such as `GetValue()` and
`SetValue(int)`. Public static fields/constants become package declarations such
as `CounterCount` and `CounterLimit`; they are not included in instance JSON.

Initialization dependencies include referenced function/static-method bodies,
not just a direct expression reading another field. This indirect cycle is
also rejected; an unused runtime branch does not remove a lexical dependency:

<<< ../snippets-invalid/static-initialization-cycle.km{ts}

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

Static properties provide the same class-name access style:

```ts
let currentLevel: int = 0;
class Logging {
  public static get level(): int { return currentLevel; }
  public static set level(next: int) { currentLevel = next; }
}
Logging.level++;
```

Accessor visibility applies separately to reads and writes. Descendants reuse
the declaring class's implementation; static properties cannot be overridden.
They are not instance members and do not supply interface implementations.
Generic classes may have static properties, but these accessors cannot use class
type parameters: access remains `ClassName.property`, shared across type
instantiations. Ordinary static methods keep their existing generic behavior.
Public accessors lower to Go package functions such as `LoggingGetLevel()` and
`LoggingSetLevel(int)`. Updates are ordered calls, not atomic operations.

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

Method signatures must match exactly across implementations, overrides, and
inherited same-name declarations. This includes nullable types nested inside
collections, objects, callbacks, and generic arguments. A method accepting
`Leaf` cannot implement a requirement accepting `Leaf | null`; a nullable result
cannot replace a non-null result either. Generic methods do not satisfy
non-generic requirements.

This example is rejected before Go generation:

<<< ../snippets-invalid/interface-nullable-contract.km{ts}

Imported Go interfaces can also appear after `implements` when the generated method set connects exactly.

An interface can inherit multiple source interfaces with
`interface Store<T> extends Reader<T>, Writer<T> {}`. Diamond inheritance
preserves compatible shared contracts; cycles and conflicting signatures are
diagnostics. Exported Go runtime interfaces can also be bases. Inherited Go
methods retain their exported Go names and signature requirements, including
variadic and Result-shaped methods. Anonymous runtime interfaces imported from
Go retain their method sets. Method-only source interface literals such as
`interface { read(offset: int): string; }` are also supported.

Interfaces also support property contracts:

```ts
interface CountReader { get count(): int; }
interface CountWriter { set count(next: int); }
interface CountCell extends CountReader, CountWriter {}

class Counter implements CountCell {
  private stored: int = 0;
  public get count(): int { return this.stored; }
  public set count(next: int) { this.stored = next; }
}
function increment(counter: CountCell): int {
  counter.count++;
  return counter.count;
}
```

Accessor signatures are implicitly public and have no body. A getter-only
contract permits reads, a setter-only contract permits writes, and updates
require both. Paired types must match exactly, even across generic or diamond
bases. Classes explicitly implement these contracts with public accessors, not
fields or similarly named ordinary methods. Abstract classes may declare the
required accessors abstract. The generated Go contract uses `GetCount()` and
`SetCount(int)` methods rather than fields.

### Generic properties and inherited contracts

Property contracts, virtual accessors, inherited setters and method-local
generic inference compose without untyped storage:

<<< ../snippets/generic-properties.km{ts}

This prints `Go! 3`. The override changes only the getter; assignment uses the
inherited setter. Both the interface read and `map`'s `this.item` read dispatch
to the override. `map` introduces its own result parameter `U`, inferred as
`int` from the callback. Such method-local generic methods are lowered to
typed Go helper functions and cannot themselves be virtual, abstract or
interface requirements.

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

Abstract classes combine shared state/concrete behavior with required method
or accessor implementations. They cannot be constructed directly, but can be
used as parameter, field and result types, including for dependency injection.
Interfaces instead describe behavior without constructors or instance storage.
See [abstract classes and DI](./functions-and-generics#abstract-classes-and-dependency-injection)
for a runnable example.

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

## Typed decorators

Decorators attach registration behavior to classes, fields, constructors,
methods, getters, setters and constructor/method parameters. They are ordinary
typed functions, not a built-in dependency injection or routing framework:

<<< ../snippets/decorator-construction.km{ts}

This prints `Hello, Kinmokusei`. `@Register` runs during generated Go package
initialization, after module bindings have been initialized, and stores the
checked construction adapter. Construction still goes through the ordinary
typed class constructor. It does not run the decorator again.

A decorator can also be a factory: `@Route("/users")` calls a module-scope
function returning a decorator callback. Imported and re-exported decorators
work the same way. Applications run in source order; inheritance does not
automatically reapply a base decorator to descendants.

| Callback parameter | Accepted target |
| --- | --- |
| `ClassDecoratorContext` | Class |
| `FieldDecoratorContext` | Field |
| `ConstructorDecoratorContext` | Constructor |
| `MethodDecoratorContext` | Ordinary method |
| `GetterDecoratorContext`, `SetterDecoratorContext` | Matching accessor |
| `ParameterDecoratorContext` | Constructor or method parameter |
| `DecoratorContext` | Any of these targets |

The callback must take one context and return `void`. A context for the wrong
target is a compile error. Contexts expose checked names, type descriptions,
identities, visibility, static status, parameter position and inheritance/
override identities. Use identities, not just class names, as registry keys
when different modules can declare the same name.

Contexts also expose `construct`, `invoke` and `invokeStatic` adapters. Check
`constructible`, `invocable` or `staticInvocable` first; the corresponding
`*UnavailableReason` explains a disabled adapter. Private/protected and abstract
methods do not acquire public invocation authority. Generic class construction
and generic method invocation are not supported by these adapters; static
accessors independent of class type parameters remain usable.

Adapters transport arguments/results through opaque `DecoratorValue` values.
`decoratorValue<T>(value)` records a concrete source type contract;
`decoratorValueAs<T>(value)` checks that contract and returns `Result<T>`.
Wrong arity, receivers and argument types produce adapter errors rather than
unchecked casts. For interface-based DI, box at the interface contract, for
example `decoratorValue<Service>(implementation)`, instead of relying on an
implicit cast during extraction. Nullable, numeric-width and nested collection
contracts remain checked; open type parameters cannot be boxed.

Invocation follows ordinary virtual dispatch. `Result` failures propagate
through the adapter, a void result has no ordinary payload, and an ordinary
multiple-result method returns a boxed `DecoratorValue[]` in declared order.
These registration/adaptation primitives let external libraries implement their
own DI containers and route registries without framework-specific compiler rules.

A struct communicates copied domain state; a class communicates shared identity and constructor invariants; an interface communicates behavior independently of either representation. [Failures, results, and exceptions](./errors-results-exceptions) adds the completion paths those APIs can expose.
