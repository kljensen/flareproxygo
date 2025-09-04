# FlareProxy Go

FlareProxy Go is a transparent HTTP proxy adapter that seamlessly forwards client requests to FlareSolverr to bypass Cloudflare and DDoS-GUARD protection. This is a Go implementation of the original [FlareProxy](https://github.com/mimnix/FlareProxy) Python project.

## Features

- Zero external dependencies - uses only Go standard library
- Minimal Docker image (~5-7MB) using scratch base
- Multi-architecture support (amd64/arm64)
- Compatible with the original FlareProxy

## Installation

### Using Pre-built Docker Images

FlareProxy Go is available as a pre-built container image from GitHub Container Registry:

```bash
docker pull ghcr.io/kljensen/flareproxygo:latest
```

Available tags:
- `latest` - Latest stable release
- `v0.1.0`, `v0.2.0`, etc. - Specific version tags
- `0.1`, `0.2`, etc. - Major.minor version tags

### Build from Source

FlareProxy Go can be built using the [Dockerfile](Dockerfile) provided in the repository root:

```bash
docker build -t flareproxygo .
```

For multi-architecture builds:
```bash
docker buildx build --platform linux/amd64,linux/arm64 -t flareproxygo .
```

## Run

To run it, replace the FLARESOLVERR_URL env var with the URL of your FlareSolverr instance. FlareProxy Go runs on port 8080.

```bash
docker run -e FLARESOLVERR_URL=http://localhost:8191/v1 -p 8080:8080 flareproxygo
```

## Usage

FlareProxy Go supports two modes of operation:

### 1. Direct Routing Mode (Default)

The primary mode accepts URLs directly in the path, no proxy configuration needed:

```bash
# Access sites by putting the domain in the path (default port 8080)
curl http://localhost:8080/www.example.com/some/path
curl http://localhost:8080/www.science.org/content/article/example

# Query parameters are preserved
curl "http://localhost:8080/example.com/search?q=test&page=1"

# Supports POST and other HTTP methods
curl -X POST http://localhost:8080/api.example.com/endpoint -d "data"
```

The direct mode:
- Extracts the domain from the first path segment
- Reconstructs the full URL (tries HTTPS first, falls back to HTTP)
- Forwards the request through FlareSolverr
- Returns the response directly

This is the simplest way to use FlareProxy Go - no client configuration required!

### 2. Proxy Mode (Optional)

When `PROXY_PORT` is configured, FlareProxy Go also runs as a traditional HTTP proxy:

```bash
# Enable proxy mode by setting PROXY_PORT
export PROXY_PORT=8888

# Use as HTTP proxy
curl --proxy 127.0.0.1:8888 http://www.google.com
```

**Important: HTTP-Only Proxy Limitation**

The proxy mode does NOT support the CONNECT method used for traditional HTTPS tunneling. This is by design because:

1. FlareSolverr needs to process the actual request content to bypass Cloudflare protection
2. Standard CONNECT tunneling creates an encrypted tunnel that would prevent FlareSolverr from seeing the request
3. The proxy automatically converts HTTP requests to HTTPS when communicating with target sites through FlareSolverr

**Always use HTTP URLs in proxy mode**, even when accessing HTTPS sites:

```bash
# Correct usage - use http:// URL even for HTTPS sites
curl --proxy 127.0.0.1:8888 http://www.google.com

# This will NOT work - CONNECT method is not supported
curl --proxy 127.0.0.1:8888 https://www.google.com
```

You can use proxy mode with [changedetection](https://github.com/dgtlmoon/changedetection.io). Navigate to Settings → CAPTCHA&Proxies and add it as an extra proxy in the list.

## Docker Compose

Add this snippet to your docker-compose stack:

```yaml
  flaresolverr:
    image: ghcr.io/flaresolverr/flaresolverr:latest
    container_name: flaresolverr
    environment:
      - LOG_LEVEL=${LOG_LEVEL:-info}
      - LOG_HTML=${LOG_HTML:-false}
      - CAPTCHA_SOLVER=${CAPTCHA_SOLVER:-none}
      - TZ=Europe/Rome
#    ports:
#      - "8191:8191"
    restart: always

  flareproxygo:
    image: ghcr.io/kljensen/flareproxygo:latest
    container_name: flareproxygo
    environment:
      - FLARESOLVERR_URL=http://flaresolverr:8191/v1
      - PROXY_PORT=8888  # Optional: enables proxy mode
      - TZ=Europe/Rome
    ports:
      - "8080:8080"  # Direct routing mode (default)
      - "8888:8888"  # Proxy mode (optional, only if PROXY_PORT is set)
    restart: always
```

## Development

### Prerequisites

- Go 1.22 or later
- Docker (for containerized testing)

### Local Development

1. Clone the repository:
```bash
git clone https://github.com/kljensen/flareproxygo.git
cd flareproxygo
```

2. Run locally:
```bash
FLARESOLVERR_URL=http://localhost:8191/v1 go run main.go
```

3. Test with curl:
```bash
curl --proxy 127.0.0.1:8080 http://www.google.com
```

### Using Just (Task Runner)

If you have [Just](https://github.com/casey/just) installed:

```bash
# Build the binary
just build

# Run locally
just run

# Build Docker image
just docker-build

# Run tests
just test
```

## Environment Variables

- `FLARESOLVERR_URL`: URL of your FlareSolverr instance (default: `http://flaresolverr:8191/v1`)
- `PORT`: Port for direct routing mode (default: `8080`)
- `PROXY_PORT`: Port for proxy mode (optional, only runs proxy server when set)

## Architecture

FlareProxy Go acts as a FlareSolverr adapter with two modes:

### Direct Mode (Primary)
1. Receives requests at `http://localhost:PORT/domain.com/path`
2. Extracts domain from URL path
3. Reconstructs full target URL (HTTPS first, HTTP fallback)
4. Forwards to FlareSolverr API to bypass Cloudflare protection
5. Returns the HTML response directly

### Proxy Mode (Optional)
1. Receives HTTP proxy requests (CONNECT not supported)
2. Transforms URLs from HTTP to HTTPS for target sites
3. Forwards to FlareSolverr API to bypass Cloudflare protection
4. Returns the HTML response to the client

Note: Neither mode supports CONNECT tunneling. This is specifically designed as an adapter for FlareSolverr, which requires visibility into request content to bypass Cloudflare challenges.

## Differences from Original Python Implementation

- Written in Go instead of Python
- Uses only standard library (no external dependencies)
- Smaller Docker image (~5-7MB vs ~50MB)
- Native multi-architecture support
- Slightly different error handling structure

## License

This project is released into the public domain under the Unlicense. See the [LICENSE](LICENSE) file for details.

## Related Projects

- [FlareSolverr](https://github.com/FlareSolverr/FlareSolverr) - The backend service that bypasses Cloudflare
- [FlareProxy](https://github.com/mimnix/FlareProxy) - Original Python implementation

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

For issues and questions, please open an issue on GitHub.
