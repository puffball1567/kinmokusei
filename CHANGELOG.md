# Changelog

All notable user-facing changes are recorded here. During pre-1.0 development,
Kinmokusei uses milestone-based minor versions. The v0.4.x series includes
compatible feature additions and fixes in patch releases; v0.5 marks completion
of the audited Go compatibility work. This differs from strict SemVer feature
numbering. Intentional source or public API breaks require a minor release and
migration notes; corrections rejecting invalid programs are documented fixes.

## [Unreleased]

- Fix decorator field/parameter type identities losing transparent aliases and
  conflating different generic instances. Use checked nominal types across
  class, interface and struct aliases/re-exports; preserve nullable contracts
  in generic arguments and leave unresolved generic consumer keys empty.

- Fix decorated overrides bypassing further-derived implementations. Method,
  getter and setter adapters now use the ordinary virtual dispatch slot across
  multiple inheritance levels, preserving Result failures and multiple returns.
  Reject uninitialized virtual receivers supplied through Go interop before
  calling a method body.

- Support checked decorator invocation of public getters and setters, including
  static properties, virtual overrides, and generic-independent static
  accessors. Preserve independent visibility, exact property types, receiver
  checks and arity checks.

- Support result-list methods through decorator invocation adapters. Return
  an ordered boxed `DecoratorValue[]` with exact per-slot contracts, preserving
  narrow types, nullable values, interfaces and concrete generic values. Keep
  ordinary error result slots separate from Result failure propagation and
  invoke instance/static, variadic and virtual methods exactly once.

- Harden external package source boundaries: reject entries, exports and
  relative imports under unhashed metadata directories, including symlink and
  case/trailing-dot aliases of `.git` and `.kinmokusei`.

- Preserve declared storage and source contracts across decorator values and
  all callable adapters. Fix numeric-width and typed-nil round-trips; reject
  nullable/context contract confusion, including nested collections/callbacks.
  Interface DI uses an explicitly boxed interface contract; open type-parameter
  payloads now produce a source diagnostic instead of erasing their contract.
  Resolve adapter parameter types before Go generation, including interfaces.

- Accept keyword-named members after `.` so decorator metadata such as
  `context.static` is accessible without special-case parsing.

- Bound LSP header size independently of payload size and reject duplicate
  Content-Length headers, including identical duplicates.

- Fix sending, select sending and closing class-valued channels returned by
  generic helpers. Reconstruct native Go storage without dropping source
  element types, nullable checks or send/receive direction restrictions.
  Accept these channels as generic arguments and prevent a compiler panic
  when storing inferred channels without cached Go storage in local variables.

- Accept constrained collection assignment with native class elements using
  Go storage compatibility plus per-term source nullability checks. Preserve
  invariant elements, array lengths and channel directions. Fix direct and
  checked receives from explicitly typed channels carrying native classes.

- Infer element/key types when generic wrappers forward constrained slices,
  arrays, maps and receive-capable channels to native generic helpers. Preserve
  caller type-parameter identity and contextual callback inference; ambiguous
  collection bounds and incompatible nullable elements remain rejected.

- Infer native generic arguments between named and unnamed slices, arrays,
  maps, pointers and channels, including imported Go collections and dependent
  callbacks. Keep named identity, array lengths, channel directions and nullable
  element checks enforced by argument compatibility.

- Infer native generic arguments across named and unnamed callback signatures,
  including scalar/multiple results, dependent arrows and generic methods.
  Preserve distinct function identity, nullable contracts and Result effects.

- Infer native generic type arguments from each callback result-list slot,
  including named function values, direct arrows, partially explicit arguments,
  generic methods, nested class arguments and dependent callback contexts.
  Diagnose conflicting results, mismatched arity and unsatisfied constraints.
  Preserve inferred arguments explicitly in Go output for interfaces nested
  inside callback result lists, consistently with single-result callbacks.

- Fix identical generic class result-list signatures being rejected for local
  arrows when Go storage types are unavailable. Compare individual result
  contracts while retaining nominal identity, type arguments and nullability;
  cover contextual/inferred arrows and inherited classes with regression tests.

