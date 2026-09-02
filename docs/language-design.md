# Language design

## Principles

OnsenTamago is a TypeScript-inspired statically typed language designed for predictable Go generation. It is not an attempt to reproduce TypeScript semantics in Go.

TypeScript inspiration means familiar type annotations, classes, interfaces, arrow functions, and object literals. It does not imply syntax or runtime compatibility. OnsenTamago may add explicit integers, Go interop types, result handling, concurrency constructs, and other features suited to generated Go.

Priorities are:

1. Predictable generated Go and runtime behavior.
2. Explicit types, ownership, mutation, and external boundaries.
3. Few implicit conversions.
4. Concise ordinary backend code.

### Predictability

- Types and declarations must reveal value versus reference behavior and copy versus alias behavior.
- Classes are consistently reference types. Nominal value objects use the separate implemented `struct` syntax.
- Pointers, nullability, ownership transfer, asynchronous work, dispatch, and interop boundaries should not be hidden by default.
- A syntactic right-hand side is evaluated once unless a construct explicitly documents otherwise.
- Errors must have an explicit path such as raw Go `error`, `Result`, or panic; paths must not silently convert into one another.
- Unsupported operations are rejected at their source location rather than approximated with a similar feature.
- Diagnostics are compatibility-sensitive behavior and must be regression-tested.

## Example direction

```ts
import { App, Context } from "ontama/http";
import { UserService } from "./user-service";

class UserController {
  constructor(private users: UserService) {}

  public function show(ctx: Context): Result<void> {
    const id: string = ctx.params.required("id");
    const user: User | null = this.users.find(id)?;
    if (user === null) {
      return ctx.json({ error: "user not found" }, 404);
    }
    return ctx.json(user, 200);
  }
}
```

The core `Result`, postfix `?`, and nil-backed nullable constructs in this example are implemented. The referenced application libraries remain design direction.

## Types

### Built-in types

| OnsenTamago | Go | Notes |
|---|---|---|
| `boolean` | `bool` | No truthy conversion |
| `string` | `string` | UTF-8 bytes |
| `int` | `int` | Independent integer type |
| `int8` | `int8` | Explicit signed width |
| `int16` | `int16` | Explicit signed width |
| `int32` | `int32` | Explicit width |
| `int64` | `int64` | Explicit width |
| `uint` | `uint` | Machine-width unsigned integer |
| `byte` | `byte` | Unsigned 8-bit binary data |
| `uint8` | `uint8` | Alias of `byte` |
| `uint16` | `uint16` | Explicit unsigned width |
| `uint32` | `uint32` | Explicit unsigned width |
| `uint64` | `uint64` | Explicit unsigned width |
| `float32` | `float32` | Explicit width |
| `float` | `float64` | Default floating type |
| `number` | `float64` | Alias of `float` |
| `float64` | `float64` | Explicit spelling of `float` |
| `T[]` | `[]T` | Slice |
| `[N]T` | `[N]T` | Fixed array; value-copy semantics |
| `Map<K, V>` | `map[K]V` | Comparable keys only |
| `Result<T>` | `(T, error)` | Function/method return effect; `Result<void>` lowers to `error` |
| `T | null` | same nil-backed Go reference type as `T` | Checked nullable reference |

Numeric types never implicitly widen or cross signedness. Use an explicit conversion. `uint8` is identical to `byte`, matching Go's alias. Integer literals are untyped in context and default to `int` when inferred without another expected type.

Explicit conversions use Go convertibility rules for representable source and
target types. In particular, `string(bytes)`, `string(runes)`, and conversions
from named Go slices whose underlying type is `[]byte` or `[]rune` preserve Go
behavior. Integer-to-string conversion produces one Unicode code point as Go
does; it is not decimal formatting. Fixed arrays do not convert directly to
strings.

### Go interop types

An imported Go named type remains qualified and preserves its Go identity:

```ts
import go http from "net/http";
import go time from "time";

const timeout: time.Duration = time.Second * 5;
let client: http.Client = http.Client{ Timeout: timeout };
let pointer: *http.Client = &client;
```

Pointers, interfaces, multiple results, variadics, generics, channels, aliases, anonymous structs, and all Go basic types remain Go types at the direct interop boundary. They do not silently become future OnsenTamago null/result/task wrappers.

### Native defined types and aliases

OnsenTamago distinguishes a new nominal type from a transparent alternate name:

```ts
type UserID = distinct string;
alias UserIDText = string;
type Values<T> = distinct T[];
type Lookup<K, V> = distinct Map<K, V>;

function parseID(value: UserIDText): UserID {
  return UserID(value);
}
```

`type Name = distinct T` emits Go `type Name T`. Values keep the new identity,
so an ordinary `string`, another defined string type, and `UserID` are not
implicitly assignable to one another. An explicit conversion uses the type name
as a one-argument call and follows Go convertibility and constant-range rules.
As in Go, a representable untyped constant may be assigned directly to a
defined numeric type. Operators preserve the defined result type and require
compatible operands.

`alias Name = T` emits Go `type Name = T` and is transparent for assignment,
calls, and returns. Aliases may refer forward or name a class reference. Defined
types and aliases work in parameters, results, maps, slices, fixed arrays,
generic calls, `Result` payloads, relative imports, and external generated-Go
APIs. A defined slice or map retains Go's reference-bearing storage behavior;
a defined fixed array retains value-copy behavior.

Defined types may declare unconstrained parameters and must be explicitly
instantiated wherever they are used. The generated definition uses ordinary Go
generics. A parameter used as a map key receives an inferred `comparable`
constraint, so `Lookup<K, V>` above emits
`type Lookup[K comparable, V any] map[K]V`. Type parameters may occur in
slices, maps, fixed arrays, pointers, nested generic defined types, generic
functions, and `Result` payloads. Different instantiations retain their Go
nominal identity and conversion, assignment, comparability, copy, and aliasing
behavior are checked through `go/types`.

