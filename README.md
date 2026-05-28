# turnfly

Self-deploying TURN server for Fly.io, written in Go.

## Overview

turnfly runs Pion TURN servers on Fly.io and can deploy itself using the Fly Machines API. It supports multi-region independent TURN operation with ephemeral HMAC credentials.

## Quick Start

### Build

```bash
make build
```

### Run locally

```bash
export TURN_REALM="turnfly.local"
export TURN_SHARED_SECRET="your-secret-here"
export ADMIN_API_TOKEN="your-admin-token"

./turnfly serve-turn
```

### Docker

```bash
make docker-build
make docker-run
```

## CLI Commands

```bash
turnfly serve-turn     # Start TURN server with control API (Phase 1)
turnfly serve-relay    # Experimental relay-pair mode (Phase 4)
turnfly deploy         # Deploy to Fly.io (Phase 2)
turnfly destroy        # Destroy deployment (Phase 2)
turnfly probe          # Synthetic measurement probes (Phase 3)
turnfly image          # Build and push Docker image
```

## Configuration

| Variable           | Required | Default    | Description                        |
|--------------------|----------|------------|------------------------------------|
| TURN_PORT          | No       | 3478       | TURN UDP/TCP listen port           |
| TURN_REALM         | **Yes**  | -          | TURN realm                         |
| TURN_SHARED_SECRET | **Yes**  | -          | HMAC shared secret for credentials |
| ADMIN_API_TOKEN    | **Yes**  | -          | Admin API bearer token             |
| HTTP_PORT          | No       | 8080       | Control API HTTP port              |
| METRICS_ADDR       | No       | :9090      | Prometheus metrics listen address  |
| LOG_LEVEL          | No       | info       | Log level (debug/info/warn/error)  |
| FLY_APP_NAME       | No       | -          | Fly.io app name                    |
| FLY_ORG            | No       | -          | Fly.io organization                |
| RELAY_MODE         | No       | false      | Enable experimental relay mode     |
| RELAY_PEERS        | No       | -          | Comma-separated relay peer addrs   |

## API Endpoints

| Method | Path            | Description              |
|--------|-----------------|--------------------------|
| POST   | /v1/credentials | Generate TURN credentials |
| GET    | /healthz        | Health check with details |
| GET    | /readyz         | Readiness check          |
| GET    | /metrics        | Prometheus metrics       |

### POST /v1/credentials

Request:
```json
{
  "user_id": "alice",
  "validity_seconds": 3600
}
```

Response:
```json
{
  "username": "1716912000:alice",
  "password": "base64hmac...",
  "ttl_seconds": 3600
}
```

## Development

```bash
make fmt        # Format code
make vet        # Run go vet
make test       # Run tests
make check      # Run all checks (fmt, vet, test)
make tidy       # Run go mod tidy
```

## Deployment to Fly.io

### Prerequisites

1. Install [flyctl](https://fly.io/docs/hands-on/install-flyctl/)
2. Create a Fly.io account

### Deploy

```bash
# Set secrets
fly secrets set TURN_SHARED_SECRET="..."
fly secrets set ADMIN_API_TOKEN="..."

# Deploy
fly deploy
```

## Architecture

```
multi-region independent TURN on Fly.io
+
ephemeral credentials
+
self-deployment via Fly Machines API
+
measurement-driven region selection
```

See [AGENTS.md](AGENTS.md) for development guidelines and [SCOPE.md](SCOPE.md) for the full implementation brief.

## Phases

- **Phase 1** ✅ Plain Fly TURN with Pion, credential endpoint, metrics, health
- **Phase 2** 🚧 Self-deployer via Fly Machines API
- **Phase 3** 🚧 Multi-region independent TURN with ICE config generation
- **Phase 4** 🚧 QUIC relay-pair experiment
- **Phase 5** 🚧 Production hardening

## License

Proprietary. All rights reserved.
