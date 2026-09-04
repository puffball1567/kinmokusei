---
layout: home
title: Kinmokusei Documentation
description: Write TypeScript-inspired source and compile it to readable, standalone Go.

hero:
  name: Kinmokusei
  text: Clear source. Readable Go.
  tagline: A TypeScript-inspired source language for the Go ecosystem—designed for explicit behavior, direct package access, and generated code you can own.
  image:
    src: /hero-kinmokusei.svg
    alt: A quiet sunset reflected across calm water
  actions:
    - theme: brand
      text: Get started
      link: /learn/quick-start
    - theme: alt
      text: Read the Manual
      link: /book/
    - theme: alt
      text: View on GitHub
      link: https://github.com/puffball1567/kinmokusei

features:
  - icon: "↗"
    title: Go ecosystem access
    details: Import standard-library and external Go modules directly while preserving named types, pointers, interfaces, methods, generics, channels, and errors.
  - icon: "Aa"
    title: Productive static syntax
    details: Use classes, value structs, generics, Result, typed exceptions, null safety, and structured tasks—with source-level checks before generation.
  - icon: "{ }"
    title: Readable generated Go
    details: Emitted source is deterministic, gofmt-formatted, standalone, and suitable for inspection or publication as an ordinary Go module.
  - icon: "✓"
    title: Executable compatibility
    details: Every registered Go-equivalent runtime contract is compared against an independently handwritten Go implementation.
---

<HomeShowcase />

<div class="yn-contract">
<strong>One language, a clear boundary.</strong> Kinmokusei borrows familiar TypeScript shapes, but it is neither TypeScript-compatible nor Go-source-compatible. Its source files use <code>.km</code>; its compilation target and runtime model are Go.
</div>
