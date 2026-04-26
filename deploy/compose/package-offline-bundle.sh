#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "${SCRIPT_DIR}/../.." && pwd)
ENV_FILE="${SCRIPT_DIR}/.env"
ENV_EXAMPLE_FILE="${SCRIPT_DIR}/.env.example"

WITH_DESKTOP=1
WITH_MONITORING=1
OUTPUT_FILE=""
BACKEND_IMAGE_OVERRIDE=""
FRONTEND_IMAGE_OVERRIDE=""

log() {
  printf '[opshub-bundle] %s\n' "$*"
}

die() {
  printf '[opshub-bundle] ERROR: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage: ./deploy/compose/package-offline-bundle.sh [options]

Options:
  --output FILE           Bundle archive path, default: dist/opshub-offline-bundle-<tag>.tar.gz
  --backend-image IMAGE   Backend image to package, default comes from deploy/compose/.env
  --frontend-image IMAGE  Frontend image to package, default comes from deploy/compose/.env
  --without-desktop       Exclude guacd and guacamole images
  --without-monitoring    Exclude prometheus image
  -h, --help              Show this help message

Examples:
  ./deploy/compose/package-offline-bundle.sh
  ./deploy/compose/package-offline-bundle.sh --without-monitoring
  ./deploy/compose/package-offline-bundle.sh --output ./dist/opshub-prod-bundle.tar.gz
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)
      [[ $# -ge 2 ]] || die "--output requires a value"
      OUTPUT_FILE="$2"
      shift 2
      ;;
    --backend-image)
      [[ $# -ge 2 ]] || die "--backend-image requires a value"
      BACKEND_IMAGE_OVERRIDE="$2"
      shift 2
      ;;
    --frontend-image)
      [[ $# -ge 2 ]] || die "--frontend-image requires a value"
      FRONTEND_IMAGE_OVERRIDE="$2"
      shift 2
      ;;
    --without-desktop)
      WITH_DESKTOP=0
      shift
      ;;
    --without-monitoring)
      WITH_MONITORING=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

command -v docker >/dev/null 2>&1 || die "docker is not installed"

if [[ ! -f "${ENV_FILE}" ]]; then
  cp "${ENV_EXAMPLE_FILE}" "${ENV_FILE}"
  log "created ${ENV_FILE} from .env.example"
fi

set -a
# shellcheck source=/dev/null
source "${ENV_FILE}"
set +a

BACKEND_IMAGE="${BACKEND_IMAGE_OVERRIDE:-${BACKEND_REPOSITORY:-opshub-api}:${IMAGE_TAG:-latest}}"
FRONTEND_IMAGE="${FRONTEND_IMAGE_OVERRIDE:-${FRONTEND_REPOSITORY:-opshub-web}:${IMAGE_TAG:-latest}}"

if [[ -z "${OUTPUT_FILE}" ]]; then
  OUTPUT_FILE="${ROOT_DIR}/dist/opshub-offline-bundle-${IMAGE_TAG:-latest}.tar.gz"
fi

mkdir -p "$(dirname "${OUTPUT_FILE}")"

images=(
  "${BACKEND_IMAGE}"
  "${FRONTEND_IMAGE}"
  "${MYSQL_IMAGE:-mysql:8.0.44}"
  "${REDIS_IMAGE:-redis:7-alpine}"
)

if [[ "${WITH_DESKTOP}" -eq 1 ]]; then
  images+=(
    "${GUACD_IMAGE:-guacamole/guacd:1.6.0}"
    "${GUACAMOLE_IMAGE:-guacamole/guacamole:1.6.0}"
  )
fi

if [[ "${WITH_MONITORING}" -eq 1 ]]; then
  images+=("${PROMETHEUS_IMAGE:-prom/prometheus:v2.54.1}")
fi

ensure_local_image() {
  local image="$1"
  if docker image inspect "${image}" >/dev/null 2>&1; then
    return 0
  fi

  log "pulling missing image ${image}"
  docker pull "${image}" >/dev/null
}

for image in "${images[@]}"; do
  ensure_local_image "${image}"
done

deploy_args=()
if [[ "${WITH_DESKTOP}" -eq 1 ]]; then
  deploy_args+=("--with-desktop")
fi
if [[ "${WITH_MONITORING}" -eq 1 ]]; then
  deploy_args+=("--with-monitoring")
fi

bundle_name="opshub-offline-bundle-${IMAGE_TAG:-latest}"
bundle_dir=$(mktemp -d)
trap 'rm -rf "${bundle_dir:-}"' EXIT
package_root="${bundle_dir}/${bundle_name}"
image_archive="${package_root}/dist/opshub-images-${IMAGE_TAG:-latest}.tar"

mkdir -p "${package_root}/deploy/compose"
mkdir -p "${package_root}/dist"

cp "${SCRIPT_DIR}/docker-compose.yml" "${package_root}/deploy/compose/"
cp "${SCRIPT_DIR}/.env.example" "${package_root}/deploy/compose/"
cp "${SCRIPT_DIR}/README.md" "${package_root}/deploy/compose/"
cp "${SCRIPT_DIR}/deploy.sh" "${package_root}/deploy/compose/"
cp "${SCRIPT_DIR}/export-baseline.sh" "${package_root}/deploy/compose/"
cp "${SCRIPT_DIR}/install-offline.sh" "${package_root}/deploy/compose/"
cp "${SCRIPT_DIR}/package-offline-bundle.sh" "${package_root}/deploy/compose/"
cp "${SCRIPT_DIR}/nginx.conf" "${package_root}/deploy/compose/"
cp -R "${SCRIPT_DIR}/sql" "${package_root}/deploy/compose/"
cp -R "${SCRIPT_DIR}/prometheus" "${package_root}/deploy/compose/"

log "saving image archive to ${image_archive}"
docker save -o "${image_archive}" "${images[@]}"

case "${OUTPUT_FILE}" in
  *.tar.gz|*.tgz)
    tar -C "${bundle_dir}" -czf "${OUTPUT_FILE}" "${bundle_name}"
    ;;
  *.tar)
    tar -C "${bundle_dir}" -cf "${OUTPUT_FILE}" "${bundle_name}"
    ;;
  *)
    die "unsupported bundle format: ${OUTPUT_FILE} (use .tar, .tar.gz or .tgz)"
    ;;
esac

log "bundle created"
log "images: ${images[*]}"
log "archive: ${OUTPUT_FILE}"
if [[ ${#deploy_args[@]} -gt 0 ]]; then
  log "install with optional services enabled: ./deploy/compose/install-offline.sh --images ./dist/opshub-images-${IMAGE_TAG:-latest}.tar ${deploy_args[*]} --host <HOST>"
fi
