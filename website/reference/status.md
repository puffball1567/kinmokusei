---
title: Implementation status
description: Review supported language and tooling features, release availability, and current safety boundaries.
---

# Implementation status

Kinmokusei is a pre-1.0 public language. The following areas are implemented and covered by automated tests:

- Typed functions, arrows, generics, enums, defined types, aliases, objects, collections, and Go-style control flow
- Reference classes, value structs, interfaces, abstract classes, explicit single inheritance, virtual dispatch, override, final, super, and conversions
- Instance/static getter/setter properties, interface property contracts, shared static fields/constants, and typed decorator registration/construction/invocation
- Optional statement semicolons, arrow-style declarations and entry points, named Go imports, source exports/re-exports, and source multiple-result signatures/forwarding
- Explicit `Result<T>`, typed exceptions, assignment-sensitive null safety, and constructor initialization checks
- Raw Go concurrency, channels, select, and single-consumption structured tasks
- Direct standard-library and external Go module interoperability
- Generated Go, C ABI export, checked incoming C FFI, projects, locks, target builds, LSP, and VS Code packaging

Since v0.4.1, [external Kinmokusei packages](/guide/external-packages) support tagged acquisition, locked imports and local replacements, with app/library project templates. Since v0.4.3, dependency installation also creates package-local short import aliases automatically.

## Release availability

The current compiler/editor release is v0.4.6. It includes typed borrowed/retained
FFI arrays, multiple handles/call-scoped callbacks, typed callback arrays and a
main-thread policy. It also separates verified UTF-8 `string` from raw `bstring`,
with checked decoding, boundary-safe slicing and matching Go/HTTP APIs. This
patch intentionally changes source/API text contracts; review
[migration notes](../project/releases#migrating-from-v045) when upgrading older
applications and source libraries. Nullable/Task control-flow corrections were
already released in v0.4.5.

## Safety boundaries

Inference must determine a single static type; supply an annotation when it
cannot. Multiple results are not tuple values. Nullable proofs and constructor
initialization remain conservative across unknown effects. Tasks require
explicit await/detach, and cancellation contexts are passed explicitly.
Decorator adapters preserve concrete source contracts and do not bypass
visibility or instantiate generic declarations automatically. FFI generation
requires explicit ownership and lifetime contracts; it does not infer them from
native pointer signatures.

The detailed [quality policy](https://github.com/puffball1567/kinmokusei/blob/main/docs/quality-and-go-compatibility.md) explains the difference between statement coverage and the independently handwritten-Go runtime contract gate.
