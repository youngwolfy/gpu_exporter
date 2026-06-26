#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-0.4.0}"
PLATFORM="${PLATFORM:-linux/amd64}"
IMAGE_NAME="${IMAGE_NAME:-gpu-exporter}"
OUTPUT_DIR="${OUTPUT_DIR:-dist}"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKERFILE="${ROOT_DIR}/examples/docker/Dockerfile"

cd "${ROOT_DIR}"
mkdir -p "${OUTPUT_DIR}"

for cuda_major in 12 13; do
  tag="${IMAGE_NAME}:${VERSION}-cuda${cuda_major}"
  archive="${OUTPUT_DIR}/${IMAGE_NAME}-image-${VERSION}-cuda${cuda_major}.tar.gz"
  tmp_archive="${archive}.tmp"

  docker build \
    --platform "${PLATFORM}" \
    --provenance=false \
    --sbom=false \
    --build-arg "DCGM_CUDA_MAJOR=${cuda_major}" \
    -t "${tag}" \
    -f "${DOCKERFILE}" \
    .

  docker save "${tag}" | gzip -c > "${tmp_archive}"
  mv "${tmp_archive}" "${archive}"

  echo "Wrote ${archive}"
done
