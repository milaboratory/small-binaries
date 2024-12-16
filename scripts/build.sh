#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail


script_dir="$(cd "$(dirname "${0}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

: "${BUILD_DIR:="$(pwd)/build"}"

pkg_content_root() {
    local _pkg_name="${1}"
    local _os_reg="${2}"
    local _arch_reg="${3}"

    echo "${BUILD_DIR}/${_os_reg}-${_arch_reg}/${_pkg_name}"
}

build_binary() {
    local _pkg_name="${1}"
    local _go_name="${2}"
    local _bin_name="${3}"
    local _os_go="${4}"
    local _arch_go="${5}"
    local _os_reg="${6}"
    local _arch_reg="${7}"
    local _ext="${8:-}"

    local _pkg_root="$(pkg_content_root "${_pkg_name}" "${_os_reg}" "${_arch_reg}")"

    printf "## os='%s', arch='%s':\n" "${_os_go}" "${_arch_go}"
    env GOOS="${_os_go}" GOARCH="${_arch_go}" \
        go build \
        -C "$(dirname "./${_go_name}")" \
        -o "${_pkg_root}/${_bin_name}${_ext}" \
        "./$(basename "${_go_name}")"
}

build_binaries() {
    local _pkg_name="${1}"
    local _go_name="${2}"
    local _bin_name="${3}"

    # OS names mapping:
    #  darwin -> macosx
    #
    # Architecture names mapping:
    #  amd64 -> x64
    #  arm64 -> aarch64

    printf "\n# Building package '%s'. go='%s' bin='%s'...\n" "${_pkg_name}" "${_go_name}" "${_bin_name}"

    build_binary "${_pkg_name}" "${_go_name}" "${_bin_name}" "windows" "amd64" "windows" "x64" ".exe"

    build_binary "${_pkg_name}" "${_go_name}" "${_bin_name}" "linux" "amd64" "linux" "x64"
    build_binary "${_pkg_name}" "${_go_name}" "${_bin_name}" "linux" "arm64" "linux" "aarch64"

    build_binary "${_pkg_name}" "${_go_name}" "${_bin_name}" "darwin" "amd64" "macosx" "x64"
    build_binary "${_pkg_name}" "${_go_name}" "${_bin_name}" "darwin" "arm64" "macosx" "aarch64"
}

pl_pkg_name="${1}"
go_build_target="${2}"
result_binary_name="${3:-${1}}"

build_binaries "${pl_pkg_name}" "${go_build_target}" "${result_binary_name}"

# build_binaries "mnz-client" "mnz-client/cmd/mnz-client" "mnz-client"
# build_binaries "table-converter" "table-converter/table-converter" "table-converter"

# build_binaries "runenv-java-stub" "dump-args.go" "bin/java"

# build_binaries "runenv-python-stub" "runenv-python-stub.go" "bin/python"
# build_binaries "runenv-python-stub" "runenv-python-stub.go" "bin/pip"

# build_binaries "hello-world" "hello-world.go" "hello-world"
# build_binaries "guided-command" "guided-command.go" "guided-command"
# build_binaries "sleep" "sleep.go" "sleep"
# build_binaries "read-with-sleep" "read-file-to-stdout-with-sleep.go" "read-with-sleep"

pl-pkg build --all-platforms

echo ""
echo "All binaries are saved to '${BUILD_DIR}'"
echo "All packages are saved to '$(pwd)/pkg-*.tgz' archives"
echo ""
