---
title: Reference
description: Precise, searchable contracts for Kinmokusei syntax, types, CLI, project files, diagnostics, and compatibility.
---

# Reference

Reference pages state the current compiler contract without tutorial narration. For a progressive introduction, start with [Learn Kinmokusei](../learn/).

<div class="doc-cards">
  <a class="doc-card" href="./glossary"><strong>Glossary</strong><span>Storage, identity, failure, task, module, and Go-boundary vocabulary used across the site.</span></a>
  <a class="doc-card" href="./cli"><strong>CLI</strong><span>Commands, options, outputs, side effects, and exit status.</span></a>
  <a class="doc-card" href="./language"><strong>Language syntax</strong><span>Declaration and statement forms with current implementation boundaries.</span></a>
  <a class="doc-card" href="./types"><strong>Type system</strong><span>Representations, identity, assignability, copy/alias behavior, and nullability.</span></a>
  <a class="doc-card" href="./operators"><strong>Operators</strong><span>Precedence, associativity, valid operands, evaluation, and failure behavior.</span></a>
  <a class="doc-card" href="./built-ins"><strong>Built-ins</strong><span>Collections, allocation, array conversion, channels, Result, and Task forms.</span></a>
  <a class="doc-card" href="./standard-library"><strong>Standard modules</strong><span>The compiler-managed <code>kinmokusei/http</code> API and runtime contract.</span></a>
  <a class="doc-card" href="./c-ffi-manifest"><strong>C FFI manifest</strong><span>Schema 1 calls, types, ownership, callbacks, handles, and thread policy.</span></a>
  <a class="doc-card" href="./go-interop"><strong>Go interop matrix</strong><span>Supported declarations, type shapes, operations, targets, CGO, and unsafe policy.</span></a>
  <a class="doc-card" href="./project-files"><strong>Project files</strong><span><code>kinmokusei.toml</code>, <code>kinmokusei.lock</code>, targets, dependencies, and unsafe policy.</span></a>
  <a class="doc-card" href="./diagnostics"><strong>Diagnostics</strong><span>JSON schema, positions, streams, categories, and process status.</span></a>
  <a class="doc-card" href="./compatibility"><strong>Compatibility</strong><span>Toolchains, platforms, Go package connectivity, C boundaries, and generated Go.</span></a>
  <a class="doc-card" href="./status"><strong>Implementation status</strong><span>Implemented, planned, experimental, and unsupported areas.</span></a>
</div>

## Status vocabulary

- <span class="status-label status-implemented">Implemented</span> is accepted by the compiler and covered by automated tests.
- **Experimental** is implemented but carries an explicitly provisional pre-1.0 contract.
- <span class="status-label status-planned">Planned</span> is design direction and is not accepted syntax or behavior.
- **Unsupported** is deliberately rejected in the current compiler.

When a guide and a design note disagree, implementation and automated tests are the behavioral source of truth. The public docs describe implemented behavior only.
