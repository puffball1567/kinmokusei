# Remaining language work: Go compatibility and OOP

This is an implementation audit, not a claim of full Go compatibility. The
98 runtime contract groups cover accepted features only. The baseline output
remains compatible with Go 1.23; later Go syntax cannot be assumed available.
The [Go language specification](https://go.dev/ref/spec) is the reference for
Go semantics, while [OOP design](oop-design.md) defines deliberate extensions.

## v0.4 development sequence

The next minor release will combine internal refactoring with additional
language features. Start with behavior-preserving semantic-analysis boundaries;
keep feature additions in separate changes with their own compatibility tests.
The first refactoring slice separates named/OOP declarations, generic inference,
type resolution, Go interop, expressions, builtins, and control/effect analysis
from the central checker. Constructor analysis consumes checked AST metadata
and explicit field facts without owning mutable checker state.

Callable return/control-transfer state now has a shared entry and restoration
boundary, with nested-callable regression coverage. Lexical/capture/nullable
state and the large parser/codegen files still need further decomposition.
Select the feature additions from the audited Go
and OOP gaps below; no additional syntax is promised by this refactoring.
Keep published package and documentation versions at v0.3.0 until v0.4.0 release
preparation.

## Approved implementation queue

These are ordered work areas, not equal-size tasks or a promise that every area
must ship in v0.4. Each needs semantic checks, runtime comparisons, editor support
where applicable, and documentation. Refactoring can accompany the relevant
feature work without mixing unrelated changes into its implementation.

| # | Work area | Status |
|---|---|---|
| 1 | Generic callback contextual inference | Implemented for direct arrow arguments; see below |
| 2 | Dependency-driven forward result inference | Queued |
| 3 | Local mutual recursion and forward bindings | Queued |
| 4 | Recursive arrows in three-clause loop initializers | Queued |
| 5 | Function values returning Result | Queued |
| 6 | Additional export forms, including re-exports and aliases | Queued; define the supported forms |
| 7 | Constraint intersections and interface composition | Queued |
| 8 | Remaining numeric constant contexts | Queued |
| 9 | Source anonymous-interface syntax | Queued |
| 10 | Source-declared multiple results | Queued |
| 11 | Abstract classes and methods | Queued |
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

## Queued v0.4 syntax work

Semantic refactoring and the source-encoding fix have landed on devel. The
following additions target v0.4, not the currently released v0.3 syntax:

- **Optional semicolons (implemented on the development branch):** newline,
  end-of-block, and end-of-file termination with multiline expressions,
  comments, restricted return/throw/branch boundaries, and explicit loop-header
  separators. Parser matrices, linked-module handwritten-Go differential tests,
  and editor navigation cover the addition. Existing multiline return/throw
  code must follow the documented restricted-newline rule.
- **Named Go imports (implemented on the development branch):** support `import go { Println } from "fmt"` alongside
  package-qualified imports. Preserve Go export identity, generic signatures,
  type/value namespaces, collision diagnostics, and ordinary qualified Go
  output without runtime wrappers. Include linked-module and editor coverage.
- **Arrow-function bindings and inference (implemented on the development branch):**
  module-level const arrows with unnamed function types become callable
  declarations, including `main`. Explicit signatures support recursion, mutual
  recursion, forward calls, and later globals. Matching function contexts infer
  parameter/result types in bindings, callbacks, fields, and returned arrows;
  block bodies also infer results without a context. Mutable function storage,
  captures, inferred try/finally returns, and editor navigation/refactoring are
  covered by `arrow_bindings_test.go`. Local const/let arrows now support self
  recursion with an explicit signature, preserve mutable storage identity, and
  have linked-module execution and editor coverage in
  `local_arrow_bindings_test.go`. Further work: recursive arrows in loop
  initializers, local mutual recursion/forward bindings, dependency-driven
  forward result inference.
- **Generic callback inference (implemented on the development branch):**
  direct arrow arguments use contexts inferred from other arguments, explicit
  callback annotations, and dependent constraints. Callback results can infer
  remaining parameters and unlock other callbacks. Native functions/methods and
  imported Go generics share contextual preparation, while the existing native
  and Go checkers retain final compatibility/constraint checks. Numeric defaults
  wait for ready typed callbacks. Cyclic or insufficient context still requires
  annotations. Covered by `generic_callbacks_test.go` and editor tests.
- **Source module exports (implemented on the development branch):** declaration
  exports (`export function`, `export class`, `export const`, and other named
  declarations) and local named lists (`export { name }`). Any source export
  opts that file into explicit visibility; `export {}` exposes none. Files
  without source exports retain legacy selective imports. Source visibility
  remains separate from Go capitalization and `export c(...)`. Covered by
  linking/diagnostic tests, editor navigation/refactoring, and handwritten-Go
  differential execution in `source_exports_test.go`.

The intended combined spelling includes named Go imports, omitted semicolons,
and `const main = (): void => { ... }`.

Keep feature work separate from refactoring and bug fixes, and retain v0.3.0
version metadata until v0.4 release preparation.

## Confirmed Go-facing gaps

| Area | Current limitation | Next implementation boundary |
|---|---|---|
| Source constraint composition | Source type-set references and unions are implemented; ordinary/imported interface terms and explicit intersections remain unavailable | Define intersection syntax and method-set composition, retaining inference and linked-declaration semantics |
| Numeric constant contexts | Indices, slicing, collection/channel sizes, generic numeric arguments, and direct floating/complex constant references are supported; source `const` aliases may still lower to runtime bindings, and declared array lengths require integer literal syntax | Audit deferred nonconstant shifts, remaining constant contexts, and target-dependent sizes without treating immutable runtime bindings as Go constants |
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
| Abstract classes/methods | Not implemented; deferred by OOP design | If adopted, forbid direct construction and enforce concrete implementations at instantiable subclasses |
| Getter/setter properties | Not implemented; use explicit methods | Define assignment lowering, visibility, receiver evaluation, and restrictions on hidden effects |
| Static fields/constants | Not implemented; use module constants or static methods | Define initialization, inheritance/name lookup, mutability, and public Go API shape |

Multiple class inheritance, prototype mutation, dynamic field creation, and
runtime metaprogramming are deliberate exclusions, not missing Go compatibility.
Single class inheritance plus multiple interface contracts remains the model.

After the queued syntax work, bounded Go additions include constraint
intersections or numeric constant-context parity. Further OOP additions require the design decisions listed
above. Each addition needs source diagnostics, linked-module and
editor coverage where applicable, and an independent Go runtime oracle.

The frontend is intended to feed KIR with three backend routes: Go, C++, and
Nim for C (via KIR-generated Go and the existing Go-to-Nim translator). Keep
common language semantics separate from backend-specific dependencies; C++-
only libraries and target-capability enforcement remain integration work, not
prerequisites for filling the common language gaps. See
[the KIR boundary](compiler-architecture.md#planned-kir-backend-boundary).
