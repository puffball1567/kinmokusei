---
title: Contributing to documentation
description: Keep Kinmokusei documentation accurate, executable, accessible, and aligned with the compiler.
---

# Contributing to documentation

Public documentation is versioned with the compiler. A page should explain implemented behavior precisely enough that a reader does not need compiler source or an internal design note.

## Before editing

- Read the relevant design document in `docs/` for settled terminology.
- Confirm edge behavior in compiler tests or with a focused executable case.
- Mark implemented, experimental, planned, and unsupported behavior explicitly.
- Do not invent syntax to complete a narrative.

## Examples

Store valid behavior-bearing examples in `website/snippets/*.km` and embed each source from at least one public page with VitePress's `<<<` syntax. When output matters, add a same-name `.stdout` file. For a built command-line program, add a same-name `.args` file with one argument per line; an argument contract must have an output contract. The docs check builds the current compiler, checks every source, runs files with output expectations, and compares exact stdout.

Store intentionally invalid examples in `website/snippets-invalid/*.km` with a same-name `.stderr` file containing one narrow stable diagnostic fragment. Do not overfit to a whole formatted message when one rule fragment proves the contract.

For Go-equivalent runtime behavior, use an independently handwritten Go oracle in compiler tests. Generated Go cannot be its own expected result.

## Choose the canonical layer

Put each explanation where readers will look for its primary purpose. Link to that page instead of maintaining parallel copies.

| Layer | Primary job |
| --- | --- |
| Language Manual | Teach connected language semantics and the reasoning behind them |
| Guide | Complete a task or make an implementation decision |
| Reference | Provide a compact, exhaustive lookup contract |
| Examples | Show a complete runnable scenario and its observable result |

## Writing

- State the rule before edge cases.
- Give every page a frontmatter title and description, and every non-home page exactly one H1.
- Keep every title and description distinct. Link a new page from navigation or another public page; use `unlisted: true` only for an intentional utility or compatibility page.
- In Manual chapters, explain syntax form, type/scope rule, evaluation behavior, and rejection boundary separately.
- Do not copy the same explanatory example into Manual, Guide, and Reference; link to the canonical layer or use a purpose-specific example.
- Make each paragraph state a contract, explain its reason, demonstrate it, or define a boundary. Remove a paragraph that only restates its heading, adjacent code, or the previous summary.
- Use one canonical term for each concept.
- Distinguish Kinmokusei, Go, and TypeScript code in prose and fences.
- Explain value/reference behavior, evaluation count/order, and failure paths where they affect correctness.
- Avoid unsupported marketing claims and incomplete parallel translations.
- Do not leave unfinished editorial markers, noncanonical product spellings, or machine-specific absolute paths in public pages, `README.md`, `CHANGELOG.md`, or `docs/`.

English is the canonical initial documentation language. URL structure and terminology should remain suitable for a complete Japanese translation later.

## Site checks

From `website/`:

```sh
npm ci --ignore-scripts
npm run docs:check-links
npm run docs:check-content
npm run docs:build
```

From the repository root:

```sh
./scripts/check-docs.sh
go test ./...
```

The site must work under the configured repository GitHub Pages base path, remain keyboard-accessible and responsive, respect reduced motion and light/dark preferences, and avoid committing `.vitepress/dist` or `node_modules`.

## Review checklist

- The compiler, tests, or a stable design document supports every behavioral statement.
- New prose lives in one canonical layer and nearby pages link to it instead of repeating it.
- Valid examples compile, invalid examples reject for the documented reason, and observable output is checked.
- Titles and descriptions are unique, links and anchors resolve, and readers can discover every ordinary page.
- Keyboard, focus, contrast, narrow-screen, reduced-motion, and light/dark behavior remain usable.
- The snippet, link, content, build, and Go test commands above all pass.

For the broader project workflow and license grant, read [CONTRIBUTING.md](https://github.com/puffball1567/kinmokusei/blob/main/CONTRIBUTING.md).
