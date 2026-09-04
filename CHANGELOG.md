# Changelog

All notable user-facing changes are recorded here. Kinmokusei uses semantic
versioning; releases before 1.0 may intentionally change source syntax or
generated APIs between minor versions.

## [Unreleased]

## [0.2.0] - 2026-09-04

- Rename the language from **OnsenTamago** to **Kinmokusei** (金木犀, romanized
  *kin-moku-sei*). The command is now `keika`, source files use `.km`,
  project metadata uses `kinmokusei.toml` and `kinmokusei.lock`, and compiler state is
  stored in `.kinmokusei`.
- Rename the Go module, standard-library import prefix, Visual Studio Code
  identity, release artifacts, generated C ABI support names, and generated
  implementation identifiers consistently with the Kinmokusei identity.
- Document the intentionally breaking v0.1 migration in
  [docs/migrating-from-v0.1.md](docs/migrating-from-v0.1.md).
- Add generic class static methods, generic class inheritance and virtual
  dispatch, descendant-aware class downcasts, and stable generic JSON interop.
- Add Go 1.23-compatible generic aliases, exported Go type-set constraints, and
  distinct types whose underlying representation is a native struct.
- Publish the initial VitePress language guide and keep its executable examples
  checked against the compiler.

## [0.1.0] - 2026-08-31

- First public preview, released under the **OnsenTamago** name with the
  `ontama` compiler command, `.otm` source extension, and matching Visual Studio
  Code client.
- TypeScript-inspired language syntax compiling to deterministic, readable Go.
- Go modules and package interop, classes and interfaces, native structs,
  generics, explicit errors and exceptions, null safety, concurrency, HTTP, and
  checked C ABI/FFI boundaries.
- Independent handwritten-Go differential testing for every registered
  Go-equivalent runtime contract.
- Release archives built with Go 1.27 and checked against supported Go 1.23,
  Go 1.26, and Go 1.27 toolchains.

[Unreleased]: https://github.com/puffball1567/kinmokusei/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/puffball1567/kinmokusei/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/puffball1567/kinmokusei/releases/tag/v0.1.0
