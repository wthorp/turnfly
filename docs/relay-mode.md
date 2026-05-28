# Relay-Pair Mode (Experimental)

> **Status: Experimental** — See SCOPE.md Phase 4 for success criteria.
> Relay mode should only be considered successful if benchmarking shows
> better throughput, lower loss, or better stability than ordinary
> multi-region independent TURN.

## Architecture

Two TURN servers in different Fly regions communicate over a private QUIC tunnel:

```
Client A
  ↓
TURN server near Client A (iad)
  ↓ QUIC tunnel over Fly private networking
TURN server near Client B (syd)
  ↓
Client B
```

## Packet Frame Format

Binary framing for relayed media packets:

```
Offset  Size  Field
0       4     Magic number (0x5455524E = "TURN")
4       16    Session ID (128-bit, crypto/rand)
20      2     Flow ID (uint16)
22      1     Direction (0=client_to_peer, 1=peer_to_client)
23      8     Timestamp (unix microseconds)
31      2     Payload length (uint16, max 1200)
33      N     Payload
```

Total header: 33 bytes. Max payload: 1200 bytes (path MTU safe).

## QUIC Transport

- **Media packets**: QUIC datagrams (unreliable, low latency)
- **Control messages**: QUIC streams (reliable, ordered)
- **TLS**: Self-signed or CA certificates
- **Port**: Default 4443

## Running Relay Mode

### Server (listener)

```bash
turnfly serve-relay \
  --server \
  --listen :4443 \
  --peer <client-addr>
```

### Client (dialer)

```bash
turnfly serve-relay \
  --peer <server-addr>:4443
```

### With Custom Certificates

```bash
turnfly serve-relay \
  --server \
  --cert /etc/turnfly/cert.pem \
  --key /etc/turnfly/key.pem \
  --peer <client-addr>
```

## Session Management

- Sessions auto-created on first packet from peer
- Idle timeout: 5 minutes (configurable)
- GC runs every 30 seconds
- Per-session stats: packets in/out, bytes in/out, drops

## Metrics

| Metric | Description |
|--------|-------------|
| `relay_quic_rtt_ms` | Current estimated tunnel RTT |
| `relay_quic_loss_estimate` | Estimated packet loss (0.0–1.0) |
| `relay_tunnel_bytes_total` | Total bytes through tunnel |

## Benchmarking

Compare relay mode against multi-region TURN:

```bash
# Multi-region baseline
turnfly probe --from iad --to sjc --count 100 --packet-size 1200

# Relay mode
turnfly serve-relay --server &
turnfly serve-relay --peer localhost:4443 &
```

Relay mode must demonstrably beat the baseline in at least one dimension:
- Lower RTT
- Higher throughput
- Lower packet loss
- Better stability under load

## Limitations

1. **Not standards-compatible** — browsers don't understand the private relay path
2. **Point-to-point only** — one pair of TURN servers per tunnel
3. **Requires TLS certificates** — self-signed only for Fly private networking
4. **Experimental** — not recommended for production without thorough benchmarking

## Failure Modes

| Scenario | Behavior |
|----------|----------|
| Tunnel disconnect | Sessions expire after timeout, packets dropped |
| Peer unreachable | Dial timeout after 30s |
| Certificate mismatch | TLS handshake failure |
| Payload too large | Frame encoding error, packet dropped |
| Session GC | Idle sessions removed after 5 minutes |

## When to Use

Consider relay mode when:
- Both peers are consistently far from each other
- Multi-region TURN shows >5% packet loss between regions
- Throughput requirements exceed single-region capabilities

Stick with multi-region independent TURN when:
- ICE candidate selection works well enough
- Users are geographically distributed
- Simplicity and standards compliance matter
