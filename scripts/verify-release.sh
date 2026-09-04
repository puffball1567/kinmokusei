#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "usage: scripts/verify-release.sh v<major>.<minor>.<patch>" >&2
  exit 2
fi

release_tag=$1
release_version=${release_tag#v}
distribution_dir=dist
expected_assets=(
  "keika_${release_version}_linux_amd64.tar.gz"
  "keika_${release_version}_linux_arm64.tar.gz"
  "keika_${release_version}_darwin_amd64.tar.gz"
  "keika_${release_version}_darwin_arm64.tar.gz"
  "keika_${release_version}_windows_amd64.zip"
  "kinmokusei-${release_version}.vsix"
  "SHA256SUMS"
)

for asset in "${expected_assets[@]}"; do
  if [[ ! -f "${distribution_dir}/${asset}" ]]; then
    echo "missing release asset: ${distribution_dir}/${asset}" >&2
    exit 1
  fi
done

actual_count=$(find "${distribution_dir}" -maxdepth 1 -type f | wc -l)
if [[ "${actual_count}" -ne "${#expected_assets[@]}" ]]; then
  echo "unexpected release assets in ${distribution_dir}" >&2
  find "${distribution_dir}" -maxdepth 1 -type f -printf '%f\n' | LC_ALL=C sort >&2
  exit 1
fi

(
  cd "${distribution_dir}"
  sha256sum -c SHA256SUMS
)

verification_root=$(mktemp -d)
trap 'rm -rf -- "${verification_root}"' EXIT
tar -xzf "${distribution_dir}/keika_${release_version}_linux_amd64.tar.gz" -C "${verification_root}"
version_output=$("${verification_root}/keika_${release_version}_linux_amd64/keika" version)
if [[ "${version_output}" != "keika ${release_tag}" ]]; then
  echo "release binary reported ${version_output@Q}, expected 'keika ${release_tag}'" >&2
  exit 1
fi
build_info=$(go version -m "${verification_root}/keika_${release_version}_linux_amd64/keika")
if [[ "${build_info}" != *": go1.27."* ]]; then
  echo "release compiler was not built with Go 1.27" >&2
  exit 1
fi

for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  archive="${distribution_dir}/keika_${release_version}_${target}.tar.gz"
  tar -tzf "${archive}" | grep -x "keika_${release_version}_${target}/keika" >/dev/null
  tar -tzf "${archive}" | grep -x "keika_${release_version}_${target}/README.md" >/dev/null
  tar -tzf "${archive}" | grep -x "keika_${release_version}_${target}/LICENSE" >/dev/null
done
unzip -Z1 "${distribution_dir}/keika_${release_version}_windows_amd64.zip" \
  | grep -x "keika_${release_version}_windows_amd64/keika.exe" >/dev/null
unzip -Z1 "${distribution_dir}/keika_${release_version}_windows_amd64.zip" \
  | grep -x "keika_${release_version}_windows_amd64/README.md" >/dev/null
unzip -Z1 "${distribution_dir}/keika_${release_version}_windows_amd64.zip" \
  | grep -x "keika_${release_version}_windows_amd64/LICENSE" >/dev/null
unzip -Z1 "${distribution_dir}/kinmokusei-${release_version}.vsix" \
  | grep -x "extension/LICENSE.txt" >/dev/null
