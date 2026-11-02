#!/bin/sh
set -eu

# Keep the release/image version compatible with 1Panel Core's installer.
version=${1:-}
case "$version" in
    ''|*[!A-Za-z0-9.-]*) echo "VERSION must be a fixed semantic version such as v2.0.0-rc1 or v2.0.0" >&2; exit 2 ;;
esac
if [ "${#version}" -gt 80 ] || ! printf '%s\n' "$version" | LC_ALL=C grep -Eq '^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$'; then
    echo "VERSION must match Core's semantic version format and be at most 80 characters" >&2
    exit 2
fi
