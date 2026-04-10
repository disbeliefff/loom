#!/usr/bin/env bash
set -euo pipefail

REPO="disbeliefff/loom"
BIN_NAME="loom"
INSTALL_DIR="/usr/local/bin"

setup_colors() {
    if [[ -t 1 ]] && command -v tput >/dev/null 2>&1; then
        RED=$(tput setaf 1)
        GREEN=$(tput setaf 2)
        YELLOW=$(tput setaf 3)
        BLUE=$(tput setaf 4)
        BOLD=$(tput bold)
        RESET=$(tput sgr0)
    else
        RED=""
        GREEN=""
        YELLOW=""
        BLUE=""
        BOLD=""
        RESET=""
    fi
}

log_info() { echo -e "${BLUE}${BOLD}==>${RESET} ${BOLD}$1${RESET}"; }
log_success() { echo -e "${GREEN}${BOLD}==> Success: $1${RESET}"; }
log_warn() { echo -e "${YELLOW}${BOLD}==>  Warning: $1${RESET}"; }
log_error() { echo -e "${RED}${BOLD}==> Error: $1${RESET}" >&2; exit 1; }


check_dependencies() {
    local deps=("curl" "tar" "grep" "sed")
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" >/dev/null 2>&1; then
            log_error "Required dependency '$dep' is not installed. Please install it and try again."
        fi
    done
}

detect_system() {
    local os
    os="$(uname -s)"
    case "$os" in
        Linux) OS_NAME="Linux" ;;
        Darwin) OS_NAME="Darwin" ;;
        *) log_error "Unsupported OS: $os" ;;
    esac

    local arch
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64) ARCH_NAME="x86_64" ;;
        aarch64|arm64) ARCH_NAME="arm64" ;;
        *) log_error "Unsupported architecture: $arch" ;;
    esac
}

get_latest_version() {
    log_info "Fetching latest release version for $REPO..."
    VERSION=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [[ -z "$VERSION" ]]; then
        log_error "Failed to fetch the latest version. GitHub API rate limit might be exceeded."
    fi
    log_info "Found latest version: ${GREEN}${VERSION}${RESET}"
}

download_and_install() {
    local download_url="https://github.com/$REPO/releases/download/${VERSION}/${BIN_NAME}_${OS_NAME}_${ARCH_NAME}.tar.gz"
    log_info "Downloading from $download_url..."

    local tmp_dir
    tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' EXIT

    # Check if the URL returns a 200 OK before piping to tar
    local http_code
    http_code=$(curl -sL -w "%{http_code}" -o /dev/null "$download_url")
    if [[ "$http_code" != "200" ]]; then
        log_error "Release asset not found at $download_url (HTTP $http_code). Note: Pre-compiled binaries might not be available for version $VERSION yet."
    fi

    if ! curl -sL "$download_url" | tar -xz -C "$tmp_dir" "$BIN_NAME"; then
        log_error "Failed to download or extract the binary. Please check if the release asset exists."
    fi

    log_info "Installing to $INSTALL_DIR..."

    local install_cmd="mv \"$tmp_dir/$BIN_NAME\" \"$INSTALL_DIR/$BIN_NAME\""

    if [[ ! -w "$INSTALL_DIR" ]]; then
        log_info "Elevated permissions required to write to $INSTALL_DIR. Prompting for sudo..."
        install_cmd="sudo $install_cmd"
    fi

    if ! eval "$install_cmd"; then
        log_error "Failed to move the binary to $INSTALL_DIR"
    fi

    local chmod_cmd="chmod +x \"$INSTALL_DIR/$BIN_NAME\""
    if [[ ! -w "$INSTALL_DIR/$BIN_NAME" ]]; then
        chmod_cmd="sudo $chmod_cmd"
    fi
    eval "$chmod_cmd"
}

main() {
    setup_colors
    log_info "Starting installation of $BIN_NAME..."

    check_dependencies
    detect_system
    get_latest_version
    download_and_install

    log_success "Successfully installed $BIN_NAME to $INSTALL_DIR/$BIN_NAME"
    log_success "Run '${BOLD}$BIN_NAME --help${RESET}' to get started!"
}

main "$@"
