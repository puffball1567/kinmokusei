# Official documentation content matrix

This matrix tracks the public site against the official-documentation plan.
Compiler behavior and automated tests are the implementation source of truth;
the design documents listed below supply settled terminology and rationale.

Status meanings:

- **Complete**: canonical public page exists, is linked in navigation, and is
  covered by the site/link build.
- **Foundation**: canonical overview exists; deeper per-feature reference or a
  larger executable example remains a future documentation increment.
- **Planned**: not yet published as if implemented.

Planned rows below describe future compiler- or release-owned capabilities.
They are not missing pages required by the v0.2 documentation baseline.

## Learn

| Journey | Public page | Primary source | Status |
| --- | --- | --- | --- |
| Product definition | `website/index.md`, `website/learn/index.md` | `README.md`, `docs/index.md` | Complete |
| Installation | `website/learn/installation.md` | `docs/installation.md`, release workflow | Complete |
| Check/run/build/emit quick start | `website/learn/quick-start.md` | CLI implementation/tests, `hello.km` | Complete |
| Continuous language tour | `website/learn/language-tour.md` | language design/tests, `language-tour.km` | Complete |
| Coming from TypeScript | `website/learn/from-typescript.md` | language design | Complete |
| Coming from Go | `website/learn/from-go.md` | language design, interop policy | Complete |
| Common product and runtime questions | `website/learn/faq.md` | language/compiler/release contracts | Complete |

## Language guide

| Topic family | Canonical page | Primary source | Status |
| --- | --- | --- | --- |
| Files, bindings, scope, control flow | `website/guide/language-basics.md` | lexer/parser/sema tests, language design | Complete |
| Scalars, collections, objects, enums, named types | `website/guide/types-and-data.md` | type/sema/codegen tests | Complete |
| Functions, arrows, variadics, generics | `website/guide/functions-and-generics.md` | parser/sema generic matrices | Complete |
| Classes, structs, methods, interfaces, inheritance | `website/guide/classes-and-structs.md` | OOP docs and compiler matrices | Complete |
| Result, Go error, exceptions, nullability | `website/guide/errors-and-nullability.md` | result/exception/nullable tests | Complete |
| Goroutines, channels, select, Task | `website/guide/concurrency.md` | task/select/compiler differential tests | Complete |
| Relative modules, projects, targets | `website/guide/projects-and-cli.md` | project implementation/tests | Complete |

## Language Manual

| Chapter | Public page | Primary source | Status |
| --- | --- | --- | --- |
| Manual reading path | `website/book/index.md` | official documentation plan | Complete |
| Executable Manual contract | `website/snippets/book-contract.km` | combined semantic/compiler behavior | Complete |
| Source text and lexical structure | `website/book/source-and-lexical.md` | lexer/token implementation and tests | Complete |
| Bindings and lexical scope | `website/book/bindings-and-scope.md` | parser/sema/module-scope tests | Complete |
| Expressions and evaluation | `website/book/expressions.md` | parser/operator/codegen differential tests | Complete |
| Control flow | `website/book/control-flow.md` | flow/select/switch/label/exception tests | Complete |
| Modules and imports | `website/book/modules-and-imports.md` | module loader/project tests | Complete |
| Types and values | `website/book/types-and-values.md` | type/sema/codegen matrices | Complete |
| Functions and generics | `website/book/functions-and-generics.md` | callable/variadic/generic tests | Complete |
| Structs, classes, and interfaces | `website/book/structs-classes-interfaces.md` | OOP/inheritance/interface tests | Complete |
| Failures, results, and exceptions | `website/book/errors-results-exceptions.md` | Result/exception/nullable tests | Complete |
| Concurrency and tasks | `website/book/concurrency-and-tasks.md` | goroutine/channel/select/task tests | Complete |

## Application guide and examples

