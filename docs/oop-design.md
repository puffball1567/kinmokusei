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
methods, class type parameters, allocations, and callbacks. An instance
initializer may read an earlier explicitly initialized field of the same class
or an accessible inherited instance field with `this.field`. The inherited
field is read after the base constructor has completed. It cannot assign to an
instance field, read a later or uninitialized field of its own class, capture
`this` in a callback, or use `super` or constructor-local parameters; other
receiver-dependent initialization belongs in the constructor. A module binding
with the same name as a constructor parameter still resolves to the module
binding in a field initializer. Fields with valid initializers satisfy definite
initialization checks, while other non-null reference fields still need a
constructor assignment. Native struct defaults remain unsupported; static field
initialization is described below.

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

Method signatures are invariant: implementations, overrides, and inherited
same-name methods must agree on both parameter and result types. This includes
`null` qualifiers inside collections, objects, callbacks, and generic arguments.
For example, `read(): Leaf | null` cannot implement `read(): Leaf`, even though
both results use pointers in Go. Narrowing a nullable parameter is also rejected;
ordinary assignment compatibility does not determine method conformance.
A generic method cannot satisfy a non-generic method requirement.

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
Embedding an imported Go contract retains its Go package identity and module
dependency in the generated Go API.

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
- Mutable static fields lower to type-prefixed package variables; `static const`
  members lower to typed Go constants.
- Mutable static state is discouraged and should require an explicit synchronization/lifecycle design.

```ts
class Counter<T> {
  public static count: int = 0;
  constructor(public value: T) { Counter.count++; }
}
```

Every static field requires an explicit type and initializer. Access it through
the class name (`Counter.count`), not an instance. A declaring class has one
storage location, shared by all generic instantiations and descendants.
Descendants cannot redeclare that field. Public, protected and private visibility
apply normally. Class type parameters, `this`, `super` and constructor parameters
are out of scope in a static initializer; module bindings remain available.

Initializers follow generated Go package dependency order and run once, not per
construction. Initialization cycles are compile errors, including dependencies
through static accessors, methods and constructors. Fields are addressable and
support ordinary assignment, compound updates and collection operations. As with
module variables, bind a nullable static value locally before narrowing it.
Updates do not acquire locks or become atomic automatically.

A public field such as `Counter.count` emits the Go package variable
`CounterCount`; private and protected fields use unexported names. Static state
is not part of instance structs or JSON. Generated-name collisions are diagnosed,
including local bindings that would shadow the selected Go variable.

### Class constants

```ts
class Limits<T> {
  public static const size: int = 32;
  public static const label: string = "items";
  private static const extra: int = 1;
  public static const capacity: int = Limits.size + Limits.extra;
}
```

Class constants require `static const`, an explicit scalar type, and an
initializer whose compile-time value can be established. Numeric, string and
boolean types (including named scalar types) are supported. Use the class name
without type arguments; inheritance, visibility and module lexical scope follow
static fields. Class type parameters are not available. Forward references to
other constants are allowed; cycles are rejected.

Constant operations preserve Go types, representability and floating-point
rounding. These values work in numeric bounds, allocation sizes, switches,
generic calls and further constant expressions. Runtime calls, mutable bindings,
accessors, collections and object references cannot initialize constants.
Constant array `len`/`cap` is permitted without reading array elements, but Go's
dependency-cycle restrictions still apply to those references.
Assignments, updates and address-taking are errors. Unlike a module `const`
binding holding an immutable runtime value, `static const` always requires a
compile-time constant.

`Limits.size` emits `const LimitsSize int = 32`, usable as a constant by Go
consumers, including Go array lengths. Kinmokusei array **type** lengths still
require integer literals; this declaration syntax does not extend them to
expressions. Enum-member values are not yet evaluated by this scalar-constant
initializer checker. Constants occupy no per-instance storage and are excluded
from JSON.

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

## Abstract classes

Since v0.4.0, an `abstract class` combines a nominal contract with
shared fields, constructors, and concrete methods. It can be a parameter, field,
return type, collection element, or dependency-injection type. A concrete
descendant is implicitly assignable to that base type, retaining identity and
virtual dispatch. Interfaces remain explicit method-only contracts; an abstract
class participates in the ordinary single-class inheritance hierarchy.

