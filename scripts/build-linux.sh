#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
    echo "usage: build-linux.sh OUTPUT_DIRECTORY ARCH" >&2
    exit 2
fi

source_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
output_dir=$1
target_arch=$2
adapter_version=${VERSION:-}

case "$target_arch" in
    386|amd64|arm64|s390x|ppc64le|riscv64) go_arch=$target_arch; go_arm=7 ;;
    arm|armv7) target_arch=armv7; go_arch=arm; go_arm=7 ;;
    *) echo "unsupported MCP architecture: $target_arch" >&2; exit 2 ;;
esac
case "$adapter_version" in
    *[!A-Za-z0-9.+_-]*) echo "VERSION contains unsupported characters" >&2; exit 2 ;;
esac
if [ "${#adapter_version}" -gt 80 ]; then
    echo "VERSION must be at most 80 characters" >&2
    exit 2
fi
if [ ! -f "$source_dir/go.mod" ]; then
    echo "MCP source go.mod not found: $source_dir" >&2
    exit 2
fi

mkdir -p "$output_dir"
output_dir=$(CDPATH= cd -- "$output_dir" && pwd)
output_file="$output_dir/mcp-1panel-linux-$target_arch"
if [ -d "$output_file" ] || [ -L "$output_file" ]; then
    echo "MCP output path must not be a directory or symbolic link: $output_file" >&2
    exit 2
fi
staged=$(mktemp "$output_dir/.mcp-build-$target_arch.XXXXXX")
trap 'rm -f "$staged"' EXIT HUP INT TERM

link_flags='-s -w'
if [ -n "$adapter_version" ]; then
    link_flags="$link_flags -X github.com/1Panel-dev/mcp-1panel/utils.Version=$adapter_version"
fi

cd "$source_dir"
# Resolve the module-selected cached toolchain before disabling automatic
# selection. The go launcher may itself be older than this module's version.
go_launcher=${MCP_GO:-go}
if selected_go_root=$(GOPROXY=off GOSUMDB=off "$go_launcher" env GOROOT 2>/dev/null); then
    :
else
    # Older Go launchers can demand a checksum-database verification even when
    # the selected module toolchain is already extracted. Resolve that cached
    # executable directly; never enable the proxy or fetch another toolchain.
    selected_version=$(awk '$1 == "go" { print $2; exit }' go.mod)
    case "${GOTOOLCHAIN:-auto}" in
        auto) ;;
        go*+auto)
            configured_version=${GOTOOLCHAIN#go}; configured_version=${configured_version%+auto}
            selected_version=$(awk -v required="$selected_version" -v configured="$configured_version" 'BEGIN {
                split(required, a, "."); split(configured, b, ".")
                for (i = 1; i <= 3; i++) {
                    if (a[i] + 0 > b[i] + 0) { print required; exit }
                    if (a[i] + 0 < b[i] + 0) { print configured; exit }
                }
                print required
            }')
            ;;
        go*+path|local|path) echo "selected Go toolchain is unavailable offline" >&2; exit 2 ;;
        go*) selected_version=${GOTOOLCHAIN#go} ;;
        *) echo "selected Go toolchain is unavailable offline" >&2; exit 2 ;;
    esac
    case "$selected_version" in
        ''|*[!0-9.]*) echo "selected Go toolchain version is invalid" >&2; exit 2 ;;
    esac
    module_cache=$(GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off "$go_launcher" env GOMODCACHE)
    host_os=$(GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off "$go_launcher" env GOHOSTOS)
    host_arch=$(GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off "$go_launcher" env GOHOSTARCH)
    selected_go_root="$module_cache/golang.org/toolchain@v0.0.1-go$selected_version.$host_os-$host_arch"
fi
case "$selected_go_root" in
    /*) ;;
    *) echo "selected Go toolchain must have an absolute GOROOT" >&2; exit 2 ;;
esac
selected_go="$selected_go_root/bin/go"
if [ ! -x "$selected_go" ]; then
    echo "selected Go toolchain is not executable" >&2
    exit 2
fi
CGO_ENABLED=0 GOOS=linux GOARCH="$go_arch" GOARM="$go_arm" \
    GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local \
    "$selected_go" build -mod=readonly -buildvcs=false -trimpath \
    -ldflags "$link_flags" -o "$staged" .
chmod 0755 "$staged"
mv -f "$staged" "$output_file"
printf 'Built %s\n' "$output_file"
