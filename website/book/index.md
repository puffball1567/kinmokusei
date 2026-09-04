---
title: Kinmokusei Language Manual
description: A chapter-by-chapter introduction to reading and writing Kinmokusei programs.
---

# Kinmokusei Language Manual

This manual builds one coherent model of the language, from accepted source text through values, control flow, modules, failures, and concurrency.

Use the Manual to understand how concepts fit together, the Guide to complete a task, and the Reference to look up an exact contract. Those layers link to one another without trying to repeat the same page three ways.

## Core language path

1. [Source text and lexical structure](./source-and-lexical): files, comments, identifiers, keywords, literals, punctuation, and semicolons.
2. [Bindings and scope](./bindings-and-scope): `const`, `let`, inference, mutation, shadowing, multiple results, and visibility.
3. [Expressions and evaluation](./expressions): values, composite literals, calls, conversions, assertions, propagation, and evaluation order.
4. [Control flow](./control-flow): conditions, loops, switches, select, labels, defer, and exception boundaries.
5. [Modules and imports](./modules-and-imports): per-file scope, explicit relative imports, Go packages, standard modules, and project dependencies.

## Program design path

6. [Types and values](./types-and-values): scalar and composite shapes, copy versus alias, nominal identity, pointers, interfaces, and nullability.
7. [Functions and generics](./functions-and-generics): signatures, arrows, callbacks, variadics, type parameters, constraints, methods, and result effects.
8. [Structs, classes, and interfaces](./structs-classes-interfaces): value/reference design, constructors, visibility, methods, contracts, and inheritance.
9. [Failures, results, and exceptions](./errors-results-exceptions): raw Go errors, `Result`, propagation, typed exceptions, panic, cleanup, and absence.
10. [Concurrency and tasks](./concurrency-and-tasks): raw goroutines, structured tasks, channels, select, cancellation, and shared state.

## Ecosystem paths

- [Go interoperability](../guide/go-interop)
- [HTTP applications](../guide/http)
- [C ABI and FFI](../guide/c-ffi)
- [Generated Go](../guide/generated-go)

## How examples are presented

Every syntax chapter separates four questions:

1. What tokens and form do I write?
2. What type and scope rules apply?
3. What evaluates, in what order, and how often?
4. What does the compiler reject before Go generation?

Runnable recipes are compiled by the documentation check. Intentionally invalid examples are checked against narrow diagnostic fragments. The implementation and its automated tests remain the behavioral source of truth.

The following compact program is the executable contract shared by these chapters:

<<< ../snippets/book-contract.km{ts}

It combines a defined domain type, enum, generic value struct, interface/class implementation, nullable narrowing, typed arrow, range loop, and value switch. The documentation check expects:

```text
[Aki]:6
guest
```

If you want to write a program immediately, use the [five-minute quick start](../learn/quick-start). If you already know the language, jump to the [Reference](../reference/). The [Glossary](../reference/glossary) resolves terms such as addressable, nominal, storage aliasing, flow fact, and Go boundary without interrupting the chapter sequence.
