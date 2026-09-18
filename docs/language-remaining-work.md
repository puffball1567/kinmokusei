# Remaining language work: Go compatibility and OOP

This is an implementation audit, not a claim of full Go compatibility. The
100 runtime contract groups cover accepted features only. The baseline output
remains compatible with Go 1.23; later Go syntax cannot be assumed available.
The [Go language specification](https://go.dev/ref/spec) is the reference for
Go semantics, while [OOP design](oop-design.md) defines deliberate extensions.

## v0.4 baseline and v0.5 milestone

v0.4.0 combines internal refactoring with additional language features.
Keep further feature additions in separate changes with their own compatibility
tests. The first refactoring slice separates named/OOP declarations, generic inference,
type resolution, Go interop, expressions, builtins, and control/effect analysis
from the central checker. Constructor analysis consumes checked AST metadata
and explicit field facts without owning mutable checker state.

Callable return/control-transfer state now has a shared entry and restoration
boundary, with nested-callable regression coverage. Lexical/capture/nullable
state and the large parser/codegen files still need further decomposition.
Select further additions from the audited Go and OOP gaps below. Compatible
features and fixes ship in v0.4.x patch releases, with matching compiler/editor
versions and documented diagnostics. Intentional source/public API breaks
require a minor release and migration notes.

v0.5 marks completion of the audited Go language compatibility work. Close or
explicitly classify each Go contract, test accepted behavior against independent
Go programs, and document deliberate source-language differences. Neither the
100 existing runtime contract groups nor statement coverage alone establishes
that milestone. OOP work continues alongside Go compatibility.

## Approved implementation queue

These are ordered work areas, not equal-size tasks or a promise that every area
is complete in v0.4.0. Each needs semantic checks, runtime comparisons, editor support
where applicable, and documentation. Refactoring can accompany the relevant
feature work without mixing unrelated changes into its implementation.

| # | Work area | Status |
|---|---|---|
| 1 | Generic callback contextual inference | Implemented for direct arrow arguments; see below |
| 2 | Dependency-driven forward result inference | Implemented for module-level bindings |
| 3 | Local mutual recursion and forward bindings | Implemented for consecutive arrow groups, including dependency-driven result inference; nonconsecutive bindings remain |
| 4 | Recursive arrows in three-clause loop initializers | Implemented with per-iteration bindings and explicit recursive signatures |
| 5 | Function values returning Result | Implemented for bindings, callbacks, fields, collections, and native named function types |
| 6 | Additional export forms, including re-exports and aliases | Implemented: named re-exports and export aliases |
| 7 | Constraint intersections and interface composition | Source/imported Go type sets, Go method interfaces, and comparable requirements can be composed; native interface contracts remain |
| 8 | Remaining constant contexts | Scalar reference chains, constant `min`/`max`, constant-string `len`, and fixed-array/array-pointer `len`/`cap` preserve values and types; bounds and size checks consume these constants; further contexts remain |
| 9 | Source anonymous-interface syntax | Queued |
| 10 | Source-declared multiple results | Queued |
| 11 | Abstract classes and methods | Implemented: explicit abstract contracts, concrete overrides, generic DI and multi-level dispatch; construction boundary documented below |
| 12 | Getter/setter properties | Queued |
| 13 | Static fields and constants | Queued |
| 14 | Receiver/constructor-dependent field initializers | Queued |
| 15 | Parser decomposition | Queued |
| 16 | Go emitter decomposition | Queued |
| 17 | Lexical, capture, and nullable-state boundaries | Queued |

## Completed through v0.3.0

- Numeric constants in generic calls: real constant values in Go inference,
  typed-argument-first native inference and numeric-kind defaults, narrow and
  fractional diagnostics, imported constant precision, generic methods,
  variadics, linked modules, and editor signatures. Differential coverage:
  `generic_numeric_test.go`.
- Integer-valued untyped floating/complex constants in indices, slice bounds,
  and collection/channel sizes, with constant bounds diagnostics. Floating and
  complex constant references preserve their checked precision and narrowing;
  runtime aliases remain runtime values. Ordered slice construction gives
  numeric constants an integer context before storing them in temporaries.
  Linked modules and generic classes are covered by `numeric_context_test.go`.
- Go numeric literal syntax: integer bases and separators, decimal and hex
  floating-point exponents, leading-dot floats, and imaginary literals. Includes
  untyped constant evaluation, narrow representability, inferred storage checks,
  base-aware fixed array lengths and integer constant checks, and Go differential
  coverage in `numeric_literal_test.go`.
- Native `complex64` / `complex128` and `complex` / `real` / `imag`, using Go's
  numeric checker for complex expressions and conversions. Includes constant
  representability and rounding, generic arithmetic, collections, linked names,
  and floating special values. Differential coverage: `complex_test.go`.
- Anonymous Go runtime interfaces with exported method sets: inferred values,
  parameters, method values, multiple results, variadics, nested collections,
  callbacks using imported type aliases, generic specialization, source nullable
  metadata, and editor completion/signatures. Differential coverage:
  `anonymous_go_interface_test.go`. Private method identities remain unsupported
  in anonymous interfaces; named Go contracts retain their existing rules.
- Generic closers adjacent to assignment, including nested `>>=`, checked
  constraints, aliases, class fields, and locals. Speculative parsing restores
  comparison/shift tokens and preserves type spans. Differential coverage:
  `generic_closer_test.go` (existing generic and bitwise contract groups).
- Integer `for (const i of n)` ranges, including named integer types, integer
  constraints with one underlying type, nonpositive bounds, fresh closures,
  single evaluation, and definite constructor initialization for positive
  constants. Differential coverage: `integer_range_test.go`.
- Source interface `extends`, multiple/diamond inheritance, generic remapping,
  inherited class implementation checks, ancestor assignment, generic inference
  from explicitly implemented contracts, and editor navigation/refactoring.
  Differential coverage: `interface_inheritance_test.go`.
- Type-parameter conversions such as `T(0)` and `T(value)`, including type-set
  convertibility, constant bounds, numeric/collection conversions, generic
  constructors and methods, and editor navigation. Differential coverage:
  `type_parameter_conversion_test.go`.
- Function iterator ranges with zero, one, and two yielded values, imported Go
  iterators, generic/virtual class methods, control transfer, defer, and runtime
  protocol validation. Differential coverage: `iterator_range_test.go`.
- Instance field initializers with per-construction evaluation, base-before-
  derived and source-field ordering, generic types, module lexical bindings,
  definite initialization, exception short-circuiting, and editor navigation.
  Differential coverage: `class_field_initializer_test.go`.
- Range over source and imported collection-shaped constraints, including
  compatible channel directions, function iterators, embedded intersections,
  native class/struct element identity, and positive fixed-array constructor
  proofs. Differential coverage: `range_constraint_test.go`.
- Dependent generic bounds with same-list/forward references, simultaneous
  argument substitution, constraint-based inference, and separately specialized
  generic method bounds for concrete owners. Linked functions, classes,
  structs, interfaces, defined types, and aliases are covered by
  `dependent_constraint_test.go`.
- Source generic type-set declarations such as `constraint Slice<E> = ~E[]`,
  including checked/forward/nested bounds, declaration-cycle diagnostics,
  native class elements, module-name isolation, editor parameter scopes, and
  public Go constraint APIs. Differential coverage:
  `source_generic_constraint_test.go`.
- Reuse and union composition of source type-set constraints, including generic
  substitution, nullable element metadata, linked references, cycle/overlap
  diagnostics, and the expanded 100-term limit. Covered by the same contract group.
- Source interfaces extending imported Go runtime interfaces, including generic
  and diamond bases, exported-name implementation checks, Result/variadic methods,
  virtual dispatch, nullable source type arguments, parent upcasts, and editor
  completion/signatures. Differential coverage: `go_interface_inheritance_test.go`.

Existing OOP support includes reference classes, constructor fields, explicit
single inheritance, virtual/override/final methods, construction-phase dispatch,
`super`, visibility, generic classes/methods, method values, interface dispatch,
identity-preserving class upcasts/downcasts, and public generated Go APIs.
Abstract classes are not necessary to use these features.

## Syntax and contract additions in v0.4.0

Semantic refactoring and source-encoding diagnostics accompany these additions
relative to v0.3.0:

- **Invariant method contracts:** implementations, overrides, and inherited
  conflicts retain nested nullable and generic qualifiers through linked
  re-exports. Compatible source `Result<T>` implementations of Go multiple-result
  methods retain runtime behavior. Covered by `method_contract_test.go`;
  source interfaces as constraint operands remain further work.

- **Optional semicolons (implemented in v0.4.0):** newline,
  end-of-block, and end-of-file termination with multiline expressions,
  comments, restricted return/throw/branch boundaries, and explicit loop-header
  separators. Parser matrices, linked-module handwritten-Go differential tests,
  and editor navigation cover the addition. Existing multiline return/throw
  code must follow the documented restricted-newline rule.
- **Named Go imports (implemented in v0.4.0):** support `import go { Println } from "fmt"` alongside
  package-qualified imports. Preserve Go export identity, generic signatures,
  type/value namespaces, collision diagnostics, and ordinary qualified Go
  output without runtime wrappers. Include linked-module and editor coverage.
- **Arrow-function bindings and inference (implemented in v0.4.0):**
  module-level const arrows with unnamed function types become callable
  declarations, including `main`. Explicit signatures support recursion, mutual
  recursion, forward calls, and later globals. Matching function contexts infer
  parameter/result types in bindings, callbacks, fields, and returned arrows;
  block bodies also infer results without a context. Mutable function storage,
  captures, inferred try/finally returns, and editor navigation/refactoring are
  covered by `arrow_bindings_test.go`. Local const/let arrows now support self
  recursion with an explicit signature, preserve mutable storage identity, and
  have linked-module execution and editor coverage in
  `local_arrow_bindings_test.go`. Module-level forward result inference now
  follows dependencies through arrows and inferred globals, checking each
  definition once in an isolated lexical context. Unresolved recursive result
  dependencies require annotations; runtime initialization cycles remain
  errors. Linked-module execution, initialization order, and editor navigation
  are covered by `forward_arrow_inference_test.go`. Consecutive direct local
  arrow declarations now form a mutually visible group. Forward result types
  follow dependencies, while unresolved recursive cycles require annotations.
  Each body is checked once against the group's lexical snapshot; capture
  effects are replayed in declaration order. Group storage precedes
  closure construction; initialization remains in source order. Tests in
  `local_arrow_groups_test.go` cover recursion, mutable peers, linked modules,
  escaping closures, loop captures, and editor scope/navigation. Recursive
  three-clause loop initializers now preserve per-iteration storage, one-time
  closure initialization, condition/post order, labels, and escaped captures;
  see `recursive_loop_arrows_test.go`. Nonconsecutive local forward bindings
  remain further work.
- **Result-returning function values (implemented in v0.4.0):**
  `Result<T>` and `Result<void>` are supported in stored function signatures,
  callback parameters, returned closures, fields, collections, and channels.
  Native aliases and defined/generic function types retain the return effect.
  Arrows need an explicit Result annotation or matching function context, plus
  a block body with explicit returns. Raw Result values remain non-storable;
  calls require `?`, explicit split binding, forwarding return, or intentional
  discard with `_`. Unused named local Result error bindings are diagnosed;
  this is not per-write, alias, or path-sensitive recovery analysis. Explicit
  function annotations can expose ABI-compatible Go callbacks as Result without
  a runtime wrapper; ordinary Go multiple-result expressions remain unchanged.
  Pipeline, independent handwritten-Go runtime comparisons, negative checks,
  editor tests, and fuzz seeds cover these boundaries in
  `result_function_values_test.go` and `result_function_runtime_test.go`.
- **Generic callback inference (implemented in v0.4.0):**
  direct arrow arguments use contexts inferred from other arguments, explicit
  callback annotations, and dependent constraints. Callback results can infer
  remaining parameters and unlock other callbacks. Native functions/methods and
  imported Go generics share contextual preparation, while the existing native
  and Go checkers retain final compatibility/constraint checks. Numeric defaults
  wait for ready typed callbacks. Cyclic or insufficient context still requires
  annotations. Covered by `generic_callbacks_test.go` and editor tests.
- **Source module exports (implemented in v0.4.0):** declaration
  exports (`export function`, `export class`, `export const`, and other named
  declarations) and named lists (`export { name }`) selecting local declarations
  or explicitly imported source bindings. Named re-exports also support
  `export { name } from "./module"` without introducing a local name. Multi-hop
  and diamond dependencies preserve the original declaration, mutable storage,
  and generic type identity; mixed import/export dependencies load in source
  order. Private/missing names, cycles, and duplicate exports are rejected.
  Independent Go runtime comparisons and editor tests cover these boundaries
  in `named_reexports_test.go` and `named_reexports_runtime_test.go`.
  Export aliases (`export { local as publicName }`) work for local declarations,
  imported source bindings, and export-from lists. Public names are unique per
  module; multiple names may select one declaration without copying its value or
  changing its nominal type. Alias imports do not expose the original spelling.
  Rename follows each explicit alias boundary independently of runtime identity.
  Pipeline, handwritten-Go differential, and editor tests in
  `export_aliases_test.go` and `export_aliases_runtime_test.go` cover the addition.
  Any source export
  opts that file into explicit visibility; `export {}` exposes none. Files
  without source exports retain legacy selective imports. Source visibility
  remains separate from Go capitalization and `export c(...)`. Covered by
  linking/diagnostic tests, editor navigation/refactoring, and handwritten-Go
  differential execution in `source_exports_test.go`.

The intended combined spelling includes named Go imports, omitted semicolons,
and `const main = (): void => { ... }`.

Keep feature work separate from refactoring and bug fixes; advance matching
compiler/editor version metadata when preparing each v0.4.x release.

## Confirmed Go-facing gaps

Generic collection mutation now accepts mixed slice/map type sets for `clear`
and common-key map type sets for `delete`, including differing map value types,
source nullable/class keys, imported constraints and generic class methods.
`generic_collection_mutation_test.go` compares generated behavior against Go in
the existing collection/OOP contract groups. `append`/`copy` now also accept
direct slice-constrained type parameters, including dependent elements and
generic class methods; `generic_slice_builtins_test.go` covers Go behavior and
source OOP invariance. Operations requiring a common element shape reject
conflicting source nullability; `delete` likewise rejects conflicting source key
qualifiers. `copyArray`/`viewArray` also accept slice-constrained parameters with
common elements, including OOP elements and generic methods. Their targets must
have a concrete fixed-array shape (`[2]E` or a named instantiation), not a bare
array-constrained parameter. `generic_array_conversion_test.go` covers shallow
copy versus shared slots, nil/zero length, short-source panics, evaluation, and
preserved class/interface/nullability contracts after indexing and reslicing.
This is not a claim of complete generic collection support.

After v0.4.0, common-underlying-shape type parameters also support direct
indexing and slicing, including writes, checked map lookup, source OOP/nullable
elements, and named slice/string result identity. Runtime aliasing, bounds,
evaluation order and nil behavior are compared with Go in
`generic_index_slice_test.go`. Operation-specific analysis now also supports
indexing unions of arrays, slices, and array pointers with identical elements,
plus read-only byte indexing and two-index slicing of string/byte-slice unions.
`mixed_generic_collections_test.go` compares array copy versus pointer/slice
mutation, OOP elements, named text results, evaluation order, bounds and nil
panics with Go. Array/slice unions are still not sliceable under Go 1.23;
different element contracts and map/non-map indexing remain rejected. This
does not broaden range, append, or copy to those mixed shapes.

| Area | Current limitation | Next implementation boundary |
|---|---|---|
| Source constraint composition | Source/imported Go type-set unions/intersections, comparable requirements, and Go interface methods are implemented, including generic terms, inference, and linked export aliases | Add native source interface contracts; preserve source-only method argument shapes; extend parameter-dependent intersections beyond matching shapes |
| Constant contexts | Indices, slicing, collection/channel sizes, generic scalar arguments, scalar reference chains, constant `min`/`max`, constant-string `len`, and fixed-array/array-pointer `len`/`cap` are supported; ordered built-ins also preserve contextual nonconstant shifts; type-parameter values and runtime bindings remain nonconstant; declared array lengths require integer literal syntax | Audit remaining deferred shift contexts, constant layout intrinsics, nullable array-pointer constants, remaining constant contexts, and target-dependent sizes without treating immutable runtime bindings as Go constants |
| Anonymous Go interfaces | Exported runtime method sets are supported through imported APIs and inference; no source anonymous-interface literal syntax | Source callback annotations can use an imported Go alias; consider source syntax separately, and retain rejection of anonymous private method identities |
| Source-declared multiple results | Raw Go multiple-result calls can be consumed; source callable results use a single type or `Result<T>` | Decide a source result-list syntax and propagation rules before expanding declarations |

These entries come from the parser, `internal/sema/checker.go`,
`internal/sema/types.go`, and `internal/sema/go_audit.go`; they do not imply that
an API classified as supported by the Go API audit has equivalent native syntax.

## OOP gaps and design decisions

| Area | Current state | Required work |
|---|---|---|
| Receiver-dependent field initializers | Module-scope defaults are implemented; `this`, `super`, and constructor parameters are intentionally unavailable in defaults | Any expansion needs read-before-initialization and receiver-escape rules; use the constructor today |
| Go interface inheritance boundary | Exported runtime contracts are supported; private methods, type-set-only bases, and unsupported interop signatures are rejected | Expand anonymous interface interop independently; preserve package identity and unsafe policy |
| Go 1.23 comparable-bound import order | Forward comparable-dependent bounds are semantically checked, but Go 1.23 vet can panic while importing their export data | Write comparable element parameters before dependent bounds when targeting Go 1.23 tooling |
| Recursive comparable source bounds | Array/struct comparability that depends on a still-resolving parameter is diagnosed rather than allowed to deadlock Go type resolution; recursive pointer and method contracts remain supported | Break the dependency with an independently constrained element parameter; broader cyclic value-bound solving remains unsupported |
| Source struct constraint terms | Terms requiring source struct value storage before it is finalized are diagnosed, including generic instances, instead of panicking | Coordinate constraint and source storage completion before enabling these terms; imported Go concrete types remain available |
| Abstract classes/methods | Explicit abstract method declarations and concrete implementation checks are implemented; abstract classes cannot be constructed directly | Interface requirements must be declared explicitly; direct constructor access to abstract methods is rejected, while indirect access to an unimplemented construction-phase slot panics; method-level generics remain nonvirtual |
| Getter/setter properties | Not implemented; use explicit methods | Define assignment lowering, visibility, receiver evaluation, and restrictions on hidden effects |
| Static fields/constants | Not implemented; use module constants or static methods | Define initialization, inheritance/name lookup, mutability, and public Go API shape |

Multiple class inheritance, prototype mutation, dynamic field creation, and
runtime metaprogramming are deliberate exclusions, not missing Go compatibility.
Single class inheritance plus multiple interface contracts remains the model.

After the queued syntax work, bounded Go additions include constraint
method-set composition or numeric constant-context parity. Further OOP additions require the design decisions listed
above. Each addition needs source diagnostics, linked-module and
editor coverage where applicable, and an independent Go runtime oracle.

Kinmokusei remains a Go-output language. Prioritize Go interoperability,
predictable language semantics, OOP completeness, diagnostics, and maintainable
compiler boundaries. Libraries need only support the Go output path; see
[the Go output boundary](compiler-architecture.md#go-output-boundary).
