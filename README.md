# turnfly

Self-deploying TURN service for Fly.io.

turnfly runs a Pion TURN server with short-lived HMAC credentials, health
checks, metrics, and multi-region Fly Machines deployment.

## Quick Start

Build the CLI:

```bash
make build
```

Deploy through APIs:

```bash
export FLY_API_TOKEN="$(fly tokens create org --name turnfly-autodeploy --expiry 720h)"

./turnfly autodeploy \
  --app your-turnfly-app \
  --org personal \
  --regions iad,ord
```

`autodeploy` builds and pushes `registry.fly.io/<app>:<timestamp>` through the
Docker Engine API, then uses the Fly Machines API to create or update the app,
IPv4 allocation, and regional Machines.

## Local Run

```bash
export TURN_REALM="turnfly.local"
export TURN_SHARED_SECRET="dev-secret-change-me"
export ADMIN_API_TOKEN="dev-token-change-me"

./turnfly serve-turn
```

## Commands

```bash
./turnfly serve-turn       # Run TURN server and control API
./turnfly autodeploy       # Build, push, and deploy via APIs
./turnfly image push       # Build and push image only
./turnfly deploy           # Deploy an existing image via Machines API
./turnfly destroy          # Destroy app Machines
./turnfly ice-config       # Generate WebRTC ICE config
./turnfly probe            # Probe framework
./turnfly serve-relay      # Experimental relay-pair mode
```

## Required Config

Runtime:

```bash
TURN_REALM
TURN_SHARED_SECRET
ADMIN_API_TOKEN
```

API deploy:

```bash
FLY_API_TOKEN
```

Useful optional config:

```bash
TURN_PORT=3478
HTTP_PORT=8080
METRICS_ADDR=:9090
FLY_APP_NAME=your-turnfly-app
FLY_ORG=personal
```

## Image and Deploy Separately

```bash
./turnfly image push --app your-turnfly-app --tag latest

./turnfly deploy \
  --app your-turnfly-app \
  --org personal \
  --regions iad,ord \
  --image registry.fly.io/your-turnfly-app:latest \
  --env TURN_REALM=your-turnfly-app.fly.dev \
  --env TURN_SHARED_SECRET="..." \
  --env ADMIN_API_TOKEN="..."
```

## HTTP API

```text
POST /v1/credentials
GET  /healthz
GET  /readyz
GET  /metrics
GET  /v1/regions
POST /v1/ice-report
POST /v1/deploy
```

Credential request:

```json
{
  "user_id": "alice",
  "validity_seconds": 3600
}
```

## Development

```bash
make check
make build
```

More detail:

- [Deployment guide](docs/deployment.md)
- [Security notes](docs/security.md)
- [Relay mode](docs/relay-mode.md)
- [Implementation scope](SCOPE.md)
