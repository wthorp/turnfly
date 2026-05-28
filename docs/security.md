# Security Guide

## Authentication Model

turnfly uses three separate authentication layers:

### 1. TURN Credentials (HMAC-SHA1)

Short-lived, ephemeral credentials for WebRTC clients:

```
username = unix_expiry_timestamp:user_id
password = base64(hmac_sha1(shared_secret, username))
```

- Default validity: 24 hours
- Same secret shared across all TURN servers in a deployment
- No persistent user database required

### 2. Admin API Token (Bearer)

Protects administrative endpoints (`/v1/deploy`):

```http
Authorization: Bearer <ADMIN_API_TOKEN>
```

- Set via `ADMIN_API_TOKEN` environment variable
- Required for deploy and management operations
- Should be a strong random string (≥ 256 bits)

### 3. Relay Peer TLS

Mutual TLS between relay-pair peers:

- Self-signed certificates for Fly private networking
- CA-signed certificates for public internet relay
- QUIC transport provides built-in encryption

## Secret Management

### Required Secrets

| Secret | Purpose | Format |
|--------|---------|--------|
| TURN_SHARED_SECRET | HMAC key for TURN credentials | Random base64, ≥ 32 bytes |
| ADMIN_API_TOKEN | Admin API auth | Random base64, ≥ 32 bytes |
| FLY_API_TOKEN | Fly.io API access | Fly personal access token |
| RELAY_PRIVATE_KEY | Relay peer auth (experimental) | PEM private key |

### Never Commit

- `.env` files
- Generated credentials
- Private keys
- API tokens
- `fly.toml` with secrets embedded

### Setting Secrets on Fly

```bash
fly secrets set TURN_SHARED_SECRET="..."
fly secrets set ADMIN_API_TOKEN="..."
```

## Quota Enforcement

### Per-User Limits

- Max 10 concurrent TURN allocations
- Max 1 MB/s bandwidth
- Max 60 credential requests/minute
- Max 10 minute allocation lifetime

### Per-IP Limits

- Max 5 concurrent TURN allocations per IP

These defaults prevent:
- Resource exhaustion by a single user
- Credential harvesting
- Bandwidth abuse

## Rate Limiting

HTTP control API endpoints are rate-limited by client IP:

- Default: 10 requests/second with burst of 20
- Returns 429 Too Many Requests when exceeded
- Retry-After header included in responses

## Abuse Detection

The following patterns are logged and metered:

- Rapid credential requests (> 60/minute per user)
- Excessive allocation churn (create/destroy cycles)
- Bandwidth exceeding quota thresholds
- Authentication failures (increment `turn_auth_failures_total`)

Monitor the Prometheus metrics dashboard for anomalies.

## Network Security

### Recommended Configuration

1. **No anonymous TURN access** — always require authentication
2. **Short-lived credentials** — default 24h, lower for sensitive applications
3. **TLS for control API** — use Fly.io's automatic TLS
4. **Private metrics** — bind metrics to internal network unless intentionally public
5. **Separate deploy token** — use a different token for deploy vs runtime

### Fly.io Network Isolation

- Machines within the same app can communicate via Fly private networking (6PN)
- Public services are exposed through Fly's global Anycast network
- Dedicated IPv4 recommended for UDP services

## Incident Response

If abuse is detected:

1. Rotate `TURN_SHARED_SECRET` immediately
2. Rotate `ADMIN_API_TOKEN`
3. Review metrics for affected time window
4. Deploy updated credentials
5. Consider reducing quota limits temporarily

## Regular Audits

- Rotate secrets quarterly
- Review quota utilization monthly
- Monitor auth failure rates daily
- Test credential validation weekly