Functions, classes, structs, interfaces, and defined types may also state the
constraint explicitly with TypeScript-shaped syntax:

```ts
function equal<T extends comparable>(left: T, right: T): boolean {
  return left === right;
}

struct Keyed<T extends comparable> {
  public key: T;
  public values: Map<T, string>;
}
```

This emits the corresponding Go `T comparable` parameter. Both explicit and
inferred calls are checked before Go generation; slices, maps, and functions do
not satisfy the constraint. Native constraint type sets beyond `comparable`
remain future work rather than being approximated as `any`.

Generic aliases are currently rejected rather than being emitted with only
partial semantic support. A direct parameter underlying type such as
`type Identity<T> = distinct T` is also rejected by Go; wrap the parameter in a
concrete composite type instead.

Distinct defined types may declare Go-compatible value and pointer receiver
methods with the same external receiver syntax as native structs:

```ts
type Score = distinct int;

public function plus(this: Score, delta: Score): Score {
  return this + delta;
}

public function add(this: *Score, delta: Score): void {
  *this += delta;
}
```

Generic receiver declarations bind the receiver's type parameters explicitly.
The binder names may differ from the type declaration and map positionally:

```ts
type Values<T> = distinct T[];

public function size<U>(this: Values<U>): int {
  return len(this);
}

public function push<U>(this: *Values<U>, value: U): void {
  *this = append(*this, value);
}
```

The receiver is not a call argument. Value methods copy the receiver; pointer
methods mutate shared storage and may be selected from an addressable value as
in Go. Method values capture the receiver using Go semantics. Public methods
form the ordinary generated Go method set, so external Go interfaces can use
the value or pointer type directly. Private methods remain module-local.
Aliases, imported receiver declarations, and defined types whose underlying
type is a pointer or interface cannot declare methods.

Distinct types may be recursively defined when at least one slice, map,
pointer, function, or channel boundary makes every cycle finite:

```ts
type Chain = distinct Chain[];
type Tree = distinct Map<string, Tree>;
type Link = distinct *Link;
type Visitor = distinct (next: Visitor) => void;
type Forest<T> = distinct Forest<T>[];
```

Mutually recursive definitions follow the same rule. Their generated forms are
ordinary recursive Go named types, including their methods and external public
APIs. Direct cycles, fixed-array-only cycles, and recursive aliases are rejected
at the source location because they either have infinite size or violate Go's
alias rules.

`type`, `alias`, and `distinct` are contextual declaration words rather than
globally reserved identifiers. The implementation deliberately rejects
`Result`/`Task`/`void` boundaries, distinct definitions over native
classes/structs/interfaces, and generic aliases. Use a transparent non-generic
alias or a native `struct` where those current boundaries apply; unsupported
cases produce source diagnostics instead of approximate Go.

### Native integer enums

An enum is a distinct integer type with members selected through its type
namespace:

```ts
enum Status {
  Pending,
  Running = 4,
  Complete,
}

enum WireCode: uint16 {
  Empty = 0,
  Ready = 41,
  Maximum = 65535,
}

function classify(status: Status): string {
  switch (status) {
    case Status.Pending { return "pending"; }
    case Status.Running { return "running"; }
    case Status.Complete { return "complete"; }
  }
  return "unknown";
}
```

The underlying type defaults to `int` and must be an integer type. The first
implicit member is zero; each later implicit member is one greater than the
previous member, including after an explicit value. Explicit initializers must
be compile-time integer constant expressions. Values outside a fixed-width
underlying type are rejected at their source location. Empty enums, duplicate
members, the blank member name `_`, and unknown members are also rejected.

An enum is nominal: a runtime `int` is not implicitly assignable to `Status`.
Use `Status(value)` or `int(status)` when an intentional conversion is needed;
representable untyped integer literals retain Go-compatible assignment rules.
Enums may be compared and ordered, used as map keys, passed through generics,
returned in `Result` values, switched over, and imported from relative modules.
Members remain qualified in OnsenTamago source, avoiding collisions between
different enums.

Enums may also declare Go-compatible value and pointer receiver methods using
the same external `this` syntax as defined types. A value receiver observes a
copy, while a pointer receiver can mutate an addressable enum variable:

```ts
public function active(this: Status): boolean {
  return this === Status.Running;
}

public function advance(this: *Status): Status {
  *this = Status(int(*this) + 1);
  return *this;
}
```

Generated Go uses a named integer type and typed constants. For example,
`Status.Pending` becomes `StatusPending Status = 0`. This keeps the generated
package readable and lets ordinary Go consumers use the enum without the
OnsenTamago compiler. `enum` is contextual, so it remains usable as an ordinary
identifier outside declaration position.

### Structural object types

Object types may be written explicitly:

```ts
function response(message: string): { message: string, count: int } {
  return { count: 1, message: message };
}
```

Field names and types define identity; declaration order does not. Generated anonymous Go structs use deterministic field ordering, public field names, and JSON tags preserving the OnsenTamago names. Expected object literals diagnose missing, extra, duplicate, and mistyped fields individually.

### Nominal value structs

Native `struct` declarations are implemented as distinct nominal Go value types:

```ts
struct Point {
  public x: int;
  public y: int;
}

function moved(point: Point): Point {
  let copy = point;
  copy.x = copy.x + 1;
  return copy;
}

const origin = Point { x: 0, y: 0 };
```

Assignment, parameter passing, and return copy the entire value exactly as Go does. `&value` and `*pointer` are the explicit way to share mutable struct storage. A copied struct is shallow: class references, slices, maps, pointers, and other reference-bearing fields still refer to the same underlying data after the outer struct is copied.