- Apply class upcasts in multiple assignment to existing base-class variables,
  including generic and nullable classes, checked map lookups and channel
  receives. Capture results once before converting and assigning them; preserve
  blank targets, repeated targets, virtual dispatch and for-post use.

- Reject unused type conversions in statement and for-clause positions during
  source checking, including native, imported Go and type-parameter conversions.
  Keep explicit discards and ordinary function calls, with evaluation verified
  against handwritten Go.

- Diagnose unused value-only built-ins (including sole-call result expansion)
  in expression statements and `go`/`defer` statements before Go generation.
  Explicit `_ = ...` discards remain available; copy/delete/clear/close and
  user-defined functions shadowing built-in names retain statement use.

- Accept sole-call multiple-result arguments in `min`, `max`, and `complex`.
  Preserve runtime operand types, generic ordered constraints, complex64 width,
  single evaluation, NaN and signed-zero behavior; reject incompatible result
  types, wrong arity and spread syntax before Go generation.

- Expand collection built-in tuple arguments through explicit bindings in
  generated Go, avoiding a Go 1.26+ vet panic while retaining single evaluation,
  generic result types and eager capture for deferred calls. Vet remains enabled.

- Reuse ordinary call lowering for all Task launches, preserving inferred and
  explicit generic calls, generic method receivers, spread arguments, and
  contextual argument types such as `byte` and nil pointers. Generate fresh
  capture names so user variables cannot be shadowed by Task temporaries.

- Support sole-call multiple-result expansion in `append`, `copy`, and `delete`,
  retaining slice element and map key checks. Fix Task launch argument capture
  for multiple-result producers, including generic functions and methods;
  evaluate the callee/receiver and producer before starting the worker.

- Accept a multiple-result call as the sole unspread argument to functions,
  methods and variadic callables: `consume(produce())`. Generic source and Go
  functions support inferred and explicit type arguments. Check result
  count and each argument type while preserving single evaluation. Calls that
  need per-value source conversions must destructure first. Generic instance
  methods capture the receiver before expanding results, including when deferred.

- Add source function, method, arrow and function-type result lists such as
  `function cut(text: string): (string, string, boolean)`. Matching multiple-result
  calls can be forwarded directly, including contextual callbacks and generic
  function aliases. Preserve nullable contracts and type references in editor
  rename/references. Comma-separated returns (`return a, b`) check each value
  in its declared context, including numeric bounds and class upcasts.
  Multiple-result returns also cross try/catch/finally, preserving evaluation
  order, typed nil values, generic types and finally overrides. Arrow result
  lists can be inferred from forwarded calls or explicit return expressions,
  including forward dependencies and try/finally. Ambiguous nil/null results,
  mismatched branches and numeric overflow are diagnosed; multiple-result
  properties remain unsupported.

- Add method-only anonymous interface type syntax, for example
  `interface { read(offset: int): string; }`. Source declarations preserve
  exported method signatures and lower directly to Go anonymous interfaces;
  fields and unsupported private identities remain rejected.
  Structural class matching substitutes class type arguments and checks source
  nullable parameter/result contracts before Go storage erases their qualifiers.

- Allow enum members in typed class constant initializers. Their existing
  generated Go constant identity and underlying enum type are preserved.

- Add typed `static const` class members with compile-time scalar initializers,
  inherited visibility, forward/module references, generic-independent scope,
  exact constant operations and typed rounding. Emit public Go constants and
  support editor completion, navigation and rename. Reject mutation, address
  taking, runtime initializers and cycles, including cyclic constant array
  `len`/`cap` dependencies.

- Add explicitly initialized mutable static class fields, shared across generic
  instantiations and descendants, with visibility checks, addressable storage,
  collection/callable values and public Go package variables. Exclude static
  state from instance construction and JSON. Diagnose initialization cycles
  through fields, static accessors/methods and constructors. Editor completion,
  navigation and rename recognize static fields and module-level type scope.

