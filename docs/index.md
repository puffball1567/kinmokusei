# OnsenTamago design documentation

This directory is the design source of truth for OnsenTamago. Documents distinguish settled decisions from proposals and future candidates. Implementation status must be stated explicitly; proposed syntax is not automatically part of the language.

## Status vocabulary

- **Decision**: a core direction that should remain stable.
- **Proposal**: an initial design to validate through prototypes and tests.
- **Future candidate**: intentionally outside the initial implementation.
- **Implemented**: covered by compiler behavior and automated tests.

## Settled core direction

1. Compile OnsenTamago source to readable, buildable Go source.
2. Treat generated Go as an inspectable, standalone, publishable artifact, not a hidden implementation detail.
3. Use TypeScript-inspired syntax without claiming TypeScript compatibility.
4. Add language-specific types and syntax whenever predictable Go generation requires them.
5. Define `number` as an alias of `float`/Go `float64`; keep `int` independent.
6. Prioritize web backends and Go library production.
7. Expose goroutine-based concurrency through typed, explicit syntax.
8. Lower classes, interfaces, composition, and polymorphism to Go's object model.
9. Treat class instances as references and native `struct` declarations as nominal Go-style values with distinct syntax and copy rules.
10. Provide explicit single inheritance with `virtual`, `override`, and `super`; do not emulate JavaScript prototypes.
11. Make `import go` the official low-level boundary to buildable Go packages.
12. Preserve Go named types, pointers, multiple results, interfaces, generics, channels, and identity without silently simplifying them.
13. Keep direct Go interop separate from higher-level OnsenTamago wrappers for errors, null safety, and application APIs.
14. Make the C ABI a first-class external boundary for explicit outgoing exports and checked incoming libraries.
15. Ship the language server with the compiler so frontend behavior and diagnostics cannot drift by version.
16. Require automated normal, edge, failure, integration, race, vet, and fuzz coverage appropriate to every feature.
17. Make runtime meaning predictable: copy versus alias, value versus reference, evaluation count/order, and failure paths must be visible from syntax and types.

## Documents

- [language-design.md](language-design.md): syntax, types, inference, value/reference semantics, operators, and core control flow.
- [oop-design.md](oop-design.md): classes, interfaces, visibility, composition, and inheritance direction.
- [oop-feasibility.md](oop-feasibility.md): feasibility and lowering constraints for OOP features.
- [compiler-architecture.md](compiler-architecture.md): frontend, type checking, Go import, lowering, diagnostics, CLI, and LSP.
- [packages-and-interop.md](packages-and-interop.md): module resolution, Go interop levels, targets, unsafe policy, locks, and licensing.
- [c-ffi.md](c-ffi.md): outgoing C ABI, implemented incoming schema 1, callback/ownership contracts, and ABI stability.
- [web-and-concurrency.md](web-and-concurrency.md): HTTP interoperability, tasks, cancellation, channels, and performance principles.
- [quality-and-go-compatibility.md](quality-and-go-compatibility.md): independent handwritten-Go differential testing, contract coverage, and generated-artifact quality gates.
- [testing-and-roadmap.md](testing-and-roadmap.md): test matrices, release gates, implementation phases, and verification policy.
- [Visual Studio Code client](../editors/vscode/README.md): thin official editor launcher, configuration, and development checks.

## Current emphasis

The current implementation track prioritizes complete, predictable connectivity to existing Go packages, practical language syntax, source-level diagnostics, deterministic generated Go, an immediately usable LSP, and exhaustive regression tests.
