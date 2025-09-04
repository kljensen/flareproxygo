#!/bin/bash
# shellcheck disable=SC2317  # Trap handlers are reachable
set -euo pipefail

# Configuration constants
readonly PORT_RANGE_START=8888
readonly PORT_RANGE_END=9999
readonly FLARESOLVERR_TIMEOUT=30
readonly PROXY_STARTUP_TIMEOUT=5
readonly CONTAINER_NAME="test-flaresolverr"
readonly TEMP_BINARY="/tmp/flareproxygo-test"
readonly TEMP_LOG="/tmp/proxy.log"
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
readonly PROJECT_ROOT

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m' # No Color

# Global variables
FLARESOLVERR_PORT=""
PROXY_PORT=""
PROXY_PID=""
VERBOSE="${VERBOSE:-0}"

# Logging functions
log() {
    [ "$VERBOSE" -eq 1 ] && echo -e "${YELLOW}[DEBUG]${NC} $*" >&2 || true
}

info() {
    echo -e "${YELLOW}$*${NC}"
}

success() {
    echo -e "${GREEN}✓ $*${NC}"
}

fail() {
    local message=$1
    echo -e "${RED}✗ $message${NC}" >&2
    
    # Capture logs for debugging
    if [ -n "${PROXY_PID:-}" ] && [ -f "$TEMP_LOG" ]; then
        echo -e "${RED}Proxy logs:${NC}" >&2
        tail -20 "$TEMP_LOG" >&2
    fi
    
    if docker logs "$CONTAINER_NAME" 2>/dev/null; then
        echo -e "${RED}FlareSolverr logs (last 20 lines):${NC}" >&2
        docker logs "$CONTAINER_NAME" 2>&1 | tail -20 >&2
    fi
    
    exit 1
}

