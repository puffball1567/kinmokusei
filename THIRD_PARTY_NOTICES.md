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

When distributing an application built with Kinmokusei, review the Go runtime,
standard-library and application dependency notices for that application and
toolchain. The notices for `keika` are not a complete inventory for arbitrary
user applications. `keika deps licenses` metadata is not a substitute for
shipping required license and notice text.

Upstream Go license: <https://go.dev/LICENSE>.
