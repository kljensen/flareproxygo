# FlareProxy Go - Task Runner

# Default recipe to display help
default:
    @just --list

# Build the Go binary
build:
    go build -o flareproxygo .

# Run the proxy locally
run:
    FLARESOLVERR_URL="${FLARESOLVERR_URL:-http://localhost:8191/v1}" go run main.go

# Format Go code
fmt:
    go fmt ./...

# Run go vet
vet:
    go vet ./...

# Run tests
test:
    go test -v ./...

# Clean build artifacts
clean:
    rm -f flareproxygo
    rm -rf tmp/

# Build Docker image
docker-build:
    docker build -t flareproxygo .

# Build multi-architecture Docker image
docker-buildx:
    docker buildx build --platform linux/amd64,linux/arm64 -t flareproxygo .

# Run Docker container
docker-run:
    docker run -e FLARESOLVERR_URL=http://localhost:8191/v1 -p 8080:8080 flareproxygo

# Test with curl (requires proxy to be running)
test-curl:
    curl --proxy 127.0.0.1:8080 http://www.google.com

# Run all checks (format, vet, build)
check: fmt vet build
    @echo "All checks passed!"

# View Docker image size
docker-size:
    docker images flareproxygo --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"

# Run go mod tidy
tidy:
    go mod tidy

# Install to GOPATH/bin
install:
    go install .

# Create a release build
release:
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o flareproxygo-linux-amd64 .
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -o flareproxygo-linux-arm64 .
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -installsuffix cgo -o flareproxygo-darwin-amd64 .
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -a -installsuffix cgo -o flareproxygo-darwin-arm64 .