Struct literals use source field names, evaluate initializers once from left to right, and require every declared field exactly once. Missing, extra, duplicate, and mistyped fields are source diagnostics. `public` fields lower to exported Go field names; `private` fields lower to package-private Go fields. Struct types are never nullable, while an explicit `*StructName` may be `nil` or participate in `T | null`.

Struct methods make receiver identity explicit:

```ts
struct Counter {
  public value: int;

  // A value receiver. Mutating this changes only the receiver copy.
  public function added(delta: int): int {
    this.value += delta;
    return this.value;
  }

  // A pointer receiver. Mutating this changes shared storage.
  public pointer function add(delta: int): void {
    this.value += delta;
  }
}
```

An ordinary struct `function` lowers to a Go value receiver. The contextual `pointer` modifier immediately before `function` lowers to a Go pointer receiver; `pointer` remains available as an identifier everywhere else. As in Go, a pointer method may be selected from an addressable value, fields and value methods may be selected through a pointer, and a pointer method on a non-addressable temporary is rejected. Method values capture their receiver with Go semantics: a value receiver is copied when the method value is formed, while a pointer receiver retains the shared pointer. A nil pointer receiver may be called, but dereferencing it retains ordinary Go panic behavior.

The same methods may be declared outside the struct with an explicit receiver. This is an alternative source organization, not an extension-method mechanism or a different runtime type:

```ts
struct Counter {
  public value: int;
}

public function added(this: Counter, delta: int): Counter {
  this.value += delta;
  return this;
}

public function add(this: *Counter, delta: int): *Counter {
  this.value += delta;
  return this;
}
```

The first parameter must be named `this`; its declared type determines value versus pointer behavior. It is a receiver declaration, not a callable argument, so callers pass only the remaining parameters. The external form does not use the contextual `pointer function` modifier. Visibility defaults to `private`; `public` and `private` may be written before `function`. Nested and external declarations enter one method set, and duplicate names are rejected across both forms. An external receiver must be a native struct declared in the same OnsenTamago module; imported and Go types cannot be retroactively extended.

Two structs with identical fields remain different types. Equality and map-key use require every field to be Go-comparable. Direct or fixed-array recursive containment is rejected; pointer, slice, or map indirection makes recursive shapes finite. Struct fields, nested/external methods, nested structs, fixed arrays of structs, relative-module imports, pointer mutation, empty Go-interface arguments such as formatting calls, source navigation, rename, and document symbols are implemented.

Structs may declare unconstrained type parameters. Instantiations always supply
the complete argument list explicitly; inference belongs to function calls,
not type annotations or literals:

```ts
struct Box<T> {
  public value: T;

  public function get(): T { return this.value; }
  public pointer function set(value: T): void { this.value = value; }
}

function replace(box: *Box<string>, value: string): string {
  const previous = box.get();
  box.set(value);
  return previous;
}

const boxed: Box<int> = Box<int> { value: 42 };
```

Each parameter lowers to Go `any`. Fields, nested collection shapes, value and
pointer receiver signatures, method values, literals, pointers, generic
functions, and `Result` payloads substitute the selected arguments. Different
instantiations such as `Box<int>` and `Box<string>` are distinct types. Value
copying, shallow slice/map/reference sharing, comparability, map-key eligibility,
and recursive pointer indirection remain exactly the corresponding Go generic
struct behavior. Class references and native defined types may be arguments.
Generic structs work across relative imports and as exported generated-Go APIs.

Method-local type parameters remain unsupported because Go does not permit
them. Nested generic struct methods use the enclosing struct parameters.
External methods bind the receiver parameters explicitly, for example
`function get<U>(this: Box<U>): U`; those parameters belong to the receiver and
do not make the method independently generic.

### Fixed arrays, slices, and conversion

Fixed arrays and slices are distinct and never implicitly convert. Assignment and parameter passing copy fixed arrays as Go does. An unannotated literal such as `[1, 2, 3]` is a slice; it becomes a fixed array only under an expected `[N]T` type.

Two- and three-index slicing use Go bounds and aliasing:

```ts
const view = values[low:high];
const limited = values[low:high:max];
```

Bounds are evaluated once. Statically known negative, reversed, or fixed-array-out-of-range bounds are diagnosed early. Dynamic violations retain Go panic behavior. String bounds are byte offsets.

Explicit reverse conversion distinguishes copy and alias:

```ts
const copied: [3]int = copyArray[[3]int](values);
const viewed: *[3]int = viewArray[[3]int](values);
```

`copyArray` returns an independent value. `viewArray` shares backing storage. Both panic like Go when the source is too short.

## Operators

### Precedence

Binary precedence is intentionally compatible with the existing expression grammar while grouping bitwise operators as Go does. From lowest to highest:

1. `||`
2. `&&`
3. equality operators
4. ordering operators
5. `+ - | ^`
6. `* / % << >> & &^`

All binary groups are left-associative. Therefore `a + b << c` means `a + (b << c)`, and `a | b + c` means `(a | b) + c`.

### Bitwise and shift

Integer types support binary `&`, `|`, `^`, `&^`, `<<`, and `>>`, plus unary complement `^`. They lower to the corresponding Go operations.

The language includes machine-width `int` and `uint`, fixed-width signed `int8`,
`int16`, `int32`, and `int64`, plus unsigned `uint16`, `uint32`, and `uint64`
alongside the identical `byte`/`uint8` spellings. They preserve Go's overflow
behavior for dynamic operations. Negative
unsigned constants and fixed-width out-of-range constants are rejected at the
source; signed/unsigned and different-width values require an explicit
conversion. Positive `uint` constant range remains target-dependent and is
validated against the selected Go build target.

