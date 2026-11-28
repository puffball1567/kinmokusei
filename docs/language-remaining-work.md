# Remaining language work: Go compatibility and OOP

This is an implementation audit, not a claim of full Go compatibility. The
111 runtime contract groups cover accepted features only. The baseline output
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
boundary, with nested-callable regression coverage. The parser has been split
into grammar-focused files while retaining shared token state and recovery.
Go generation likewise has responsibility-focused files, retaining identical
lowering and runtime helpers. Channel send/receive/close checking is now grouped
in a dedicated semantic-checking module without changing operation-specific
diagnostics or source type rules. Callable nullable/reachability/loop-exit state
and closure lexical entry, restoration and capture-effect publication now have
shared boundaries. Closure bodies are checked independently of enclosing
unreachability; unreachable nested closure creation does not publish capture
effects to enclosing bodies. Wider checker-state decomposition remains.
Select further additions from the audited Go and OOP gaps below. Compatible
features and fixes ship in v0.4.x patch releases, with matching compiler/editor
versions and documented diagnostics. Intentional source/public API breaks
normally require a minor release and migration notes. v0.4.6 is an explicit
exception for the verified-text source/API migration.

Imported Go method expressions now use the receiver's exact method set and
prepend its type to the callable's parameter list. Value/pointer/interface and
promoted methods, callbacks, variadics, multiple results, explicit nil behavior,
editor completion and signature help have focused coverage and an independent
Go oracle. Direct `<T>`/`[T]` receiver applications and exported Go aliases of
concrete instantiations work; uninstantiated generic receivers are rejected.
Native structs, enums, distinct defined types and source
interfaces now support explicit-receiver method expressions as well, including
source aliases of concrete generic instantiations. These retain source
text/nullability contracts, Results and multiple results, value/pointer method
sets, receiver-aware argument hints and exact-method-set completion (with enum
constants retained in the enum type namespace). Native method bodies
participate in global initialization-cycle checking even when referenced
through a stored method value or expression. Independent runtime coverage is in
`native_method_expressions_test.go`. Direct generic receiver instantiation is
implemented for native and Go types, preserving constraints and lexical
shadowing. v0.4.13 builds add class instance method expressions with an
explicit receiver, including inherited and virtual methods, generic class
applications, aliases, visibility and invocation checks. Methods with their
own type parameters remain follow-up work.