```ts
abstract class Repository<T> {
  public abstract function find(id: int): T;
  public function first(): T { return this.find(0); }
}
class Names extends Repository<string> {
  public override function find(id: int): string { return "name"; }
}
class Service {
  constructor(private repository: Repository<string>) {}
  public function run(): string { return this.repository.first(); }
}
```

An abstract method has a signature and terminator, but no body. It is implicitly
virtual and must be public or protected. Concrete overrides must use `override`
and preserve visibility and the full signature, including nullable types.
Abstract intermediate classes may leave inherited abstract methods unresolved,
or redeclare an inherited virtual method with `abstract override`. Every
nonabstract class must implement all abstract methods, even if never constructed.
An abstract class may have no abstract methods, but `new` is still forbidden.
`abstract` cannot combine with a final class or a static/final method.
Method-level type parameters remain unsupported on virtual/abstract methods;
class type parameters are supported.

An abstract class may explicitly `implements` source or Go interfaces. It must
declare their methods as abstract or concrete (or inherit matching declarations);
missing interface signatures are not synthesized. `super` cannot access an
abstract method because no base implementation exists.

Construction retains phase-local virtual dispatch. Direct access to an abstract
method on `this` in a constructor is rejected. An indirect call through a helper,
callback, or escaped receiver can still reach an abstract slot during that phase;
it **panics**, rather than invoking an uninitialized descendant or returning a
zero value. Do not call abstract-dependent behavior during construction; invoke
it after construction completes. This is not a whole-program escape analysis.
Generated Go omits the public `NewAbstractClass` factory but retains the internal
base initializer. A Go-created zero-value abstract struct also panics if an
unimplemented slot is invoked; external Go code can bypass frontend construction
rules just as it can bypass ordinary class initializers.

## Properties

Concrete instance properties use `get` and `set` accessors:

```ts
class Counter {
  private raw: int = 0;
  public get value(): int { return this.raw; }
  private set value(next: int) { this.raw = next; }
  public function increment(): void { this.value++; }
}
const counter = new Counter();
counter.increment();
const value = counter.value;
// counter.value = 3; // private setter
```

A getter has no parameters and an explicit non-void return type. A setter has
one typed, non-rest parameter and returns void; its `: void` annotation is
optional. Paired types must match exactly, including nullability and generic
arguments. Each accessor has its own public/protected/private visibility;
the default is private. Getter-only properties are read-only and setter-only
properties are write-only. Properties do not declare storage or initialize
backing fields on behalf of a constructor.

Property reads and writes lower to method calls. Public `get value` / `set value`
generate `GetValue()` / `SetValue(value)` for ordinary Go consumers. These names
cannot collide with other declared or inherited members. Nonpublic accessors
remain unexported. Properties are not JSON fields and are not addressable;
value structs/arrays returned by a getter are copies, while returned references
and slices retain their ordinary aliasing behavior.

`receiver.value += rhs`, other compound assignments, and `++`/`--` evaluate the
receiver once, call the getter once, evaluate the right-hand side, then call the
setter once. An exception or panic stops that sequence. Both accessors must be
accessible for updates. Inherited properties retain their access rules and
generic substitution; `super.value` operates on the existing base instance.
Ordinary class fields and methods cannot hide a property.

Accessors are synchronous calls, not stable storage reads. Repeated nullable
getter reads are not narrowed by an earlier null check: bind the result locally
and check that binding. Accessor calls invalidate potentially aliased field
proofs, just like ordinary method calls. Property types cannot be `Result` or
`Task`; there is no implicit error propagation or awaiting. Bodies retain
ordinary explicit statements and exception/panic behavior. Prefer explicit
methods for operations whose effects should be visible at the call site.

### Static properties

Static accessors use the same signature and paired-type rules, but are accessed
through a class name rather than an instance:

```ts
let configuredLimit: int = 10;
class Settings {
  public static get limit(): int { return configuredLimit; }
  public static set limit(next: int) { configuredLimit = next; }
}
Settings.limit += 2;
```

Both halves of a pair must be static. Visibility is checked independently,
including private and protected access from class methods and subclasses.
Inherited access uses the declaring class's accessor; it does not create a new
property per subclass. Static accessors cannot be virtual, abstract, overridden,
or hidden by a descendant. Use a class name, not `this` or `super`, to access them.
They do not satisfy instance interface contracts.

