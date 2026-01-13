#!/usr/bin/env bash
#
# Octopus Installation Script for Linux/macOS
# This script installs all dependencies and builds Octopus
#
# Usage:
#   ./scripts/install.sh [OPTIONS]
#
# Options:
#   --dev           Install development dependencies (air, golangci-lint)
#   --docker-only   Only install Docker dependencies
#   --skip-docker   Skip Docker installation
#   --help          Show this help message
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# Configuration
GO_VERSION="1.21.6"
MIN_GO_VERSION="1.21"
INSTALL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Flags
INSTALL_DEV=false
DOCKER_ONLY=false
SKIP_DOCKER=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --dev)
            INSTALL_DEV=true
            shift
            ;;
        --docker-only)
            DOCKER_ONLY=true
            shift
            ;;
        --skip-docker)
            SKIP_DOCKER=true
            shift
            ;;
        --help)
            head -20 "$0" | tail -15
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "\n${PURPLE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}${CYAN}  $1${NC}"
    echo -e "${PURPLE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

# Print banner
print_banner() {
    echo -e "${CYAN}"
    cat << 'EOF'
    ____        __
   / __ \______/ /_____  ____  __  _______
  / / / / ___/ __/ __ \/ __ \/ / / / ___/
 / /_/ / /__/ /_/ /_/ / /_/ / /_/ (__  )
 \____/\___/\__/\____/ .___/\__,_/____/
                    /_/

    VM Migration & Disaster Recovery Tool
EOF
    echo -e "${NC}"
    echo -e "${BOLD}Installation Script v1.0${NC}\n"
}

# Detect OS
detect_os() {
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if [ -f /etc/debian_version ]; then
            OS="debian"
            PKG_MANAGER="apt-get"
        elif [ -f /etc/redhat-release ]; then
            OS="redhat"
            PKG_MANAGER="yum"
        elif [ -f /etc/arch-release ]; then
            OS="arch"
            PKG_MANAGER="pacman"
        elif [ -f /etc/alpine-release ]; then
            OS="alpine"
            PKG_MANAGER="apk"
        else
            OS="linux"
            PKG_MANAGER="unknown"
        fi
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        OS="macos"
        PKG_MANAGER="brew"
    else
        log_error "Unsupported operating system: $OSTYPE"
        exit 1
    fi
    log_info "Detected OS: ${BOLD}$OS${NC} (Package manager: $PKG_MANAGER)"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Install system dependencies
install_system_deps() {
    log_step "Installing System Dependencies"

    case $OS in
        debian)
            log_info "Updating package lists..."
            sudo apt-get update -qq
            log_info "Installing build essentials..."
            sudo apt-get install -y -qq build-essential git curl wget sqlite3 libsqlite3-dev ca-certificates gnupg
            ;;
        redhat)
            log_info "Installing development tools..."
            sudo yum groupinstall -y "Development Tools"
            sudo yum install -y git curl wget sqlite sqlite-devel ca-certificates
            ;;
        arch)
            log_info "Installing base-devel..."
            sudo pacman -S --noconfirm --needed base-devel git curl wget sqlite
            ;;
        alpine)
            log_info "Installing build dependencies..."
            sudo apk add --no-cache build-base git curl wget sqlite sqlite-dev
            ;;
        macos)
            if ! command_exists brew; then
                log_info "Installing Homebrew..."
                /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
            fi
            log_info "Installing dependencies via Homebrew..."
            brew install git curl wget sqlite3
            ;;
        *)
            log_warning "Unknown package manager. Please install: git, curl, wget, sqlite3, build-essential manually."
            ;;
    esac

    log_success "System dependencies installed"
}