v0.5 marks completion of the [audited Go language compatibility work](#go-compatibility-areas).
Close or explicitly classify each Go contract, test accepted behavior against
independent Go programs, and document deliberate source-language differences.
The 111 runtime contract groups measure the already implemented surface.
OOP work continues alongside Go compatibility.

## Go compatibility areas

The ten numbered areas form the v0.5 implementation audit. The status applies
to the scope described in each row.

- **Verified** means the implemented scope has focused evidence and the row
  has no open implementation item.
- **Partial** means the row has an implemented subset and specific
  unimplemented or unresolved cases.
- **Audit pending** means the main forms are implemented and remaining
  combinations still need review.

Update the matching row when a change expands support. Record the accepted
subset and link its tests, then keep any remaining combinations listed.
Classify intentional source-language differences and language extensions
explicitly before closing a row for v0.5.

| # | Work area | Status | Implemented | Remaining work and boundaries | Evidence |
|---|---|---|---|---|---|
| 1 | Named results and bare returns | Verified | v0.4.13 supports zero-safe named results in functions, methods and block arrows, with bare returns, defer and try/finally. | Named results follow the source zero-value safety rules. | [Named results](../internal/compiler/named_results_test.go) |
| 2 | Struct embedding and member promotion | Verified | v0.4.13 supports embedding source structs and distinct types, Go structs and named runtime interfaces, and eligible pointers. Development builds add imported named non-struct types such as `time.Duration`, pointer aliases and named source interfaces. Embedding a source interface declares conformance when the promoted method set still matches. | Source interface fields require initialization. Source property selectors use the embedded field or an interface-typed receiver. | [Source structs](../internal/compiler/struct_embedding_test.go), [Imported Go types](../internal/compiler/imported_struct_embedding_test.go), [Go interfaces](../internal/compiler/go_interface_embedding_test.go), [Source interfaces](../internal/compiler/source_interface_embedding_test.go) |
| 3 | Instantiated generic function values | Partial | Full explicit type arguments since v0.4.7. Since v0.4.12, contextual inference and partial argument lists work for source and Go functions, including callbacks, assignments, returns and typed aggregates. | **Unimplemented** — inferred function values that need an unnameable Go type. **Source rule** — require an explicit type when a callback cycle supplies insufficient inference information. Generic method values are tracked in #4. | [Function values and private-type diagnostics](../internal/compiler/contextual_function_values_test.go), [Inference diagnostics](../internal/sema/contextual_function_values_test.go) |
| 4 | Generic receiver method expressions | Partial | Explicitly instantiated receiver types and aliases work for source structs, interfaces and defined types, and imported Go types. v0.4.13 adds class expressions, including inherited and virtual methods. | **Unimplemented language extension** — method values and expressions for methods with their own type parameters. | [Go receivers](../internal/compiler/go_method_expressions_test.go), [Source receivers](../internal/compiler/native_method_expressions_test.go), [Class receivers](../internal/compiler/class_method_expressions_test.go) |
| 5 | Enum method expressions | Verified | Since v0.4.7, value, pointer and alias method sets support explicit receivers and callbacks, with enum constant precedence, nil behavior and linked visibility. | Enum constants retain their source namespace. | [Enum receiver comparisons](../internal/compiler/native_method_expressions_test.go) |
| 6 | Named source interface constraints | Verified | Direct bounds since v0.4.7. v0.4.13 also composes named source interfaces with type sets, `comparable` and compatible Go method interfaces, preserving source method/property contracts and dependent inference. | Substitution-dependent intersections are tracked in #8. | [Source interface composition](../internal/compiler/source_constraint_composition_test.go) |
| 7 | Struct constraint terms | Partial | v0.4.13 supports exact named terms, generic instances and aliases, forward references, collection terms, dependent inference and generic method bounds. | **Audit pending** — recursive comparable value bounds. Keep the cycle diagnostic for invalid recursive storage and determine which valid bounds can be resolved. | [Struct constraints](../internal/compiler/struct_constraints_test.go), [Storage diagnostics](../internal/sema/struct_constraints_test.go) |
| 8 | Dependent constraint intersections | Partial | Intersections normalize overlapping concrete terms and exclude provably disjoint function and record shapes. Source text and nullable contracts remain checked. The checker reports [unsafe dependency order in Go export data](#generated-go-dependency-order-bug) before emission. | **Unimplemented** — intersections whose overlap is resolved only after substituting type arguments. | [Intersection behavior](../internal/compiler/constraint_intersection_test.go), [Dependent-term diagnostics](../internal/sema/constraint_intersection_test.go), [Go export diagnostics](../internal/compiler/go_export_bounds_test.go) |
| 9 | Anonymous interface composition | Partial | Method-only anonymous interfaces support structural matching, constraints, other anonymous interfaces and Go method interfaces as bases. Named source interfaces can extend anonymous bases. | **Unimplemented** — embed a named source interface in an anonymous interface. **Unimplemented language extension** — anonymous properties. | [Anonymous interface emission](../internal/compiler/anonymous_interface_source_test.go), [Anonymous interface constraints](../internal/sema/anonymous_interface_constraints_test.go) |
| 10 | Constant contexts, inference and multiple results | Audit pending | Constant and shift contexts preserve target types. Sole-call forwarding works for covered built-ins, including collection constructors. A single callable type-parameter signature supports variadics, multiple results, Result, defer and Task. | Audit remaining combinations of contextual typing and built-ins. Record deliberate differences in the [Go-facing gaps](#confirmed-go-facing-gaps). | [Shift contexts](../internal/compiler/shift_context_test.go), [Multiple results](../internal/compiler/collection_multiple_results_test.go), [Constrained callback calls](../internal/compiler/constrained_callback_calls_test.go) |

### Generated Go dependency-order bug

When `Pair<E>` combines `comparable` with an array term such as `~[2]E`,
Go can panic while importing a public declaration with type parameters ordered
as `<T extends Pair<E>, E extends comparable>`. Kinmokusei reports this
ordering at the source bound on public functions and types, including private
functions reachable from exported bodies.

Declare the element parameter first, as in
`<E extends comparable, T extends Pair<E>>`. External Go callers can use
inference or pass explicit type arguments in that declaration order.
Forward references through bounds that require no such comparison are supported.
See the [external Go caller test](../internal/compiler/imported_constraint_test.go).

## Recent patch work and remaining areas

### Nullability safety coverage (v0.4.12, #117)

v0.4.7 builds preserve explicit `T | null` state when receiving raw Go
references. A matching pointer/interface/function/collection storage type no
longer establishes a non-null assignment proof. Null guards, already checked
snapshots, fresh literals, address-taking and collection allocation still
establish proofs. Aliases, linked modules, callbacks, mutable assignments and
Go interface typed-nil semantics have checker and independent runtime coverage
in `go_nullable_flow_test.go`.

v0.4.7 builds also implement runtime-checked postfix `!` with single
evaluation, filename/line/column panic messages and editor type metadata.
Checks remain effective in fixed-array `len`/`cap` operands; Result/Task errors
cannot be discarded through an assertion. Nil-capable generic constraints,
linked aliases, named types, callbacks and typed-nil Go interfaces have runtime
comparison coverage in `non_null_assertions_test.go`. Nullable `unsafe.Pointer`
assignment follows the same conservative storage rule as ordinary Go pointers.

v0.4.12 builds reject raw `nil` assignments and conversions to non-null
source classes, interfaces and function signatures, including aliases, native
defined function types and generic bounds. `native_raw_nil_test.go` covers
these restrictions and the retained raw Go pointer/collection/callback rules;
compiler and LSP tests preserve source diagnostics and nullable edit recovery.

v0.4.12 builds apply recursive zero-safety to slice `make`/`makeSlice`,
`clear` and reslicing, including canonical struct fields, fixed arrays, source
nullable generic arguments and every alternative of a bound. Empty reserved
slices and initialized appends remain supported; unsafe-zero slices cannot
reslice beyond their current length. Runtime comparisons cover aliases,
integer widths, private Go types and fixed-array len/cap check retention.

v0.4.12 builds support lazy `??` for nullable and raw nil-capable values,
including class/interface conversions, generics, contextual literal fallbacks,
conditional Task/flow checks and editor navigation. The operand is evaluated
once; a nil interface differs from an interface holding a typed nil pointer.
Private Go result storage works with static zero-argument fallback functions
or explicit public result contexts; other unnameable signatures are rejected.

v0.4.12 builds preserve missing-key absence in one-value map reads.
Unsafe-zero references produce nullable results; unsafe structs/arrays and
unknown generic elements require checked bindings. Normal checked map,
receive and assertion bindings require the matching success flag before using
unsafe-zero payloads. The relation follows declaration identities, assignments,
joins, loop fixed points and captures; boolean aliases do not transfer proofs.
Checked assertions cannot forward an unchecked unsafe payload through a result
list. Constrained map alternatives must agree on key/value source contracts.

v0.4.12 builds apply the same absence rules to single-value receives and
select bindings. Checked select payloads without safe zeros require their local
success flag. Receive assignment conversions preserve source null, including
class inheritance and interface targets. Selected targets are evaluated before
writes; field/index/pointer/property targets are supported without running
unselected target expressions. Layout intrinsics still inspect unevaluated
storage, while nested callable bodies retain ordinary checks.

v0.4.12 builds validate the reference itself when a Go type assertion
targets a non-null source function signature, including aliases, native defined
types and nil-capable generic constraints. Checked assertions reject typed nil
functions; forced assertions report a source-located panic. Nullable targets
and imported Go function types retain Go assertion behavior. Constraints mixing
source non-null references and non-nil-capable values require a concrete target.
Independent runtime coverage is in `assertion_non_null_test.go`.

v0.4.12 builds support direct map/receive `??` fallbacks for scalar,
record and fixed-array payloads, including uniform value-type generic bounds.
The success flag selects the default; present zeros are preserved, and ordinary
reference payloads keep the nil test. Open or mixed reference/value bounds
require explicit checked bindings. Private Go values can use a public interface
result context; other unnameable results are diagnosed. Independent comparisons
cover operand order, lazy defaults, conditional effects, source maps and editor
navigation in `absence_coalescing_test.go` and its Go type fixture.

v0.4.12 builds validate explicit conversions to non-null native references,
including function aliases, defined functions, nil-capable generic bounds and
anonymous source interface contracts. The operand runs once and a nil result
panics at the original conversion location. Raw Go target conversions and
typed-nil interface behavior are unchanged. Conversion/assertion checks share
lowering; runtime guards cannot be elided by fixed-array `len`/`cap`, and their
panic ordering is retained. Checked conversions and forced source assertions
establish nullable-flow and coalescing proofs. Independent Go comparisons are
in `conversion_non_null_test.go`.

v0.4.12 builds recognize presence checks through compile-time boolean
constants, negation, nested equality and boolean switches. A flag or its
comparison/negation may be matched against a constant, or a constant tag may
match the guard in a case. Failed tests carry proofs to later cases/defaults;
grouped labels and fallthrough merge every entry path. Declaration identity,
mutable-state invalidation and independent nullable payload contracts remain
required. Independent Go comparisons are in `presence_guards_test.go`.

v0.4.12 builds check named non-null source reference parameters at callable
entry. Functions, expression/block/contextual arrows, methods, setters and
constructors reject nil before source body effects or field/base initialization.
Rest parameters check every element; an empty rest slice remains valid. Explicit
blank parameters discard values. Nullable and raw Go contracts retain their nil
semantics, including interfaces containing typed nil pointers. Generic parameter
checks require every bound alternative to promise a nil-comparable non-null
reference; open or mixed bounds do not establish this guarantee. Independent Go
caller/callback comparisons cover source locations, argument order and public
generic wrappers in `parameter_boundaries_test.go`.

v0.4.12 builds check single-value transfers into non-null source reference
storage/results. Bindings, assignments, field and explicitly typed aggregate
initializers, per-expression return slots, `ok` payloads, non-spread append elements and
channel sends validate unproven nil-capable references before publication. The
check preserves the destination contract across flow rechecking and reports the
original value expression's location. Existing call entry checks retain argument
evaluation order. Raw Go and nullable destinations remain unchanged. Independent
Go comparisons cover private function types, package initialization, retained old
values after failed writes, finally ordering and fixed-array `len`/`cap` in
`storage_boundaries_test.go`; editor tests preserve source hover/navigation.

v0.4.12 builds validate forwarded ordinary multiple results and ordinary
parallel assignments per destination contract. The producer runs once and all
required checks complete before publishing any result or writing any target.
Typed checks preserve interface boxing of typed nil pointers. Nullable/raw Go
results and explicitly discarded assignment slots retain their behavior. Other
slot mismatches remain errors, including incompatible text and class storage.
Independent comparisons cover private Go functions, qualified callback types,
generics, temporary-name hygiene, duplicate targets, for-post clauses and finally
in `multiple_storage_boundaries_test.go`; editor tests preserve source navigation.

v0.4.12 builds validate checked assignments to existing non-null reference
bindings using the producer's success flag. Map lookups, receives, assertions
and selected receives check the payload on `ok`. `Result` assignments check
it on a nil error. Discarding the flag still evaluates and uses it for the
check. Selected receive targets are evaluated before validation, and a failed
check prevents all writes. Independent Go comparisons cover absence, nil on
success, typed-nil values and errors, private types, and evaluation order in
`checked_storage_boundaries_test.go`.

v0.4.12 builds apply successful-reference validation to inferred checked
split declarations, including map lookups, receives, assertions and `Result`.
Initializers capture the producer once and validate before body effects, even
for discarded success flags, unused named payloads and for initializers.
Selected receive declarations validate before the selected body. Generic native
references, nullable contracts and typed-nil interface semantics have independent
Go comparisons in `split_storage_boundaries_test.go`.

v0.4.12 builds validate inferred non-null native references in ordinary
multiple-result declarations before following source statements. Per-slot checks
use the inferred storage directly, including unused named values. Private Go
companion types remain inferred. For initializers validate once before their
first condition, retaining Go iteration variables and labeled branch/goto
behavior. Independent comparisons cover source locations, closure snapshots,
multiple slots, nullable/raw values, interfaces, generics and finally behavior
in `inferred_tuple_boundaries_test.go`.

v0.4.12 builds validate non-null native references when forwarding a
`Result` on a nil error. Propagated `?` bindings validate after the error branch
and destination interface boxing, including inferred and unused named bindings.
The operand runs once. Forwarding preserves error results and their original
payloads; propagation exits with the caller's zero payload on error. Independent
comparisons cover typed nil, private Go callbacks, source aliases, generic
references, Task joins and finally behavior in `result_boundaries_test.go`.

v0.4.12 builds require a successful error check before reading a split
`Result<T>` payload whose zero is unsafe or unknown. The matching local error
binding must compare equal to nil (or null for nullable error storage). Branches,
short-circuit conditions, boolean and direct error switches carry the proof;
copies, reassignments, escapes and mutable captures cannot reuse a stale proof.
Safe-zero and nullable payloads retain their zero contracts. Independent Go
comparisons cover typed-nil errors, failure payloads, generic values, closures,
loops and reassignment in `result_presence_test.go`.

v0.4.12 builds support optional member/index/call chains with lazy arguments
and indices, nullable reference results, checked value results, `??` and checked
postfix `!`. Parentheses end short-circuiting. Independent Go comparisons include
linked declarations, nested chains, nil versus empty slices, source-located
assertions and generated temporary-name hygiene in `optional_chaining_test.go`.

v0.4.12 builds validate mixed reference/value bounds selectively at callable
entry and storage/result publication, conversions, nil-capable assertions,
checked splits/receives and Result boundaries. Scalar zeros and raw Go nils of
other storage kinds remain valid. Native/raw alternatives sharing the same
nil-capable kind require a concrete type or separate uniform constraints; mixed
reference/value assertion targets still require a concrete target. Nominal
source and imported callback identities are retained when restoring source
constraint terms. Selective checks do not prove that raw alternatives are
non-null. Independent Go coverage is in `mixed_non_null_test.go`.

v0.4.12 builds also validate native non-null references extracted from
Go-owned fields, collection elements, callbacks and other unproven storage.
Each read checks its own reference before source use, including nested and
cyclic data, without eagerly traversing the entire object graph. Checked local
snapshots retain their identity; mutation of shared storage is checked again
when that storage is read. Raw Go and explicit nullable contracts retain their
nil semantics, and interface checks do not inspect hidden implementation
pointers. Concurrent Go mutation still requires normal Go synchronization.
Independent Go coverage, single-evaluation and fixed-array effect checks are in
`reference_reads_test.go`; checker and editor regressions cover rechecking,
success-proof joins and navigation. These complete the scoped #117 work; the
documented raw Go behavior and concrete-type requirements remain intentional.

## Approved implementation queue

Decorator language support is now requested in addition to this queue. See
[decorator design and implementation stages](decorators-design.md). Syntax and
AST retention, factory expression checking, typed decorator context target
descriptors, per-factory target restrictions and Go `init` registration are
implemented. Override chains are exposed without implicitly replaying inherited
decorators. Canonical external-package identities and dependency-first
registration are verified across checkout paths and import aliases. Checked
constructor adapters are implemented for concrete classes, including variadic
constructors. Public instance method adapters are implemented with checked
receivers and arguments, Result propagation, and virtual dispatch. Static method
adapters use a separate receiver-free call. Public getter/setter adapters reuse
these checked calls, preserving independent visibility and virtual dispatch;
generic-independent static accessors are supported as well. Multiple-result
methods return an ordered array of individually checked boxed values through
the same invocation adapters, preserving error slots separately from Result
propagation and leaving ordinary typed calls unchanged. Override adapters
preserve dispatch through further-derived implementations, including accessors;
uninitialized virtual receivers supplied by Go interop are rejected. Field and
parameter nominal identities resolve transparent aliases and distinguish
concrete generic arguments, including nested nullable contracts; open generic
consumers do not claim concrete DI keys.
Concrete generic instantiations
remain; context type/field completion and hover are implemented.

Constrained collection forwarding now infers common slice/array/map/channel
element types, including native class elements. Assignment checks both Go
storage compatibility and source nullable contracts for every constraint term;
class inheritance does not make writable collections covariant.

Explicit native and imported Go conversions now retain source nullable
contracts through function signatures (including Result/multiple results),
collections, struct fields, named types and generic constraint terms. Matching
conversions still lower directly to Go, with linked-module runtime comparisons
and editor diagnostics in `conversion_contracts_test.go`. This closes an
explicit-conversion bypass; it is not a claim that every interop path has been
audited.

The same checked-storage comparison now covers assignments, returns, arguments
and generic class/interface ancestor matching, including nested named containers
and callable types. Recursive generic named storage uses semantic Go type
identity to terminate even when substitution creates fresh instances. Accepted
aliasing, callback and generic DI behavior is compared with independent Go in
`assignment_contracts_test.go`. Indexing and slicing native named collections
retain their declared element/key qualifiers, including after generic
substitution and checked map indexing.

Native defined collections also retain these contracts during range, channel
send/receive/select and map deletion, including sole multiple-result delete
arguments. Native named iterator signatures preserve their yielded source
types. Coverage in `named_collection_contracts_test.go` includes linked export
aliases, nullable keys/elements, array-pointer views and editor inference.

Nullable conditions now retain separate true/false flow states through `&&`,
`||` and `!`, including short-circuit operands and loop exits. Mutation effects
and Task consumption are merged only across the paths that can evaluate them;
compile-time boolean left operands preserve guaranteed/skipped awaits.
`nullable_conditions_test.go` covers semantic rejection, independent Go runtime
comparison and editor diagnostics. Boolean aliases do not retain null proofs.

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
| 7 | Constraint intersections and interface composition | Source/imported Go type sets, Go method interfaces, and comparable requirements compose; v0.4.13 builds also preserve named/anonymous source interface contracts through composition |
| 8 | Remaining constant contexts | Scalar reference chains, constant `min`/`max`, constant-string `len`, and fixed-array/array-pointer `len`/`cap` preserve values and types; bounds and size checks consume these constants; further contexts remain |
| 9 | Source anonymous-interface syntax | `interface { name(parameters): result; }` lowers to a Go anonymous interface. v0.4.13 builds allow embedded Go and anonymous method interfaces in value types and bounds |
| 10 | Source-declared multiple results | Implemented for `(T, U)` signatures, explicit returns, forwarding, contextual callbacks, arrow result inference, try/catch/finally and sole-argument expansion including generic source/Go functions, instance methods, Task launches, append/copy/delete/min/max/complex, and permitted unsafe Add/Slice/String. v0.4.13 builds add named results with valid zero values and accept paired slice lengths and capacities in `make` and `makeSlice`. Audit remaining builtin argument forms |
| 11 | Abstract classes and methods | Implemented: explicit abstract contracts, concrete overrides, generic DI and multi-level dispatch; construction boundary documented below |
| 12 | Getter/setter properties | Instance/static accessors and interface contracts implemented, including independent visibility, generic inheritance, virtual/abstract/final instance overrides, DI, ordered updates and editor support |
| 13 | Static fields and constants | Mutable fields and typed scalar `static const` members implemented, with inherited visibility, generic-independent scope, Go variables/constants and initialization checks |
| 14 | Receiver/constructor-dependent field initializers | Earlier initialized fields and accessible inherited fields may be read through `this`; constructor parameters and receiver capture remain |
| 15 | Parser decomposition | Implemented: grammar-focused files, shared token state/checkpoints and recovery; parser behavior unchanged |
| 16 | Go emitter decomposition | Implemented: responsibility-focused emission modules with unchanged lowering and runtime helpers |
| 17 | Lexical, capture, and nullable-state boundaries | Shared callable-flow and closure lexical/capture boundaries implemented; broader checker-state decomposition remains |

Local arrows can return parameterized classes using an enclosing function's
type parameter in result lists, including inherited classes:

```ts
class Box<T> { constructor(public value: T) {} }
function example<T>(value: T): void {
  const pair = (): (Box<T>, int) => { return new Box<T>(value), 1; };
}
```

Result-list signature comparison checks each slot even before a complete Go
storage type is available. Nominal class identity, type arguments, nullability,
result count and element types remain checked. Generic inherited-class arrows
and their use in multiple assignment are covered by runtime tests.

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

Available since v0.4.7: native class, struct/pointer,
source interface and distinct-type values can satisfy Go method requirements on
source generic declarations. Method metadata preserves inherited/generic owner
substitution, dependent inference, getter/setter names, Result/variadic lowering,
visibility, pointer method sets, text/nullability and type-set requirements.
Differential coverage: `native_constraint_methods_test.go`. Native source
interface operands are supported in v0.4.13; direct Go generic-call
restrictions remain separate work.

Direct native source interface bounds (`T extends Reader<E>`) are implemented
in v0.4.7. Source-aware structural method matching, dependent inference,
generic owner/method substitution, properties, Result/multiple/variadic results,
forward declaration checks and editor names preserve checked contracts.
Early type applications are revalidated after complete method declarations.
Differential coverage: `source_interface_constraints_test.go`. v0.4.13 builds
also support composition into source type-set declarations, preserving source
signatures, property contracts, dependent inference and generic owner boundary
checks. Differential coverage: `source_constraint_composition_test.go`.

## Syntax and contract additions in v0.4.0

Semantic refactoring and source-encoding diagnostics accompany these additions
relative to v0.3.0:

- **Invariant method contracts:** implementations, overrides, and inherited
  conflicts retain nested nullable and generic qualifiers through linked
  re-exports. Compatible source `Result<T>` implementations of Go multiple-result
  methods retain runtime behavior. Covered by `method_contract_test.go`;
  v0.4.13 builds also preserve these source signatures in constraint operands.

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
  Arrows need an explicit Result annotation or matching function context.
  Expression bodies follow the same checks as explicit returns, including
  exhaustive matches returning a Result. Result calls require `?`, explicit
  split binding, matching, forwarding return, or intentional discard with `_`.
  Unused named local Result error bindings are diagnosed;
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
  Local named-import aliases (`import { A as B }`, including `import go`)
  preserve original type/storage identity and support generic values and source
  re-exports. Compiler differential and editor tests in `import_aliases_test.go`
  cover alias boundaries, collisions, shadowing and unsafe permissions.
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

Permitted `unsafe.Slice`/`SliceData` calls now support constrained pointer/slice
operands and source class elements without erasing source nullability. Shared
integer-context checks handle constant reference chains, integer-valued untyped
floating/complex constants and nonconstant shifts in lengths/offsets. Coverage
in `unsafe_collections_test.go` compares generic helpers, generic class methods,
aliasing, writes, nil/empty behavior and evaluation order with independent Go.
Negative/nil-invalid runtime lengths are checked in ordinary runs; race runs
retain valid operations because Go checkptr makes those failures unrecoverable.
The explicit unsafe permission and its lifetime/pointer obligations are unchanged.

Permitted `unsafe.Sizeof`, `Alignof` and `Offsetof` now retain target-dependent
`uintptr` constants for fixed layouts. Variable-size generic operands remain
nonconstant; concrete pointer/slice headers keep fixed sizes. Source and class
constant aliases, Go export constants, promoted Go fields, unevaluated operands
and nested `len` classification are checked in `unsafe_layout_constants_test.go`.
Source structs and structural objects also support `Offsetof`, including
aliases, distinct types, pointers and generics. `source_offset_constants_test.go`
compares private storage names, padding, zero-length fields, linked exports and
unevaluated selectors with independent Go. Class storage and property/method
selectors remain excluded.

After v0.4.3, `make[T](...)` allocates concrete, named and constrained slices,
maps and channels without losing target identity. Source and imported bounds,
generic class methods, nullable elements, integer-valued constant sizes,
ordered evaluation, empty/non-nil allocation and dynamic size failures are
compared with independent Go in `collection_make_test.go`. Different underlying
slice/map types and conflicting channel directions remain source errors.
Constructor nonempty-range proofs distinguish slice lengths from map hints
and channel capacities. Existing element-oriented allocation helpers remain.

After v0.4.1, `closeGoChannel` accepts send-capable channel type parameters,
including mixed element types, named channels, source/imported constraints and
generic class methods. Buffered draining, nil/double-close/send-after-close
panics, evaluation count, deferred/concurrent close and nullable OOP elements
are compared with Go in `generic_channel_close_test.go`. Generic send/receive
expressions, checked receives and select communication now also accept compatible
directional bounds with one element type and source nullability. Source/imported
constraints, generic class methods, nil/default behavior, class upcasts and
evaluation counts are compared in `generic_channel_operations_test.go`.
Generic channel range is already supported for compatible receive-capable bounds.

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
common elements, including OOP elements and generic methods. `copyArray` also
accepts array-constrained target parameters with common element contracts,
including differing lengths, imported bounds and generic class methods.
`viewArray` still requires a concrete fixed-array target shape (`[2]E` or a named
instantiation), not a bare array-constrained parameter.
`generic_array_target_test.go` covers parameter targets and runtime length
evaluation; `generic_array_conversion_test.go` covers shallow
copy versus shared slots, nil/zero length, short-source panics, evaluation, and
preserved class/interface/nullability contracts after indexing and reslicing.
This is not a claim of complete generic collection support.

In v0.4.1, common-underlying-shape type parameters also support direct
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

After v0.4.1, nullable fixed-array pointers participate in the same constant
`len`/`cap` rules as non-null pointers, including named arrays/pointers and
generic elements. Calls/receives still evaluate at runtime, and a constant
length does not establish non-nullness. Bounds and size diagnostics consume
these constants; differential coverage is in `nullable_array_constants_test.go`.

| Area | Current limitation | Next implementation boundary |
|---|---|---|
| Import alias name isolation | Dependency and canonical Go package names avoid local bindings and type parameters; explicit local shadowing and shared imported storage are preserved | Aliased references between multiple explicitly supplied root modules that would capture a local binding are diagnosed; rename that binding rather than changing the root's public Go name |
| Source constraint composition | Source/imported Go type sets, comparable and Go method interfaces compose; v0.4.13 builds add named/anonymous source interface operands, preserving generic source signatures, properties, Result/text/nullability, inference, linked aliases and editor metadata | Raw Go generic method arguments retain interop restrictions; incompatible source signatures are rejected even when Go storage agrees. Intersections whose overlap depends on later substitution remain conservative |
| Constant contexts | Indices, slicing, collection/channel sizes, generic scalar arguments, scalar reference chains, constant `min`/`max`, constant-string `len`, and fixed-array/array-pointer `len`/`cap` (including nullable pointers) are supported; nonconstant shifts preserve destination/peer types through assignments, returns, conversions, arithmetic, comparisons and native/Go generic calls (`shift_context_test.go`); machine-width and unsafe layout constants use selected target sizes in semantic and generated Go checking; type-parameter values and runtime bindings remain nonconstant. v0.4.7 builds accept integer constant expressions in array type lengths, fold them into Go array types and preserve LSP references (`array_type_lengths_test.go`) | Audit remaining contextual-expression combinations and constant contexts without treating immutable runtime bindings as Go constants |
| Anonymous Go interfaces | Exported runtime method sets are supported through imported APIs, inference and source method-only `interface { ... }` type literals | Source fields, private method identities and richer anonymous-interface members remain rejected |
| Source-declared multiple results | Source functions, methods, arrows and function types accept `(T, U)` signatures, explicit returns and forwarding, including try/catch/finally; v0.4.13 builds add named slots with safe zero initialization and bare returns | Inference rejects ambiguous nil/null slots and incompatible branches. Multiple-result properties remain unsupported |

These entries come from the parser, `internal/sema/checker.go`,
`internal/sema/types.go`, and `internal/sema/go_audit.go`; they do not imply that
an API classified as supported by the Go API audit has equivalent native syntax.

Value-switch constant checks now default untyped subjects, preserve interface
dynamic-type identity, and check duplicate scalar expressions after contextual
rounding. Imported/re-exported constants and runtime-versus-constant bindings
are covered by `switch_constants_test.go`; `switch_go_parity_test.go` checks
the duplicate rules directly against Go's type checker. Repeated boolean and
complex cases follow Go's first-match behavior. The existing duplicate
`nil`/`null` case restriction remains a source-language difference.

## OOP gaps and design decisions

| Area | Current state | Required work |
|---|---|---|
| Receiver-dependent field initializers | Module-scope defaults and reads of earlier initialized or accessible inherited fields through `this.field` are implemented; later local fields, `super`, constructor parameters and receiver capture remain unavailable | Extend only with explicit initialization-order and receiver-escape rules; use the constructor for other dependencies |
| Go interface inheritance boundary | Exported runtime contracts are supported; private methods, type-set-only bases, and unsupported interop signatures are rejected | Expand anonymous interface interop independently; preserve package identity and unsafe policy |
| Recursive comparable source bounds | Array/struct comparability that depends on a still-resolving parameter is diagnosed rather than allowed to deadlock Go type resolution; recursive pointer and method contracts remain supported | Break the dependency with an independently constrained element parameter; broader cyclic value-bound solving remains unsupported |
| Source struct constraint terms | Implemented in v0.4.13: native storage completes before dependent constraints, including generic instances, aliases, forward references, arrays/slices and method bounds; source nullable/text contracts survive inference and record matching | Named terms are exact, not structural wildcards; `~NamedStruct` and cyclic value storage are rejected. Field selection on a type parameter remains unavailable; expose behavior through method contracts |
| Abstract classes/methods | Explicit abstract method declarations and concrete implementation checks are implemented; abstract classes cannot be constructed directly | Interface requirements must be declared explicitly; direct constructor access to abstract methods is rejected, while indirect access to an unimplemented construction-phase slot panics; method-level generics remain nonvirtual |
| Getter/setter properties | Instance/static properties and interface contracts support independent visibility, exact paired types, generic/diamond interface inheritance, DI, virtual/abstract/final and partial instance overrides, phase-local construction dispatch, single-evaluation updates, nullable-flow invalidation and public Go accessor APIs; differential coverage in `class_properties_test.go`, `virtual_properties_test.go`, `interface_properties_test.go` and `static_properties_test.go` | Static accessors cannot capture class type parameters or be overridden; adding a missing accessor to an inherited class property is rejected; no property storage, implicit Result/Task handling or stable getter narrowing |
| Static fields/constants | Mutable fields and typed compile-time scalar constants implemented with explicit initializers, enum-member references, shared inherited/generic scope and Go package APIs; v0.4.12 supports accessible static constants in array type lengths, including forward/inherited constants | Continue auditing contextual-expression combinations and initialization dependencies |

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
