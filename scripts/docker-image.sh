#!/bin/sh
set -eu

if [ "$#" -ne 3 ]; then
    echo "usage: docker-image.sh build|save SOURCE_DIRECTORY OUTPUT_DIRECTORY" >&2
    exit 2
fi
operation=$1
source_dir=$2
output_dir=$3
version=${VERSION:-}
image=${DOCKER_IMAGE:-1panel/mcp-server}
platform=${DOCKER_PLATFORM:-linux/amd64}
script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)

case "$operation" in build|save) ;; *) echo "unsupported Docker operation: $operation" >&2; exit 2 ;; esac
sh "$script_dir/validate-version.sh" "$version"
case "$platform" in
    linux/386) arch=386 ;;
    linux/amd64) arch=amd64 ;;
    linux/arm64|linux/arm64/v8) platform=linux/arm64; arch=arm64 ;;
    linux/arm|linux/arm/v7) platform=linux/arm/v7; arch=armv7 ;;
    linux/s390x) arch=s390x ;;
    linux/ppc64le) arch=ppc64le ;;
    linux/riscv64) arch=riscv64 ;;
    *) echo "DOCKER_PLATFORM must select one supported Linux platform" >&2; exit 2 ;;
esac
case "$image" in
    ''|-*|*@*) echo "DOCKER_IMAGE must be an image repository, without a tag or digest" >&2; exit 2 ;;
esac
repository_name=${image##*/}
case "$repository_name" in
    *:*) echo "DOCKER_IMAGE must not include a tag" >&2; exit 2 ;;
esac
if printf '%s' "$image" | LC_ALL=C grep -q '[[:space:]]'; then
    echo "DOCKER_IMAGE must be an image repository without whitespace" >&2
    exit 2
fi
command -v docker >/dev/null 2>&1 || { echo "Docker CLI is required" >&2; exit 2; }
image_ref="$image:$version"

if [ "$operation" = build ]; then
    source_dir=$(CDPATH= cd -- "$source_dir" && pwd)
    docker buildx build --load --platform "$platform" --build-arg "VERSION=$version" --tag "$image_ref" "$source_dir"
    printf 'Built %s for %s\n' "$image_ref" "$platform"
    exit 0
fi

command -v python3 >/dev/null 2>&1 || { echo "Python 3 is required to verify Docker archives" >&2; exit 2; }
# Resolve the tag once. Verify the saved config digest against this immutable ID
# before replacing an earlier archive, so concurrent retagging cannot change it.
image_id=$(docker image inspect --format '{{json .}}' "$image_ref" |
    python3 "$script_dir/verify-docker-archive.py" inspect "$platform" "$version")
mkdir -p "$output_dir"
output_dir=$(CDPATH= cd -- "$output_dir" && pwd)
output_file="$output_dir/mcp-1panel-linux-$arch-$version.tar"
if [ -d "$output_file" ] || [ -L "$output_file" ]; then
    echo "Docker archive output must not be a directory or symbolic link: $output_file" >&2
    exit 2
fi
staged=$(mktemp "$output_dir/.docker-save-$arch.XXXXXX")
trap 'rm -f "$staged"' EXIT HUP INT TERM
docker save --output "$staged" "$image_ref"
python3 "$script_dir/verify-docker-archive.py" archive "$platform" "$version" "$image_id" "$image_ref" "$staged"
mv -f "$staged" "$output_file"
printf 'Saved %s\n' "$output_file"
