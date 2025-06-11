# Build stage
FROM golang:1.24.4-alpine AS builder

WORKDIR /workspace

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY internal/ internal/

# Build the operator
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo \
    -o manager \
    ./cmd/operator

# Runtime stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/manager .

USER 65534:65534

ENTRYPOINT ["/manager"]