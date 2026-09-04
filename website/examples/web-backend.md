---
title: React with Gin and Fiber
description: Run one React frontend against interchangeable Gin and Fiber backends written in Kinmokusei.
---

# React with Gin and Fiber

The complete repository example pairs one React/TypeScript frontend with interchangeable Gin and Fiber backends written in Kinmokusei.

## What it demonstrates

- imports of real external Go framework modules;
- a shared Kinmokusei todo store;
- health, list, create, toggle, and delete HTTP operations;
- locked external dependencies and ordinary generated Go;
- the same HTTP contract exercised against independent handwritten Go servers;
- a Vite development proxy and frontend component tests.

## Project tree

```text
examples/react-web-frameworks/
├── README.md
├── verify.sh
├── backend/
│   ├── kinmokusei.toml
│   ├── kinmokusei.lock
│   ├── store.km
│   ├── gin/main.km
│   └── fiber/main.km
└── frontend/
    ├── package.json
    └── src/
        ├── App.tsx
        └── App.test.tsx
```

## Prerequisites

- a supported Go toolchain;
- Node.js for the React frontend;
- compiler source checkout for the repository verification script;
- an available local Go module cache for the already locked backend graph.

## Verify

From the example directory:

```sh
./verify.sh
```

The verification compiles/runs both Kinmokusei backends, tests their HTTP behavior, and exercises the frontend against the expected API contract. It does not use generated Go as its own expected-value implementation.

Browse the [complete example source](https://github.com/puffball1567/kinmokusei/tree/main/examples/react-web-frameworks) for the exact run commands and current endpoint matrix.
