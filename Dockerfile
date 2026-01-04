# Build stage
# Note: Using latest stable Go for build environment
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Copy source
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY prices ./prices

# Build binary
# CGO_ENABLED=0 for static binary
RUN CGO_ENABLED=0 go build -o plarix-scan ./cmd/plarix-scan

# Runtime stage
# minimal alpine image
FROM alpine:latest
WORKDIR /app

# Install certificates for HTTPS (needed for provider calls)
RUN apk --no-cache add ca-certificates

# Copy binary and assets
COPY --from=builder /app/plarix-scan .
COPY --from=builder /app/prices ./prices

# Setup user
RUN adduser -D -g '' plarix
USER plarix

EXPOSE 8080
VOLUME /data

ENTRYPOINT ["./plarix-scan"]
# Default to proxy mode
CMD ["proxy", "--port", "8080", "--ledger", "/data/plarix-ledger.jsonl"]
