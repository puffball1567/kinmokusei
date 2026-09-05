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
- After the compatibility push run succeeds on the resulting `devel` commit,
  run the **Promote release** workflow from `devel` with the intended version
  tag. It verifies the successful gate, fast-forwards `main`, creates the tag,
  and starts artifact publication and documentation deployment.
- Do not update `main` or create release tags manually during an ordinary
  release. The promotion workflow keeps the branch and tag on the same verified
  commit without requiring a duplicate `devel` to `main` pull request.
- Use `hotfix/*` only for an urgent correction based on `main`. Merge the fix
  into `devel` through the ordinary review gate, then publish it with the same
  promotion workflow as a patch release.

Repository Rulesets require pull requests, review resolution, and the stable
compatibility gate on `devel`. The `main` Ruleset rejects deletion,
non-fast-forward updates, and commits without a successful compatibility gate.
The promotion workflow additionally requires the exact current `devel` head,
so ordinary releases never need a second pull request to `main`.

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