- Typed bitwise operands must have identical types; defined type identity is preserved.
- An untyped integer constant may combine with a typed operand only when representable.
- Shift operands must be integers but need not share a type.
- A shift result always has the left operand's type.
- Negative constant shifts and Go's constant-shift implementation limit are diagnosed before generation.
- Dynamic negative shifts retain Go runtime panic behavior.
- Unary `&value` is address acquisition; binary `left & right` is bitwise AND.

### Compound assignment and increment/decrement

Supported update statements are `+=`, `-=`, `*=`, `/=`, `%=`, `&=`, `|=`, `^=`, `&^=`, `<<=`, `>>=`, `++`, and `--`.

- Updates are statements, never value-producing expressions.
- Targets may be identifiers, writable fields, assignable indexes, or pointer dereferences.
- Compound assignment lowers directly to Go so selectors and indexes are evaluated once.
- The corresponding binary operator and final assignment rules must both succeed.
- Strings support only `+=`; remainder and bitwise updates require integers.
- Numeric `++`/`--` also work for imported Go numeric types.
- Constants, methods, string indexes, and nonaddressable temporary fields/array indexes are rejected. Map indexes are writable.
- Fixed-width overflow, negative constant shifts, and constant integer zero divisors are source diagnostics.
- Dynamic integer zero divisors preserve Go panic behavior.

## Collections

Compiler built-ins map directly to Go:

```ts
const values = makeSlice[int](2, 4);
const extended = append(values, 3, 4);
const lower = min(3, 1, 2);
clear(values);
const copied = copy(values, extended);
const size = len(extended);
const capacity = cap(extended);

const lookup = makeMap[string, int]();
delete(lookup, "obsolete");

const [value, present] = lookup["key"];
let [nextValue, nextPresent] = lookup["next"];
[nextValue, nextPresent] = lookup["replacement"];
```

- `len`: strings, arrays, array pointers, slices, maps, and channels.
- `cap`: arrays, array pointers, slices, and channels.
- `append`: returns the slice; it never silently reassigns the original variable.
- `copy`: returns the number of elements copied.
- `delete`: removes a map key; missing keys and nil maps are no-ops.
- A two-name binding or reassignment from `map[key]` performs Go's checked
  lookup and yields `(value, present)`. Missing and nil maps produce the value
  type's zero value and `false`; a stored zero value produces `true`. The map
  and key expressions are each evaluated exactly once. This form accepts
  ordinary, generic, defined, and imported Go map types, but not arrays,
  slices, or strings.
- `clear`: zeroes every slice element or removes every map entry, including
  named Go collections; nil slices/maps remain safe exactly as in Go.
- `min`/`max`: require one or more operands of one ordered numeric or string
  type. Named Go ordered types, mixed typed/untyped numeric literals, NaN,
  signed zero, and left-to-right operand evaluation retain Go behavior.
- `makeSlice`/`makeMap`: typed allocation with static negative/capacity diagnostics and Go dynamic panic behavior. `makeSlice` evaluates length before capacity exactly once; generated code fixes this order independently of Go toolchain intrinsic lowering.

Visible user declarations may shadow compiler built-ins. Generated names remain deterministic and do not confuse a user call with a built-in lowering.

### Range

```ts
for (const value of values) { consume(value); }
for (const [index, value] of values) { consumeAt(index, value); }
for (const [key, value] of lookup) { consumeEntry(key, value); }
```

The single-binding form always binds the value, deliberately differing from the first Go range variable. Slice/array indexes are `int`; map keys keep their type; string indexes are UTF-8 byte offsets and values are `int32` code points. Invalid UTF-8 follows Go `RuneError` behavior. Map order is unspecified. Sources execute once, and range values are copies.

## Variables and inference

- `const` cannot be reassigned; `let` can.
- Parameters and public boundaries require explicit types.
- Local inference is limited to Go-like cases: initialized locals, contextual untyped literals, multiple-result bindings, and generic call arguments.
- Return types and class/interface fields are explicit.
- The language should not introduce whole-program TypeScript-style inference.

```ts
const retryCount = 3;          // int
const ratio: float = 1;        // contextual float
let timeout: time.Duration = time.Second;
```

## Functions and arrows

Functions have explicit parameter and return types. Arrow functions support expression and block bodies, function-type annotations, callbacks, and Go function values. A block body with a non-void result must return on every path.

```ts
const add = (left: int, right: int): int => left + right;
const checked = (value: int): int => { return value; };
```

Rest parameters use TypeScript-shaped slice annotations and lower directly to
Go variadics. The rest parameter must be final. Inside the declaration it is a
slice; calls may pass zero or more individual elements or expand one final
slice. The same rule applies to functions, methods, interface methods, arrows,
function types, and constructors.

```ts
function sum(prefix: int, ...values: int[]): int {
  let total = prefix;
  for (const value of values) { total += value; }
  return total;
}

const direct = sum(10, 1, 2);
const values = [3, 4];
const expanded = sum(10, values...);
```

Top-level functions may declare unconstrained type parameters. Calls infer type
arguments from their ordinary arguments, or provide a leading partial or full
list with either TypeScript-shaped angle brackets or Go-shaped square brackets:

```ts
function identity<T>(value: T): T { return value; }
function second<T, U>(left: T, right: U): U { return right; }

const inferred = identity("onsen");
const explicit = identity<string>(inferred);
const goShaped = identity[string](explicit);
const partial = second<int>(1, goShaped);
```

Each native type parameter lowers to a Go `any` parameter. Repeated uses must
infer the same type, every uninferred parameter must be supplied explicitly,
and an uninstantiated generic function cannot be stored as a function value.
Type parameters are scoped to the declaration and may appear recursively in
slices, fixed arrays, pointers, maps, function types, structural objects,
channels, and result signatures. Generic receiver parameters on external
methods are receiver binders rather than independently callable method type
parameters. Native generic classes, structs, interfaces, and defined types are
implemented; generic defined map keys infer `comparable` where required.

