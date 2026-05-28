FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Cache go modules
COPY go.mod go.sum ./
RUN go mod download

# Build the binary
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /turnfly ./cmd/turnfly

FROM alpine:3.21

# Install ca-certificates for TLS
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN adduser -D -g '' turnfly

COPY --from=builder /turnfly /usr/local/bin/turnfly

USER turnfly

EXPOSE 3478/udp
EXPOSE 3478/tcp
EXPOSE 8080/tcp
EXPOSE 9090/tcp

ENTRYPOINT ["turnfly"]
CMD ["serve-turn"]
