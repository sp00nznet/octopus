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
NC='\033[0m'
BOLD='\033[1m'

# Configuration
PORT=5005
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

# Check if Go is installed
check_go() {
    if ! command -v go &> /dev/null; then
        echo -e "${RED}[ERROR]${NC} Go is not installed. Please run ./scripts/install.sh first."
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
        check_go
        echo -e "${CYAN}[INFO]${NC} Building Octopus..."
        cd "$SERVER_DIR"
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
build_if_needed
setup_data_dir
start_server
