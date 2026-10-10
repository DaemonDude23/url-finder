#!/usr/bin/env bash
# Build/scan once, then publish the saved images without rebuilding them.
set -euo pipefail
stage="${1:?Expected build-scan, push, or promote}"
release_tag="${TAG:?Set TAG}"
repository="${GITHUB_REPOSITORY:?Set GITHUB_REPOSITORY}"
image_dir="${IMAGE_DIR:-${RUNNER_TEMP:?Set RUNNER_TEMP or IMAGE_DIR}/release-images}"
[[ "$release_tag" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]
[[ "$repository" =~ ^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$ ]]
# Preserve the original public package path.
registry="ghcr.io/${repository,,}/url-finder"
mkdir -p "$image_dir"
metadata() { printf '%s\n%s\n' "$release_tag" "$registry"; }
require_marker() {
  if ! cmp -s <(metadata) "$image_dir/$1"; then
    echo "Missing or mismatched $1 marker for $release_tag; run the preceding stage successfully" >&2
    return 1
  fi
}
architectures=(amd64 arm64 386)
case "$stage" in
  build-scan)
    scanner="${TRIVY_IMAGE:?Set TRIVY_IMAGE}"
    rm -f "$image_dir/scanned" "$image_dir/pushed"
    mkdir -p "$image_dir/cache"
    : > "$image_dir/scanned.sha256"
    for arch in "${architectures[@]}"; do
      image="$registry:$release_tag-$arch"
      archive="$image_dir/url-finder-$arch.tar"
      docker buildx build --platform "linux/$arch" -f Dockerfile \
        --build-arg "VERSION=${release_tag#v}" -t "$image" \
        --output "type=docker,dest=$archive" .
      docker run --rm --user "$(id -u):$(id -g)" \
        -v "$archive:/image.tar:ro" -v "$image_dir/cache:/cache" \
        -e TRIVY_CACHE_DIR=/cache "$scanner" image \
        --input /image.tar --scanners vuln --severity HIGH,CRITICAL \
        --ignore-unfixed --exit-code 1
      (cd "$image_dir"; sha256sum "url-finder-$arch.tar" >> scanned.sha256)
    done
    metadata > "$image_dir/scanned"
    ;;
  push)
    require_marker scanned
    rm -f "$image_dir/pushed"
    (cd "$image_dir"; sha256sum --check scanned.sha256)
    for arch in "${architectures[@]}"; do test -f "$image_dir/url-finder-$arch.tar"; done
    digests=()
    for arch in "${architectures[@]}"; do
      image="$registry:$release_tag-$arch"
      docker load --input "$image_dir/url-finder-$arch.tar"
      docker push "$image"
      digest=$(docker image inspect --format '{{index .RepoDigests 0}}' "$image")
      [[ "$digest" == "$registry@sha256:"* ]]
      [[ "${digest##*@}" =~ ^sha256:[a-f0-9]{64}$ ]]
      digests+=("$digest")
    done
    printf '%s\n' "${digests[@]}" > "$image_dir/url-finder.digests"
    docker buildx imagetools create -t "$registry:$release_tag" "${digests[@]}"
    metadata > "$image_dir/pushed"
    ;;
  promote)
    require_marker pushed
    mapfile -t digests < "$image_dir/url-finder.digests"
    test "${#digests[@]}" -eq "${#architectures[@]}"
    for digest in "${digests[@]}"; do
      [[ "$digest" == "$registry@sha256:"* ]]
      [[ "${digest##*@}" =~ ^sha256:[a-f0-9]{64}$ ]]
    done
    docker buildx imagetools create -t "$registry:latest" "${digests[@]}"
    ;;
  *) echo 'Expected build-scan, push, or promote' >&2; exit 1 ;;
esac
