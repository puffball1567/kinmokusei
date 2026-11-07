---
title: Type-system reference
description: Precise Kinmokusei type representations, identity, assignability, generics, nullability, and special effects.
---

# Type-system reference

## Type syntax

| Family | Syntax | Representation |
| --- | --- | --- |
| Scalar | `int`, `string`, `bstring`, `boolean` | Corresponding Go scalar; text contracts remain distinct |
| Multiple results | `(T, U)` in a callable result position | Separate Go results, not a tuple value |
| Slice | `T[]` | `[]T` |
| Fixed array | `[N]T` | `[N]T` |
| Map | `Map<K, V>` | `map[K]V` |
| Pointer | `*T` | `*T` |
| Channel | `GoChannel<T>` | `chan T` |
| Send channel | `GoSendChannel<T>` | `chan<- T` |
| Receive channel | `GoReceiveChannel<T>` | `<-chan T` |
| Function | `(name: T, ...rest: U[]) => R` | Go function type |
| Structural object | `{ name: T, count: int }` | Deterministic anonymous Go struct |
| Named generic | `Box<T>`, `Lookup<K, V>` | Instantiated native named type |
| Nullable | `T \| null` | Nil-capable `T` without a wrapper |
| Result | `Result<T>` | Function return `(T, error)` effect |
| Task | `Task<T>` | Compiler-tracked local task state |

Parentheses and generic brackets disambiguate nested shapes. `Result` and `Task`
have restricted positions described below.

### Fixed-array lengths

Since v0.4.7, `[N]T` accepts a compile-time
integer expression for `N`, not just an integer literal. The value must be
non-negative and fit the selected Go target's `int`. Integer-valued untyped
floating/complex constants are allowed; typed floating values are not. Length
is part of type identity: `[1 + 2]int` and `[3]int` are the same type.

<<< ../snippets/array-type-lengths.km{ts}

The expression can use source or imported Go constants, enum members and
accessible static class constants, including forward-declared constants.
Ordinary constant arithmetic/conversions and constant `len`, `cap`, `min` and
`max` use the same rules as value expressions. `len("湯")` is three UTF-8 bytes,
not one code point. Compile-time lengths produce ordinary numeric Go array
types; there is no runtime allocation or expression evaluation for the length.

A `const` initialized by a runtime call is not a compile-time constant. Function
parameters and `let` scalar bindings likewise cannot supply runtime lengths:

<<< ../snippets-invalid/array-type-runtime-length.km{ts}

`len`/`cap` of a concrete array or array pointer can be constant even when the
array is mutable or the pointer is nil. An explicitly annotated global array's
initializer is not needed to obtain its length. Calls or channel receives inside
the operand make these operations nonconstant; a generic type-set value also
does not supply a constant length. Declaration cycles are rejected:

<<< ../snippets-invalid/array-type-length-cycle.km{ts}

The older v0.4.6 compiler requires a non-negative representable integer literal
in `[N]T`. Both versions reject negative lengths and integer overflow.

## Built-in correspondence

| Kinmokusei | Go | Notes |
| --- | --- | --- |
| `boolean` | `bool` | No truthiness |
| `string` | `string` | Immutable verified UTF-8 text |
| `bstring` | `string` | Immutable arbitrary bytes; runtime Go string boundary |
| `int`, `uint` | `int`, `uint` | Target machine width |
| `int8`…`int64` | same Go type | Fixed signed width |
| `byte`, `uint8` | `byte`, `uint8` | Identical aliases |
| `uint16`…`uint64` | same Go type | Fixed unsigned width |
| `float32` | `float32` | Fixed width |
| `float`, `number`, `float64` | `float64` | Identical types |
| `complex64`, `complex128` | same Go type | `complex`, `real`, `imag`, and imaginary constants |
| `T[]` | `[]T` | Slice |
| `[N]T` | `[N]T` | Fixed array |
| `Map<K, V>` | `map[K]V` | Comparable key |
| `T \| null` | same nil-backed Go type | Checked nullable reference |
| `Result<T>` | `(T, error)` | Return effect only |

`error` and imported Go named/interface types keep their original Go package identity. Kinmokusei does not clone or structurally approximate them.

## Inference and expected types

Local initialized bindings may infer their type:

```ts
const count = 1;          // int
const label = "hello";   // string
const values = [1, 2, 3]; // int[]
```

