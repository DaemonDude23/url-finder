FROM docker.io/library/golang:1.25.4 AS builder

WORKDIR /app

# Copy go mod and sum files
COPY ./src/go.mod ./src/go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY ./src/ ./

# Build the Go app
RUN go build -o url-finder .

FROM docker.io/library/golang:1.25.4

WORKDIR /app

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/url-finder /app

# Command to run the executable
CMD ["/app/url-finder"]
