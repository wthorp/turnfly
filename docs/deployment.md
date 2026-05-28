# Fly.io Deployment Guide

## Prerequisites

1. [Fly.io account](https://fly.io)
2. [flyctl CLI](https://fly.io/docs/hands-on/install-flyctl/)
3. Docker (for building images)

## Quick Deploy

```bash
# Set required secrets
fly secrets set TURN_SHARED_SECRET="$(openssl rand -base64 32)"
fly secrets set ADMIN_API_TOKEN="$(openssl rand -base64 32)"

# Deploy a single-region instance
fly deploy
```

## Multi-Region Deploy

Use the self-deployer for multi-region:

```bash
# Build and push image
docker build -t registry.fly.io/turnfly:latest .
docker push registry.fly.io/turnfly:latest

# Deploy to multiple regions
turnfly deploy \
  --app turnfly \
  --org personal \
  --regions iad,ord,sjc,lhr \
  --image registry.fly.io/turnfly:latest \
  --env TURN_REALM=turnfly.example.com
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
turnfly deploy --dry-run --regions iad,ord --image test:latest
```

## Teardown

```bash
turnfly destroy --app turnfly --yes
```