Multiple Go results are locally destructured:

```ts
const [value, err] = strconv.Atoi(text);
[value, err] = strconv.Atoi(other);
```

Multi-values are not first-class values and cannot silently discard `error`.

## Labels and explicit control transfer

Labels are scoped to one function or method. `break label` may target an
enclosing loop, switch, or select; `continue label` must target an enclosing
loop. `goto label` supports forward and backward transfers. Jumps into nested
blocks, over local declarations, or across lowered `try`/`catch`/`finally`
boundaries are rejected before lowering and generated Go validation remains a
second check. Duplicate, undefined, unused, non-enclosing, and wrong-kind
labels are diagnosed at their OnsenTamago source locations. Nullable flow facts
are conservatively cleared at arbitrary transfers.

```ts
function advance(limit: int): int {
  let value = 0;
  goto check;
  next: value++;
  check: if (value < limit) { goto next; }
  return value;
}
```

## Result and error propagation

`Result<T>` is an explicit function or method return effect. It is not a
storable runtime wrapper: `Result<T>` cannot be used for variables, parameters,
fields, collection elements, or nested results. This keeps the generated Go API
direct and predictable:

```ts
import go strconv from "strconv";
import go errors from "errors";

function parse(text: string): Result<int> {
  const value = strconv.Atoi(text)?;
  if (value < 0) { return fail(errors.New("negative value")); }
  return ok(value);
}

function double(text: string): Result<int> {
  const value = parse(text)?;
  return ok(value * 2);
}
```

`Result<T>` lowers to `(T, error)` and `Result<void>` lowers to one `error`.
`ok(value)` returns the success value and a nil error; `ok()` is the
`Result<void>` form. `fail(error)` returns the value type's zero value and the
error. Returning another OnsenTamago result with the identical type forwards it
directly.

Postfix `?` accepts an OnsenTamago `Result<T>`, a direct Go `(T, error)` operation, or
a single Go `error`. It evaluates the operation once and immediately returns a
non-nil error from the enclosing Result function. A non-void result must be the
initializer of one variable; a void result may be an expression statement.
`?` is not accepted in a nested expression or a `for` initializer, so its
control-flow boundary remains visible.

Raw Go multiple results never convert implicitly. For example,
`return strconv.Atoi(text);` is rejected in a `Result<int>` function; use
`const value = strconv.Atoi(text)?; return ok(value);`. Explicit split binding
remains available when the caller wants to inspect the `error` without
propagating it:

```ts
const [value, err] = strconv.Atoi(text);
const [cleanupErr] = cleanup();
```

This is direct multiple-result binding, not object destructuring. A real object
is generated only when the called API actually returns an object.

## Typed exceptions

`try` is reserved for exception handling rather than Result propagation:

```ts
let outcome = "";
try {
  if (invalid) { throw errors.New("invalid input"); }
  outcome = "accepted";
} catch (err: error) {
  outcome = err.Error();
} finally {
  audit(outcome);
}
return outcome;
```

`Exception` is the built-in structured exception root. It has a public
`message: string`, implements Go's `error` contract, and can be extended by
application exception classes. A class may also opt into typed throwing and
catching directly with `implements error`. `throw` accepts any `error` value;
the root `Exception` catch also handles such values by preserving their error
message in a structured exception, including across generated package
boundaries.

Catch clauses are tested in source order, so specific exception classes should
precede their base classes or the generic `error` catch. The checker rejects a
duplicate catch or a derived/specific catch already covered by an earlier base,
`Exception`, or `error` catch. Each catch binding has its declared immutable
block-local type; `_` discards it:

```ts
class NotFoundException extends Exception {
  constructor(message: string) { super(message); }
}

try {
  const user = repository.require(id);
  return response.ok(user);
} catch (err: NotFoundException) {
  return response.notFound(err.message);
} catch (err: Exception) {
  return response.internalError();
} finally {
  repository.close();
}
```

Inside a catch block, bare `throw;` rethrows the currently handled exception
without changing its concrete type. It remains available through nested blocks
and nested exception statements in that catch, but not through a nested
function or arrow-function boundary.

`finally` runs after normal completion, a caught or re-thrown OnsenTamago
exception, a `return` from `try` or `catch`, and an ordinary Go/runtime panic. A
`return` in `finally` replaces an earlier return or exception, matching the
usual structured-exception rule. Branches targeting a loop or switch outside
the exception boundary remain rejected; loops and arrow functions wholly
contained in the block retain their local control flow.

Catch handles only values carrying the structural
`OnsenTamagoExceptionError() error` marker. Bounds panics, ordinary explicit Go
panics, and FFI panics continue unwinding after `finally`. Generated `throw`
values carry this marker, so independently generated packages can interoperate
without importing a compiler runtime. A Go type opts into catch behavior only
by implementing that explicit marker.

Publishable Go output exposes `Exception`, `NewException`, generated exception
classes and constructors, their `Error()` methods, and the ordinary public class
upcast/downcast helpers. Exception transport and return-unwinding helpers remain
package-private, and generated output has no compiler-runtime dependency.

## Value switches

An ordinary switch compares one value against case expressions:

```ts
switch (status) {
  case 200 { handleOK(); }
  case 400, 404 { handleClientError(); }
  default { handleOther(); }
}
```