# Check prerequisites
check_prerequisites() {
    local missing=()
    
    command -v docker >/dev/null 2>&1 || missing+=("docker")
    command -v go >/dev/null 2>&1 || missing+=("go")
    command -v curl >/dev/null 2>&1 || missing+=("curl")
    command -v lsof >/dev/null 2>&1 || missing+=("lsof")
    
    if [ ${#missing[@]} -gt 0 ]; then
        fail "Missing required tools: ${missing[*]}"
    fi
    
    # Check if Docker is running
    if ! docker info >/dev/null 2>&1; then
        fail "Docker is not running"
    fi
    
    log "All prerequisites checked"
}

# Find an available port
find_free_port() {
    local port
    for port in $(seq "$PORT_RANGE_START" "$PORT_RANGE_END"); do
        if ! lsof -Pi :"$port" -sTCP:LISTEN -t >/dev/null 2>&1; then
            echo "$port"
            return 0
        fi
    done
    return 1
}

# Wait for a service to be ready
wait_for_service() {
    local url=$1
    local timeout=$2
    local service_name=$3
    local elapsed=0
    
    log "Waiting for $service_name at $url (timeout: ${timeout}s)"
    
    while [ $elapsed -lt "$timeout" ]; do
        if curl -s "$url" >/dev/null 2>&1; then
            log "$service_name is ready after ${elapsed}s"
            return 0
        fi
        sleep 1
        ((elapsed++))
        
        # Show progress every 5 seconds
        if [ $((elapsed % 5)) -eq 0 ]; then
            log "Still waiting for $service_name... (${elapsed}s elapsed)"
        fi
    done
    
    return 1
}

# Cleanup function
cleanup() {
    info "Cleaning up..."
    
    # Kill proxy if running
    if [ -n "${PROXY_PID:-}" ]; then
        log "Stopping proxy (PID: $PROXY_PID)"
        kill "$PROXY_PID" 2>/dev/null || true
        # Wait a moment for clean shutdown
        sleep 0.5
    fi
    
    # Remove test binary and log
    rm -f "$TEMP_BINARY" "$TEMP_LOG"
    
    # Stop and remove FlareSolverr container
    if docker ps -q -f name="$CONTAINER_NAME" 2>/dev/null | grep -q .; then
        log "Stopping FlareSolverr container"
        docker stop -t 2 "$CONTAINER_NAME" >/dev/null 2>&1 || true
    fi
    
    if docker ps -aq -f name="$CONTAINER_NAME" 2>/dev/null | grep -q .; then
        log "Removing FlareSolverr container"
        docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
    fi
}

# Set up environment
setup_environment() {
    # Set trap for cleanup on exit
    trap cleanup EXIT INT TERM
    
    # Clean up any existing test container
    docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
    
    log "Environment setup complete"
}

# Start FlareSolverr container
start_flaresolverr() {
    FLARESOLVERR_PORT=$(find_free_port) || fail "Could not find available port for FlareSolverr"
    info "Starting FlareSolverr on port ${FLARESOLVERR_PORT}"
    
    local container_id
    container_id=$(docker run -d \
        --name "$CONTAINER_NAME" \
        -p "${FLARESOLVERR_PORT}:8191" \
        -e LOG_LEVEL=info \
        -e LOG_HTML=false \
        -e CAPTCHA_SOLVER=none \
        ghcr.io/flaresolverr/flaresolverr:latest 2>&1) || fail "Failed to start FlareSolverr container"
    
    log "Container started with ID: ${container_id:0:12}"
    
    info "Waiting for FlareSolverr to be ready..."
    if wait_for_service "http://localhost:${FLARESOLVERR_PORT}/health" "$FLARESOLVERR_TIMEOUT" "FlareSolverr"; then
        success "FlareSolverr is ready!"
        return 0
    else
        fail "FlareSolverr failed to start within ${FLARESOLVERR_TIMEOUT} seconds"
    fi
}

# Build and start proxy
build_and_start_proxy() {
    PROXY_PORT=$(find_free_port) || fail "Could not find available port for proxy"
    info "Building and starting flareproxygo on port ${PROXY_PORT}"
    
    # Build the binary
    cd "$PROJECT_ROOT" || fail "Failed to change to project root"
    
    log "Building binary to $TEMP_BINARY"
    if ! go build -o "$TEMP_BINARY" main.go 2>&1; then
        fail "Failed to build proxy binary"
    fi
    
    # Start the proxy
    log "Starting proxy with FlareSolverr at http://localhost:${FLARESOLVERR_PORT}/v1"
    FLARESOLVERR_URL="http://localhost:${FLARESOLVERR_PORT}/v1" PORT="$PROXY_PORT" \
        "$TEMP_BINARY" > "$TEMP_LOG" 2>&1 &
    PROXY_PID=$!
    
    log "Proxy started with PID: $PROXY_PID"
    
    # Wait for proxy to be ready
    info "Waiting for proxy to be ready..."
    sleep "$PROXY_STARTUP_TIMEOUT"
    
    # Check if proxy is still running
    if ! kill -0 "$PROXY_PID" 2>/dev/null; then
        fail "Proxy failed to start. Check logs above for details."
    fi
    
    success "Proxy is running!"
    return 0
}

# Test basic proxy functionality
test_basic_proxy() {
    local test_url=$1
    local test_name=${2:-"Basic proxy test"}
    
    info "Running test: $test_name ($test_url)"
    
    local response http_code body
    response=$(curl -s --proxy "127.0.0.1:${PROXY_PORT}" -w "\n%{http_code}" "$test_url" 2>/dev/null || true)
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    log "HTTP response code: $http_code"
    log "Response body length: ${#body} bytes"
    
    if [ "$http_code" = "200" ]; then
        # Check if response contains HTML (check first 1000 chars to avoid issues with large responses)
        if echo "${body:0:1000}" | grep -qi "<html\|<!doctype" >/dev/null 2>&1; then
            success "$test_name passed (HTTP 200, HTML content received)"
            return 0
        else
            # Debug: show first 200 chars of response
            log "First 200 chars of response: ${body:0:200}"
            fail "$test_name: Got HTTP 200 but response doesn't look like HTML"
        fi
    else
        fail "$test_name: Expected HTTP 200, got $http_code"
    fi
}

# Run test suite
run_test_suite() {
    info "Running integration test suite..."
    
    # Test 1: Basic proxy functionality with Google
    test_basic_proxy "http://www.google.com" "Google.com proxy test"
    
    # Test 2: Another site
    test_basic_proxy "http://example.com" "Example.com proxy test"
    
    # Add more tests as needed
    
    success "All tests passed!"
}

# Main function
main() {
    info "Starting FlareSolverr integration tests..."
    
    check_prerequisites
    setup_environment
    
    start_flaresolverr
    build_and_start_proxy
    
    run_test_suite
    
    success "Integration test suite completed successfully!"
    
    # Cleanup is handled by trap
    exit 0
}

# Run main function
main "$@"