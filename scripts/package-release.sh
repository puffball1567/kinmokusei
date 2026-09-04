#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "usage: scripts/package-release.sh v<major>.<minor>.<patch>" >&2
  exit 2
fi

release_tag=$1
release_version=${release_tag#v}
repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
distribution_dir="${repository_root}/dist"
active_staging_root=

cleanup() {
  if [[ -n "${active_staging_root}" && -d "${active_staging_root}" ]]; then
    rm -rf -- "${active_staging_root}"
  fi
}
trap cleanup EXIT

rm -rf -- "${distribution_dir}"
mkdir -p "${distribution_dir}"

targets=(
  "linux amd64 tar.gz"
  "linux arm64 tar.gz"
  "darwin amd64 tar.gz"
  "darwin arm64 tar.gz"
  "windows amd64 zip"
)

for target in "${targets[@]}"; do
  read -r target_os target_arch archive_format <<<"${target}"
  archive_base="keika_${release_version}_${target_os}_${target_arch}"
  staging_root=$(mktemp -d)
  active_staging_root=${staging_root}
  staging_dir="${staging_root}/${archive_base}"
  mkdir -p "${staging_dir}"

  executable_name=keika
  if [[ "${target_os}" == windows ]]; then
    executable_name=keika.exe
  fi

  CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" \
    go build -trimpath -buildvcs=false \
      -ldflags="-s -w -X github.com/puffball1567/kinmokusei/internal/product.Version=${release_tag}" \
      -o "${staging_dir}/${executable_name}" ./cmd/keika

  cp README.md LICENSE "${staging_dir}/"

  find "${staging_dir}" -exec touch -t 202001010000 {} +
  if [[ "${archive_format}" == zip ]]; then
    (
      cd "${staging_root}"
      zip -X -q -r "${distribution_dir}/${archive_base}.zip" "${archive_base}"
    )
  else
    tar --sort=name --mtime='UTC 2020-01-01' --owner=0 --group=0 --numeric-owner \
      -C "${staging_root}" -czf "${distribution_dir}/${archive_base}.tar.gz" "${archive_base}"
  fi
  rm -rf -- "${staging_root}"
  active_staging_root=
done
