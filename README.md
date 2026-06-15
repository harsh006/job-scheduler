# Job Scheduler

A cron-style job scheduler written in Go. Register HTTP webhook jobs on a schedule, execute them at the right time, and keep a full history of every run.

---

## Quickstart

```bash
# 1. Install Docker, Colima, Go, and MySQL client
make install-prereqs

# 2. Start everything (Colima → MySQL → app → migrations)
make run
```

That's it. The API is now live at `http://localhost:8080/api/v1`.

---

## Authentication

Every request must include a Bearer token in the `Authorization` header.

```bash
-H "Authorization: Bearer dev-secret"
```

Missing or wrong token returns `401 Unauthorized`.

---

## API Reference

### Variables used in examples below

```bash
BASE="http://localhost:8080/api/v1"
TOKEN="Authorization: Bearer dev-secret"
```

---

### Create a job

`POST /api/v1/jobs`

**Required fields:** `name`, `cron_expr`, `url`

**All fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | — | Human-readable label |
| `cron_expr` | string | — | 5-field standard cron (`MIN HOUR DOM MON DOW`) |
| `url` | string | — | Webhook URL to call |
| `http_method` | string | `POST` | HTTP verb |
| `payload` | string | — | Raw JSON body sent to the webhook |
| `headers` | object | — | Extra HTTP headers sent with the request |
| `max_retries` | int | `3` | Max attempts before marking run as failed |
| `retry_delay_secs` | int | `5` | Seconds to wait between retries |

```bash
curl -s -X POST "$BASE/jobs" \
  -H "$TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "health-check",
    "cron_expr": "*/5 * * * *",
    "url": "https://your-service.com/webhook",
    "http_method": "POST",
    "payload": "{\"event\": \"ping\"}",
    "headers": {"X-Source": "scheduler"},
    "max_retries": 3,
    "retry_delay_secs": 5
  }'
```

**Response `201`**
```json
{
  "ID": "a91ba920-deac-4601-a1da-c14a46c1415d",
  "Name": "health-check",
  "CronExpr": "*/5 * * * *",
  "URL": "https://your-service.com/webhook",
  "HTTPMethod": "POST",
  "Payload": "{\"event\": \"ping\"}",
  "Headers": {"X-Source": "scheduler"},
  "Status": "active",
  "MaxRetries": 3,
  "RetryDelaySecs": 5,
  "NextRunAt": "2026-06-15T10:25:00Z",
  "CreatedAt": "2026-06-15T10:20:43Z",
  "UpdatedAt": "2026-06-15T10:20:43Z"
}
```

---

### List all jobs

`GET /api/v1/jobs`

Returns all non-deleted jobs (active + paused).

```bash
curl -s "$BASE/jobs" -H "$TOKEN"
```

**Response `200`**
```json
[
  {
    "ID": "a91ba920-deac-4601-a1da-c14a46c1415d",
    "Name": "health-check",
    "Status": "active",
    "CronExpr": "*/5 * * * *",
    "NextRunAt": "2026-06-15T10:25:00Z",
    ...
  }
]
```

---

### Get a job

`GET /api/v1/jobs/:id`

```bash
JOB_ID="a91ba920-deac-4601-a1da-c14a46c1415d"

curl -s "$BASE/jobs/$JOB_ID" -H "$TOKEN"
```

**Response `200`** — full job object (same shape as create response)

**Response `404`**
```json
{"error": "job not found"}
```

---

### Update a job

`PUT /api/v1/jobs/:id`

All fields are optional. Only provided fields are updated.
If `cron_expr` changes, `next_run_at` is recomputed and the scheduler updates immediately.

```bash
curl -s -X PUT "$BASE/jobs/$JOB_ID" \
  -H "$TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "health-check-v2",
    "cron_expr": "0 * * * *",
    "max_retries": 5
  }'
```

**Response `200`** — updated job object

---

### Delete a job

`DELETE /api/v1/jobs/:id`

Soft-delete — the job stops executing immediately. Run history is preserved.
The record remains queryable via `GET /api/v1/jobs/:id` with `"Status": "deleted"`.

```bash
curl -s -X DELETE "$BASE/jobs/$JOB_ID" -H "$TOKEN" -w "HTTP %{http_code}"
```

**Response `204`** — no body

---

### Pause a job

`PATCH /api/v1/jobs/:id/pause`

Stops the job from firing on its next scheduled trigger. Run history is preserved.

```bash
curl -s -X PATCH "$BASE/jobs/$JOB_ID/pause" -H "$TOKEN"
```

**Response `200`** — job object with `"Status": "paused"`

---

### Resume a job

`PATCH /api/v1/jobs/:id/resume`

Re-activates a paused job. It will fire on the next scheduled trigger.

```bash
curl -s -X PATCH "$BASE/jobs/$JOB_ID/resume" -H "$TOKEN"
```

**Response `200`** — job object with `"Status": "active"`

---

### List runs for a job

`GET /api/v1/jobs/:id/runs`

Returns execution history for a specific job, newest first.

