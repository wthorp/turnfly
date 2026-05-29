# Fly.io Deployment Guide

## Prerequisites

1. [Fly.io account](https://fly.io)
2. [flyctl CLI](https://fly.io/docs/hands-on/install-flyctl/)
3. Docker (for building images)

## API Autodeploy

The preferred turnfly workflow is API-only:

1. Build and push the image through the Docker Engine API.
2. Create or verify the Fly app through the Fly Machines API.
3. Allocate a dedicated IPv4 for UDP.
4. Create or update Machines in the requested regions.

Create an org-scoped deploy token because first deploys may need to create the
app and allocate resources:

```bash
export FLY_API_TOKEN="$(fly tokens create org --name turnfly-autodeploy --expiry 720h)"
```

Run a single-region deployment:

```bash
turnfly autodeploy \
  --app my-turnfly \
  --org personal \
  --regions iad
```

Run a multi-region deployment:

```bash
turnfly autodeploy \
  --app my-turnfly \
  --org personal \
  --regions iad,ord,sjc,lhr
```

`autodeploy` generates `TURN_SHARED_SECRET` and `ADMIN_API_TOKEN` if they are
not provided. To keep credentials stable across deploys, provide them explicitly:

```bash
turnfly autodeploy \
  --app my-turnfly \
  --org personal \
  --regions iad,ord \
  --turn-shared-secret "$TURN_SHARED_SECRET" \
  --admin-api-token "$ADMIN_API_TOKEN"
```

The generated values are sent in the Machine environment through the Machines
API. Fly app-vault secret management is still a future hardening step for this
API-only path.

## Separate Image and Deploy Steps

For debugging and CI, publish the image first and deploy that immutable image:

```bash
turnfly image push \
  --app my-turnfly \
  --tag latest

turnfly deploy \
  --app my-turnfly \
  --org personal \
  --regions iad,ord,sjc,lhr \
  --image registry.fly.io/my-turnfly:latest \
  --env TURN_REALM=my-turnfly.fly.dev \
  --env TURN_SHARED_SECRET="$TURN_SHARED_SECRET" \
  --env ADMIN_API_TOKEN="$ADMIN_API_TOKEN"
```

## Region Selection Strategy

For optimal latency, deploy to regions near your users:

| Region | Location | Best for |
|--------|----------|----------|
| iad | Ashburn, VA | US East Coast |
| ord | Chicago, IL | US Central |
| sjc | Sunnyvale, CA | US West Coast |
| lhr | London, UK | Europe |
| nrt | Tokyo, Japan | Asia-Pacific |
| syd | Sydney, AU | Oceania |

## Networking Requirements

### UDP Service

Fly.io requires a dedicated IPv4 for UDP services:

```bash
fly ips allocate-v4 --shared
```

The TURN server binds to `fly-global-services` for UDP. TCP services bind to `0.0.0.0`.

### Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 3478 | UDP | TURN relay |
| 3478 | TCP | TURN fallback |
| 8080 | TCP | Control API |
| 9090 | TCP | Prometheus metrics |

## Health Checks

After deployment, verify:

```bash
# Check health
curl https://turnfly.fly.dev/healthz

# Get ICE config
curl https://turnfly.fly.dev/v1/regions

# Get credentials
curl -X POST https://turnfly.fly.dev/v1/credentials \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test", "validity_seconds": 3600}'
```

## Cost Management

turnfly uses Fly.io's shared-cpu-1x VMs (256MB RAM) by default. Expected costs:

- Shared CPU-1x: ~$2.50/month per machine
- Dedicated IPv4: ~$2/month per region
- Total for 4 regions: ~$18/month

Set cost controls via Fly.io organization budgets.

## Dry Run

Plan deployments without creating resources:

```bash
turnfly autodeploy \
  --dry-run \
  --app my-turnfly \
  --org personal \
  --regions iad,ord
```

## Teardown

```bash
turnfly destroy --app turnfly --yes
```