The generated public API is `SettingsGetLimit()` / `SettingsSetLimit(int)`.
These are package functions, not instance methods. Generated names must not
collide with other package declarations. Updates call the getter, evaluate the
right-hand side, then call the setter; ordinary assignment calls only the setter.
No locking is added: a compound update is not automatically atomic. Global
initialization cycles through static accessor and method bodies are rejected.
If a local binding or type parameter hides the generated Go function name at
a static member access, compilation reports the collision; rename that binding.

A generic class can also declare static accessors, but its type parameters are
out of scope in their signatures and bodies. Access with `Box.value`, without
type arguments. There is one accessor implementation, not one per `Box<T>`.
This restriction does not change generic static **methods**, which retain their
existing explicit or inferred generic call syntax. Static properties declare
no storage; module bindings or static fields supply backing state when needed.

### Interface properties

Interfaces declare accessor signatures without bodies or visibility modifiers;
all required accessors are public. The ordinary accessor arity and exact paired
type rules apply, including when separate generic ancestors supply the getter
and setter.

```ts
interface Readable<T> { get value(): T; }
interface Writable<T> { set value(next: T); }
interface Cell<T> extends Readable<T>, Writable<T> {}

class Box<T> implements Cell<T> {
  constructor(private raw: T) {}
  public get value(): T { return this.raw; }
  public set value(next: T) { this.raw = next; }
}
function increment(cell: Cell<int>): int {
  cell.value++;
  return cell.value;
}
```

As with methods, implementation is explicit. Required accessors may be inherited
from a class or declared abstract by an abstract implementer. A field or ordinary
method named `getValue` does not satisfy a source `get value` contract. A
getter-only interface exposes only reads even if the concrete class also has a
setter; setter-only contracts work analogously. These interfaces can be used for
DI without inheriting a shared class implementation.

Generated Go interfaces expose `GetValue`/`SetValue`, so handwritten Go types with
those methods can implement the generated API. A source interface can also
extend an imported Go interface with the same accessor methods when signatures
match exactly. Source method/property name collisions and generated accessor
name collisions are diagnosed. Property evaluation, nullability, mutation
effects and non-addressability are unchanged when accessed through an interface.

### Virtual and abstract accessors

Each accessor independently supports the ordinary `virtual`, `override`, `final`
and `abstract` method rules. Overrides must preserve its visibility and exact
type, including nested nullability. Nonvirtual accessors cannot be overridden.
Overriding only a getter preserves the inherited setter, and vice versa; adding
a previously absent accessor to an inherited property is not supported. A final
override closes that accessor, not the other half of the property.

```ts
abstract class Setting<T> {
  public abstract get value(): T;
  public abstract set value(next: T);
}
class Count extends Setting<int> {
  private raw: int = 0;
  public override get value(): int { return this.raw; }
  public override set value(next: int) { this.raw = next; }
}
function increment(setting: Setting<int>): int {
  setting.value++;
  return setting.value;
}
```

Abstract accessors have signatures without bodies and are implicitly virtual.
A concrete descendant must implement each abstract accessor; an abstract
intermediate class may redeclare an inherited virtual accessor with
`abstract override`. These abstract classes work as ordinary DI types.

Access through a base reference dispatches to the most-derived override,
including updates and public Go `GetValue`/`SetValue` method calls or bound
method values. `super.value` deliberately bypasses virtual dispatch and uses
the inherited implementation. Reading or writing an abstract accessor through
`super` is an error, including an abstract getter needed for a compound update.

Construction uses the same phase-local dispatch as ordinary methods. Direct
access to an abstract accessor on `this` during construction is rejected;
indirect access through helpers can still reach an unimplemented slot and
panics. An abstract getter is not read by a simple assignment through a concrete
setter. Getter/setter bodies do not supply definite-initialization proofs for
backing fields. Go-created zero values use the ordinary wrapper fallback, and
unimplemented abstract slots fail explicitly rather than returning zero values.

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
- Abstract classes/methods, explicit concrete implementation checks, and generic
  abstract-base dependency injection are implemented in v0.4.0.

### Later candidates

- Additional constant-expression contexts, including nonliteral array type lengths.
- Discriminated-union integration.

### Out of scope

- Multiple class inheritance.
- Prototype mutation and monkey patching.
- Runtime metaclasses.
- Pervasive implicit dynamic dispatch.
