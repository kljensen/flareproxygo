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

Set FlareProxy Go as a proxy in your browser or in your agent. Please note: use `http` protocol even if you want to fetch HTTPS resources. FlareProxy Go will automatically switch to the HTTPS protocol when establishing the upstream connection.

```bash
curl --proxy 127.0.0.1:8080 http://www.google.com
```

You can use it as a proxy in [changedetection](https://github.com/dgtlmoon/changedetection.io). Navigate to Settings → CAPTCHA&Proxies and add it as an extra proxy in the list. Then you can set up your watch using any fetch method.

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
      - TZ=Europe/Rome
    ports:
      - "8080:8080"
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

## Architecture

FlareProxy Go acts as an HTTP proxy that:
1. Receives HTTP GET/CONNECT requests from clients
2. Transforms the URL from HTTP to HTTPS if needed
3. Forwards the request to FlareSolverr API
4. Extracts the HTML response from FlareSolverr's JSON response
5. Returns the HTML content to the client

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