- Add static getter/setter properties accessed through class names, with
  inherited owner lookup, independent visibility, ordered updates and public
  Go package functions. Generic classes may declare type-argument-independent
  static accessors. Diagnose global initialization cycles through static
  accessors and methods; keep ordinary assignments independent of getters.
  Reject local bindings that would shadow a selected generated static accessor
  or method, preventing calls from being silently redirected.

- Add instance getter/setter properties with independent visibility, generic
  and inherited types, read/write diagnostics and single-evaluation updates.
  Support virtual/abstract accessors, explicit and final overrides, partial
  accessor overrides and phase-local construction dispatch. Generated Go exposes
  accessor methods; LSP supports completion, navigation and inheritance-aware
  paired getter/setter rename. Interfaces can declare public property contracts,
  including generic/diamond inheritance and class or abstract-class DI.

- Extend permitted `unsafe.Slice`/`SliceData` calls to compatible pointer/slice
  type parameters and source class elements, preserving source nullability.
  Accept integer-valued untyped constants and contextual shifts in unsafe lengths
  and offsets; diagnose negative lengths and overflowing constant reference
  chains. The explicit unsafe permission and Go lifetime obligations are unchanged.

- Extend scalar constant checking for value switches: default the subject's
  type, reject overflowing subjects/cases, recognize constant expression and
  re-export chains, and account for floating-point rounding and interface
  dynamic-type identity when checking duplicates. Preserve Go's first-match
  behavior for repeated boolean/complex cases and runtime bindings. Separate
  value-switch and constant-case checks from channel/type-switch checking.

- Preserve contextual types for nonconstant shifts in assignments, returns,
  conversions, operators and generic calls. Diagnose overflowing left operands
  and floating-point shift contexts before Go generation, while accepting
  integer-valued untyped floating shifts in indices, slice bounds and allocation
  sizes. Keep runtime bindings typed and preserve evaluation and panic behavior.

- Prevent slice-allocation temporaries from shadowing user size bindings,
  including existing `makeSlice` calls with explicit capacity.

- Add `make[T](...)` for concrete, named and constrained slice, map and channel
  targets, preserving the requested type and source nullability. Check type-set
  compatibility, sizes and channel directions; retain ordered size evaluation,
  constructor slice-cardinality proofs and editor signatures/navigation. Existing
  element-oriented allocation helpers remain supported.

## [0.4.3] - 2026-09-19

- Automatically register a source import alias when `keika deps add` installs
  a Kinmokusei package. Derive it from the module name (ignoring a trailing major
  version), allow `--alias` overrides and reject collisions transactionally.
  No manual manifest edit or separate lock operation is needed.

- Split Go generation into declaration, class, type, statement, expression,
  Result, exception and runtime-support modules without changing generated code.

- Split the parser into grammar-focused files while retaining its shared token
  checkpoints, source spans and error recovery. No syntax or diagnostic changes.

- Keep an imported module's `main` binding module-local even when the entry
  has no same-named declaration. Imported constants, variables and ordinary
  functions named `main` no longer inherit entry-arrow restrictions or supply
  an accidental executable entry point. Retain source names in editor features.

- Add manifest `[imports]` aliases for external Kinmokusei source imports and
  re-exports, including submodules. Resolve aliases in the importing package's
  own manifest without changing canonical dependency or type identities.
  Validate reserved/colliding paths, preserve aliases through dependency edits,
  and remove aliases when their direct dependency is removed. Cover offline CLI
  workflows, compiler output identity, package isolation and editor navigation.

- Allow array-constrained type parameters as `copyArray` targets, including
  differing lengths, imported/re-exported constraints and generic class methods.
  Preserve element identity/nullability and shallow-copy semantics; compare
  generated behavior with independent Go implementations and add editor tests.

## [0.4.2] - 2026-09-19

- Include Go and applicable third-party license, patent and source notice
  materials in every CLI archive, collected from the actual build toolchain
  and target dependency graph. Verify their contents before publication and
  refuse unreviewed external dependencies. Preserve existing output directories.

