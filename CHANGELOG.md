# Changelog

All notable user-facing changes are recorded here. Kinmokusei uses semantic
versioning; releases before 1.0 may intentionally change source syntax or
generated APIs between minor versions.

## [Unreleased]

- Diagnose uninstantiated Go generic functions used as values before Go
  generation, for both qualified and named imports. Direct generic calls and
  ordinary Go function values retain their existing behavior.
- Support local mutual recursion and forward references within consecutive
  direct arrow declarations, using explicit signatures for forward references.
  Peer names shadow outer bindings throughout the group; ordinary statements
  end the group. Preserve mutable storage, captures, initialization order, and
  editor navigation/completion, and show inferred local types in hover.
  Recursive/forward local storage that conflicts with a type, type parameter,
  or Go package name is diagnosed at the source reference.
- Infer module-level arrow results through forward declaration dependencies,
  including later inferred globals and linked-module callables. Check each
  dependency once in its own lexical context without changing runtime order;
  require annotations for unresolved recursive inference cycles.
- Diagnose raw NUL characters in source text, including strings and comments,
  before Go generation. Escaped NUL bytes in string values remain supported.
- Infer direct arrow callback types in native and imported Go generic calls,
  including generic methods, partial type arguments, dependent collection
  bounds, variadic callbacks, and callback-result inference. Resolve callback
  dependencies without reordering runtime arguments; preserve numeric constant
  checks, captures, and editor navigation for inferred parameters.
- Support self-recursive local `const` and `let` arrows with an explicit result
  or binding function type. Preserve lexical captures and mutable binding
  identity across reassignment, source-module linking, and editor operations.
  A direct arrow initializer now sees its own binding rather than an outer
  binding of the same name; other initializer expressions retain their scope.
- Accept an arrow immediately after a generic return type (`Box<int>=>`),
  preserving generic type spans and ordinary comparison/shift tokenization.
- Support arrow-style entry points (`const main = () => { ... }`) and emit
  module-level const arrows with unnamed function types as Go functions.
  Explicit signatures support recursion, mutual recursion, forward calls, and
  references to later globals. These callable declarations are not addressable;
  Go consumers now receive functions rather than assignable function variables.
- Infer arrow parameter and result types from matching function contexts in
  bindings, callbacks, returns, and fields. Infer block-body results when no
  result context is present, including nested arrows and try/finally. Preserve
  mutable function values and lexical captures, and improve arrow navigation,
  rename, completion, and inferred-signature hover.
- Diagnose cyclic global initialization through global/function references at
  the source binding while preserving legitimate function recursion.
- Add source-module exports: `export function`, `export class`, `export const`,
  other named declarations, and `export { name }`. Files with source exports
  expose only selected declarations; `export {}` exposes none. Files without
  source exports retain legacy import behavior. Preserve Go naming and C ABI
  rules, and support export-list navigation, references, and rename.
- Add named Go imports such as `import go { Println } from "fmt"`, retaining
  generic signatures, named types, constant precision, and variable identity.
  Support file-local bindings, qualified-import coexistence, shadowing,
  source diagnostics, editor navigation, completion, and signature help.
- Preserve constructor parameters that share their class's name without hiding
  the allocation type in generated Go, including generic and variadic calls.
- Diagnose malformed UTF-8 at its source byte, including inside strings and
  comments, instead of failing during generated Go validation. Unicode text
  and escaped arbitrary string bytes remain supported.
- Allow omitted statement/declaration semicolons at line breaks, before `}`,
  and at end of file. Expressions can continue across lines; three-clause
  `for` separators remain explicit. A newline immediately after `return` or
  `throw` now ends that statement, and break/continue labels stay on the same
  line. Keep a returned/thrown value on the keyword's line, or start a grouped
  expression there, when migrating multiline code.

## [0.3.0] - 2026-09-09

- Prevent invalid interface members from stalling parser recovery. Require a
  terminating final statement for value-returning functions, matching Go;
  switch/select breaks no longer count as guaranteed returns. Trailing
  unreachable statements after a return must be removed or moved before it.
- Preserve failing CI fuzz inputs as downloadable regression artifacts.
- Diagnose type parameters that hide their enclosing type in generated class
  constructors or generic method helpers; use a distinct type parameter name.
- Preserve actual numeric constant values in imported Go generic inference and
  reject overflow/fractional arguments. Native generic calls infer from typed
  arguments before defaulting mixed numeric constants, with explicit/partial
  arguments, variadics, generic methods, and editor signatures. Imported
  untyped floating/complex constants retain their kind and precision.
- Accept integer-valued untyped numeric constants in indexing, slicing, and
  collection/channel sizes. Check constant bounds and preserve integer context
  in ordered slice construction; retain precision and overflow checks through
  direct floating/complex constant references without folding runtime aliases.
- Add Go numeric literal bases, separators, decimal/hexadecimal floating-point
  exponents, leading-dot floats, and imaginary literals; preserve untyped
  constant precision and check representability at numeric storage boundaries.
- Add `complex64` / `complex128` and `complex` / `real` / `imag`, with Go-checked
  complex arithmetic and conversions, constant overflow diagnostics, typed
  constant rounding, generic operators, and editor completion/signatures.
- Support imported anonymous Go runtime interfaces without erasing their method
  sets, including method values, multiple results, variadics, collections,
  callbacks using Go aliases, generic source qualifiers, and editor completion.
  Preserve rejection of anonymous private methods and unsafe-policy violations.
