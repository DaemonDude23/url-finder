# Builder stage
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.27.3@sha256:e519d3b0a5b285e5aa53ab9eef02bc6bbb9cb04608389b743a136a9ce75f0d1e AS builder
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