| Query param | Default | Description |
|-------------|---------|-------------|
| `limit` | `20` | Max records to return |

```bash
curl -s "$BASE/jobs/$JOB_ID/runs?limit=10" -H "$TOKEN"
```

**Response `200`**
```json
[
  {
    "ID": "e1846db8-510a-4e28-a9cd-9eb5c766eeac",
    "JobID": "a91ba920-deac-4601-a1da-c14a46c1415d",
    "Status": "succeeded",
    "Attempt": 1,
    "StartedAt": "2026-06-15T10:25:00.007Z",
    "FinishedAt": "2026-06-15T10:25:00.682Z",
    "DurationMs": 675,
    "ResponseCode": 200,
    "ErrorMessage": null
  }
]
```

**Run status values:**

| Status | Meaning |
|--------|---------|
| `running` | Currently in-flight |
| `succeeded` | Webhook returned 2xx |
| `failed` | All retries exhausted, non-2xx or connection error |
| `missed` | Process was down when this run was due |

---

### List recent runs (all jobs)

`GET /api/v1/runs`

Returns the most recent runs across all jobs, newest first.
Useful for a global activity feed.

| Query param | Default | Description |
|-------------|---------|-------------|
| `limit` | `50` | Max records to return |

```bash
curl -s "$BASE/runs?limit=20" -H "$TOKEN"
```

**Response `200`** — same shape as job runs array above

---

## Error responses

All errors return JSON with an `error` field.

```bash
# Missing Authorization header
curl -s "$BASE/jobs"
# {"error":"unauthorized: missing Authorization header"}  HTTP 401

# Wrong token
curl -s "$BASE/jobs" -H "Authorization: Bearer wrong"
# {"error":"unauthorized: invalid api key"}  HTTP 401

# Job not found
curl -s "$BASE/jobs/does-not-exist" -H "$TOKEN"
# {"error":"job not found"}  HTTP 404

# Invalid cron expression
curl -s -X POST "$BASE/jobs" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"bad","cron_expr":"not-a-cron","url":"http://example.com"}'
# {"error":"invalid cron expression \"not-a-cron\": expected exactly 5 fields, found 1: [not-a-cron]"}  HTTP 400

# Missing required field
curl -s -X POST "$BASE/jobs" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"cron_expr":"*/5 * * * *","url":"http://example.com"}'
# {"error":"name is required"}  HTTP 400
```

---

## Cron expression format

Standard 5-field cron: `MIN HOUR DOM MON DOW`

| Expression | Meaning |
|------------|---------|
| `*/5 * * * *` | Every 5 minutes |
| `0 * * * *` | Every hour (on the hour) |
| `0 9 * * 1-5` | 9 AM on weekdays |
| `30 18 * * *` | 6:30 PM every day |
| `0 0 1 * *` | Midnight on the 1st of every month |

---

## Configuration

Set via environment variables or a `.env` file.

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `DB_DSN` | — | ✓ | MySQL connection string |
| `API_KEY` | — | ✓ | Bearer token for all API requests |
| `SERVER_PORT` | `8080` | — | HTTP listen port |
| `HTTP_TIMEOUT_SEC` | `30` | — | Per-webhook-request timeout in seconds |

---

## Makefile reference

```
make install-prereqs   Install Docker, Colima, Go, MySQL client (macOS)
make run               Start full stack and apply migrations
make stop              Stop all containers
make clean             Stop containers and delete all data
make logs              Tail app container logs
make migrate           Re-apply migrations (containers must be running)
make test              Run all unit tests
make test-race         Run tests with race detector
make build             Compile binary to bin/scheduler
make dev               Run locally without Docker (requires .env)
make fmt               Format Go source files
```

---

## Architecture

```
┌─────────────────────────────────────────────────┐
│  HTTP API (chi router)                          │
│  POST/GET/PUT/DELETE  /api/v1/jobs              │
│  PATCH                /api/v1/jobs/:id/pause    │
│  GET                  /api/v1/jobs/:id/runs     │
│  GET                  /api/v1/runs              │
└──────────────┬──────────────────────────────────┘
               │ JobChangeNotifier (interface)
               ▼
┌─────────────────────────────────────────────────┐
│  MinHeapScheduler                               │
│  - Single goroutine owns the min-heap           │
│  - Fires jobs as concurrent goroutines          │
│  - Next run time via robfig/cron schedule.Next  │
└──────────────┬──────────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────────┐
│  HTTPExecutor                                   │
│  - Calls webhook URL with configured method     │
│  - Retries up to MaxRetries with fixed delay    │
└──────────────┬──────────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────────┐
│  MySQL                                          │
│  jobs  — source of truth for schedules          │
│  runs  — full execution history                 │
└─────────────────────────────────────────────────┘
```

All major components (`Scheduler`, `JobExecutor`, `JobRepository`, `RunRepository`, `JobChangeNotifier`, `Authenticator`) are Go interfaces. Swap any implementation — Redis scheduler, JWT auth, custom executor — without touching the rest of the codebase.
