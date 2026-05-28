# AGENTS.md

## Purpose

This repository implements `turnfly`, a Go service that runs TURN servers on Fly.io, can deploy itself using the Fly Machines API, and may later support experimental relay-pair mode over an optimized private transport.

Agents working in this repository must follow the implementation brief in the companion planning document. When in doubt, prefer the simpler, standards-compatible, measurable design over speculative optimization.

Primary architecture bias:

```text
multi-region independent TURN on Fly.io
+
ephemeral credentials
+
self-deployment via Fly Machines API
+
measurement-driven region selection
```

Experimental relay-pair mode must not be treated as the default architecture.

READ THE FILE SCOPE.md for a full list of features.

---

## Operating Principles

1. Prefer correctness, security, observability, and deployability over cleverness.
2. Keep the codebase idiomatic Go.
3. Make small, reviewable changes.
4. Add or update tests with every meaningful feature.
5. Run formatters, linters, and tests before committing.
6. Do not leave the repository in a broken state.
7. Do not commit secrets, tokens, credentials, private keys, or generated local config.
8. Commit and push once a feature or fix is complete and validated.
9. If a task is too large, split it into clearly named incremental commits.
10. Document assumptions, especially around Fly.io networking and TURN behavior.

---

## Required Tooling

Use standard Go tooling wherever possible.

Required commands before committing Go changes:

```bash
gofmt -w .
go test ./...
go vet ./...
```

If the repository includes additional tooling, use it as well:

```bash
golangci-lint run
staticcheck ./...
go mod tidy
```

Run only the tools that are installed or configured for the repo. If a tool is missing, do not block forever; note that it was unavailable.

If `Makefile` targets exist, prefer them:

```bash
make fmt
make lint
make test
make vet
make check
```

If both direct commands and `make` targets exist, use the repo’s `make` targets as the source of truth.

---

## Go Code Standards

Write idiomatic Go.

Requirements:

```text
Use gofmt.
Use go vet clean code.
Keep packages cohesive and small.
Avoid global mutable state where practical.
Return errors instead of panicking.
Wrap errors with useful context.
Keep public APIs documented.
Prefer context-aware functions for network, deploy, and long-running operations.
Prefer structured logging over ad-hoc printf debugging.
Prefer table-driven tests for pure logic.
Prefer interfaces at package boundaries, not everywhere.
```

Error handling should usually look like:

```go
if err != nil {
	return fmt.Errorf("create fly machine: %w", err)
}
```

Avoid:

```text
Ignoring errors
Panics in server paths
Hardcoded credentials
Hidden network side effects in constructors
Large untested functions
Circular package dependencies
Overly clever abstractions
```

---

## Repository Layout

Prefer this layout unless the repo already has a better established structure:

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

Package responsibilities:

```text
/cmd/turnfly
  CLI entrypoint and command wiring only.

/internal/turnserver
  TURN server integration, allocation policy, auth hooks, relay settings.

/internal/relay
  Experimental server-to-server relay transport and packet framing.

/internal/flydeploy
  Fly Machines API client, app creation, machine creation, IP allocation, secrets, deploy orchestration.

/internal/config
  Config structs, env parsing, file parsing, validation.

/internal/auth
  TURN credential generation, admin API auth, token validation.

/internal/metrics
  Prometheus collectors and metric registration.

/internal/health
  Health and readiness checks.

/internal/probe
  Synthetic measurement tooling.

/internal/controlapi
  HTTP API handlers and middleware.
```

Do not put business logic directly in `main.go`.

---

## Feature Workflow

For each feature:

1. Read the relevant part of the implementation brief.
2. Inspect the current repository state.
3. Identify the smallest useful change.
4. Implement the change.
5. Add or update tests.
6. Run formatting.
7. Run tests and checks.
8. Update docs or examples if behavior changed.
9. Commit with a clear message.
10. Push the branch.

Suggested feature completion checklist:

```text
[ ] Code implemented
[ ] Unit tests added or updated
[ ] Integration tests added where practical
[ ] gofmt run
[ ] go test ./... passes
[ ] go vet ./... passes
[ ] go mod tidy run if dependencies changed
[ ] Documentation updated if user-facing behavior changed
[ ] No secrets committed
[ ] Commit created
[ ] Commit pushed
```

---

## Commit and Push Policy

When a feature, bug fix, refactor, or documentation update is complete and validated, commit and push it.

Before committing:

```bash
git status
git diff
gofmt -w .
go test ./...
go vet ./...
go mod tidy
git status
```

Use clear commit messages:

```text
feat: add ephemeral TURN credential generation
feat: add Fly Machines app creation client
fix: handle missing TURN realm config
test: add relay packet framing tests
docs: add Fly deployment notes
refactor: split CLI command construction
```

After committing:

