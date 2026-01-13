#!/usr/bin/env bash
#
# Octopus Startup Script for Linux/macOS
# Builds (if needed) and starts the Octopus server on port 5005
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[0;33m'
NC='\033[0m'
BOLD='\033[1m'

# Configuration
PORT=5005
GO_VERSION="1.21.13"
GO_MIN_VERSION="1.21"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="$SCRIPT_DIR/octopus"
SERVER_DIR="$SCRIPT_DIR/server"

echo -e "${CYAN}"
cat << 'EOF'
    ____        __
   / __ \______/ /_____  ____  __  _______
  / / / / ___/ __/ __ \/ __ \/ / / / ___/
 / /_/ / /__/ /_/ /_/ / /_/ / /_/ (__  )
 \____/\___/\__/\____/ .___/\__,_/____/
                    /_/
EOF
echo -e "${NC}"

# Check if running as root for Go installation
need_sudo() {
    if [ "$EUID" -ne 0 ]; then
        echo "sudo"
    fi
}

# Compare version numbers (returns 0 if $1 >= $2)
version_ge() {
    [ "$(printf '%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]
}

# Get current Go version
get_go_version() {
    if command -v go &> /dev/null; then
        go version | grep -oP 'go\K[0-9]+\.[0-9]+(\.[0-9]+)?' | head -1
    else
        echo "0"
    fi
}

# Install or update Go
install_go() {
    local current_version=$(get_go_version)

    if [ "$current_version" != "0" ] && version_ge "$current_version" "$GO_MIN_VERSION"; then
        echo -e "${GREEN}[OK]${NC} Go $current_version is installed (>= $GO_MIN_VERSION required)"
        return 0
    fi

    if [ "$current_version" = "0" ]; then
        echo -e "${YELLOW}[INFO]${NC} Go is not installed. Installing Go $GO_VERSION..."
    else
        echo -e "${YELLOW}[INFO]${NC} Go $current_version is too old. Installing Go $GO_VERSION..."
    fi

    # Detect architecture
    local arch=$(uname -m)
    case $arch in
        x86_64)  arch="amd64" ;;
        aarch64) arch="arm64" ;;
        armv*)   arch="armv6l" ;;
        *)       echo -e "${RED}[ERROR]${NC} Unsupported architecture: $arch"; exit 1 ;;
    esac

    # Detect OS
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')

    local go_tarball="go${GO_VERSION}.${os}-${arch}.tar.gz"
    local go_url="https://go.dev/dl/${go_tarball}"

    echo -e "${CYAN}[INFO]${NC} Downloading Go $GO_VERSION for $os/$arch..."

    # Download to temp directory
    local tmp_dir=$(mktemp -d)
    cd "$tmp_dir"

    if command -v wget &> /dev/null; then
        wget -q --show-progress "$go_url" -O "$go_tarball"
    elif command -v curl &> /dev/null; then
        curl -L --progress-bar "$go_url" -o "$go_tarball"
    else
        echo -e "${RED}[ERROR]${NC} Neither wget nor curl found. Please install one of them."
        exit 1
    fi

    echo -e "${CYAN}[INFO]${NC} Installing Go to /usr/local/go..."

    local SUDO=$(need_sudo)
    $SUDO rm -rf /usr/local/go
    $SUDO tar -C /usr/local -xzf "$go_tarball"

    # Cleanup
    cd "$SCRIPT_DIR"
    rm -rf "$tmp_dir"

    # Update PATH for current session
    export PATH=$PATH:/usr/local/go/bin

    # Verify installation
    if command -v /usr/local/go/bin/go &> /dev/null; then
        echo -e "${GREEN}[SUCCESS]${NC} Go $GO_VERSION installed successfully"

        # Check if /usr/local/go/bin is in PATH
        if [[ ":$PATH:" != *":/usr/local/go/bin:"* ]]; then
            echo -e "${YELLOW}[NOTE]${NC} Add the following to your ~/.bashrc or ~/.profile:"
            echo -e "       export PATH=\$PATH:/usr/local/go/bin"
        fi
    else
        echo -e "${RED}[ERROR]${NC} Go installation failed"
        exit 1
    fi
}

# Build the binary if it doesn't exist or if source is newer
build_if_needed() {
    local need_build=false

    if [ ! -f "$BINARY" ]; then
        echo -e "${CYAN}[INFO]${NC} Binary not found. Building..."
        need_build=true
    else
        # Check if any Go source files are newer than the binary
        if find "$SERVER_DIR" -name "*.go" -newer "$BINARY" 2>/dev/null | grep -q .; then
            echo -e "${CYAN}[INFO]${NC} Source files changed. Rebuilding..."
            need_build=true
        fi
    fi

    if [ "$need_build" = true ]; then
        cd "$SERVER_DIR"
        echo -e "${CYAN}[INFO]${NC} Downloading dependencies..."
        go mod tidy
        echo -e "${CYAN}[INFO]${NC} Building Octopus..."
        CGO_ENABLED=1 go build -o "$BINARY" ./cmd/main.go
        cd "$SCRIPT_DIR"
        echo -e "${GREEN}[SUCCESS]${NC} Build complete."
    fi
}

# Create data directory if it doesn't exist
setup_data_dir() {
    mkdir -p "$SCRIPT_DIR/data"
}

# Start the server
start_server() {
    echo -e "${GREEN}[INFO]${NC} Starting Octopus on port ${BOLD}$PORT${NC}..."
    echo -e "${CYAN}[INFO]${NC} Web UI: ${BOLD}http://localhost:$PORT${NC}"
    echo -e "${CYAN}[INFO]${NC} Default credentials: admin / admin"
    echo ""

    export PORT=$PORT
    export DATABASE_PATH="${DATABASE_PATH:-$SCRIPT_DIR/data/octopus.db}"

    exec "$BINARY"
}

# Main
install_go
build_if_needed
setup_data_dir
start_server