The subject is evaluated exactly once. Case expressions are evaluated in source
order only until a match is found. Cases do not fall through implicitly.
Explicit `fallthrough;` as the final direct statement of a non-final case enters
the next clause unconditionally without evaluating that clause's case
expressions. It may also enter a following `default` clause, and a non-final
`default` may fall through to the clause after it. `fallthrough` is rejected in
the final case, nested blocks, `if`, type switches, and `select`. `break` exits
the switch explicitly when an early exit from a case block is useful. The subject
and every case must be comparable under the same rules as generated Go.
Duplicate constant cases, incompatible values, non-comparable types, and
multiple defaults are source errors. Class cases compare explicit reference
identity because classes currently lower to pointers; fixed arrays compare by
value when their element type is comparable.

## Classes and interfaces

Classes are reference types. Visibility, constructor fields, instance/static methods, and explicit interface implementation lower to Go structs, constructors, methods, functions, and conformance checks.

Static methods lower to package functions because Go has no type-level methods.
A public static method such as `Meter.create` is exposed to Go consumers as the
idiomatic `MeterCreate`; its parameters and results, including `Result<T>` as
`(T, error)`, remain unchanged. Private and protected static methods lower to
unexported implementation functions, so publishing generated Go does not widen
OnsenTamago visibility. All generated package-level names participate in the
same collision diagnostics before code generation.

```ts
interface Reader {
  function read(index: int): string;
}

class Text implements Reader {
  constructor(private value: string) {}
  public function read(index: int): string { return this.value; }
}
```

Classes may declare type parameters and must be instantiated explicitly in
type positions and `new` expressions. Fields, constructor parameters, instance
methods, method values, nested generic shapes, and implemented generic
interfaces substitute the selected arguments:

```ts
interface ValueReader<T> {
  function read(): T;
}

class Box<T> implements ValueReader<T> {
  constructor(public value: T) {}
  public function read(): T { return this.value; }
  public function set(value: T): void { this.value = value; }
}

const box: Box<string> = new Box<string>("onsen");
const reader: ValueReader<string> = box;
```

Each class parameter lowers to a Go type parameter, and each class value
remains one Go pointer. Consequently assignment preserves reference identity,
while different instantiations such as `Box<int>` and `Box<string>` remain
different static types. `T extends comparable` is available when the class
needs that constraint. Generated constructors and methods form ordinary public
generic Go APIs, and generic classes work across relative imports. Generic
class static methods use the class parameters as function type parameters.
Calls may infer them from arguments or supply them explicitly, and the
generated public API is an ordinary generic Go function. Generic class
inheritance substitutes the selected base arguments through inherited state,
constructors, methods, and interface contracts. A base may use the child type
parameters directly, remap them, or fix some arguments concretely, and the
substitution continues through multi-level hierarchies:

```ts
class Base<T> {
  constructor(protected value: T) {}
  public function get(): T { return this.value; }
}

class Middle<A, B> extends Base<B> {
  constructor(value: B) { super(value); }
}

class Leaf<X> extends Middle<int, X> {
  constructor(value: X) { super(value); }
}

const value: string = new Leaf<string>("onsen").get();
```

Generic inheritance, `virtual`/`override`/`final`, construction-phase-safe
dispatch, implicit upcasts, exact checked/forced downcasts, and the
corresponding public generic Go APIs are supported. Dispatch lowers to a typed
generic Go interface for each virtual owner, preserving substituted parameters,
results, `super` selection, method values, and ordinary external Go calls. A
downcast to an intermediate generic class currently requires the runtime class
to be that exact target; recovering a more-derived generic class through an
intermediate target remains future work.

Native interfaces may declare unconstrained type parameters and must be fully
instantiated wherever they are used. Type parameters may appear recursively in
method parameters and results, including collection, function, native generic
struct, and `Result<T>` positions. An implementing class names each explicit
instantiation, and its public instance methods are checked after substituting
the declared type arguments:

```ts
interface Transformer<T, U> {
  function transform(value: T): U;
}

class Length implements Transformer<string, int> {
  public function transform(value: string): int { return len(value); }
}

function apply<T, U>(transformer: Transformer<T, U>, value: T): U {
  return transformer.transform(value);
}
```

Different instantiations such as `Transformer<string, int>` and
`Transformer<int, int>` retain distinct static identities. A class may name
multiple instantiations when one method set satisfies all of them. Generated Go
uses ordinary generic interfaces with `any` parameters, so interface dispatch,
identity, comparability, method values, and public package use follow the same
Go behavior.

Single class inheritance is explicit. `extends` reuses base state and methods,
only `virtual` methods dynamically dispatch, replacements require `override`,
and `super(...)`/`super.method()` statically select the immediate base. Derived
references implicitly upcast through nil-preserving generated helpers, so nil
behavior and repeated-upcast identity remain stable. A base reference uses
`as? Derived` for a checked `(Derived, boolean)` downcast and `as! Derived` for
a panic-on-failure downcast. Both preserve derived identity, accept deeper
descendants when targeting a non-generic intermediate class, and evaluate the
source once. Generic conversions preserve the exact instantiated ancestor and
target types; the intermediate-target limitation described above applies.
`protected` members are limited to their declaring class and descendants.
`final class` prevents further inheritance, while `final override` closes an
existing virtual method slot. Ordinary nonvirtual methods are already closed
and therefore do not accept a redundant `final` modifier. This is not a
JavaScript prototype model.

The generated Go package exposes `UpcastDerivedToBase`,
`DowncastBaseToDerived`, and `MustDowncastBaseToDerived` functions for every
declared ancestor relationship. Exported virtual method names are dispatch
entry points, so calls and method values obtained through a base pointer retain
OnsenTamago virtual semantics even in an importing Go package. `super` uses a
separate unexported direct-implementation slot.

## Go interface assertions and type switches

Unchecked assertions panic on failure; checked assertions return value plus `boolean`:

```ts
const forced = value as! *strings.Reader;
const [reader, ok] = value as? *strings.Reader;
```

