# Contributing to Kinmokusei

Kinmokusei accepts focused changes that preserve readable generated Go and
explicit compatibility with the Go ecosystem. Before contributing, read the
[language design](docs/language-design.md),
[compiler architecture](docs/compiler-architecture.md), and
[quality and Go compatibility policy](docs/quality-and-go-compatibility.md).

## Branch and release workflow

- Create feature, fix, documentation, and CI branches from `devel`.
- Open ordinary pull requests against `devel`; development changes do not land
  directly on `main`.
- Keep `devel` green under the required compatibility check.
- Prepare a release on `release/vX.Y.Z`, then merge that branch into `devel`.
- Open the release pull request from `devel` to `main` after all release checks
  pass. Merge this pull request with a merge commit so `devel` remains an
  ancestor of `main`; do not squash or rebase it.
- Create a version tag from `main` only after the release pull request merges.
- Use `hotfix/*` only for an urgent correction based on `main`, then return the
  same correction to `devel`.

Repository Rulesets require pull requests, review resolution, and the stable
compatibility gate for both protected branches. Pull requests to `main` are
accepted only from this repository's `devel` or `hotfix/*` branches.

## Quality requirements

- Add focused positive, negative, boundary, and alternate-form tests for new
  syntax or compiler behavior.
- Every implemented Go-equivalent runtime contract must compare against an
  independently handwritten Go program. Generated Go is never its own oracle.
- Preserve deterministic, formatted, portable generated Go without an
  unnecessary runtime dependency.
- Keep repository-wide and per-package coverage above the enforced floors.
- Update public documentation when syntax, CLI behavior, compatibility, or
  generated APIs change.

## Contribution license

Unless explicitly stated otherwise, contributions intentionally submitted for
inclusion in Kinmokusei are provided under the
[Apache License 2.0](LICENSE), including its contributor patent grant. Submit
only work that you have the right to license under these terms.
