# Build Write-Up: Job Scheduler with AI Assistance

---

## 1. What did you ask the AI to do, and what did you write or decide yourself?

**What I brought to the table:**

The core product requirement was mine — a cron-style job scheduler with run history, CRUD endpoints, and clean extensible design. I also made every significant product decision before a line of code was written:

- **Go** as the language, **MySQL** as storage, **Docker + Colima** as the runtime
- **HTTP webhooks** as the job execution model (chose Option A when the AI laid out three options)
- **Concurrent execution** — always fire, even if a previous run is in-flight
- **In-process notification** for job changes (chose Option A, but explicitly asked how it could scale to Redis — which led to the `JobChangeNotifier` interface design)
- **Retry with fixed delay** over no-retry or exponential backoff
- **`Authorization: Bearer`** instead of `X-API-Key` — I caught that the AI's first suggestion was non-standard and pushed back
- **chi** as the HTTP framework
- **Min-heap** as the scheduling approach — I didn't just accept the AI's recommendation (per-job goroutines); I asked what the industry standard actually was, and switched to Approach C after understanding it
- **Plan** - After this I curated the fully executable plan, which AI could pick up and implement.

**What I delegated to the AI:**

Once I made the plan and the design decisions were locked, I let the AI handle implementation in full:
- Class design, interface definitions, and package structure
- All Go code — domain models, repositories, executor, scheduler engine, API handlers, middleware, `cmd/main.go`
- Database schema and migrations
- Dockerfile, docker-compose, Makefile
- Unit tests for scheduler, executor, and handlers
- GitHub repo creation and commit history
- Debugging the Colima port-forwarding issue during live verification
- This README and write-up

---

## 2. Where did you override, correct, or threw away the AI's output and why?

**Auth header design:**
The AI suggested `X-API-Key` as a custom header. I pushed back — `Authorization: Bearer` is the HTTP standard (RFC 7235) and works with all API clients without any special configuration. The AI agreed, updated the design, and correctly noted it makes the `Authenticator` interface more future-proof for JWT.

**Scheduler approach:**
The AI recommended Approach B (per-job goroutines) citing "clarity" and "maps 1:1 to the data model." I didn't take that at face value and asked what production schedulers actually do. The honest answer was Approach C (min-heap), which is how `robfig/cron` works internally, and how Celery beat, Quartz, and similar systems are built. I switched. The AI's first recommendation was optimising for explainability over correctness.

**Scope of the session:**
The AI would have kept asking clarifying questions. I cut it shorter than it wanted. Once I had enough signal on the design, I moved to implementation rather than answering every optional question about pagination limits and auth edge cases. The goal was a working, learning-grade system — not a production-hardened one.

---

## 3. The two or three biggest trade-offs I made, and the alternatives I considered

**Trade-off 1: In-process scheduler vs Redis-backed distributed scheduler**

I chose the in-process min-heap. This means a single binary owns the schedule — there is no horizontal scaling and no automatic failover. If the process dies, no jobs run until it restarts.

The alternative is a Redis sorted set as the job queue, where multiple worker instances compete to claim jobs atomically using a Lua script. That's the production-correct approach and we designed the `JobChangeNotifier` interface specifically to make this swap possible. But implementing Redis adds operational complexity (another dependency, distributed locking, watchdog loops) that was out of scope for a learning project. The interface design means this trade-off is reversible without touching any handler or repository code.

**Trade-off 2: Concurrent execution (always fire) vs skip-if-running**

I chose to always fire a job even if the previous run is still in-flight. This is simpler and means a slow webhook never blocks the schedule. The downside is that a consistently slow job can pile up goroutines and database connections. A `skip` policy (check if a run with `status=running` exists before firing) would prevent this at the cost of slightly more complex logic and a DB read on every trigger. I decided concurrent was the right default for a learning system since it matches what most job queue systems do, and the trade-off becomes visible quickly when you look at the runs table.

**Trade-off 3: Static API key vs JWT**

The `Authenticator` interface accepts any implementation, but the only one built is `StaticKeyAuthenticator` — a single pre-shared token in an env var. This is fine for internal/learning use but isn't usable in a multi-tenant or externally distributed context. JWT would give per-issuer tokens, expiry, and scoped permissions. The `Authenticator` interface deliberately has a single `Authenticate(token string) error` method so swapping in a JWT verifier is a one-file change with no handler touches.

---

## 4. What's missing, or what you'd do with another day?

**Reliability gaps (found during live verification):**

- **Panic recovery in fire goroutines** — an unhandled panic in a webhook execution kills the entire process. A `defer recover()` wrapper around each `fire()` call would contain the blast radius to that single run.
- **Graceful shutdown** — `Stop()` closes the scheduler loop but goroutines already executing a webhook continue running. On a `SIGTERM`, some in-flight runs never get their `UpdateStatus` call. A `sync.WaitGroup` tracking in-flight runs, waited on at shutdown, would fix this.
- **Job execution timeout** — `fire()` passes `context.Background()` which has no deadline. A webhook that hangs holds a goroutine indefinitely. `context.WithTimeout` set to a configurable max duration would bound this.
- **TLS certificate verification failing in Alpine** — during live testing, HTTPS webhook calls failed with `x509: certificate signed by unknown authority` despite `ca-certificates` being installed in the Docker image. Needs investigation — likely a `SSL_CERT_FILE` env var or explicit cert bundle path for the Go TLS stack.

**Observability:**
There is no metrics endpoint, no structured logging, and no tracing. In production you'd want at minimum: a Prometheus `/metrics` endpoint with counters for `jobs_executed_total{status}` and a histogram for `job_execution_duration_seconds`, plus structured JSON logging via `slog` with a `run_id` field on every log line so you can trace a specific run end-to-end.

**Operational:**
- No `/healthz` endpoint — a Kubernetes liveness probe or load balancer has no signal on whether the app is healthy
- No versioned migration tool — the current approach is two raw SQL files applied by `make migrate`. `golang-migrate` or `goose` would give ordered, versioned, rollback-capable migrations
- No pagination on list endpoints — `GET /api/v1/jobs` returns all jobs; with thousands of jobs this becomes a problem

**Scale:**
With the current in-process design, running a second instance fires every job twice. Distributing the scheduler to Redis sorted sets with atomic Lua claiming (designed, documented, not built) is the next concrete step if this were going to production.
