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
- After the compatibility push run succeeds on the exact resulting `devel`
  commit, open a pull request from `devel` to `main`. This release pull request
  verifies and reuses that successful run instead of repeating the full suite.
- Merge release pull requests with a merge commit. After the merge, run the
  **Publish release** workflow from `main` with the intended version tag. It
  tags the exact `main` head and starts artifact publication without changing
  either protected branch.
- Use `hotfix/*` only for an urgent correction based on `main`. A hotfix pull
  request to `main` always runs the full compatibility suite. After publishing
  the patch release, open a synchronization pull request from `main` to
  `devel`; that pull request also runs the full suite.
- Pull requests to `main` are accepted only from this repository's `devel` or
  `hotfix/*` branches. Direct pushes to `main` are not permitted.

Repository Rulesets require pull requests and the stable compatibility gate on
both protected branches. The `main` Ruleset also requires the approved-source
gate, rejects deletion and non-fast-forward updates, and permits merge commits
only. Ordinary release pull requests reuse CI only when the exact `devel` head
has a successful Compatibility push run and the pull request merge tree is
identical to that verified head.

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
