# Quality and Go compatibility

OnsenTamago verifies compatible behavior against independently handwritten Go
programs. Successfully generating and compiling Go is necessary, but it is not
treated as proof that the source program has the intended runtime semantics.

## Independent differential oracles

For every implemented runtime contract with a direct Go equivalent, the test
suite contains two independent implementations:

1. An OnsenTamago program compiled to Go by the current compiler.
2. A handwritten Go reference program authored from the documented language
   rule.

The two programs run separately and their observable results are compared. The
reference side must not import the generated package, reuse generated
declarations or types, invoke generated helpers, or calculate expectations by
inspecting generated source. These isolation requirements are enforced by the
test harness.

Generated-source assertions are still used to check lowering and public API
shape, but they are structural checks rather than runtime oracles.

## What is compared

Each differential scenario compares every observable property relevant to the
feature, including:

- returned values and errors;
- mutation and side-effect order;
- operand evaluation order and evaluation count;
- value copying versus shared storage;
- reference identity;
- nil and nullable behavior;
- panic presence and recovery behavior;
- channel, task, and concurrency results.

When Go intentionally permits multiple outcomes, such as a `select` with
multiple ready cases, tests compare an independently defined set of valid
outcomes or an invariant instead of requiring two executions to make the same
nondeterministic choice.

## Compatibility contract gate

The compiler maintains a registry of implemented Go-equivalent runtime
contracts. The current registry covers 82 of 82 contract groups. Automated
checks require every registered contract to have an isolated handwritten-Go
scenario and reject unregistered differential scenarios.

A new accepted runtime feature is incomplete until the same change adds:

- its compatibility contract;
- an independently handwritten Go reference;
- normal, boundary, and failure cases;
- comparisons for all relevant observable behavior.

The 82/82 figure describes contract coverage for the implemented runtime
surface. It is not a claim that every Go feature or every planned OnsenTamago
feature has already been implemented.

## Complementary quality gates

Not every correctness property has an executable Go counterpart. Parser and
semantic rejection behavior, source diagnostics, CLI and LSP protocols, C ABI
contracts, and malformed input are verified with dedicated test matrices.

Depending on the affected boundary, changes are also checked with:

- lexer, parser, semantic, and code-generation tests;
- generated-Go formatting, type checking, compilation, and execution;
- external Go consumer packages;
- race detection and `go vet`;
- lexer and parser fuzzing;
- cross-target builds and locked module graphs;
- real C fixtures, ABI compatibility checks, and ownership/release matrices.

Tests should cover multiple valid source forms and feature combinations, not a
single representative spelling. Normal behavior, boundary values, edge cases,
and failures all belong to the feature contract.

## Statement coverage

Statement coverage is measured separately from compatibility contract
coverage. `scripts/coverage.sh` instruments every Go package and currently
enforces an 87.0% repository-wide floor plus an 80.0% floor for each
implementation package. Statement coverage shows which implementation paths
ran; the independent-Go gate shows whether implemented Go-equivalent behavior
matches its reference semantics. Neither measurement replaces the other.

## Generated Go quality

Generated Go is a first-class artifact. It must remain deterministic,
`gofmt`-formatted, readable, buildable as an ordinary Go module, and suitable
for publication or consumption by ordinary Go projects. Generated packages
must not depend on machine-specific paths, private workspace state, or the
OnsenTamago compiler at consumer build time.
