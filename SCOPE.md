# Implementation Brief: Fly.io Self-Deploying TURN Service in Go

You are building a production-quality Go service called `turnfly`.

The application runs on Fly.io and provides TURN server functionality for WebRTC clients. It must also be able to deploy and manage itself on Fly.io using the Fly Machines API.

The project should be implemented as a single Go codebase and preferably a single Go binary.

---

## Primary Goals

1. Build an embeddable TURN server in Go.
2. Run the TURN server on Fly.io.
3. Allow the application to deploy itself to Fly.io.
4. Support normal single-region TURN operation.
5. Support multi-region independent TURN operation.
6. Optionally support an experimental relay-pair mode where two TURN servers communicate over a private optimized transport.
7. Prioritize correctness, security, observability, and deployability over speculative optimization.

---

## Core Binary

The binary should expose these commands:

```bash
turnfly serve-turn
turnfly serve-relay
turnfly deploy
turnfly destroy
turnfly probe
turnfly image
```

Suggested CLI library:

```go
github.com/spf13/cobra
```

Suggested config library:

```go
github.com/spf13/viper
```

---

## Repository Layout

```text
/cmd/turnfly
/internal/turnserver
/internal/relay
/internal/flydeploy
/internal/config
/internal/auth
/internal/metrics
/internal/health
/internal/probe
/internal/controlapi
```

---

## Recommended Go Libraries

```text
github.com/pion/turn/v2
github.com/pion/stun
github.com/quic-go/quic-go
github.com/prometheus/client_golang
golang.org/x/sync/errgroup
github.com/spf13/cobra
github.com/spf13/viper
```

---

## Operating Modes

### 1. Single-Region TURN Mode

One Fly Machine runs a TURN server.

```text
WebRTC Client A ── TURN ── Fly TURN Server ── Peer B
```

This is the first implementation target.

---

### 2. Multi-Region Independent TURN Mode

Multiple Fly Machines run independent TURN servers in different regions.

Clients receive multiple TURN URLs in their ICE config.

```js
iceServers: [
  { urls: "turn:iad.turn.example.com:3478", username, credential },
  { urls: "turn:sjc.turn.example.com:3478", username, credential },
  { urls: "turn:lhr.turn.example.com:3478", username, credential }
]
```

This should be preferred before building custom relay behavior.

---

### 3. Experimental Relay-Pair Mode

Two Fly Machines run in different regions.

```text
Client A
  ↓
TURN server near Client A
  ↓ private optimized tunnel
TURN server near Client B
  ↓
Client B
```

This mode is experimental. It should only be considered successful if benchmarking shows better throughput, lower loss, or better stability than ordinary single-region or multi-region TURN.

---

## Fly.io Constraints

The implementation must respect these Fly.io realities:

1. UDP requires careful Fly service configuration.
2. TURN normally uses UDP/TCP 3478.
3. TURN over TLS normally uses 5349.
4. Fly UDP services may require a dedicated IPv4 address.
5. Fly UDP apps may need to bind to `fly-global-services`.
6. TCP services should usually bind to `0.0.0.0`.
7. Fly private networking is available through 6PN.
8. Public UDP relay-port ranges may be difficult or impossible to expose exactly like a traditional coturn deployment.

Because of this, the first version should use a constrained TURN configuration and validate Fly UDP behavior early.

---

## Deployment Requirements

The app should deploy itself using the Fly Machines API.

Do not rely primarily on shelling out to `flyctl`, although a fallback or development path may be acceptable.

The `turnfly deploy` command should:

```text
1. Read FLY_API_TOKEN.
2. Create or verify the Fly app.
3. Allocate required public IPs.
4. Create Machines in selected regions.
5. Configure secrets.
6. Configure public UDP/TCP services.
7. Start Machines.
8. Poll health endpoints.
9. Output WebRTC ICE server configuration.
```

Example CLI:

