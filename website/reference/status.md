# Implementation status

OnsenTamago is a pre-1.0 public language. The following areas are implemented and covered by automated tests:

- Typed functions, arrows, generics, enums, defined types, aliases, objects, collections, and Go-style control flow
- Reference classes, value structs, interfaces, explicit single inheritance, virtual dispatch, override, final, super, and conversions
- Explicit `Result<T>`, typed exceptions, assignment-sensitive null safety, and constructor initialization checks
- Raw Go concurrency, channels, select, and single-consumption structured tasks
- Direct standard-library and external Go module interoperability
- Generated Go, C ABI export, checked incoming C FFI, projects, locks, target builds, LSP, and VS Code packaging

Known future work includes broader OnsenTamago package distribution, automatic task cancellation/context inheritance, broader constructor cardinality proofs, and remaining advanced generic-class and FFI ownership cases.

The detailed [quality policy](https://github.com/puffball1567/onsentamago/blob/main/docs/quality-and-go-compatibility.md) explains the difference between statement coverage and the independently handwritten-Go runtime contract gate.
