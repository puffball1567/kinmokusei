---
layout: home

hero:
  name: OnsenTamago
  text: TypeScript-inspired syntax. Readable Go output.
  tagline: Build Go backends and libraries with classes, null safety, explicit errors, structured tasks, direct Go packages, and checked C FFI.
  actions:
    - theme: brand
      text: Get started
      link: /guide/getting-started
    - theme: alt
      text: See the language
      link: /guide/language-basics
    - theme: alt
      text: GitHub
      link: https://github.com/puffball1567/onsentamago

features:
  - title: Ordinary Go at the boundary
    details: Generated source is formatted, readable, standalone Go that can be reviewed, built, and published as a normal Go module.
  - title: Productive language features
    details: Classes, value structs, generics, Result, typed exceptions, null safety, tasks, inheritance, enums, and TypeScript-inspired syntax are checked before Go generation.
  - title: The Go ecosystem stays available
    details: Import standard-library and external Go modules directly, preserving named types, pointers, interfaces, methods, multiple results, generics, and channels.
  - title: Compatibility is executable
    details: Every registered Go-equivalent runtime contract is compared with an independently handwritten Go implementation.
---

## One source file, an ordinary executable

<<< ./snippets/hello.otm{ts}

```sh
ontama check hello.otm
ontama run hello.otm
```

```text
Hello from OnsenTamago
```

<div class="otm-contract">
OnsenTamago is its own language. It borrows familiar TypeScript shapes, but it does not claim TypeScript source compatibility. Its compilation target and interoperability model are Go.
</div>
