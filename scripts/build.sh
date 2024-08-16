#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

: "${BUILD_DIR:="build"}"

script_dir="$(cd "$(dirname "${0}")" && pwd)"
cd "${script_dir}/.."

build_binary() {
    local _bin_name="${1}"
    local _os_go="${2}"
    local _arch_go="${3}"
    local _os_reg="${4}"
    local _arch_reg="${5}"
    local _ext="${6:-}"

    local _group="${_os_go}-${_arch_go}"

    printf "## os='%s', arch='%s':\n" "${_os_go}" "${_arch_go}"

    env GOOS="${_os_go}" GOARCH="${_arch_go}" \
        go build \
        -o "${BUILD_DIR}/${_group}/${_bin_name}/main${_ext}" \
        "./${_bin_name}.go"
    printf "\n"

    pl-pkg build package \
        --package-name="common/${_bin_name}" \
        --os="${_os_reg}" \
        --arch="${_arch_reg}" \
        --content-root="${BUILD_DIR}/${_group}/${_bin_name}/"
    printf "\n"
}

build_binaries() {
    local _bin_name="${1}"

    # OS names mapping:
    #  darwin -> macosx
    #
    # Architecture names mapping:
    #  amd64 -> x64
    #  arm64 -> aarch64

    printf "\n# Building '%s'...\n\n" "${_bin_name}"

    build_binary "${_bin_name}" "windows" "amd64" "windows" "x64" ".exe"

    build_binary "${_bin_name}" "linux" "amd64" "linux" "x64"
    build_binary "${_bin_name}" "linux" "arm64" "linux" "aarch64"

    build_binary "${_bin_name}" "darwin" "amd64" "macosx" "x64"
    build_binary "${_bin_name}" "darwin" "arm64" "macosx" "aarch64"

    pl-pkg build descriptor \
        --name="${_bin_name}" \
        --package-name="common/${_bin_name}"
}

rm -rf "${script_dir}/${BUILD_DIR}"

build_binaries "guided-command"
build_binaries "sleep"
build_binaries "read-file-to-stdout-with-sleep"

echo ""
echo "All binaries are saved to '${script_dir}/${BUILD_DIR}'"
echo "All packages are saved to '${script_dir}/pkg-*.tgz' archives"
echo ""