```bash
turnfly deploy \
  --app my-turnfly \
  --org personal \
  --regions iad,ord,sjc,lhr \
  --mode multi-region-independent \
  --image registry.fly.io/my-turnfly:latest
```

---

## Required Secrets

```text
TURN_REALM
TURN_SHARED_SECRET
ADMIN_API_TOKEN
RELAY_PRIVATE_KEY
RELAY_PEER_CONFIG
FLY_API_TOKEN
```

`FLY_API_TOKEN` must never be exposed to browsers.

---

## TURN Server Requirements

Implement or integrate support for:

```text
STUN binding
TURN allocate
TURN refresh
TURN channel bind
TURN send/data indications
Long-term credential auth
Ephemeral HMAC credentials
UDP relay
Optional TCP relay
Optional TLS relay
Per-user allocation limits
Per-IP allocation limits
Bandwidth accounting
Rate limiting
Prometheus metrics
Structured logs
Health checks
```

Use short-lived TURN credentials.

Credential format:

```text
username = unix_expiry_timestamp:user_id
password = base64(hmac_sha1(shared_secret, username))
```

---

## HTTP Control API

Expose a private/admin API.

Required endpoints:

```http
POST /v1/credentials
GET  /healthz
GET  /readyz
GET  /metrics
GET  /v1/regions
POST /v1/deploy
POST /v1/relay-sessions
```

Security requirements:

```text
/v1/credentials may be public only if properly authenticated by the app’s users.
/v1/deploy must never be public without strong admin authentication.
/metrics should be protected or bound privately unless intentionally public.
```

---

## Suggested `fly.toml`

```toml
app = "turnfly-demo"
primary_region = "iad"

[env]
  TURN_PORT = "3478"
  TURN_REALM = "turnfly.local"

[[services]]
  protocol = "udp"
  internal_port = 3478

  [[services.ports]]
    port = 3478

[[services]]
  protocol = "tcp"
  internal_port = 3478

  [[services.ports]]
    port = 3478

[[services]]
  protocol = "tcp"
  internal_port = 8080

  [[services.ports]]
    port = 80

  [services.concurrency]
    type = "connections"
    hard_limit = 1000
    soft_limit = 800
```

Validate this against real Fly behavior before assuming production suitability.

---

## Relay Mode Design

Relay mode should use QUIC first.

Recommended relay transport:

```text
quic-go with QUIC datagrams for media packets
QUIC streams for control messages
TLS-based authentication between relay nodes
```

Do not use TCP for media relaying unless there is no alternative.

Possible packet frame:

```text
session_id: 128-bit
direction: client_to_peer | peer_to_client
flow_id: allocation/channel
timestamp
payload_len
payload
```

Relay mode should have a separate benchmark-driven success gate.

Do not assume relay mode improves performance.

---

## Relay Mode Warning

The browser ICE stack does not understand your private relay path.

Therefore, two possible designs exist.

---

### Preferred First Design: Multi-Region TURN

Give clients several regional TURN servers and let ICE choose.

This is standards-compatible and much simpler.

---

### Experimental Design: TURN-Aware Relay Proxy

The entry TURN server terminates the TURN session and forwards relayed packets through the private tunnel to the exit node.

This requires custom allocation and routing behavior. It is not simply a Pion TURN configuration flag.

Build this only after the simpler multi-region mode has been measured.

---

## Observability

Expose Prometheus metrics:

```text
turn_allocations_active
turn_allocations_total
turn_bytes_in_total
turn_bytes_out_total
turn_packets_dropped_total
turn_auth_failures_total
turn_relay_rtt_ms
relay_quic_rtt_ms
relay_quic_loss_estimate
relay_tunnel_bytes_total
region_candidate_wins_total
```

Structured logs should include:

```text
session_id
allocation_id
region
peer_region
client_ip_hash
protocol
bytes_in
bytes_out
duration
close_reason
```

---

## Probe Tool

Build a synthetic test command:

```bash
turnfly probe \
  --from iad \
  --to sjc \
  --duration 60s \
  --packet-size 1200 \
  --bitrate 5mbps
```