Empty collections, `nil`, and `null` need an expected or explicit type when no element/value establishes one. Public fields, parameters, results, and interface members remain explicit. Expected types flow into literals, generic calls, and untyped constants but never perform an implicit numeric conversion.

## Assignability

- Identical types are assignable.
- Scalar `string` may widen to `bstring`; the reverse requires checked decoding.
- Shared storage and function signatures preserve text contracts: `string[]`
  cannot become `bstring[]` by assignment or a no-copy conversion.
- A representable untyped constant may adopt a compatible expected numeric type.
- Numeric widths and signedness do not change implicitly.
- Go named types preserve package identity and Go assignability.
- `alias Name = T` is transparent.
- `type Name = distinct T` is nominal and requires an explicit conversion except for compatible untyped constants.
- Distinct enum types are not interchangeable with their underlying runtime integer.
- Generic instantiations with different arguments are different types.

Go values use `go/types` assignability, addressability, method-set, and constraint rules where possible.

### Recursive generic instantiations

Development after v0.4.11 diagnoses unbounded generic instantiation during
`keika check`, including checks in the editor. For example, this does not reach
Go emission:

<<< ../snippets-invalid/generic-instantiation-cycle.km{ts}

The type argument would grow through `T`, `T[]`, `T[][]`, and so on. The same
rule applies across mutually recursive functions, generic methods, and type
declarations such as `class Node<T> { public next: Node<T[]> | null = null; }`.
The diagnostic identifies a recursive source use, rather than a generated Go
line. Correcting the source clears the editor diagnostic.

Recursion that reuses the same arguments or a finite set of arguments is
allowed, including type-parameter permutations. Transparent aliases are checked
after their Go 1.23 expansion, so an unused, erased alias argument does not
create a false cycle. This restriction concerns compile-time type expansion;
ordinary runtime recursion still needs an appropriate stopping condition.

An incompatible initialized binding is rejected at its declaration:

<<< ../snippets-invalid/type-mismatch.km{ts}

## Explicit conversions

Write the destination type as a one-argument call:

```ts
const wide = int64(count);
const text = string(codePoint);
const id = UserID(raw);
const rawAgain = string(id);
```

- Numeric conversions follow Go width, signedness, truncation, and constant representability.
- `string(integer)` creates the UTF-8 encoding of one Unicode code point; use `fmt` or `strconv` for decimal formatting.
- `string(bstringValue)` and `string(byteSlice)` return `Result<string>` and
  reject invalid UTF-8. Conversion to a native defined `string` type likewise
  returns `Result<ThatType>` when the input needs validation.
