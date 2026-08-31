#!/usr/bin/env bash
set -euo pipefail

distribution_dir=${1:-dist}
if [[ ! -d "${distribution_dir}" ]]; then
  echo "release directory does not exist: ${distribution_dir}" >&2
  exit 2
fi

(
  cd "${distribution_dir}"
  find . -maxdepth 1 -type f ! -name SHA256SUMS -printf '%f\n' \
    | LC_ALL=C sort \
    | xargs sha256sum > SHA256SUMS
)