The probe should compare:

```text
single-region TURN
multi-region TURN
relay-pair mode
direct UDP path where possible
QUIC tunnel behavior
packet loss
jitter
RTT
throughput
```

---

## Security Requirements

TURN servers are abuse-prone.

Minimum protections:

```text
No anonymous TURN
Short-lived credentials
Per-account quotas
Per-IP quotas
Bandwidth caps
Rate limits
Destination allow/deny policy
Admin API authentication
Separate deploy token from runtime token
Audit logs for deploy actions
Cost guardrails
Optional disablement of TCP relay
```

---

## Development Phases

### Phase 1: Plain Fly TURN

Deliver:

```text
Go TURN server using Pion
Dockerfile
fly.toml
Dedicated IPv4 deployment instructions
Health endpoint
Metrics endpoint
Credential endpoint
Browser WebRTC test page
```

Success criteria:

```text
WebRTC relay candidate works through Fly
No anonymous access
Metrics show allocations and bytes
Works in at least two Fly regions
```

---

### Phase 2: Self-Deployer

Deliver:

```text
Fly Machines API client
turnfly deploy command
App creation
Machine creation
Secret management
Region support
Idempotent deploys
Destroy command
Health polling
```

Success criteria:

```text
Fresh Fly app can be created from the CLI
Repeated deploys converge safely
Rollback or destroy works
```

---

### Phase 3: Multi-Region Independent TURN

Deliver:

```text
Deploy same TURN service to multiple regions
Generate ICE config containing multiple regional TURN URLs
Collect client-side ICE result reports
Expose region choice metrics
```

Success criteria:

```text
Clients usually choose useful nearby regions
Multi-region mode beats single-region baseline
No custom relay complexity yet
```

---

### Phase 4: Private Relay Experiment

Deliver:

```text
QUIC tunnel over Fly private networking
Relay-pair deploy mode
Relay-pair benchmark mode
Session framing
Relay metrics
```

Success criteria:

```text
Relay mode demonstrably improves throughput, packet loss, or stability enough to justify extra complexity
```

If it does not beat multi-region independent TURN, keep relay mode experimental only.

---

### Phase 5: Production Hardening

Deliver:

```text
Autoscaling
Quota enforcement
Abuse detection
Cost limits
Admin dashboard
Alerting
Formal load tests
Failure-mode tests
Documentation
```

---

## Recommended Implementation Order

Implement in this exact order:

```text
1. Pion TURN on Fly, single region.
2. Credential endpoint.
3. Health and metrics.
4. Docker image and fly.toml.
5. Self-deployment through Fly Machines API.
6. Multi-region independent TURN.
7. Client ICE config generation.
8. Measurement and probe tooling.
9. QUIC relay-pair experiment.
10. Production hardening.
```

---

## Design Bias

Favor:

```text
Standards-compatible WebRTC behavior
Simple regional TURN servers
Fly-native deployment
Short-lived credentials
Measurable performance improvements
Explicit cost controls
Security-first defaults
```

Avoid:

```text
Anonymous TURN access
Premature custom relay logic
Assuming private relay is faster
Huge public UDP port ranges without testing Fly support
Exposing deployment APIs publicly
Building custom crypto before trying QUIC
```

---

## Expected Output

Produce:

```text
1. A detailed technical design.
2. A Go package layout.
3. Initial code skeleton.
4. Fly deployment configuration.
5. Self-deploy implementation plan using Fly Machines API.
6. TURN credential implementation.
7. Metrics and health endpoints.
8. Relay-mode design, but clearly marked experimental.
9. Testing plan.
10. Security checklist.
```

---

## Final Recommendation

The likely best production architecture is:

```text
multi-region independent TURN on Fly.io
+
ephemeral credentials
+
self-deployment via Fly Machines API
+
measurement-driven region selection
```

The relay-pair QUIC mode should be built only as an experimental extension after the basic service is working and measurable.
