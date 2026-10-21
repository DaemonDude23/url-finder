# Builder stage
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.27.3@sha256:f20d83a83e18dbd2a4714b8ffd4b30b287b6243962dd75608e86abcc63ba4efc AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /build
# Copy go mod files and download dependencies
COPY ./src/go.mod ./src/go.sum ./
RUN go mod download
# Copy the rest of the source code
COPY ./src/ ./
# Build the url-finder binary with CGO disabled for a fully static binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-s -w -X main.version=${VERSION}" -o url-finder .

# Final stage
FROM scratch AS final
# Copy the url-finder binary to a known location
COPY --from=builder /build/url-finder /usr/local/bin/url-finder
# Provide CA certificates for TLS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
# By default, set the binary as the entry point in case you want to run it
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/url-finder"]
