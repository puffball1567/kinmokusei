---
title: Learn Kinmokusei
description: A progressive path from installing Kinmokusei to understanding its language and Go boundary.
---

# Learn Kinmokusei

This path builds a working mental model in a deliberate order. You will begin with one `.km` file, then move through types, failures, packages, and generated Go.

<div class="doc-cards">
  <a class="doc-card" href="./installation"><strong>1 · Installation</strong><span>Install <code>keika</code>, verify the Go toolchain, and connect an editor.</span></a>
  <a class="doc-card" href="./quick-start"><strong>2 · Five-minute quick start</strong><span>Check, run, build, and emit Go from your first source file.</span></a>
  <a class="doc-card" href="./language-tour"><strong>3 · Language tour</strong><span>Follow one small program through declarations, data, errors, and Go imports.</span></a>
  <a class="doc-card" href="../book/"><strong>4 · Language Manual</strong><span>Build a coherent model from source text through expressions, control flow, modules, and types.</span></a>
  <a class="doc-card" href="./from-typescript"><strong>Coming from TypeScript</strong><span>Separate familiar surface syntax from different runtime contracts.</span></a>
  <a class="doc-card" href="./from-go"><strong>Coming from Go</strong><span>Map Go values, errors, packages, and concurrency to Kinmokusei.</span></a>
  <a class="doc-card" href="./faq"><strong>Frequently asked questions</strong><span>Get concise answers about types, memory, packages, generated Go, and releases.</span></a>
</div>

## What Kinmokusei is

Kinmokusei is a statically typed, TypeScript-inspired source language for web backends and Go libraries. The compiler checks `.km` source and emits deterministic, formatted Go that uses the ordinary Go module graph, toolchain, ABI, runtime, and package ecosystem.

It is not a TypeScript transpiler, JavaScript runtime, npm environment, or alternate spelling of Go source. Where predictable Go generation needs a distinct rule—integer widths, value copying, pointers, errors, tasks, package imports—the language makes that rule explicit.

## How to use these docs

- **Learn** introduces concepts in sequence.
- **Manual** connects language concepts in a deliberate reading order.
- **Guide** explains how to write applications and reason about behavior.
- **Reference** is the precise, searchable contract.
- **Examples** provides complete programs and project layouts.

All examples that carry behavior are stored as `.km` source files and checked by the repository documentation test.