| Topic | Canonical page | Primary source | Status |
| --- | --- | --- | --- |
| Direct Go packages and unsafe policy | `website/guide/go-interop.md` | packages/interop doc and audit tests | Complete |
| HTTP server/client boundaries | `website/guide/http.md`, `website/examples/http-router.md` | HTTP kernel and dogfood tests | Complete |
| Outgoing C ABI and incoming FFI | `website/guide/c-ffi.md`, `website/reference/c-ffi-manifest.md` | C FFI design and compiler tests | Complete |
| LSP and VS Code | `website/guide/editor.md` | LSP/editor implementation tests | Complete |
| Operational troubleshooting | `website/guide/troubleshooting.md` | CLI/project/target diagnostic tests | Complete |
| Inspecting/publishing generated Go | `website/guide/generated-go.md` | compiler architecture/publication tests | Complete |
| Application and generated-package testing | `website/guide/testing.md` | generated public API and external Go tests | Complete |
| Numeric and bitwise operator recipe | `website/examples/numeric-operators.md` | bitwise/sema/codegen differential tests | Complete |
| Comparison and short-circuit recipe | `website/examples/comparisons-and-short-circuit.md` | parser/sema/codegen evaluation contracts | Complete |
| Native command-line application | `website/examples/command-line-app.md` | compiler build and argument/output contract | Complete |
| Example index | `website/examples/index.md` | repository `examples/` | Complete |
| React plus Gin/Fiber application | `website/examples/web-backend.md` | full-stack example and verifier | Complete |
| Result parsing and validation | `website/examples/result-parsing.md` | result/Go error tests | Complete |
| Collections and conversion | `website/examples/collections.md` | collection differential tests | Complete |
| UTF-8 byte and code-point behavior | `website/examples/unicode-strings.md` | string/range differential tests | Complete |
| Generics | `website/examples/generics.md` | native generic matrices | Complete |
| Variadics and slice spread | `website/examples/variadics.md` | variadic semantic/compiler tests | Complete |
| Defined domain types | `website/examples/defined-types.md` | defined-type semantic/compiler tests | Complete |
| Interface polymorphism | `website/examples/polymorphism.md` | interface differential tests | Complete |
| Go interface type switch | `website/examples/type-switch.md` | type-switch differential tests | Complete |
| Inheritance and class downcasts | `website/examples/inheritance.md` | inheritance differential tests | Complete |
| Typed exceptions and cleanup | `website/examples/exceptions.md` | exception differential tests | Complete |
| Stable nullable flow | `website/examples/nullable-flow.md` | nullable member/flow tests | Complete |
| Control-transfer boundaries | `website/examples/control-flow-boundaries.md` | defer/fallthrough/label tests | Complete |
| Value/pointer receivers | `website/examples/struct-receivers.md` | native struct differential tests | Complete |
| Structured tasks | `website/examples/tasks.md` | task lifecycle/differential tests | Complete |
| Channels | `website/examples/channels.md` | channel/select differential tests | Complete |
| Select | `website/examples/select.md` | select differential tests | Complete |
| Relative source modules | `website/examples/modules.md` | module scope/compiler tests | Complete |
| JSON structural value | `website/examples/json.md` | JSON dogfood tests | Complete |
| Filesystem Result and cleanup | `website/examples/filesystem-round-trip.md` | Go importer/result/defer contracts | Complete |
| Go standard-library value boundary | `website/examples/go-standard-library.md` | Go importer/method-set/multiple-result tests | Complete |
| Port-free HTTP router test | `website/examples/http-router.md` | HTTP router differential tests | Complete |
| Bounded HTTP client workflow | `website/examples/bounded-http-fetch.md` | embedded fetch kernel and HTTP dogfood tests | Complete |
| External Go module workflow | `website/examples/external-go-module.md` | project transaction tests | Complete |
| C ABI export | `website/examples/c-abi.md` | C ABI tests | Complete |
| Incoming C FFI generation | `website/examples/incoming-c-ffi.md` | C FFI generator command tests | Complete |
| Future application recipes | future pages under `website/examples/` | compiler fixtures and independent Go oracles | Planned |

## Reference

| Contract | Public page | Primary source | Status |
| --- | --- | --- | --- |
| Cross-document terminology | `website/reference/glossary.md` | Manual/reference canonical vocabulary | Complete |
| Command and option inventory | `website/reference/cli.md` | `cmd/keika/main.go` and tests | Complete |
| Declaration/statement syntax index | `website/reference/language.md` | lexer/parser/sema | Complete |
| Representations and assignability | `website/reference/types.md` | `go/types`, sema tests | Complete |
| Operator precedence/operands/failures | `website/reference/operators.md` | parser/sema/codegen matrices | Complete |
| Compiler built-ins | `website/reference/built-ins.md` | builtin semantic/differential tests | Complete |
| Compiler-managed standard modules | `website/reference/standard-library.md` | embedded source and dogfood tests | Complete |
| Incoming C FFI schema 1 | `website/reference/c-ffi-manifest.md` | checked C FFI generator | Complete |
| Go interop support matrix | `website/reference/go-interop.md` | importer/audit/differential tests | Complete |
| Manifest and lock schema | `website/reference/project-files.md` | project parser/lock tests | Complete |
| Diagnostic envelope and positions | `website/reference/diagnostics.md` | CLI/LSP implementation tests | Complete |
| Go/toolchain/C compatibility | `website/reference/compatibility.md` | CI matrices, interop/FFI tests | Complete |
| Implemented versus planned status | `website/reference/status.md` | status registry and roadmap | Complete |
| Generated formal grammar | future compiler-owned data | parser | Planned |
| Machine-generated diagnostic code registry | future compiler-owned data | diagnostic package | Planned |

## Project and maintenance

| Topic | Public page or check | Status |
| --- | --- | --- |
| Independent-Go and artifact quality policy | `website/project/quality.md` | Complete |
| Documentation contribution rules | `website/project/contributing.md`, `CONTRIBUTING.md` | Complete |
| Preview labels, release sources, and pre-1.0 expectations | `website/project/releases.md`, `CHANGELOG.md` | Complete |
| Local search and responsive sidebar | VitePress config/theme | Complete |
| Canonical/OG metadata, favicon, social card, sitemap | VitePress config/public assets | Complete |
| Useful 404 | `website/404.md` | Complete |
| Reduced motion, focus, contrast, mobile overflow | custom theme | Complete |
| Referenced valid/runnable/invalid snippet, artifact-pair, and diagnostic-JSON checks | `scripts/check-docs.sh` | Complete |
| Internal link, anchor, and page-discoverability check | `scripts/check-doc-links.mjs` | Complete |
| Metadata, repository prose/branding hygiene, and Manual/Guide/Reference separation check | `scripts/check-doc-content.mjs` | Complete |
| Pull-request build without deployment | `.github/workflows/docs.yml` | Complete |
| Versioned historical snapshots | future release work | Planned |

## Archived and redirected public paths

- `guide/getting-started` remains as a forwarding page to the new Learn path.
- Existing `guide/language-basics`, `guide/types-and-data`,
  `guide/classes-and-structs`, `guide/errors-and-nullability`,
  `guide/concurrency`, `guide/go-interop`, `guide/c-ffi`, `guide/editor`,
  `guide/projects-and-cli`, `examples/web-backend`, `reference/cli`, and
  `reference/status` URLs remain canonical and were expanded in place.

This prevents the v0.2 baseline URLs from becoming dead links while giving new
content stable human-readable paths.