# Install Go
install_go() {
    log_step "Installing Go"

    if command_exists go; then
        CURRENT_GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        log_info "Go is already installed: v$CURRENT_GO_VERSION"

        # Check if version is sufficient
        if [[ "$(printf '%s\n' "$MIN_GO_VERSION" "$CURRENT_GO_VERSION" | sort -V | head -n1)" == "$MIN_GO_VERSION" ]]; then
            log_success "Go version is sufficient (>= $MIN_GO_VERSION)"
            return 0
        else
            log_warning "Go version is too old. Installing newer version..."
        fi
    fi

    # Determine architecture
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            GO_ARCH="amd64"
            ;;
        aarch64|arm64)
            GO_ARCH="arm64"
            ;;
        armv7l)
            GO_ARCH="armv6l"
            ;;
        *)
            log_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    # Determine OS for Go download
    GO_OS="linux"
    if [[ "$OS" == "macos" ]]; then
        GO_OS="darwin"
    fi

    GO_TAR="go${GO_VERSION}.${GO_OS}-${GO_ARCH}.tar.gz"
    GO_URL="https://go.dev/dl/${GO_TAR}"

    log_info "Downloading Go $GO_VERSION for $GO_OS/$GO_ARCH..."
    wget -q --show-progress -O "/tmp/$GO_TAR" "$GO_URL"

    log_info "Installing Go to /usr/local/go..."
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "/tmp/$GO_TAR"
    rm "/tmp/$GO_TAR"

    # Add to PATH
    if ! grep -q '/usr/local/go/bin' ~/.bashrc 2>/dev/null; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    fi
    if ! grep -q '/usr/local/go/bin' ~/.zshrc 2>/dev/null; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.zshrc 2>/dev/null || true
    fi

    export PATH=$PATH:/usr/local/go/bin

    log_success "Go $GO_VERSION installed successfully"
}

# Install Docker
install_docker() {
    log_step "Installing Docker"

    if command_exists docker; then
        DOCKER_VERSION=$(docker --version | awk '{print $3}' | tr -d ',')
        log_info "Docker is already installed: v$DOCKER_VERSION"

        # Start Docker if not running
        if ! docker info >/dev/null 2>&1; then
            log_info "Starting Docker daemon..."
            if [[ "$OS" == "macos" ]]; then
                open -a Docker
                log_info "Please wait for Docker Desktop to start..."
                sleep 10
            else
                sudo systemctl start docker 2>/dev/null || sudo service docker start 2>/dev/null || true
            fi
        fi

        log_success "Docker is ready"
        return 0
    fi

    case $OS in
        debian)
            log_info "Installing Docker via official script..."
            curl -fsSL https://get.docker.com | sudo sh
            sudo usermod -aG docker "$USER"
            sudo systemctl enable docker
            sudo systemctl start docker
            ;;
        redhat)
            log_info "Installing Docker..."
            sudo yum install -y yum-utils
            sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            sudo yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
            sudo systemctl enable docker
            sudo systemctl start docker
            sudo usermod -aG docker "$USER"
            ;;
        arch)
            log_info "Installing Docker..."
            sudo pacman -S --noconfirm docker docker-compose
            sudo systemctl enable docker
            sudo systemctl start docker
            sudo usermod -aG docker "$USER"
            ;;
        alpine)
            log_info "Installing Docker..."
            sudo apk add docker docker-compose
            sudo rc-update add docker boot
            sudo service docker start
            sudo addgroup "$USER" docker
            ;;
        macos)
            log_warning "Please install Docker Desktop manually from: https://www.docker.com/products/docker-desktop"
            log_info "Checking if Docker Desktop is installed..."
            if [ -d "/Applications/Docker.app" ]; then
                open -a Docker
                log_info "Docker Desktop found. Please wait for it to start..."
            else
                log_error "Docker Desktop not found. Please install it and run this script again."
                exit 1
            fi
            ;;
        *)
            log_warning "Please install Docker manually for your system."
            ;;
    esac

    log_success "Docker installed successfully"
    log_warning "You may need to log out and back in for Docker group permissions to take effect."
}

# Install Docker Compose
install_docker_compose() {
    log_step "Installing Docker Compose"

    if command_exists docker-compose || docker compose version >/dev/null 2>&1; then
        if docker compose version >/dev/null 2>&1; then
            COMPOSE_VERSION=$(docker compose version | awk '{print $4}')
        else
            COMPOSE_VERSION=$(docker-compose --version | awk '{print $4}' | tr -d ',')
        fi
        log_info "Docker Compose is already installed: v$COMPOSE_VERSION"
        log_success "Docker Compose is ready"
        return 0
    fi

    log_info "Installing Docker Compose..."

    # Most recent Docker installations include compose as a plugin
    # Install standalone as fallback
    COMPOSE_VERSION="2.24.0"
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            COMPOSE_ARCH="x86_64"
            ;;
        aarch64|arm64)
            COMPOSE_ARCH="aarch64"
            ;;
        *)
            log_error "Unsupported architecture for Docker Compose: $ARCH"
            return 1
            ;;
    esac

    sudo curl -L "https://github.com/docker/compose/releases/download/v${COMPOSE_VERSION}/docker-compose-$(uname -s)-${COMPOSE_ARCH}" -o /usr/local/bin/docker-compose
    sudo chmod +x /usr/local/bin/docker-compose

    log_success "Docker Compose installed successfully"
}

