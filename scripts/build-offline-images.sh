#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-0.5.0}"
PLATFORM="${PLATFORM:-linux/amd64}"
IMAGE_NAME="${IMAGE_NAME:-gpu-exporter}"
OUTPUT_DIR="${OUTPUT_DIR:-distr}"
SECURITY_SCAN_ENABLED="${SECURITY_SCAN_ENABLED:-1}"
TRIVY_BIN="${TRIVY_BIN:-trivy}"
BLOCKED_CVES="${BLOCKED_CVES:-CVE-2026-42496,CVE-2026-8376,CVE-2023-45853,CVE-2026-39824,CVE-2026-39822}"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKERFILE="${ROOT_DIR}/examples/docker/Dockerfile"

cd "${ROOT_DIR}"
mkdir -p "${OUTPUT_DIR}"

if [[ "${SECURITY_SCAN_ENABLED}" == "1" ]] && ! command -v "${TRIVY_BIN}" >/dev/null 2>&1; then
  echo "trivy is required for the release security gate" >&2
  exit 1
fi

archives=()
scan_dirs=()

cleanup() {
  local scan_dir
  for scan_dir in "${scan_dirs[@]}"; do
    [[ -d "${scan_dir}" ]] && rm -rf -- "${scan_dir}"
  done
}

trap cleanup EXIT

for cuda_major in 12 13; do
  tag="${IMAGE_NAME}:${VERSION}-cuda${cuda_major}"
  archive="${OUTPUT_DIR}/${IMAGE_NAME}-image-${VERSION}-cuda${cuda_major}.tar.gz"
  tmp_archive="${archive}.tmp"
  scan_dir="$(mktemp -d "${OUTPUT_DIR}/.scan-cuda${cuda_major}.XXXXXX")"
  scan_dirs+=("${scan_dir}")
  image_tar="${scan_dir}/image.tar"
  trivy_report="${scan_dir}/trivy.json"

  docker build \
    --pull \
    --platform "${PLATFORM}" \
    --provenance=false \
    --sbom=false \
    --build-arg "VERSION=${VERSION}" \
    --build-arg "DCGM_CUDA_MAJOR=${cuda_major}" \
    -t "${tag}" \
    -f "${DOCKERFILE}" \
    .

  docker save --output "${image_tar}" "${tag}"

  if [[ "${SECURITY_SCAN_ENABLED}" == "1" ]]; then
    "${TRIVY_BIN}" image \
      --input "${image_tar}" \
      --scanners vuln \
      --severity HIGH,CRITICAL \
      --exit-code 1 \
      --no-progress \
      --skip-version-check

    "${TRIVY_BIN}" image \
      --input "${image_tar}" \
      --scanners vuln \
      --format json \
      --output "${trivy_report}" \
      --no-progress \
      --skip-version-check

    IFS=',' read -r -a blocked_cves <<< "${BLOCKED_CVES}"
    for cve in "${blocked_cves[@]}"; do
      if grep -Fq "${cve}" "${trivy_report}"; then
        echo "release blocked: ${cve} was found in ${tag}" >&2
        exit 1
      fi
    done
  fi

  gzip -c "${image_tar}" > "${tmp_archive}"
  mv "${tmp_archive}" "${archive}"
  archives+=("$(basename "${archive}")")

  rm -rf -- "${scan_dir}"

  echo "Wrote ${archive}"
done

(
  cd "${OUTPUT_DIR}"
  sha256sum "${archives[@]}" > SHA256SUMS.tmp
  mv SHA256SUMS.tmp SHA256SUMS
)

echo "Wrote ${OUTPUT_DIR}/SHA256SUMS"