Type-switch cases explicitly state the case-local static type and support `nil` and `default`.
The `const name as Type` or `let name as Type` case shape selects a type
switch. Mixing type-binding and ordinary value cases is rejected. A switch
containing only value expressions, `nil`, or `default` is an ordinary value
switch.

## Concurrency primitives

Low-level Go channel interop is implemented:

```ts
const channel = goChannel[int](1);
channel <- 42;
const [value, open] = <-channel;
closeGoChannel(channel);
```

Directional `GoChannel<T>`, `GoSendChannel<T>`, and `GoReceiveChannel<T>` types preserve Go direction rules. `select`, channel range, raw `go` call statements, and `defer` lower directly to Go.

Structured task expressions retain a result:

```ts
const task: Task<User> = go loadUser(id);
const user: User = await task;

const checked: Task<Result<User>> = go loadChecked(id);
const checkedUser: User = await checked?;

detach go refreshCache();
```

`Task<T>` is a non-escaping local capability. It cannot be copied, reassigned,
captured by a closure, stored in a field/container/global, or exposed through a
function or method signature. Every task binding must be consumed exactly once
with `await` or `detach` on every continuing path. `go f();` remains the raw Go
statement form; `go f()` in an expression creates a tracked task.

The callee and arguments are evaluated once, in source order, before the worker
goroutine starts. A panic in the worker is contained until `await`, which
re-panics in the awaiting goroutine. `detach` explicitly discards a result; a
detached panic is re-raised in its waiter goroutine and remains process-fatal as
ordinary unhandled Go goroutine panics are. Context values are passed explicitly
today. Automatic request-context inheritance and cancellation propagation remain
future runtime work.

## C ABI exports

Ordinary functions use the Go ABI. Explicit exports use a checked stable gateway:

```ts
function add(left: int32, right: int32): int32 {
  return left + right;
}

const sub = (left: int32, right: int32): int32 => left - right;
export c("ontama_add", "ontama_sub") {add, sub};
```

The symbol and target lists pair by position and must have equal lengths.
Targets may be top-level functions or top-level `const` arrows whose return
types are explicit. Both lists accept multiline formatting and trailing
commas. The single-function inline form
`export c("ontama_add") function add(...): int32 { ... }` remains valid.

The outgoing boundary accepts `boolean`, fixed-width scalar parameters (`byte`/`uint8`, `int8`, `int16`, `int32`, `int64`, `uint16`, `uint32`, `uint64`, `float32`, and float aliases), fixed-width native integer enums, and those results plus `void`. Enum parameters and results use their ultimate fixed-width integer representation in the C header and ABI manifest while remaining the named enum inside generated Go. Unknown representable values pass through without implicit validation. Booleans use a stable `uint8_t` transport: zero is false, any nonzero input is true, and outputs are normalized to zero or one. Machine-width `int`/`uint` and enums based on them, strings, collections, objects, classes, interfaces, pointers, and `error` are rejected.

Generated C functions return an `int32_t` status and write values through a final out parameter. Status `0` is success, `1` is a contained panic, and `2` is an invalid argument such as a null out pointer. Symbols are explicit ASCII identifiers and are checked for duplication and generated-name collisions.

## Error and null direction

The low-level interop boundary exposes Go multiple results and raw `error`.
`Result<T>` and postfix `?` provide an explicit higher-level propagation path
without silently converting raw values. Typed exceptions are a separate,
visible `throw` plus `try`/`catch`/`finally` mechanism.
Nil-backed nullable references, direct-assignment-sensitive local flow rules,
structured joins, declaration-identity local escape/capture invalidation, and
definite constructor initialization for non-null reference fields are
implemented. Stable member facts and direct boolean/integer/string plus
`append`/`makeSlice` constructor proofs are implemented. Those proofs propagate
through local, `for`-initializer, same-file global, and explicitly imported
`const` chains; mutable/dynamic bindings are deliberately not treated as
proofs. Broader cardinality proofs remain future work.

## Nullable references

`T | null` marks a nil-backed reference as explicitly nullable. The dedicated
`null` literal belongs to this language-level type system; existing `nil`
remains the raw Go interop literal. They do not assign to one another
implicitly.

```ts
class User { constructor(public name: string) {} }

function maybeUser(present: boolean): User | null {
  if (present) { return new User("onsen"); }
  return null;
}

function displayName(present: boolean): string {
  const user = maybeUser(present);
  if (user === null) { return "missing"; }
  return user.name;
}
```

The initial representation is intentionally allocation-free: `T | null`
lowers to the same Go pointer/interface/slice/map/channel or other nil-capable
type as `T`. Scalars, fixed arrays, structural value objects, `void`, and
`Result` itself cannot be nullable because they have no suitable nil
representation. Nullable types may appear inside `Result`, maps, object fields,
and other representable type positions.

Member access, calls, indexing, slicing, pointer operators, and channel
operations require a prior non-null proof. Direct equality/inequality
comparisons with `null` narrow immutable locals and stable mutable locals or
parameters in the proven branch.
An equality guard whose null branch definitely returns also narrows the
remaining statements; the inverse form works when the else branch definitely
returns. Comparisons work with `null` on either side.

A mutable local or parameter now keeps its declared nullable type separately
from its flow-observed type. Direct assignment of `null` or a maybe-null value
invalidates a non-null fact at that statement; direct assignment of a proven
non-null value establishes a new fact. `if` branches retain only facts shared by
all reachable continuing paths, and value/type switches and `select` use the
same conservative join. Loop headers iterate entry, fallthrough, and `continue`
backedges to a stable nullable state; `break` states feed the loop exit instead.
Terminal paths do not let unreachable later assignments overwrite the state
that actually reaches a backedge or exit.