- Broaden generic constraint intersections to safely discard disjoint collection
  shapes, array lengths, channel directions and nominal types. Support combined
  writable/readable bounds and slice/array/map narrowing through source/imported
  constraints, re-exports and generic class methods. Keep substitution-dependent
  matches and nullable conflicts rejected; add Go comparisons and editor/doc tests.

- Support send/receive, checked receives and `select` on channel-constrained
  type parameters, including imported bounds and generic class methods. Preserve
  element identity/nullability and reject incompatible directions or elements.
  Keep directional channel metadata through constraint substitution and class
  upcasts in select sends. Show inferred types for destructured locals in editor
  hover/completion. Add independent Go comparisons and editor/doc tests.

- Support `closeGoChannel` on send-capable channel type parameters, including
  mixed element types, imported constraints and generic class methods. Reject
  receive-only/non-channel bounds; retain evaluation, buffered-drain and panic
  behavior. Add independent Go comparisons, editor and executable-doc tests.

- Treat eligible `len`/`cap` calls on nullable fixed-array pointers as typed
  constants, including named arrays/pointers and generic elements. Preserve
  nil safety and runtime call/receive evaluation; diagnose constant bounds and
  negative sizes without narrowing the pointer. Keep explicit slice/array
  annotations in Go variable declarations, fixing nullable slice initialization
  with `null`. Add independent Go comparisons and editor regression coverage.

## [0.4.1] - 2026-09-19

- Add `keika new app` and `keika new library` with runnable/exported arrow-function
  samples, manifests, offline initial locks, README and ignore files. Support
  custom names, module paths and library license metadata. Refuse existing
  destinations and prepare toolchain-dependent state before publishing output.
  Verify generated libraries can be consumed through local replacements.

- Retain external-package editor context from workspace folders, including legacy
  initialization roots, without requiring an open consumer document. Handle
  workspace additions/removals with refreshed diagnostics and stale-request
  suppression. Restore definition, hover and document symbols for fields declared
  through constructor parameters, including generic classes.

- Resolve external libraries' Go imports from their public source graph rather
  than every file in the checkout. Follow imports, re-exports and public
  submodules; ignore unrelated samples and unfinished tests. Diagnose reachable
  source errors transactionally. Preserve shared local replacements when removing
  direct dependencies and prune replacements that become unreachable.

- Add external Kinmokusei source packages with package metadata, explicit entry
  and submodule exports, exact direct/transitive dependencies, consumer-owned
  sibling replacements, tagged Go-module acquisition, and locked content hashes.
  Integrate ordinary imports with checking, Go emission, builds, execution and
  LSP completion/navigation. Keep local source edits live and generated names
  independent of checkout/cache locations. Add dependency listing, source-package
  updates and lock-preserving fetch; default project commands to their entry.
  Schema 4 locks retain readable Go-only schema 3 compatibility. Normal builds
  and editor analysis disable private VCS and automatic toolchain downloads too.

- Support indexing and slicing type parameters with a common underlying slice,
  array, array-pointer, map, or string shape. Preserve named/parameter slice and
  string results, source OOP/nullability contracts, addressability, checked map
  lookups, bounds diagnostics, and Go runtime aliasing/panic/evaluation behavior.
  Diagnose overflowing or fractional constant map keys before Go generation,
  including ordinary and named maps. Add independent Go comparisons, editor
  tests, fuzz seeds, and executable documentation.

- Extend generic indexing to mixed array lengths, slices, and array pointers
  with identical element types. Support read-only byte indexing and two-index
  slicing over string/byte-slice unions, preserving the operand type. Check
  bounds against every possible array length and retain source nullability
  through generic method specialization and constraint argument validation.
  Reject incompatible nullable fields nested in object element contracts,
  including common-shape constraints.
  Add independent Go runtime comparisons, editor tests, and executable examples.

## [0.4.0] - 2026-09-18