# Install development tools
install_dev_tools() {
    log_step "Installing Development Tools"

    # Install air for hot-reload
    log_info "Installing air (hot-reload)..."
    go install github.com/cosmtrek/air@latest

    # Install golangci-lint
    log_info "Installing golangci-lint..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v1.55.2

    # Add GOPATH/bin to PATH
    if ! grep -q 'go env GOPATH' ~/.bashrc 2>/dev/null; then
        echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
    fi

    export PATH=$PATH:$(go env GOPATH)/bin

    log_success "Development tools installed"
}

# Download Go dependencies
download_go_deps() {
    log_step "Downloading Go Dependencies"

    cd "$INSTALL_DIR/server"
    log_info "Running go mod download..."
    go mod download
    log_info "Running go mod tidy..."
    go mod tidy

    log_success "Go dependencies downloaded"
}

# Build the application
build_app() {
    log_step "Building Octopus"

    cd "$INSTALL_DIR"

    log_info "Building server binary..."
    cd server
    CGO_ENABLED=1 go build -o ../octopus ./cmd/main.go
    cd ..

    if [ -f "octopus" ]; then
        log_success "Build successful! Binary: ${BOLD}$INSTALL_DIR/octopus${NC}"
    else
        log_error "Build failed"
        exit 1
    fi
}

# Setup configuration
setup_config() {
    log_step "Setting Up Configuration"

    mkdir -p "$INSTALL_DIR/docker/config"

    if [ ! -f "$INSTALL_DIR/docker/config/config.yaml" ]; then
        log_info "Creating configuration file from template..."
        cp "$INSTALL_DIR/docker/config/config.yaml.example" "$INSTALL_DIR/docker/config/config.yaml"
        log_success "Configuration file created: docker/config/config.yaml"
        log_warning "Please edit this file with your settings before running Octopus"
    else
        log_info "Configuration file already exists"
    fi
}

# Build Docker images
build_docker() {
    log_step "Building Docker Images"

    cd "$INSTALL_DIR"

    log_info "Building Octopus Docker image..."
    docker build -t octopus:latest -f docker/Dockerfile.server .

    log_success "Docker image built: octopus:latest"
}

# Print completion message
print_completion() {
    echo -e "\n${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}${GREEN}  Installation Complete!${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

    echo -e "${BOLD}Quick Start:${NC}"
    echo -e "  ${CYAN}1.${NC} Edit configuration: ${BOLD}vim docker/config/config.yaml${NC}"
    echo -e "  ${CYAN}2.${NC} Run with Docker:    ${BOLD}make docker-run${NC}"
    echo -e "  ${CYAN}3.${NC} Or run locally:     ${BOLD}make run${NC}"
    echo -e "  ${CYAN}4.${NC} Access web UI:      ${BOLD}http://localhost:8080${NC}"
    echo ""
    echo -e "${BOLD}Default Credentials (development mode):${NC}"
    echo -e "  Username: ${CYAN}admin${NC}"
    echo -e "  Password: ${CYAN}admin${NC}"
    echo ""
    echo -e "${BOLD}Useful Commands:${NC}"
    echo -e "  ${CYAN}make help${NC}        - Show all available commands"
    echo -e "  ${CYAN}make docker-logs${NC} - View container logs"
    echo -e "  ${CYAN}make docker-stop${NC} - Stop the containers"
    echo ""

    if [ "$INSTALL_DEV" = true ]; then
        echo -e "${BOLD}Development Commands:${NC}"
        echo -e "  ${CYAN}make dev${NC}   - Run with hot-reload"
        echo -e "  ${CYAN}make test${NC}  - Run tests"
        echo -e "  ${CYAN}make lint${NC}  - Run linter"
        echo ""
    fi

    echo -e "${PURPLE}Documentation: ${BOLD}docs/README.md${NC}"
    echo -e "${PURPLE}Issues: ${BOLD}https://github.com/sp00nznet/octopus/issues${NC}"
    echo ""
}

# Main installation flow
main() {
    print_banner
    detect_os

    if [ "$DOCKER_ONLY" = true ]; then
        install_docker
        install_docker_compose
        build_docker
        print_completion
        exit 0
    fi

    install_system_deps
    install_go

    if [ "$SKIP_DOCKER" = false ]; then
        install_docker
        install_docker_compose
    fi

    if [ "$INSTALL_DEV" = true ]; then
        install_dev_tools
    fi

    download_go_deps
    build_app
    setup_config

    if [ "$SKIP_DOCKER" = false ]; then
        build_docker
    fi

    print_completion
}

# Run main
main "$@"
