---
title: Quality promise
description: How Kinmokusei verifies semantic compatibility, diagnostics, generated Go, projects, tools, and C boundaries.
---

# Quality promise

Generating Go that compiles is necessary, but it is not treated as proof that a Kinmokusei program has the intended behavior.

## Independent Go oracles

Every implemented runtime contract with a direct Go equivalent is tested with two isolated programs:

1. Kinmokusei source compiled by the current compiler.
2. An independently handwritten Go reference authored from the documented rule.

The reference package cannot import generated output, reuse its declarations/types/helpers, or calculate expectations by inspecting generated source.

The comparison covers values and errors, mutation, evaluation order/count, copy versus alias behavior, identity, nil/null behavior, panics, channels, tasks, and concurrency. Nondeterministic Go behavior is compared by an independent valid-outcome set or invariant.

## Contract coverage

The registry currently covers **75 of 75** implemented Go-equivalent runtime groups. This is 100% coverage of the registered implemented surface—not line coverage, not a claim that all Go features exist, and not a roadmap-completion percentage.

A new accepted runtime feature must add its compatibility contract, independent reference, normal/boundary/failure cases, and all relevant observable comparisons in the same change.

## Complementary gates

Language-specific rejection and non-runtime boundaries use dedicated tests:

- lexer, parser, semantic, lowering, and diagnostic matrices;
- generated-Go format, type, build, execution, and external-consumer checks;
- CLI and LSP protocols, incremental editing, cancellation, and race checks;
- targets, locked/offline module graphs, CGO, unsafe policy, and licensing;
- real C fixtures, ABI comparison, callback/thread, ownership, and release matrices;
- race detection, `go vet`, and lexer/parser fuzzing.

## Statement coverage

Repository statement coverage is measured separately. The current gate enforces an 87.0% repository floor and an 80.0% floor for each implementation package. Statement coverage shows which implementation paths ran; independent differential testing shows whether Go-equivalent behavior matches a separately implemented reference. Neither replaces the other.

## Generated artifacts

Published generated Go must be deterministic, `gofmt`-formatted, buildable/testable by ordinary Go tools, consumable from an external module, free of machine-specific paths, and independent of a Kinmokusei compiler runtime.
