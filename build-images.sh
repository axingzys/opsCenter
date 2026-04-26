#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: ./build-images.sh [repository-prefix] [tag] [options]

Examples:
  ./build-images.sh
  ./build-images.sh registry.example.com/opshub v1.0.0 --push
  ./build-images.sh registry.example.com/opshub v1.0.0 --push --save

Options:
  --push    Push images after build
  --save    Save images to dist/opshub-images-<tag>.tar
  -h, --help
EOF
}

REPOSITORY_PREFIX=""
TAG="latest"
PUSH_IMAGES=0
SAVE_IMAGES=0
POSITIONAL=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --push)
      PUSH_IMAGES=1
      shift
      ;;
    --save)
      SAVE_IMAGES=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      POSITIONAL+=("$1")
      shift
      ;;
  esac
done

if [[ ${#POSITIONAL[@]} -ge 1 ]]; then
  REPOSITORY_PREFIX="${POSITIONAL[0]}"
fi
if [[ ${#POSITIONAL[@]} -ge 2 ]]; then
  TAG="${POSITIONAL[1]}"
fi
if [[ ${#POSITIONAL[@]} -gt 2 ]]; then
  usage
  exit 1
fi

if [[ -n "${REPOSITORY_PREFIX}" ]]; then
  REPOSITORY_PREFIX="${REPOSITORY_PREFIX%/}"
  BACKEND_IMAGE="${REPOSITORY_PREFIX}/opshub-api:${TAG}"
  FRONTEND_IMAGE="${REPOSITORY_PREFIX}/opshub-web:${TAG}"
else
  BACKEND_IMAGE="opshub-api:${TAG}"
  FRONTEND_IMAGE="opshub-web:${TAG}"
fi

DIST_DIR="dist"
ARCHIVE_PATH="${DIST_DIR}/opshub-images-${TAG}.tar"

echo "================================================"
echo "OpsHub image build"
echo "================================================"
echo "backend image:  ${BACKEND_IMAGE}"
echo "frontend image: ${FRONTEND_IMAGE}"
echo "push images:    ${PUSH_IMAGES}"
echo "save archive:   ${SAVE_IMAGES}"
echo "================================================"

docker build -t "${BACKEND_IMAGE}" -f Dockerfile .
docker build -t "${FRONTEND_IMAGE}" -f Dockerfile.frontend .

if [[ "${PUSH_IMAGES}" -eq 1 ]]; then
  docker push "${BACKEND_IMAGE}"
  docker push "${FRONTEND_IMAGE}"
fi

if [[ "${SAVE_IMAGES}" -eq 1 ]]; then
  mkdir -p "${DIST_DIR}"
  docker save -o "${ARCHIVE_PATH}" "${BACKEND_IMAGE}" "${FRONTEND_IMAGE}"
  echo "saved image archive: ${ARCHIVE_PATH}"
fi

echo "================================================"
echo "backend repository:  ${BACKEND_IMAGE%:*}"
echo "frontend repository: ${FRONTEND_IMAGE%:*}"
echo "tag:                 ${TAG}"
echo "================================================"