- `bstring(bytes)` preserves arbitrary bytes; `string(int32Slice)` encodes
  code points, including compatible named slice types. Converting either
  string type to a byte/code-point slice uses a corresponding
  type or transparent alias, for example `alias Bytes = byte[]; Bytes(text)`.
  Byte conversions preserve raw bytes; decoding raw bytes into code points
  replaces invalid encodings with U+FFFD, and encoding invalid code points
  likewise uses U+FFFD. Checked byte-to-`string` decoding never does that. See
  [strings and Unicode](../book/types-and-values#strings-and-unicode).
- Named defined types convert to/from compatible underlying types explicitly.
- Conversions to a non-null source function signature or method-bearing
  anonymous source interface validate the converted reference at runtime.
  A nil result panics at the conversion's source filename, line and column.
  The operand runs once; this includes function aliases, native defined
  functions and compatible nil-capable generic bounds. Raw Go target types
  retain Go conversion behavior. An interface holding a typed nil pointer
  remains a non-nil interface. Direct `nil` and nullable source operands still
  need the appropriate explicit guard rather than an unchecked conversion.
- Enum conversion is explicit in both directions.
- Slice and fixed array do not convert implicitly; use `copyArray` or `viewArray` for the checked supported boundary.
- Class upcasts are implicit only along the declared inheritance chain; downcasts use `as?` or `as!`.
- Go interface assertions use `as?`/`as!`. Raw Go target types retain Go assertion
  failure behavior. When the target is a non-null source function signature,
  `as?` also returns `false` for a typed nil function; `as!` panics at the assertion
  with its source filename, line and column. This includes transparent aliases,
  native defined function types and nil-capable generic constraints. Use a
  concrete assertion target for constraints mixing non-null source references
  with non-nil-capable value types. A nullable function target accepts a matching
  nil function, so check that payload against `null` before calling it.
  An assertion cannot establish a UTF-8 contract: assert `bstring`, then decode
  explicitly rather than asserting `string` or a verified-text-containing type.

<<< ../snippets/function-assertions.km{ts}

<<< ../snippets/function-assertions.stdout{text}

An explicitly checked non-null conversion or forced source assertion can
establish a non-null local snapshot even when its declared type is nullable:

<<< ../snippets/reference-conversions.km{ts}

<<< ../snippets/reference-conversions.stdout{text}

For an optional Go callback, bind it to a nullable source function type and
check `null` before calling it. A raw Go target conversion supplies no proof.

<<< ../snippets-invalid/nil-function-conversion.km{ts}

Conversions do not implicitly change reference ownership, recursively copy
reference-bearing collection elements, or turn `null` into raw Go `nil`.
String/byte/code-point conversions copy or encode/decode their scalar content;
later slice mutations do not change an already converted string.

A defined type does not accept its underlying runtime type without that explicit conversion:

<<< ../snippets-invalid/defined-type-mismatch.km{ts}

## Copy and alias behavior

Scalar, fixed array, native struct, enum, and structural object assignment copies the value. A copied struct/object is shallow: a nested slice, map, pointer, channel, or class continues to share its referenced storage.

Slice assignment copies a header and aliases backing storage. Map/channel assignment copies a descriptor for the same runtime object. Class assignment copies a reference and preserves identity. Pointer assignment copies an address.

Address acquisition with `&` requires a named binding, an addressable field/index, or dereferenced pointer. Call results, conversion results, map indexes, string indexes, and other temporary values are not addressable. Pointer receiver methods likewise require a pointer or an addressable value from which the compiler can take one.

## Zero values and absence

Generated code preserves the corresponding Go zero value: numeric zero, `false`, empty string, zeroed array/struct/object, and nil for nil-backed reference shapes. Source-level construction rules may require explicit initialization even when Go has a zero value—for example, every non-null class field must be initialized on every completing constructor path.

| Form | Use |
| --- | --- |
| `null` | Checked absence for a declared `T \| null` |
| `nil` | Raw Go nil at nil-capable low-level boundaries |
| Missing map key | Plain safe-zero reads retain Go zeros; use checked lookup or a direct `??` default for value payloads |
| Closed channel receive | Plain safe-zero reads retain Go behavior; use checked receive or a direct `??` default for value payloads |

Development builds require a safe element zero value for nonempty `makeSlice`
and slice `make`, and for slice `clear`. Numbers, booleans, text, `T | null`,
slices, maps and channels have valid zeros. Native structs, structural objects
and fixed arrays require safe zeros in every stored field/element; a zero-length
array contains no elements. All alternatives of a generic bound must qualify;
an unconstrained `T` does not provide this guarantee. Imported Go implementation
state remains a platform boundary rather than a checked native field contract.

A non-null class, pointer, interface or function cannot be zero-initialized:

<<< ../snippets-invalid/slice-non-null-zero.km{ts}

Use `makeSlice<User | null>(count)` when empty slots are intended. Otherwise,
use `makeSlice<User>(0, capacity)` and append initialized users. The empty
allocation must have a compile-time zero length. See
[initialized collection elements](../examples/collections#non-null-elements-and-initialization).

When a slice's element zero is unsafe or unknown, slicing also checks that the
new high bound does not exceed its current length. The check reports the source
location and remains active inside fixed-array `len`/`cap` expressions. Three-index
slicing can reserve capacity beyond the length, but cannot expose those slots.
After shortening a view, keep the original longer view if you need its full range.
Safe-zero elements retain ordinary Go reslicing up to capacity. Fixed arrays
remain fully initialized storage and retain their ordinary slicing behavior.

Development builds type a one-value map read as `T | null` when `T` has a nil
representation and its zero would violate a non-null contract: classes,
pointers, interfaces and functions. Use a null guard, `??`, or an explicit `!`.
The stored element type is unchanged: `Map<int, User>` still accepts only a
non-null `User` on writes. Safe Go zeros (including scalar zeros and nil
slices/maps/channels) retain their ordinary one-value behavior.

For a struct/array payload, use a checked `[value, ok]` or a direct
`values[key] ?? initializedDefault` / `(<-channel) ?? initializedDefault`.
The latter tests success and preserves present zero values. An unconstrained
`T` still requires a checked pair because its absence representation is unknown.
For ordinary checked map, channel and `as?` bindings whose
payload zero is unsafe or unknown, the checker rejects any use of the payload
before its matching flag proves success. A flag cannot prove that an explicitly
nullable stored value is non-null; check that independently.

<<< ../snippets/map-absence.km{ts}

Use the original local flag in `if (ok)`, `if (!ok)` with an early exit, a
boolean comparison or short-circuit expression. Compile-time boolean constants
and nested equality/negation preserve this relation. Boolean switches support
`switch (ok)`, `switch (!ok)`, comparisons of the guard, and `switch (true/false)`
with guard case expressions. A failed `case false` test can prove success for
later labels or a default, including a default written first.

<<< ../snippets/presence-guards.km{ts}

<<< ../snippets/presence-guards.stdout{text}

Every path entering a grouped/fallthrough body must prove success. A case
label alone does not prove success when failure can fall through into it:

<<< ../snippets-invalid/presence-fallthrough.km{ts}

Reassigning the flag/payload or
letting mutable storage escape can invalidate the relation. Copying a boolean
does not copy a proof. Copy a verified payload into a stable local before
capturing it; direct assignment of an unverified unsafe payload to global
storage is rejected. Safe-zero payloads do not require a presence check.

<<< ../snippets-invalid/map-value-without-guard.km{ts}

These checks preserve raw Go boundaries: a successful lookup can still contain
a nil raw Go pointer/interface explicitly stored by Go. Success proves presence,
not the behavior of foreign implementations.

Development builds apply the same rule to `<-channel`: an unsafe-zero reference
has type `T | null`, while an unsafe record/array or unknown generic element
requires `[value, open]`. A checked `select` binding likewise requires `open`
before an unsafe payload can be read. Nullable payloads still need a null guard
after success. Bare receives and `_ = <-channel` discard without exposing a
payload. Safe-zero elements and channel range retain their Go behavior; range
does not yield a zero element after close.

<<< ../snippets/channel-absence.km{ts}

To require a non-null received reference, use `(<-channel)!`; for expected
absence, use a null guard, `??`, or the checked pair. A nil raw Go channel still blocks, and
nil select channels still disable their cases. Selected class-to-interface
assignment preserves null rather than boxing it into a non-null interface.

`null === null` without a concrete nullable context is rejected because it supplies no base type. A nullable value must be narrowed before member access, indexing, slicing, calling, dereference, or channel operations that require a concrete non-null receiver.

## Comparability

Scalars, pointers, channels, class references, interfaces, and enums are comparable where the corresponding Go value is comparable. Arrays and structs are comparable only when every contained field/element is comparable. Slices, maps, and functions are not comparable except for permitted nil/null checks at their boundary.

Map keys and `T extends comparable` use the same constraint.

## Collections

Fixed array length is part of identity. No implicit array/slice conversion exists. Slice bounds use Go byte/index rules and aliasing; static invalid bounds are rejected and dynamic invalid bounds panic.

Map lookup with one value returns the zero value for a missing key. Two-name binding or reassignment yields `[value, present]`, evaluating map and key once.

Slices support two-index and full three-index slicing. The result aliases the original backing array. `append` may reuse or replace that array exactly as Go does; use the returned slice. Map iteration order is unspecified. Channel operations retain blocking, close, nil-channel, and panic behavior.

## Native named types

- `struct Name` creates a nominal Go-style value type.
- `class Name` creates a pointer-backed reference type.
- `interface Name` creates an explicitly implemented interface contract.
- `enum Name` creates a nominal integer type and typed constants.
- `type Name = distinct T` creates a Go-compatible defined type.
- `alias Name = T` or `alias Name<T> = U` creates a transparent alias; generic aliases are expanded in generated Go.

Finite recursive struct/defined shapes require slice, map, pointer, function, or channel indirection. Direct and fixed-array-only cycles are rejected.

Structural objects differ from native structs: their field-name/type set determines identity and declaration order is irrelevant. They are useful at Go data-transfer boundaries but cannot declare methods. Native structs and classes are nominal even when their fields match.

## Functions, interfaces, and method sets

Function identity includes parameter order/types, variadic status, and result. A variadic `(prefix: int, ...values: int[]) => int` is not assignable to a function taking one ordinary `int[]` parameter.

Interfaces are reference-bearing contracts. Kinmokusei classes name contracts explicitly with `implements`; imported Go interface connectivity is checked against the generated public method set. Value versus pointer receiver methods affect native struct method sets just as in Go. A pointer method requires addressable storage when automatic addressing is used.

Source and imported Go callables may have multiple results. A matching result
list can be destructured, forwarded by `return call();`, or expanded as the sole
argument to a matching call. An explicit `return first, second;` is also valid.
Result lists cannot be stored as tuple values or mixed with additional call
arguments. `Result<T>` remains a separate checked error effect.

## Generics

Functions, class/struct methods, classes, structs, interfaces, aliases, and
distinct defined types accept generic parameters. Bounds support `comparable`,
native `constraint` declarations, dependent parameters, and imported Go type
sets/method interfaces. Exact terms, underlying `~T` terms, unions, and
intersections are checked rather than approximated.

Generic named types require explicit full instantiation. Function and generic
method calls can infer arguments or accept a partial/full explicit prefix.
Known type arguments and preceding typed arguments contextualize fresh array
and object literals, including nested literals and variadic arguments:

```km
function accept<T>(values: T[]): void {}
accept<bstring>(["a", "b"]);
```

This constructs new raw-text storage; it does not reinterpret an existing
`string[]` as `bstring[]`. Existing arrays retain their element contracts, even
when nested inside a fresh literal. Untyped scalar constants still defer to
typed arguments during inference, and argument evaluation stays in source order.

Generic classes support inherited state, virtual dispatch, abstract contracts,
and static methods. Static fields/accessors remain shared across instantiations
and cannot refer to class type parameters. Generic aliases expand transparently;
distinct native-struct definitions preserve nominal identity without inheriting
the underlying struct's methods. Native class/interface underlyings remain
unsupported for `distinct`.

## Nullability

`T | null` requires a nil-capable representation, including functions as well as
classes, pointers, slices, maps, channels and interfaces. The union does not
allocate a wrapper. Scalars, fixed arrays, native structs, structural objects,
`void` and `Result` cannot be nullable directly.

Since v0.4.11, a type parameter also needs a constraint
that guarantees a nil representation for every member of its type set:

```ts
constraint Pointer = ~*int;
function orElse<T extends Pointer>(value: T | null, fallback: T): T {
  if (value === null) { return fallback; }
  return value;
}
```

An unconstrained `T | null`, or a bound mixing pointers and scalar values, is
rejected at the declaration even if the function is never called. `*T | null`
and `T[] | null` still work with unconstrained element types because the
container itself has a nil representation. Use an explicit presence/result
representation when arbitrary scalar or struct values may be absent.

The compiler tracks maybe-null, definitely-null, and non-null facts separately from declared types. Facts join by guarantees across reachable flow and are invalidated by writes or effect boundaries that may change storage.

Raw Go `nil` remains separate and is used at the low-level Go boundary.

Development builds reject raw `nil` assignments and conversions to non-null
source classes, native interfaces, source anonymous method contracts and source
function signatures. Aliases, native defined function types and generic bounds
retain that rule. For example, this is rejected:

<<< ../snippets-invalid/native-raw-nil.km{ts}

Migrate `const user: User = nil` to `const user: User | null = null`, then
narrow the value before use. For callbacks, declare an alias such as
`alias Callback = () => int` and use `Callback | null`. Defensive `nil`
comparisons remain permitted; a nil-backed representation alone does not make
raw `nil` a valid initializer for a non-null source reference.

Raw pointers, slices, maps and channels retain Go nil storage. Imported Go
interfaces and callback types, including aliases of those types, also retain
Go's raw nil rules. This change rejects explicit native nil construction; it
does not yet add automatic boundary assertions or close every zero-value
entry point described above.

A raw Go pointer, interface or function type does not imply that its value is
non-nil. Assigning one to an explicit `T | null` local or field preserves the
nullable state; check it against `null` before access. Aliases and generic named
types obey the same storage rule. Slices, maps and channels also permit nil
storage, so their unqualified type alone does not establish a non-null proof.
Fresh literals, address-taking, collection allocation, and already checked
nullable snapshots can establish a proof without another guard.

<<< ../snippets/nullable-go.km{ts}

This prints `missing`. Converting a source `Class | null` value to a compatible
`Interface | null` preserves absence: a null class becomes a nil interface, not
an interface boxing a typed nil pointer. This also applies to nullable Go
interfaces, including `error | null`. The source expression is evaluated once,
and a non-null class retains its reference identity and method dispatch.

An existing raw Go interface containing a typed nil pointer is not a nil
interface: its comparison with `null` is false, just as comparison with `nil`
in Go would be. The guard checks the interface value, not hidden pointees or
the behavior of the implementation's methods. Direct raw Go use retains Go's
nil behavior; this rule does not add automatic runtime boundary assertions.

Postfix `value!` explicitly checks a nullable or raw nil-capable reference and
yields its non-null type, panicking at the assertion when the value is nil.
The operand is evaluated once; an interface check concerns the interface
itself, not a typed nil pointer it may contain. This does not validate arbitrary
Go-owned mutable storage. See [assertions](./operators#assertions-and-conversions).

Development builds also support `value ?? fallback`: the operand runs once,
and a nil value evaluates the fallback. Direct map/receive expressions with
non-nil-capable payloads instead use the success flag to select the fallback.
A compatible non-null fallback
gives a non-null result; a nullable fallback retains `T | null`. This does not
narrow the original binding or inspect typed nil pointees inside Go interfaces.
See [null coalescing](./operators#null-coalescing).

Narrowing applies to stable bindings and member paths only while the proof remains valid. Assignment, aliasing writes, address-taking, mutable capture, unknown effects, and control-flow joins may discard it. The declared `T | null` type does not change; only the current flow fact does.

Loop checks include facts arriving from later iterations; a proof valid only
on the first iteration does not make an access safe. Development after v0.4.11
reuses completed nested-loop checks when their full entry state and enclosing
analysis context are unchanged. This includes exception handlers, inferred
arrows, constructors, field initializers, labels and captured writes. Diagnostics,
intermediate exceptional paths and Task ownership checks are preserved; a cache
hit never treats a first-iteration proof as valid on later iterations. Checking
cost still depends on source size and the number of distinct flow states.

<<< ../snippets-invalid/null-proof-invalidated.km{ts}

## Result and Task

`Result<T>` is valid only as a function/method return effect. It cannot be stored or nested. `Task<T>` is a local non-escaping single-consumption capability; it cannot appear in public signatures or storage positions.

`Result<void>` lowers to one `error`; `Result<T>` lowers to `(T, error)`. `Task<Result<T>>` retains both layers: `await` joins the worker, and the following `?` propagates the operation error. Neither form is an ordinary heap wrapper visible to Go callers.

## Decorator contexts and values

Decorator context types are compiler-provided nominal contracts, not
interchangeable structural object literals. `DecoratorContext` accepts any
supported target; target-specific contexts restrict application to a class,
field, constructor, method, getter, setter or parameter. See the
[target table](../book/structs-classes-interfaces#typed-decorators).

All context types expose this surface:

| Fields | Type and meaning |
| --- | --- |
| `kind` | `string`: target kind |
| `identity`, `classIdentity`, `baseIdentity` | `string`: target, declaring class and base identities |
| `overrideChain` | `string[]`: inherited slot identities |
| `className`, `memberName`, `parameterName` | `string`: source names |
| `parameterIndex` | `int`: zero-based parameter position, otherwise `-1` |
| `static`, `visibility` | `boolean`, `string`: member access contract |
| `valueType`, `valueIdentity` | `string`: source type description and nominal identity when applicable |
| `constructible`, `constructUnavailableReason` | `boolean`, `string`: construction availability |
| `construct` | `(arguments: DecoratorValue[]) => Result<DecoratorValue>` |
| `invocable`, `invokeUnavailableReason` | `boolean`, `string`: instance invocation availability |
| `invoke` | `(receiver: DecoratorValue, arguments: DecoratorValue[]) => Result<DecoratorValue>` |
| `staticInvocable`, `staticInvokeUnavailableReason` | `boolean`, `string`: static invocation availability |
| `invokeStatic` | `(arguments: DecoratorValue[]) => Result<DecoratorValue>` |

`DecoratorValue` is opaque apart from its readable `typeIdentity: string`.
Boxing and extraction preserve concrete source contracts, including nullable
qualifiers, numeric widths, nominal identities and nested type structure.
Extraction does not implicitly convert a concrete implementation to an
interface: box using that interface contract explicitly. Open type parameters
cannot be transported. The [decorator built-ins](./built-ins#decorator-values)
perform checked boxing and extraction.

## Type-related runtime failure

The checker rejects statically known type, range, comparability, addressability, and nullability violations. Operations whose validity depends on runtime data preserve Go behavior, including bounds panic, nil dereference, integer division by zero, send on a closed channel, and forced assertion/downcast panic. Checked lookup, checked receive, `as?`, `Result`, and nullable flow are the explicit non-panicking alternatives for their respective boundaries.
