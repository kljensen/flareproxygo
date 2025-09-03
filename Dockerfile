# Build stage
FROM golang:1.22-alpine AS builder

# Install CA certificates
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy go mod files
COPY go.mod ./

# Copy source code
COPY main.go ./

# Build the binary with static linking
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o flareproxygo .

# Final stage - minimal scratch container
FROM scratch

# Copy CA certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /app/flareproxygo /flareproxygo

# Expose port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/flareproxygo"]