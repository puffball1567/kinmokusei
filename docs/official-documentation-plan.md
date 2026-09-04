# Official documentation and GitHub Pages plan

## Purpose

Build an official Kinmokusei documentation site comparable in usefulness to
the documentation of established programming languages. A new visitor must be
able to understand the language, install the toolchain, write a program, find
precise rules, use Go packages, and diagnose failures without reading compiler
source or internal design notes.

Documentation describes implemented behavior only. It must not invent syntax
or present roadmap direction as current behavior.

## Branch and stack

- Develop the site on a dedicated documentation branch.
- Preserve and extend the VitePress application under `website/`.
- Keep changes limited to documentation, examples/tests, site assets, and the
  GitHub Pages workflow.
- Keep the site functional under the repository Pages base path.

## Readers and layers

The site serves evaluators, new users, application developers, and language or
tooling contributors through separate layers:

- **Learn** introduces concepts in a deliberate order.
- **Guide** solves language and application-development tasks.
- **Reference** gives precise searchable rules.
- **Examples** provides complete copyable source trees.
- **Project** explains quality, compatibility, and contribution policy.

One concept should have one canonical explanation with cross-links rather than
several drifting copies.

## Target content

### Home and onboarding

- Define Kinmokusei as a TypeScript-inspired source language for the Go
  ecosystem, not a TypeScript or Go-source compatibility layer.
- Show verified `.km` input, command, output, and readable Go.
- Provide clear paths to install, quick start, language tour, examples,
  reference, releases, and GitHub.
- Add migration-oriented introductions for TypeScript and Go developers.

### Language and application guides

Cover declarations, types, collections, functions, generics, enums, aliases,
defined types, structs, classes, inheritance, control flow, `Result`, typed
exceptions, nullability, concurrency, modules, projects, Go interoperability,
HTTP, C ABI/FFI, editor diagnostics, and generated Go.

Where correctness depends on it, pages state value versus reference behavior,
copy versus alias behavior, evaluation count/order, failure behavior, and the
generated-Go boundary.

### Reference

Provide CLI commands/options, language syntax, type/assignability rules,
operator precedence, project/lock schema, diagnostics, platform/toolchain and
interop compatibility, implementation status, and pre-1.0 expectations.

Do not create a second handwritten compiler registry when structured
compiler-owned data can prevent drift.

## Accuracy and examples

- Treat compiler behavior and automated tests as the implementation source of
  truth; use `docs/` for settled terminology and design intent.
- Use implemented, experimental, planned, and unsupported consistently.
- Keep valid examples as `.km` sources and check them with the current
  compiler.
- Compare behavior-bearing snippets with committed exact output.
- Keep invalid examples separate and assert one stable diagnostic fragment.
- Use an independently handwritten Go oracle for Go-equivalent runtime
  behavior; generated Go is never its own expected-value implementation.
- Format and build emitted Go whenever a page promises publishable output.

## Experience and accessibility

- Provide local search, a desktop/mobile sidebar, stable URLs, visible preview
  status, edit links, and a useful 404 page.
- Add canonical, Open Graph, description, sitemap, favicon, and social-preview
  metadata.
- Keep code blocks from forcing page-wide horizontal overflow.
- Use semantic headings, keyboard-visible focus, sufficient contrast, alt
  text, and non-color status labels.
- Respect reduced motion and light/dark preferences.
- Keep assets modest and content readable without application state.
- Keep English canonical initially while leaving room for a complete future
  Japanese translation rather than a partial parallel tree.

## Verification and deployment

Documentation CI must verify clean dependency installation, production site
build, internal Markdown links/anchors, valid and runnable snippets, expected
invalid diagnostics, and absence of committed generated site/dependency output.

Pull requests build and test the site without deploying. Production Pages
deployment occurs only from the protected publication branch after the build
job succeeds.

## Delivery phases

1. Inventory content, define navigation, preserve public URLs, and create a
   tracked content matrix.
2. Complete home, installation, quick start, language tour, and migration
   pages.
3. Publish focused language guides and executable examples.
4. Publish Go, project, HTTP, generated-Go, editor, and C boundary guides.
5. Complete reference content and automated checks.
6. Finish metadata, responsive presentation, accessibility, 404, and release
   readiness.

## v0.2 delivery status

The documentation implementation is complete on `docs/official-site` and is
based on `release/v0.2.0`. Publication remains a repository integration step:
review, commit, merge, and later tag/deploy through the protected workflows.

All six delivery phases are represented in the content matrix. The release
baseline currently includes:

- 82 public pages with unique titles and descriptions;
- a ten-chapter Language Manual plus Learn, Guide, Reference, Examples, and
  Project sections;
- 34 compiler-checked valid programs and 17 checked invalid programs;
- exact output, command-argument/failure, JSON diagnostic, generated-package,
  generated-Go, and C FFI checks;
- internal-link, anchor, discoverability, metadata, machine-path, and
  cross-layer duplication checks;
- production rendering, sitemap generation, responsive navigation, local
  search, reduced-motion behavior, and accessible focus/contrast treatment.

The following tracked items are intentionally outside the v0.2 documentation
baseline rather than incomplete handwritten pages:

- a formal grammar generated from compiler-owned parser data;
- a diagnostic-code registry generated after stable codes exist in the
  diagnostic package;
- historical site snapshots and breaking-version migration pages once more
  than one documentation-bearing release exists.

Before publication, run the same gates used by pull-request and tag workflows:

```sh
./scripts/check-docs.sh
npm --prefix website run docs:check-links
npm --prefix website run docs:check-content
npm --prefix website run docs:build
git diff --check
```

## Definition of done

The effort is complete when a new user can reach a working program from the
home page; TypeScript and Go developers can identify semantic differences;
implemented feature families have canonical guide/reference homes; examples
are checked; future work is clearly separated; search, navigation, responsive
layout, metadata, accessibility basics, links, Pages base-path behavior, and
deployment checks pass; and no private paths, credentials, or internal
operational details appear in public content.

See [documentation-content-matrix.md](documentation-content-matrix.md) for the
tracked implementation inventory.