```bash
git push
```

If pushing fails because no upstream is configured, use:

```bash
git push -u origin HEAD
```

Do not force push unless explicitly instructed.

Do not rewrite published history unless explicitly instructed.

---

## Branching

If the current branch is appropriate, work on it.

If a new branch is needed, use descriptive branch names:

```text
feat/turn-credentials
feat/fly-machines-deploy
feat/multi-region-ice-config
feat/relay-quic-prototype
fix/fly-udp-bind-address
docs/agents
```

Avoid mixing unrelated changes on one branch.

---

## Testing Requirements

Every meaningful code change should include tests.

Minimum expectations:

```text
Pure logic gets unit tests.
Auth and credential generation get deterministic tests.
Config validation gets table-driven tests.
Fly API clients get tests using mocked HTTP servers.
HTTP handlers get httptest-based tests.
Relay packet framing gets encode/decode round-trip tests.
Metrics registration should be tested enough to avoid duplicate registration panics.
```

Prefer table-driven tests:

```go
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		// cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

Tests must not require real Fly.io credentials by default.

Any tests that require external services must be clearly marked as integration tests and disabled unless explicitly requested.

Suggested pattern:

```bash
go test ./...
TURNFLY_INTEGRATION=1 go test ./... -run Integration
```

---

## Fly.io Deployment Safety

Self-deployment code can create real cloud resources and spend real money.

Requirements:

```text
Never run real deploys in tests by default.
Never require FLY_API_TOKEN for unit tests.
Use dry-run mode where practical.
Make destructive operations explicit.
Require app name and org to be clear.
Log planned actions before making changes.
Keep deploy operations idempotent.
Prefer convergence over blind recreation.
Protect /v1/deploy behind strong admin authentication.
```

Deployment code should support a dry run:

```bash
turnfly deploy --dry-run
```

Destroy operations should be explicit:

```bash
turnfly destroy --app my-turnfly
```

If interactive confirmation is implemented, also provide a CI-safe non-interactive flag:

```bash
turnfly destroy --app my-turnfly --yes
```

---

## Security Requirements

Never commit:

```text
FLY_API_TOKEN
TURN_SHARED_SECRET
ADMIN_API_TOKEN
Private keys
TLS private keys
Generated credentials
.env files
Local machine config
```

Use `.gitignore` for local and generated sensitive files.

TURN security defaults:

```text
No anonymous TURN access.
Short-lived credentials only.
Per-user allocation limits.
Per-IP allocation limits.
Bandwidth caps.
Rate limits.
Admin API authentication.
Metrics protected unless intentionally public.
Deploy API never public without strong authentication.
```

Credential generation should be deterministic, testable, and isolated in `/internal/auth`.

Use constant-time comparison where comparing secrets or tokens.

---

## Configuration Rules

Configuration should come from:

```text
CLI flags
Environment variables
Optional config file
```

Precedence should be documented and consistent.

Required runtime config should be validated at startup.

Bad config should fail fast with a useful error.

Example required config:

```text
TURN_REALM
TURN_SHARED_SECRET
ADMIN_API_TOKEN
```

Example optional config:

```text
TURN_PORT
HTTP_PORT
METRICS_ADDR
FLY_APP_NAME
FLY_ORG
RELAY_MODE
RELAY_PEERS
```

Do not silently guess security-sensitive configuration.

---

## Logging and Metrics

Use structured logging.

Logs should include useful context:

```text
component
region
machine_id
allocation_id
session_id
relay_peer
duration
error
```

Do not log raw secrets, TURN passwords, API tokens, private keys, or full client IPs unless explicitly justified. Prefer hashed or truncated identifiers.

Prometheus metrics should be exposed for:

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

Avoid high-cardinality labels such as raw user IDs, full IP addresses, allocation IDs, or session IDs.

---

## HTTP API Standards

Handlers should be small and testable.

Use middleware for:

```text
request IDs
logging
panic recovery
admin authentication
metrics
timeouts
```

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

Admin endpoints must require authentication.

Return JSON errors with useful messages, but do not leak secrets or internal stack traces.

---

## TURN Server Implementation Guidance

Use Pion TURN unless there is a strong reason not to.

Start with the simplest working TURN server on Fly.io before adding custom behavior.

Implementation order:

```text
1. STUN/TURN server boot.
2. Ephemeral credential validation.
3. UDP relay support.
4. Health and metrics.
5. TCP fallback only if required.
6. TLS only after basic behavior is validated.
7. Multi-region ICE config generation.
8. Relay-pair experiment.
```

Avoid implementing a custom TURN protocol stack unless Pion cannot support a required behavior.

---

## Relay Mode Guidance

Relay-pair mode is experimental.

Do not let relay mode contaminate the simple TURN server architecture.

Preferred relay transport:

```text
QUIC datagrams for media packets
QUIC streams for control messages
TLS authentication between relay nodes
```

Relay code should be isolated in `/internal/relay`.

Relay tests should cover:

```text
Packet frame encode/decode
Session mapping
Peer authentication
Timeout behavior
Backpressure behavior
Dropped packet accounting
```

Relay mode must be benchmarked against ordinary single-region and multi-region TURN before being considered successful.

If relay mode does not clearly improve real measurements, keep it experimental.

---

## Fly.io Networking Notes

Fly.io networking behavior must be validated empirically.

Important assumptions to test early:

```text
UDP service binding behavior
Dedicated IPv4 requirements
Port mapping behavior
3478 UDP availability
3478 TCP availability
Private 6PN connectivity between Machines
Regional behavior of public services
Metrics and logs under UDP traffic
```

Do not assume large public UDP relay port ranges work like traditional coturn deployments.

Document all Fly-specific discoveries in the repo.

---

## Documentation Requirements

Update documentation when adding or changing:

```text
CLI commands
Environment variables
Deployment behavior
Security behavior
Fly.io requirements
TURN credential behavior
Relay mode behavior
Testing commands
```

Suggested docs:

```text
README.md
docs/fly-deployment.md
docs/security.md
docs/relay-mode.md
docs/testing.md
```

Keep examples copy-pasteable.

Use placeholders for secrets:

```text
export FLY_API_TOKEN="..."
export TURN_SHARED_SECRET="..."
```

Never include real tokens.

---

## Dependency Policy

Before adding a dependency:

1. Prefer the Go standard library if reasonable.
2. Prefer established, maintained libraries.
3. Avoid large frameworks unless they clearly reduce risk.
4. Keep dependency scope narrow.
5. Run `go mod tidy`.
6. Check that tests still pass.

Reasonable dependencies for this project include:

```text
github.com/pion/turn/v2
github.com/pion/stun
github.com/quic-go/quic-go
github.com/prometheus/client_golang
github.com/spf13/cobra
github.com/spf13/viper
golang.org/x/sync/errgroup
```

Avoid adding custom crypto dependencies unless relay mode proves QUIC is insufficient.

---

## Generated Files

Generated files must be clearly marked.

If code generation is introduced, document:

```text
Generator command
Input files
Output files
When to regenerate
How to verify generated output
```

Generated files should be deterministic.

Do not hand-edit generated files unless the repo explicitly allows it.

---

## CI Expectations

If CI is present, keep it passing.

If CI is not present, prefer adding a simple GitHub Actions workflow:

```yaml
name: Go

