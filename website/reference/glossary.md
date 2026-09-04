---
title: Glossary
description: Precise meanings for the storage, type, failure, concurrency, module, and Go-boundary terms used throughout the Kinmokusei documentation.
---

# Glossary

These terms carry specific meanings in the Kinmokusei documentation. This page resolves vocabulary; the linked Manual and Reference pages remain the full semantic contracts.

## Names and type identity

### Binding

A name associated with a value in a scope. `const` prevents reassignment of the name; `let` permits it. Neither spelling makes referenced storage deeply immutable. See [Bindings and scope](../book/bindings-and-scope).

### Type alias

A transparent alternate spelling declared with `alias Name = T;`. An alias does not create new type identity and is unrelated to two runtime values sharing storage. See [Native named types](./types#native-named-types).

### Defined type

A nominal type declared with `type Name = distinct T;`. It has its own identity and may have receiver methods; crossing to or from its underlying type normally needs an explicit conversion. See [Defined types and aliases](../book/types-and-values#defined-types-and-aliases).

### Nominal type

A type identified by its declaration rather than only its shape. Structs, classes, interfaces, enums, defined types, and imported Go named types are nominal. Two declarations with matching members remain different types.

### Structural object

An anonymous record type such as `{ message: string, count: int }`. Its field-name/type set determines identity, field order does not, and assignment copies the value. Structural objects do not declare methods. See [Structural objects](../book/types-and-values#structural-objects).

## Values and storage

### Addressable

Describes an expression backed by storage whose address may be taken. Named bindings, suitable fields/indexes, and pointer dereferences are addressable; call results and map or string indexes are not. See [Pointers](../book/types-and-values#pointers).

### Copy

Creating another value by assignment, argument passing, or return. Scalars and fixed arrays copy their full value; structs and structural objects copy their outer value shallowly. A copied slice, map, pointer, channel, class, or nested reference-bearing field can still reach shared storage.

### Storage aliasing

Two runtime values reaching the same mutable storage. Slice assignment shares a backing array; map/channel assignment shares a runtime object; pointers share an addressed location. This is different from a transparent type alias.

### Reference identity

The stable identity of one class instance. Assigning or passing a class value preserves that instance rather than copying its fields into an independent object. Pointer identity is explicit and separate from class identity.

### Zero value

The default value of the corresponding generated Go type: numeric zero, `false`, empty string, zeroed value aggregate, or nil for nil-backed shapes. Source initialization rules may still require an explicit value before generation. See [Zero values and absence](./types#zero-values-and-absence).

### Nullable type

A declared `T | null` type for a nil-backed representation. It adds no wrapper and remains nullable even while control flow proves the current value non-null. Raw Go `nil` is a separate low-level boundary value. See [Nullability](./types#nullability).

### Flow fact

A checker-known fact valid at one program point, such as a nullable path being definitely non-null or a task being unconsumed. Assignments, aliases, unknown effects, branches, loops, and jumps may invalidate or merge facts without changing the declared type.

## Completion and failure

### Multiple result

A compiler-known list returned by a Go call, `Result`, assertion, map lookup, or checked channel receive. It can initialize or assign a matching binding list but is not a tuple value that can be stored as one object.

### Raw Go error

An ordinary value implementing Go's `error` interface, usually exposed as one result of an imported Go call. Kinmokusei does not automatically throw it or hide it. Handle it explicitly or propagate a compatible call with `?` inside a `Result` function.

### `Result<T>`

A function return effect for expected failure. `Result<T>` lowers to `(T, error)` and `Result<void>` to `error`; it is not a storable sum object. `ok`, `fail`, and postfix `?` define its completion paths. See [Failures, results, and exceptions](../book/errors-results-exceptions).

### Exception

An error-compatible value transferred by `throw` and matched at an explicit typed `catch` boundary. `finally` runs across normal and exceptional completion. Exceptions are distinct from expected `Result` failure and from ordinary runtime panic.

### Panic

Go-compatible runtime unwinding caused by failures such as invalid bounds, nil dereference, failed forced assertion, or an explicit Go panic. A panic runs `finally` but is not silently converted into a typed exception.

### `Task<T>`

A local structured-concurrency capability returned by task-form `go call()`. It must be consumed exactly once by `await` or `detach` on every continuing path and cannot escape into parameters, results, fields, collections, or closures. See [Concurrency and tasks](../book/concurrency-and-tasks).

## Files and boundaries

### Module

One `.km` file and its declaration scope. Relative imports select declarations from another file explicitly; directory proximity and transitive imports do not create a shared namespace. A Go module is a different concept managed by project dependencies.

### Go boundary

The point where Kinmokusei source uses an imported Go package, type, function, method, callback, channel, or error. The compiler preserves supported Go type identity, method sets, evaluation, failure, and runtime behavior rather than routing calls through reflection.

### Generated Go

The deterministic Go source emitted after checking. It is readable and inspectable output, but Kinmokusei source and diagnostics define the user-facing boundary. Generated helper names are implementation details unless a documented public API promises them.

### Compiler-managed standard module

A Kinmokusei-facing module shipped and checked by the compiler, currently `kinmokusei/http`. It is imported with the language module form rather than `import go`, even when its implementation lowers to Go packages. See [Standard modules](./standard-library).
