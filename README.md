# Job Scheduler

A cron-style job scheduler in Go. Register HTTP webhook jobs on a schedule, run them at the right time, and keep a history of each run.

## Architecture

- **Scheduler**: min-heap (`container/heap`) with a single timing goroutine. One goroutine owns the loop; jobs fire as concurrent goroutines.
- **Cron parsing**: `robfig/cron/v3` for expression parsing only (`schedule.Next(t)`).
- **Storage**: MySQL — source of truth for jobs and run history.
- **API**: chi router with Bearer token auth.
- **Extensibility**: all major components are interfaces (`Scheduler`, `JobExecutor`, `JobRepository`, `RunRepository`, `JobChangeNotifier`, `Authenticator`). Swap implementations without touching callers.

## Quickstart

```bash
# Start MySQL + app
make docker-up

# Wait for containers to be healthy, then apply migration
make migrate

# Create a job (fires every minute)
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Authorization: Bearer dev-secret" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "health-check",
    "cron_expr": "*/1 * * * *",
    "url": "https://httpbin.org/post",
    "http_method": "POST",
    "payload": "{\"hello\": \"world\"}"
  }'

# List run history after ~60s
curl http://localhost:8080/api/v1/runs \
  -H "Authorization: Bearer dev-secret"
```

## API Reference

All endpoints require `Authorization: Bearer <API_KEY>`.

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/jobs` | Create a job |
| GET | `/api/v1/jobs` | List all jobs |
| GET | `/api/v1/jobs/:id` | Get a job |
| PUT | `/api/v1/jobs/:id` | Update a job |
| DELETE | `/api/v1/jobs/:id` | Soft-delete a job |
| PATCH | `/api/v1/jobs/:id/pause` | Pause a job |
| PATCH | `/api/v1/jobs/:id/resume` | Resume a paused job |
| GET | `/api/v1/jobs/:id/runs` | List runs for a job |
| GET | `/api/v1/runs` | List recent runs (all jobs) |

### Job payload

```json
{
  "name": "string (required)",
  "cron_expr": "* * * * * (required, 5-field standard cron)",
  "url": "https://... (required)",
  "http_method": "POST (default)",
  "payload": "{} (optional JSON string)",
  "headers": {"X-Custom": "value"},
  "max_retries": 3,
  "retry_delay_secs": 5
}
```

## How next-run time is computed

`robfig/cron/v3` parses the cron expression into a `cron.Schedule` interface with a single method:

```go
Next(t time.Time) time.Time
```

The scheduler calls `schedule.Next(now)` to compute the first trigger, inserts the entry into a min-heap sorted by `next`. The scheduling loop pops the soonest entry, fires the job in a goroutine, calls `schedule.Next(now)` again, and re-inserts.

## Extending to Redis

Replace `InProcessNotifier` with a `RedisNotifier` that publishes job-change events to a Redis channel. The scheduler subscribes on startup. No handler code changes — both implement `JobChangeNotifier`.

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `DB_DSN` | — | MySQL DSN (required) |
| `SERVER_PORT` | `8080` | HTTP listen port |
| `API_KEY` | — | Bearer token (required) |
| `HTTP_TIMEOUT_SEC` | `30` | Per-request HTTP timeout |

## Development

```bash
# Run tests
make test

# Run with race detector
make test-race

# Run locally (needs .env)
cp .env.example .env
make run
```