on:
  push:
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: gofmt -w .
      - run: git diff --exit-code
      - run: go test ./...
      - run: go vet ./...
```

If adding CI, ensure it does not require real Fly credentials.

---

## Review Checklist

Before considering work complete, verify:

```text
[ ] The change matches the implementation brief.
[ ] The change is small enough to review.
[ ] Code is formatted.
[ ] Tests pass.
[ ] Vet passes.
[ ] Dependencies are tidy.
[ ] Security-sensitive behavior is tested.
[ ] No secrets are committed.
[ ] Documentation is updated.
[ ] The feature is committed.
[ ] The commit is pushed.
```

---

## Definition of Done

A feature is complete only when:

```text
1. It is implemented.
2. It has tests.
3. It is formatted with gofmt.
4. It passes go test ./...
5. It passes go vet ./...
6. Documentation is updated if needed.
7. No secrets or local-only files are included.
8. The change is committed.
9. The commit is pushed to the remote repository.
```

---

## Agent Behavior

Agents should act like careful senior Go engineers.

When working:

```text
Inspect before editing.
Prefer minimal changes.
Keep implementation and tests together.
Use clear names.
Avoid speculative abstractions.
Avoid unrelated cleanup.
Do not hide failures.
Commit and push completed work.
```

When blocked:

```text
Record what was attempted.
Record the exact failing command.
Explain the likely cause.
Suggest the smallest next action.
Do not fake successful tests or deployments.
```

When external credentials or cloud access are unavailable:

```text
Use mocks.
Use dry-run mode.
Add integration-test hooks.
Document what still requires real Fly.io validation.
```

---

## Non-Goals

Do not prioritize:

```text
Custom TURN stack implementation
Custom cryptography
Relay-pair mode before basic TURN works
Complex autoscaling before measurement exists
Huge public UDP relay-port assumptions
Anonymous public TURN
Unbounded bandwidth usage
Unreviewable mega-commits
```

---

## Final Reminder

The best initial product is a secure, observable, self-deploying, multi-region TURN service on Fly.io.

The QUIC relay-pair mode is an experiment. Build it only after the normal TURN service, self-deployer, metrics, and probe tooling exist.