- Accept generic closers immediately before assignment (`Box<int>=...` and
  nested `Box<Box<int>>=...`). Restore split tokens after speculative generic
  parsing, preserve source spans, and leave caller-owned lexer tokens intact.
- Allow source interfaces to extend imported Go runtime interfaces, including
  generic/diamond contracts, exported-name class implementations, parent
  upcasts, virtual dispatch, Result and variadic methods, and editor completion
  and signature help. Reject incompatible, private, unsafe-policy-denied, and
  unsupported inherited signatures before emission.
- Add reuse and union composition of source constraints, including generic
  references, nullable metadata preservation, cycle/overlap checks, and an
  expanded 100-term limit; generated Go retains named constraint references.
- Add source generic type-set constraints such as `constraint Slice<E> = ~E[]`,
  with checked and forward parameter bounds, declaration-cycle diagnostics,
  native class elements, linked-module isolation, editor navigation, and
  exported generic Go constraint declarations.
- Preserve source nullable qualifiers through collection-shaped bounds,
  dependent inference, and generic method-owner specialization; reject
  constraint matches that only agree after erasing nullability to Go storage.
- Reject concrete class/struct returns to an arbitrary generic type parameter;
  constraint satisfaction alone no longer bypasses assignment checking.
- Add dependent generic bounds with forward references, simultaneous bound
  substitution, constraint-based type inference, and per-owner generic method
  specialization. Preserve linked declarations and editor navigation, and
  distinguish imported Go `Map` types from the native map constructor.
- Document the planned KIR boundary with Go, C++, and Nim-for-C backend routes,
  keeping common semantics separate from target-specific library dependencies.
- Add range over source and imported collection-shaped type constraints,
  including embedded intersections, channel directions, function iterators,
  class/struct element identity, and positive fixed-array constructor proofs.
  Preserve class storage identities during declaration, support class elements
  in fixed arrays, and reject range annotations that imply an ungenerated upcast.
- Add class instance field initializers with per-construction evaluation,
  base/derived ordering, generic and linked-module support, independent lexical
  scope, definite initialization, exception behavior, and editor navigation.
- Run isolated compiler differential tests in bounded parallel groups, retain
  sequential environment-changing fixtures, and preserve explicit test
  parallelism settings.
- Add `T(value)` conversions to in-scope type parameters, with constraint and
  constant checks, generic constructor/method support, linked modules, editor
  navigation, and Go-compatible runtime behavior.
- Add function iterator ranges, including Go `iter.Seq` / `iter.Seq2`, generic
  and virtual class iterator methods, zero/one/two-value yields, control transfer,
  defer, and conservative nullable-field invalidation.
- Show inferred range-binding types in editor hover, preserve escaped Go names
  for type parameters, and recognize returns inside nested lexical blocks.

- Add integer `for ... of` ranges with named types, single-underlying-integer
  generic constraints, fresh captured bindings, and constant-bound constructor
  initialization proofs, backed by handwritten-Go differential tests.
- Add source interface `extends` with multiple and diamond bases, generic
  substitutions and inference, inherited class contracts, ancestor assignments,
  cycle/signature diagnostics, linked modules, and editor support.
- Validate generic class-to-interface assignments using instantiated parameter
  identities, not matching type-parameter spellings from different declarations.
- Report malformed or truncated `else` blocks without a parser panic.

- Add source-declared exact and underlying type-set constraints with
  `constraint Name = T | ~U`, including relative imports, editor support,
  compile-time misuse diagnostics, and handwritten-Go differential coverage.
- Add generic class and struct methods with inference, explicit or partial type
  arguments, constraints, value/pointer receiver behavior, inheritance and
  `super`, arbitrary expression receivers such as `new Box().method<T>()`,
  editor support, and public generated-Go helper functions.
- Keep generic type substitutions scoped to their declarations across
  inherited methods, receivers, and enclosing callers, preventing same-name
  type parameters from being confused or incorrectly inferred.
- Infer function and method type arguments from native generic classes,
  structs, and interfaces, including nested collection/pointer shapes and
  remapped class ancestors. Apply class upcasts to generic-call arguments
  while preserving reference identity and single evaluation.
- Accept constructor field initialization through directly length-guarded
  nonempty collection ranges while preserving conservative rejection for empty
  paths, mismatched collections, intervening mutations, and channels. A
  terminating empty guard clause may prove the immediately following range
  nonempty. Side-effect-free `&&`, `||`, and negated compound guards combine
  proofs for one or more collections without accepting ambiguous branches.
  `switch (len(collection))` also proves grouped positive cases and a default
  branch that follows an explicit zero case, while fallthrough remains
  conservative and effectful case expressions invalidate the proof. An outer
  length fact now also reaches both sides of an immediately nested,
  side-effect-free condition. Side-effect-free local `const` and `let`
  declarations may intervene before the range or nested condition, including
  in length switches and after terminating empty guards.
- Allow block-bodied callbacks declared inside constructors to return from
  their own body, including nested callbacks and `Result` returns. Keep early
  constructor returns and deferred `super()` constructor calls rejected.
- Correct the Visual Studio Code grammar's source suffix to `.km` and add
  compatibility CI for Go 1.24 and 1.25. CI now performs bounded fuzz
  exploration, mandatory C ABI/FFI execution, goroutine-leak checks, compiler
  benchmarks, exact diagnostic contracts, and assertion-aware Go differential
  validation.

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

[Unreleased]: https://github.com/puffball1567/kinmokusei/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/puffball1567/kinmokusei/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/puffball1567/kinmokusei/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/puffball1567/kinmokusei/releases/tag/v0.1.0
