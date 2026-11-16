#!/usr/bin/env bash
set -euo pipefail

# Build container images for all SuIM services, or selected ones.
cd "$(dirname "$0")/.."

REGISTRY="${REGISTRY:-suim}"
TAG="${TAG:-dev}"
ALL=(user group third relation conversation msg msggateway msgtransfer push rtc apigateway crontask)

if [[ $# -gt 0 ]]; then
  targets=("$@")
else
  targets=("${ALL[@]}")
fi

for svc in "${targets[@]}"; do
  found=false
  for known in "${ALL[@]}"; do
    [[ "$known" == "$svc" ]] && found=true && break
  done
  if [[ "$found" != true ]]; then
    echo "unknown service: $svc" >&2
    exit 2
  fi
  echo "building ${REGISTRY}/${svc}:${TAG}"
  docker build --build-arg "SERVICE=${svc}" -t "${REGISTRY}/${svc}:${TAG}" .
done
