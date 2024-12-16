#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail


script_dir="$(cd "$(dirname "${0}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

: "${BUILD_DIR:="$(pwd)/build"}"

pkg_content_root() {
    local _os_reg="${1}"
    local _arch_reg="${2}"

    echo "${BUILD_DIR}/${_os_reg}-${_arch_reg}"
}

build_binary() {
    local _go_name="${1}"
    local _bin_name="${2}"
    local _os_go="${3}"
    local _arch_go="${4}"
    local _os_reg="${5}"
    local _arch_reg="${6}"
    local _ext="${7:-}"

    local _pkg_root="$(pkg_content_root "${_os_reg}" "${_arch_reg}")"

    printf "## os='%s', arch='%s':\n" "${_os_go}" "${_arch_go}"
    env GOOS="${_os_go}" GOARCH="${_arch_go}" \
        go build \
        -C "$(dirname "./${_go_name}")" \
        -o "${_pkg_root}/${_bin_name}${_ext}" \
        "./$(basename "${_go_name}")"
}

build_binaries() {
    local _go_name="${1}"
    local _bin_name="${2}"

    # OS names mapping:
    #  darwin -> macosx
    #
    # Architecture names mapping:
    #  amd64 -> x64
    #  arm64 -> aarch64

    printf "\n# Building package. go='%s' bin='%s'...\n" "${_go_name}" "${_bin_name}"

    build_binary "${_go_name}" "${_bin_name}" "windows" "amd64" "windows" "x64" ".exe"

    build_binary "${_go_name}" "${_bin_name}" "linux" "amd64" "linux" "x64"
    build_binary "${_go_name}" "${_bin_name}" "linux" "arm64" "linux" "aarch64"

    build_binary "${_go_name}" "${_bin_name}" "darwin" "amd64" "macosx" "x64"
    build_binary "${_go_name}" "${_bin_name}" "darwin" "arm64" "macosx" "aarch64"
}

go_build_target="${1}"
result_binary_name="${2}"

build_binaries "${go_build_target}" "${result_binary_name}"

# build_binaries "runenv-java-stub" "dump-args.go" "bin/java"

# build_binaries "sleep" "sleep.go" "sleep"
# build_binaries "read-with-sleep" "read-file-to-stdout-with-sleep.go" "read-with-sleep"

echo ""
echo "All binaries are saved to '${BUILD_DIR}'"
echo ""