Escape invalidation is keyed by declaration identity and starts at the source
expression that creates the escape. Taking `&local` invalidates later facts for
that local, and creating an arrow that can mutate an outer local invalidates
later facts for that exact captured declaration. Safe accesses before the
address or arrow expression remain valid, and writes to a shadowing declaration
do not affect the outer fact. Nested arrows propagate mutable captures to the
declaration they can reach. A read-only arrow does not invalidate the caller's
fact, but its body does not inherit a temporary mutable narrowing from the
creation site because it may execute later.

An address of a nullable slot preserves the slot type through dereference, so
`*pointer = null` is checked against `T | null`, matching the generated Go
pointer-to-slot representation. After an address escapes, later direct checks
cannot promote the mutable slot; bind a fresh `const` snapshot when a stable
fact is needed.

Direct class-field paths also carry nullable flow facts. A check such as
`holder.user !== null` narrows repeated access through that exact receiver and
field path, including `this` and nested non-null class fields. Direct non-null
field assignment establishes a fact, and structured joins retain it only when
every continuing path agrees. Because class fields are direct storage rather
than overridable getters, repeated reads are stable until an observable
mutation boundary. Any possibly aliased class-field write, receiver
reassignment, address-taking, closure with possible member mutation, or call
with unknown effects invalidates the reachable fact. A later check or non-null
assignment can establish it again. Go fields and structural object properties
are not included in this class-field rule. Precise call-in-place closure effects
and broader alias precision remain future work.

### Target flow-sensitive nullable analysis

The implemented direct-local transfer and structured joins above are the first
path-sensitive step, not the final usability model. The target checker keeps a
variable's declared type fixed while maintaining a separate nullable flow state
at each control-flow point. A declaration such as `let user: User | null`
therefore always accepts both members of its declared type, while a proven
program point may observe `user` as non-null for member access.

The target transfer rules are:

- A comparison with `null` splits the true and false control-flow edges into
  the corresponding null and non-null states.
- Assignment of a non-null expression establishes a non-null state. Assignment
  of `null` establishes a null state, and assignment of an expression that may
  be null restores the maybe-null state. Assignability is always checked
  against the declared type, not the temporary flow state.
- A control-flow join retains non-null only when every reachable incoming edge
  proves non-null. Early return, exhaustive branches, `switch`, `select`, and
  loops participate in the same reachability model; loops are solved to a
  stable fixed point rather than trusting one iteration.
- Direct local writes invalidate only facts that can reach the write. A later
  check or definitely non-null assignment may establish a new fact.
- Address-taking, mutable capture by a nested function, escaping aliases, and
  concurrent mutation prevent a fact from being used across the possible
  write. A future explicit call-in-place contract may preserve facts for
  non-escaping synchronous callbacks, but ordinary unknown calls must not be
  guessed safe when they can reach mutable storage.
- Direct OnsenTamago class fields may be promoted because they have no custom
  getter or override hook, but all possibly aliased field writes and unknown
  effect boundaries conservatively invalidate them. Future properties or
  overridable accessors require a stronger stability contract; otherwise code
  takes an explicit local snapshot.

For example, the final model accepts the first access and rejects the access
after the write without weakening null safety:

```ts
let user: User | null = findUser();
if (user !== null) {
  use(user.name); // proven non-null here
  user = null;
  use(user.name); // error: the assignment invalidated the proof
}
```

When storage is not promotable, the standard recovery is a snapshot whose
identity cannot change:

```ts
const snapshot = this.currentUser;
if (snapshot !== null) {
  use(snapshot.name);
}
```

Diagnostics should identify the write, capture, alias, or control-flow join
that invalidated a proof and suggest either a new null check or a `const`
snapshot. This analysis changes only compile-time acceptance and diagnostics;
`T | null` keeps its existing allocation-free Go representation.

Separately declared class fields whose non-null type has a nil-backed Go
representation must be initialized by their constructor. Constructor parameter
fields are initialized automatically. For ordinary fields, direct `this.field`
assignments in straight-line code, nested blocks, every arm of an `if/else`,
every completing value/type-switch case, and every completing `select` case
establish definite initialization. A switch without `default` also includes
the unmatched path; a `break` contributes the fields initialized before it,
and unreachable assignments after a terminal statement do not count. A
nonempty `select` has no unmatched path, but every communication/default case
that can complete must initialize the field. Assignment in a loop alone is not
a proof when the loop may execute zero times. Conditionless loops and
compile-time-true boolean expressions are recognized as guaranteed to enter.
The constant evaluator covers boolean negation and logical composition,
integer arithmetic/conversion comparisons, and constant string comparison.
Range is guaranteed for a nonempty array literal, nonempty constant string
expression, positive-length fixed array or pointer to one, `append` that
provably preserves or adds at least one element, and `makeSlice` with a positive
constant length. Every reachable `break`, natural range completion, and
`continue` path must already have initialized the field; possibly empty slices,
zero-length arrays, dynamic conditions or lengths, empty append/spread results,
and terminating infinite loops without a proven exit remain conservatively
rejected. These constant and cardinality facts propagate through transitive
local, `for`-initializer, same-file global, and explicitly imported `const`
declarations. Imported resolution follows normal module visibility and can
continue through private dependency-local constants used by the imported
initializer. Resolution uses declaration identity, so shadowing is respected;
`let`, parameters, and `const` snapshots of dynamic values do not become proofs.
The proof changes acceptance only; generated Go retains the original condition,
evaluation order, bindings, and range source.
Constructors cannot return early, and an intentionally absent field must be
declared as `T | null`.

Operations whose nil behavior is explicitly safe in Go remain null-tolerant for
nullable slices/maps where applicable, including `len`, `cap`, `append`,
`copy`, `delete`, `clear`, and range. Potentially panicking or indefinitely blocking
operations still require narrowing. Raw `nil` APIs retain their existing
low-level Go semantics.
