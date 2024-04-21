#!/usr/bin/env bash

set -o errexit
set -o nounset

script_dir="$(cd "$(dirname "${0}")" && pwd)"
cd "${script_dir}"

build_binaries() {
    local _os="${1}"
    local _arch="${2}"
    local _bin_suffix="${3}"

    printf "Building for %s, %s:\n"  "${_os}" "${_arch}"

    ls *.go |
        while read -r file; do
            printf "\t'%s'... "  "${file}"
            env GOOS="${_os}" GOARCH="${_arch}" \
                go build \
                    -o "build/${file%.go}-${_bin_suffix}" \
                    "./${file}"
            printf "\n"
        done
}

# OS names mapping:
#  darwin -> macosx
#
# Architecture names mapping:
#  amd64 -> x64
#  arm64 -> aarch64

if ! [ -e "build" ]; then
    mkdir build
fi

build_binaries "windows" "amd64" "windows-x64"

build_binaries "linux" "amd64" "linux-x64"
build_binaries "linux" "arm64" "linux-aarch64"

build_binaries "darwin" "amd64" "macosx-x64"
build_binaries "darwin" "arm64" "macosx-aarch64"

echo ""
echo "All binaries are saved to '${script_dir}/build'"
echo ""
