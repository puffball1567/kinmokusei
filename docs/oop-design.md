# Object-oriented design

## Goal

OnsenTamago provides familiar classes and interfaces while keeping their Go lowering explicit and predictable. OOP syntax is a static authoring model, not a JavaScript prototype system.

## Implemented OOP foundation

### Classes

Classes are reference types and lower to Go structs plus pointer-receiver methods and constructor functions.

```ts
class Counter {
  private value: int;

  constructor(initial: int) {
    this.value = initial;
  }

  public function increment(): void { this.value++; }
  public function current(): int { return this.value; }
}
```

`new Counter(1)` returns a class reference. Assignment aliases the same instance; it does not copy class state.

### Encapsulation

- `public` maps to exported Go-facing members where a public boundary is generated.
- `private` is restricted to the declaring class/module rules enforced by the frontend.
- `protected` is accessible in the declaring class and its descendants, but not
  from free functions or unrelated classes. This lexical descendant rule also
  applies to constructor fields and static methods. Protected members lower to
  unexported Go names and cannot satisfy a public OnsenTamago or Go interface.

### Interfaces and polymorphism

Interfaces define method contracts. A class conforms only through explicit `implements`; accidental structural implementation is not accepted as an OnsenTamago declaration.

```ts
interface Reader {
  function read(index: int): string;
}

class Text implements Reader {
  public function read(index: int): string { return "value"; }
}
```

Interface-typed values lower to Go interfaces and dispatch through the Go interface method set. Classes may also explicitly implement imported Go interfaces when signatures match after lowering.

### Composition

Composition is the preferred state/implementation reuse mechanism before inheritance:

```ts
class Service {
  constructor(private repository: Repository) {}
}
```

Fields remain ordinary named fields; embedding and promoted Go methods are not introduced implicitly.

### Static members

- Static methods lower to stable type-prefixed package functions.
- Immutable static constants may lower to Go constants/package variables.
- Mutable static state is discouraged and should require an explicit synchronization/lifecycle design.

## Deliberate differences from TypeScript/JavaScript

- No prototype chain or prototype mutation.
- No runtime field creation.
- No dynamic `this` binding.
- No implicit method binding when a method value is extracted.
- No dynamic class patterns that cannot be represented predictably in Go.
- No arbitrary runtime decorator rewriting in the initial language.

## Single inheritance

The implemented inheritance model is explicit:

```ts
class Animal {
  public virtual function speak(): string { return "..."; }
}

class Dog extends Animal {
  public override function speak(): string { return "woof"; }
}
```

Required semantics:

- A derived class reuses base state and non-private implementation.
- A derived instance safely upcasts to its base type.
- Only methods declared `virtual` dispatch to derived overrides through a base reference.
- Replacing a method requires explicit `override`; accidental replacement is an error.
- `super` directly invokes base constructors/methods.
- A class extends at most one class and may implement multiple interfaces.
- Reference identity survives upcasts and checked downcasts.

Base constructors run before derived constructor state and bodies. A required
`super(...)` call is the first derived-constructor statement; a zero-argument
base constructor is called implicitly when omitted. During construction,
virtual calls dispatch to the class whose constructor phase is currently
running, and the internal dispatch target advances after each base phase.
Generated upcast helpers evaluate their operand once, preserve nil, and return
the stable embedded base-state address, preserving identity across repeated
upcasts.

The existing assertion forms also handle class downcasts:

```ts
const [dog, ok] = animal as? Dog;
const requiredDog = animal as! Dog;
```

`as?` returns `(value, boolean)` and produces `(nil, false)` for nil or an
incompatible dynamic class. `as!` returns the derived reference and panics on
failure. A downcast may only move from a base class to one of its descendants;
same-type assertions, upcasts, unrelated hierarchies, and non-class targets are
compile errors. The operand is evaluated once, and successful round trips
preserve the original derived identity. Downcasting to an intermediate class
also succeeds when the dynamic object belongs to a deeper descendant.

Protected virtual methods may be overridden with protected visibility. An
override must preserve the inherited visibility exactly, keeping the generated
virtual method set stable.

Inheritance may be closed explicitly:

```ts
final class Leaf extends Base {
  public final override function run(): int { return super.run(); }
}
```

A `final class` cannot be extended. A `final override` still participates in
virtual dispatch but cannot be overridden again. `final` is intentionally not
accepted on a new nonvirtual method: ordinary nonvirtual methods are already
not overridable, so that spelling would add no semantic constraint.

Generated Go exposes nil-preserving hierarchy conversions with stable names:

```go
animal := hierarchy.UpcastGuideDogToAnimal(guide)
dog, ok := hierarchy.DowncastAnimalToDog(animal)
guide = hierarchy.MustDowncastAnimalToGuideDog(animal)
```

Public virtual methods are dispatch wrappers, while uniquely named unexported
methods hold the implementation selected by `super`. Consequently, calling
`animal.Speak()` from another Go package still reaches the most-derived
OnsenTamago override. Bound Go method values behave the same way. A nil receiver
or a Go-created zero-value base object falls back to the base implementation;
constructed objects dispatch through their initialized dynamic target. Public
conversion names participate in generated-name collision checking.

JSON layout policy remains to be completed.

## Abstract classes and properties

Abstract classes are not required for the initial model. Interfaces provide contracts, ordinary classes provide shared state, and delegation/package functions provide shared behavior. Abstract classes may be reconsidered only after inheritance proves insufficient without them.

Getter/setter properties are also future work. If added, they must have explicit lowering and cannot hide arbitrary asynchronous or fallible behavior behind field-looking syntax.

## Stages

### Implemented MVP

- Class, constructor, reference identity.
- Public/private fields.
- Instance/static methods.
- Interface and explicit `implements`.
- Interface polymorphism and composition.

### Inheritance stage

- Single `extends`, base reuse, implicit upcast, `virtual`, `override`, `super`,
  checked/forced downcasts, multiple interfaces, and multi-level
  constructor/identity/dispatch tests are implemented.
- Protected fields/methods and final classes/overrides are implemented.

### Later candidates

- Getter/setter properties.
- Discriminated-union integration.
- Abstract classes.

### Out of scope

- Multiple class inheritance.
- Prototype mutation and monkey patching.
- Runtime metaclasses.
- Pervasive implicit dynamic dispatch.
