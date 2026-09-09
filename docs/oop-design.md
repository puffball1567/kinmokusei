# Object-oriented design

## Goal

Kinmokusei provides familiar classes and interfaces while keeping their Go lowering explicit and predictable. OOP syntax is a static authoring model, not a JavaScript prototype system.

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

### Instance field initializers

Fields can have an explicitly typed default expression:

```ts
class Bucket<T> {
  private items: T[] = [];
  public function add(value: T): void { this.items = append(this.items, value); }
}
```

Each `new Bucket<int>()` evaluates its own initializers; defaults are not shared
class-level values. An explicit constructor is optional unless the base class
requires arguments. Initialization proceeds through the base constructor, then
the current class's field initializers in declaration order, constructor-field
parameter assignments, and finally its constructor body. An exception or panic
stops that sequence; later fields and bodies are not evaluated.

Initializers can use module bindings, ordinary function calls, accessible static
methods, class type parameters, allocations, and callbacks. They cannot reference
`this`, `super`, or constructor-local parameters, including through a callback;
receiver-dependent initialization belongs in the constructor. A module binding
with the same name as a constructor parameter still resolves to the module
binding in a field initializer. Fields with valid initializers satisfy definite
initialization checks, while other non-null reference fields still need a
constructor assignment. Native struct defaults and static fields remain separate,
unsupported features.

Generated `NewClass(...)` functions perform initialization. Constructing a Go
struct literal directly does not run source-language initializers. JSON decoding
retains the existing rule of updating an already constructed instance.

### Encapsulation

- `public` maps to exported Go-facing members where a public boundary is generated.
- `private` is restricted to the declaring class/module rules enforced by the frontend.
- `protected` is accessible in the declaring class and its descendants, but not
  from free functions or unrelated classes. This lexical descendant rule also
  applies to constructor fields and static methods. Protected members lower to
  unexported Go names and cannot satisfy a public Kinmokusei or Go interface.

### Interfaces and polymorphism

Interfaces define method contracts. A class conforms only through explicit `implements`; accidental structural implementation is not accepted as a Kinmokusei declaration.

```ts
interface Reader {
  function read(index: int): string;
}

class Text implements Reader {
  public function read(index: int): string { return "value"; }
}
```

Interface-typed values lower to Go interfaces and dispatch through the Go interface method set. Classes may also explicitly implement imported Go interfaces when signatures match after lowering.

Source interfaces can extend multiple source interfaces, including generic
contracts and forward declarations:

```ts
interface Reader<T> { function read(): T; }
interface Writer<T> { function write(value: T): void; }
interface Store<T> extends Reader<T>, Writer<T> {}

class Cell<T> implements Store<T> {
  constructor(private value: T) {}
  public function read(): T { return this.value; }
  public function write(value: T): void { this.value = value; }
}
```

`Store<T>` embeds its bases in generated Go. Implementing a child contract
requires all inherited methods and also permits use through its ancestors.
Interface-to-ancestor assignment preserves identity and supports nullable
values. Generic inference follows declared interface ancestry and explicit
class implementations. Unrelated interfaces still require a declared
relationship; structural similarity alone does not grant source conformance.
When several ancestor instantiations could infer different arguments, provide
explicit or partial type arguments to select a contract. The compiler emits
resolved arguments where Go cannot infer them from interface methods alone,
including marker interfaces with no methods.

Diamond inheritance and same-signature method redeclarations are accepted.
Cycles, duplicate direct bases, and incompatible same-name signatures are
rejected. Generic substitution follows each inheritance edge, even when type
parameters are reordered. Completion, signature help, navigation, and rename
include inherited source contracts.

Source interfaces may also extend imported runtime Go interfaces:

```ts
import go fmt from "fmt";
interface Named extends fmt.Stringer {}
class Label implements Named {
  public function string(): string { return "label"; }
}
function describe(value: Named): string { return value.String(); }
```

Inherited Go methods retain their exported spelling (`String`), while class
implementations are matched by the emitted public Go name (`string` emits
`String`). Method values and virtual overrides retain ordinary dispatch. Source
interfaces and their declared class implementations can upcast to their Go
ancestors without wrappers; Go values do not implicitly become source interfaces.
Generic imported contracts, variadic methods, and Go multiple results are
preserved; a compatible source `Result<T>` method can implement `(T, error)`.
Generic source wrappers retain native class identity and nullable arguments.
Direct Go type arguments retain the existing Go-representability restrictions.

Conflicts are checked by emitted Go name as well as source spelling. Unexported
Go methods, type-set-only constraints, and unsupported interop method signatures
are rejected as bases. The unsafe interop policy also applies to inherited
signatures. Completion and checked-call signature help include inherited Go
methods; these are external contracts, not renameable source declarations.
Embedding an imported Go contract remains a Go-module dependency for future KIR
backend-capability tracking, not an implicitly portable C++ or Nim library.

### Iteration over class-defined collections

A class can expose a bound iterator method without allocating an intermediate
slice. The callback receives each value and returns whether iteration should
continue:

```ts
class Sequence<T> {
  constructor(private items: T[]) {}
  public function iterate(yield: (value: T) => boolean): void {
    for (const item of this.items) {
      if (!yield(item)) { return; }
    }
  }
}

function sum(sequence: Sequence<int>): int {
  let result = 0;
  for (const value of sequence.iterate) { result += value; }
  return result;
}
```

Virtual iterator methods dispatch through the dynamic class implementation;
`super` can invoke the base iterator. Generic class elements retain their
reference identity. Always stop when `yield` returns false: the generated Go
runtime detects protocol violations. See [range semantics](language-design.md#range).

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
- Static fields/constants are not implemented; use module constants or
  explicit static methods today.
- Mutable static state is discouraged and should require an explicit synchronization/lifecycle design.

## Deliberate differences from TypeScript/JavaScript

- No prototype chain or prototype mutation.
- No runtime field creation.
- No dynamic `this` binding.
- Extracted method values retain their receiver using Go method-value semantics.
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
also succeeds when the dynamic object belongs to a deeper descendant. Generic
hierarchies apply the same rule through typed projections: every ancestor and
target type argument must match, including remapped or concretely fixed base
arguments.

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
Kinmokusei override. Bound Go method values behave the same way. A nil receiver
or a Go-created zero-value base object falls back to the base implementation;
constructed objects dispatch through their initialized dynamic target. Public
conversion names participate in generated-name collision checking.

Public class state uses JSON tags that preserve Kinmokusei field names.
Private and protected fields, identity roots, and virtual-dispatch slots remain
unexported and are not serialized. Decode into an instance created by its
constructor so its private invariants, hierarchy identity, and virtual-dispatch
slots remain intact. Automatic `encoding/json` allocation does not run a class
constructor. A user-defined `unmarshalJSON(data: byte[]): error` method may
define allocation-time initialization when a type needs that behavior.

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
