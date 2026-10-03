# Third-party notices

Kinmokusei's own code is licensed under Apache-2.0; see `LICENSE`.

The `keika` executable includes Go runtime and standard-library code. The Go
compiler command is not bundled: building user programs uses an installed Go
toolchain. This distinction does not remove the binary's notice requirements.

CLI release archives include `licenses/go/INDEX.txt`, the original Go license
and patent grant, and applicable nested license/notice files from the exact
toolchain used to build that target. Source files containing additional notices
are retained verbatim as `.txt` supplements, including their complete copyright,
permission and disclaimer text. These supplements are attribution material, not
a bundled compiler. Collection is conservative at package level: some retained
code may have been removed by the linker.

The release verification regenerates these materials from the build toolchain
and compares their bytes with every archive. New non-standard Go dependencies
require a separate licensing review before packaging can proceed.

`keika build -o app` writes the executable and an adjacent
`app-licenses/` directory. Distribute that entire directory with the executable.
The published v0.4.5 compiler uses `app.licenses/`; v0.4.6 uses the
hyphenated name and leave existing legacy directories untouched.
It contains original Go runtime/standard-library licenses, patent grants,
additional notices and complete source supplements from the selected toolchain
and target, plus detected notices for imported Go modules, the locked external
Kinmokusei package graph and Kinmokusei-generated runtime helpers.

`INDEX.json` records toolchain/target/build-tag identities, dependency identities,
notice hashes and the executable hash, without local checkout paths. Dependencies
without detected LICENSE/LICENCE/COPYING text stop the build; manifest SPDX
metadata and `keika deps licenses` hashes are not substitutes for original text.
Normal builds remain offline and do not update the manifest or dependency lock.

Collection is conservative, not a legal-compliance certification. Review the
actual terms, any source-offer or modification requirements, and native/system
libraries linked outside the Go/source package graph. `emit-go`, `run` and direct
`go build` do not produce this application notice directory. The notices for
`keika` itself are not an inventory for arbitrary user applications.

Upstream Go license: <https://go.dev/LICENSE>.