- Share Result error-use tracking with lazy global arrow analysis, preventing
  a crash on split bindings and retaining unused-error diagnostics across
  forward dependencies and nested closures.

- Support `copyArray` and `viewArray` with slice-constrained type parameters,
  dependent elements, named slices/arrays, and generic class methods. Preserve
  class/interface identity and nullable element contracts when indexing or
  reslicing specialized arrays and pointer views. Keep Go's shallow copy,
  shared-view, zero-length/nil, short-source panic, and evaluation semantics.
  Add independent Go comparisons, diagnostics, editor tests, fuzz seeds, and
  executable documentation.

- Support `append` and `copy` with slice-constrained type parameters, including
  dependent element types, named slices, imported/linked constraints, generic
  class methods, contextual callbacks/objects, and byte/string operations.
  Preserve the destination type and Go's aliasing, overlap, growth, nil, and
  single-evaluation behavior. Insert required class upcasts for individual
  appended elements; enforce invariant generic identity and nested nullability
  for slice copies/spreads. Diagnose a missing spread-append source without
  crashing the compiler. Add differential, editor, fuzz, and executable examples.

- Support generic `clear` across slice/map type-set unions, and generic `delete`
  across maps with a common key type even when value types differ. Preserve
  source class identity and nullable keys, linked/imported constraints, generic
  class methods, nil behavior, shared storage, and operand evaluation order.
  Diagnose incompatible key types/qualifiers and numeric constant key overflow
  or fractions, including ordinary maps. Add Go differential, editor, fuzz, and
  executable documentation coverage.
  Isolate a Go 1.26/1.27 printf-vet crash on valid map-union deletion in the
  affected differential fixture; retain runtime tests and all other vet checks,
  and document the generated-module workaround.

- Preserve `min`/`max` constant values, precision, explicit types, and rounding
  through reference chains, source exports, and Go imports. Apply Go's ordered
  operand rules to named types, generic constraints, and mixed typed/untyped
  arguments. Check constant bounds, sizes, fractional results, overflow, and
  address-taking; retain runtime evaluation and storage for nonconstant inputs.
  Preserve contextual nonconstant shifts without losing left-operand overflow
  checks. Split ordered built-in analysis into its own file and add differential,
  editor, fuzz, and executable documentation coverage.

- Preserve typed `int` constants from fixed-array and array-pointer `len`/`cap`,
  including named/imported arrays, generic element types, reference chains, and
  compile-time bounds/size checks. Keep Go's unevaluated-operand behavior without
  suppressing function calls or channel receives. Support `len`/`cap` on generic
  type sets when every member supports the operation; these remain runtime values.
  Add Go differential tests, editor regressions, fuzz seeds, and executable docs.

- Preserve string and boolean constants through reference chains, source
  imports/exports, and Go constant imports. Keep named scalar types in operations
  and prefer typed arguments during generic inference. Support named booleans in
  conditions and constant-string `len` (typed `int`, measured in bytes); diagnose
  constant string index/slice bounds and taking a scalar constant's address.
  Keep calls, mutable copies, string slices, and loop bindings as runtime values.
  Add independent Go comparisons, editor regressions, and executable examples.

- Add abstract classes and bodyless abstract methods with explicit concrete
  overrides, generic dependency-injection types, multi-level inheritance and
  re-abstraction. Preserve interface/method-value dispatch and hierarchy identity.
  Diagnose direct construction, incomplete concrete classes, incompatible
  modifiers/signatures, and abstract super access. Preserve construction-phase
  dispatch with explicit panic on indirect unimplemented slots; omit abstract
  Go constructor factories. Add editor details/completion, regressions, and docs.
- Resolve local callable bindings before same-named top-level functions, so
  extracted methods and callback parameters obey lexical shadowing.

- Preserve numeric constant semantics through local and module reference chains,
  arithmetic, source export aliases, and Go constant imports. Retain precision,
  explicit types and rounding for integer contexts and narrow/generic arguments.
  Diagnose typed constant arithmetic overflow and taking a numeric constant's
  address; retain runtime storage for mutable values, call results, and loop
  initializers.
- Diagnose recursive comparable source type-set bounds that depend on unresolved
  array/struct element constraints instead of hanging during Go type resolution.
  Diagnose source struct constraint terms whose storage is not yet resolved
  instead of panicking. Preserve supported recursive pointer and method constraints.

- Support deliberate Result discard with local `const _ = call()`,
  `let _ = call()`, or `_ = call()`, preserving single evaluation and panics.
  Diagnose unused named local error bindings from Result splits instead of
  silently suppressing them in generated Go. Keep explicit split blanks and
  success-only discard with `const _ = call()?`; preserve type checks and Task
  consumption rules. Blank bindings no longer appear as local editor symbols.

- Compose imported Go type-set constraints with source constraints, including
  nested embeddings, unions, generic collection terms, and explicit
  `comparable` requirements. Preserve source element shapes, method contracts,
  inference, and linked export aliases. Diagnose empty sets and forbidden unions;
  prevent dependent comparable inference from panicking on uninitialized bounds.
- Enforce invariant interface and override method contracts, including nullable
  types nested in callbacks, collections, objects, and generic arguments.
  Reject conflicting inherited contracts and generic methods implementing
  non-generic requirements before Go generation. Preserve compatible source
  `Result<T>` implementations of Go multiple-result methods. Share method
  signature validation and separate interface declaration resolution.
- Compose source constraints with ordinary Go interfaces, including generic
  method contracts, method-only compositions, and type-set intersections.
  Preserve method requirements through constraint reuse and export aliases,
  diagnose conflicting signatures and lossy source type arguments, and offer
  constrained-method completion for generic parameters.
- Support source type-set intersections (`constraint Narrow = A & B`),
  including exact/underlying terms, generic constraint references, inferred
  collection elements, linked export aliases, and editor navigation. Emit Go
  interface embeddings; diagnose empty intersections, erased-nullability
  conflicts, and unmatched parameter-dependent terms before generation.
- Support export aliases (`export { local as publicName }`), including imported
  source bindings and export-from lists. Multiple public names retain one
  declaration and shared storage. Keep implementation-name and public-alias
  rename identities separate across chains, and prevent original names from
  leaking through alias imports. Preserve qualified Go type names when a source
  alias has the same spelling. Split source rewriting from module graph linking.
- Support named source re-exports with `export { name } from "./module"` and
  export lists selecting explicitly imported source bindings. Preserve original
  declaration/type identity, shared mutable storage, dependency initialization
  order, visibility, and editor navigation/refactoring across re-export chains.
- Support function values returning `Result<T>` or `Result<void>` in bindings,
  callbacks, returned closures, fields, collections, and channels. Preserve
  recursive captures, native named/generic function signatures, nullable result
  contracts, Go callback interoperability, and editor navigation/signatures.
  Result calls still require propagation, explicit splitting, or return.
- Support recursive arrows in three-clause loop initializers, preserving
  per-iteration bindings, mutable captures, condition/post evaluation order,
  labeled control flow, and editor navigation.
- Align the architecture and roadmap with Go-only output and direct Go lowering.
- Infer local arrow results through forward dependencies within consecutive
  declaration groups. Check each body once in the group's lexical environment,
  retain generic/receiver context, and preserve source-order capture effects.
  Unresolved recursive result cycles still require annotations.
- Diagnose uninstantiated Go generic functions used as values before Go
  generation, for both qualified and named imports. Direct generic calls and
  ordinary Go function values retain their existing behavior.
- Support local mutual recursion and forward references within consecutive
  direct arrow declarations.
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

[Unreleased]: https://github.com/puffball1567/kinmokusei/compare/v0.4.1...HEAD
[0.4.1]: https://github.com/puffball1567/kinmokusei/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/puffball1567/kinmokusei/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/puffball1567/kinmokusei/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/puffball1567/kinmokusei/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/puffball1567/kinmokusei/releases/tag/v0.1.0
