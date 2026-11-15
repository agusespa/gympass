<p align="center"> <img src="web/static/icon-192.png" alt="Gym Tracker Icon" width="128"> </p>

# Gym Tracker

A personal, offline-first workout logger PWA for iPhone, installed via **Add to Home Screen**. Log your sets at the gym without signal; everything syncs to your own server when you're back online.

### Features
- **Routines**: Build reusable templates with exercises, sets, target reps and weight, and start a workout from one with a single tap.
- **Workout logging**: Track weight, reps and rest per set. Mark sets as warm-ups, group exercises into supersets, and reorder them as you go.
- **Running workouts**: A workout stays open with a live timer until you finish or discard it, even if you close the app.
- **Finish screen**: A session summary (exercises, sets, reps, volume) with the option to save the workout as a new routine.
- **History**: Every finished workout in the Profile tab.
- **Body weight**: Log your weight in Settings and choose, per exercise, how much of it each rep moves (e.g. 100% for pull-ups, 65% for push-ups) so bodyweight work counts toward volume.
- **Offline-first**: The app and your data live on the phone, so it works with no connection.
- **Sync**: Changes sync to your own server, so nothing is lost if you lose or reset the phone.
- **Webhook**: Optionally send each finished workout to an external service.

## Quick Start

The application is distributed as a multi-platform Docker image (linux/amd64, linux/arm64) via the GitHub Container Registry. Use the provided `docker-compose.yml`. Navigate to the project root and run:

```bash
mkdir -p data && sudo chown 65532:65532 data
```

```bash
docker compose up -d
```

> [!NOTE]
> The image runs as the distroless `nonroot` user (UID 65532), so it needs to own the `data` directory.

Open `http://localhost:8080` on your phone and choose **Share → Add to Home Screen**.

### Configuration
The application is configured using environment variables. All are **optional**:
- `DB_PATH`: Filesystem path to the SQLite database (default: `data/gym.db`; the compose file sets `/app/data/gym.db`).
- `PORT`: The port to listen on (default: `8080`).
- `WEBHOOK_URL`: Endpoint to POST finished workouts to. The webhook is off when unset.
- `WEBHOOK_SECRET`: Sent as `Authorization: Bearer <secret>`.
- `WEBHOOK_DELAY`: Wait after the last change before sending, so a burst of edits becomes one send (default: `30s`).
- `API_TOKEN`: Enables the read endpoints of the [automation API](#automation-api); requests must send `Authorization: Bearer <token>`. Use a long random value, e.g. `openssl rand -hex 32`.
- `API_WRITE_TOKEN`: Allows creating and changing routines through the automation API, and reading everything `API_TOKEN` can. Must differ from `API_TOKEN`. The API is off while both are unset.

Set them in the `environment` section of `docker-compose.yml`, or in a `.env` file next to it.

## Secure Hosting

The app has no built-in authentication, so host it on a local server and expose it with a Cloudflare Tunnel. The tunnel (`cloudflared`) makes an outbound-only connection to Cloudflare's network, so you can reach the app from anywhere without opening ports on your router.

Point the tunnel at `http://localhost:8080`, then add an Access Application in the Cloudflare Dashboard with a Policy that only allows your own email address.

If `cloudflared` runs in Docker on the same network as this service, use
`http://gym-tracker:8080` instead. Inside the tunnel container, `localhost` refers
to that container.

> [!TIP]
> When the Access session expires, the app keeps your local changes and shows a banner asking you to reload. The reload goes through the Access login and the kept changes sync afterwards.

## Data Persistence

The application uses a **Bind Mount** so your data persists on the host machine.

- **Host Path**: `./data` (relative to your `docker-compose.yml`)
- **Mount Point**: `/app/data` (inside the container)
- **Database File**: `./data/gym.db`

The phone also keeps a full copy in IndexedDB, so the server can be offline for a while without losing anything.

## Health & Monitoring

| Endpoint | Purpose |
|---|---|
| `GET /api/healthz` | Liveness: returns `200` and `{"status":"ok"}` while the HTTP server is responsive. |
| `GET /api/readyz` | Readiness: verifies SQLite can update its sync metadata in a rolled-back transaction. Returns `200` with `{"status":"ready"}`, or `503` with `{"status":"not_ready"}`. |
| `GET /metrics` | Prometheus metrics for sync request counts by HTTP response code and request duration. |

These responses are not cached. Readiness has a two-second database deadline;
it does not depend on the optional webhook receiver. Health checks never commit
changes or advance the sync cursor. Use these explicit endpoints rather than
`/` or `/health`, since the SPA fallback can return HTML with HTTP 200.

The image includes a health check every 30 seconds, with a 10-second startup
period and three consecutive failures before Docker marks it unhealthy. It uses
the Go binary directly, so it works in the distroless image without a shell:

```bash
docker compose exec gym-tracker /app/gym-tracker healthcheck
docker compose ps
```

The command checks `http://127.0.0.1:${PORT:-8080}/api/readyz` with a three-second
timeout. It accepts only HTTP 200 with the expected JSON readiness response,
rejects redirects, and exits nonzero on failure. It does not open the database
or start the server or webhook worker. Docker's health status does not by itself
restart an unhealthy container.

Configure Prometheus to scrape `gym-tracker:8080/metrics` on the shared Docker
network. The app exports:

- `gym_tracker_sync_requests_total{code="200|400|500"}`: successful syncs,
  validation failures, and server failures. The `code` label contains one actual
  status code per series; probes and metric scrapes are excluded.
- `gym_tracker_sync_request_duration_seconds`: a histogram of sync request
  duration, including `_bucket`, `_sum`, and `_count` series.
- `gym_tracker_sync_rejected_rows_total`: rows a sync refused (for example a
  value out of range). The rest of that sync is stored; each refused row stays
  unsynced on the phone that sent it, which shows a banner, and is sent again on
  later syncs. The server log names the first refused row of each sync.

For example, a server-error alert can use:

```promql
sum(rate(gym_tracker_sync_requests_total{code="500"}[5m])) > 0
```

Blackbox probes, scrape and backup alerts, dashboards, and automatic monitoring
configuration reloads are managed by the home-lab infrastructure repository.

## Backups & Recovery

The database runs in **Write-Ahead Logging (WAL)** mode, which allows safe "hot" backups while the application is running.

### Atomic Backups to NAS

Use `sqlite3` on the host for a consistent backup (copying the file directly can miss data still in the WAL):

```bash
sqlite3 ./data/gym.db ".backup '/path/to/your/nas/gym_backup.db'"
```

The distroless image has no `sqlite3` shell; run that command on the host.
Back up the complete database, including sync cursors, deletion tombstones, and
the webhook outbox. Server backups contain only data already synced from phones;
pending offline edits remain on the device until its next successful sync.

> [!TIP]
> Automate it with a crontab entry on the host:
> ```bash
> 0 3 * * * sqlite3 /path/to/gym-tracker/data/gym.db ".backup '/path/to/nas/gym_backup_$(date +\%Y\%m\%d).db'"
> ```

### Restore Procedure
1. Stop the container: `docker compose down`.
2. Remove the old WAL files and overwrite the DB: `rm -f ./data/gym.db-wal ./data/gym.db-shm && cp /path/to/nas/gym_backup_20260101.db ./data/gym.db`.
3. Fix ownership so the container can write to it: `sudo chown 65532:65532 ./data/gym.db`.
4. Start the container: `docker compose up -d`.
5. Check readiness with `docker compose exec gym-tracker /app/gym-tracker healthcheck`.

Test restores against a temporary copy before replacing live data, including
reconnecting a client with an existing sync cursor and pending local edits.
Restoring an older database can replay pending webhooks, so the receiver should
continue to upsert by workout ID. Scheduled backups, retention, backup-freshness
alerts, and automated restore checks belong in the home-lab configuration.

## Webhook

Set `WEBHOOK_URL` and the server POSTs each **finished** workout there. A running workout is never sent, and neither is one you discard.

- Pending sends survive restarts. Failed sends are retried with backoff (30 s doubling up to 1 h).
- Editing a finished workout sends the whole workout again. Deleting a workout that was already delivered sends `workout.deleted`. Upsert on `workout.id` in the receiver.

```json
{
  "event": "workout.saved",
  "sent_at": "2026-11-14T18:31:02Z",
  "workout": {
    "id": "…", "name": "Push", "date": "2026-11-14",
    "started_at": "2026-11-14T17:17:30Z", "finished_at": "2026-11-14T18:30:00Z", "duration_seconds": 4350,
    "exercises": [{ "name": "Bench", "superset_id": null,
      "sets": [{ "reps": 8, "weight_kg": 82.5, "warmup": false, "rest_seconds": 120 }] }],
    "totals": { "exercises": 1, "working_sets": 1, "reps": 8, "volume_kg": 660 }
  }
}
```

`totals` count working sets only. `volume_kg` adds `reps × (weight_kg + body_weight_kg × bodyweight_pct / 100)` per set: `body_weight_kg` is the latest weigh-in on or before the workout's date (or the first one, for earlier workouts), and an exercise's `bodyweight_pct` is the share of body weight it moves. Both are left out when they don't apply. Logging a new body weight does not resend workouts. `started_at` and `duration_seconds` are left out for workouts created before start times were recorded. A deletion is `{ "event": "workout.deleted", "workout": { "id": "…" } }`.

## Automation API

Lets automations and AI assistants read your workouts (e.g. an n8n schedule building a weekly summary) and manage your routines. Every request needs `Authorization: Bearer <token>`; without a valid one the server answers `401`. `API_TOKEN` reaches the `GET` endpoints, and `API_WRITE_TOKEN` reaches those plus the routine writes. Give an automation the read token unless it needs to change routines.

### Workouts

Running, discarded and deleted workouts are never returned.

| Endpoint | Returns |
|---|---|
| `GET /api/workouts?from=YYYY-MM-DD&to=YYYY-MM-DD` | Finished workouts whose `date` is in the range (both ends included, at most 366 days), oldest first, plus totals for the whole range. [Paged](#paging). |
| `GET /api/workouts/{id}` | One finished workout, or `404`. |

Each workout has the same shape as `workout` in the [webhook](#webhook) payload. The list looks like this:

```json
{
  "from": "2026-11-09", "to": "2026-11-15",
  "totals": { "workouts": 3, "working_sets": 54, "reps": 412, "volume_kg": 24310.5, "duration_seconds": 12600 },
  "workouts": [{ "id": "…", "name": "Push", "date": "2026-11-10", "…": "…" }],
  "next_cursor": "…"
}
```

`totals` count working sets only. `duration_seconds` adds up the workouts that have a recorded start time. `date` is the day the phone logged the workout on, so pick the range in your own time zone; in n8n, `{{ $now.minus({ days: 6 }).toFormat('yyyy-MM-dd') }}` to `{{ $now.toFormat('yyyy-MM-dd') }}` is the last seven days.

### Routines

| Endpoint | Token | Does |
|---|---|---|
| `GET /api/routines` | read | `{"routines": [...], "next_cursor": "…"}`, routines sorted by name. [Paged](#paging). |
| `GET /api/routines/{id}` | read | One routine, or `404`. |
| `POST /api/routines` | write | Creates a routine and returns it with `201`. Any ids in the body are ignored. |
| `PUT /api/routines/{id}` | write | Makes the routine look exactly like the body and returns the result. |

There is no delete; remove routines in the app.

```json
{
  "id": "…", "name": "Push",
  "exercises": [
    { "id": "…", "name": "Bench", "superset_id": null,
      "sets": [{ "id": "…", "reps": 8, "weight_kg": 82.5, "warmup": false, "rest_seconds": 120 }] }
  ]
}
```

To change a routine, `GET` it, edit the JSON and `PUT` it back:

- Exercises and sets that keep their `id` are updated, ones without an `id` are added, and ones you leave out are deleted. An `id` must belong to that routine (a set's to that exercise), otherwise the request fails with `400`.
- Order in the array is the order in the app.
- `reps` and `weight_kg` are required; `warmup` defaults to `false` and `rest_seconds` to `120` (0–3600). Unknown fields are rejected, so a typo like `weight` fails instead of being dropped.
- Adjacent exercises with the same `superset_id` form a superset. Any string works as a label for a new one; a lone exercise's label is cleared.
- Only rows that actually changed are written, and they reach your phone on its next sync like an edit made in the app. If the same set is edited in the app around the same time, the newest edit wins.

### Paging

The list endpoints return at most `limit` items (1–200, default 50). While more remain, the response has a `next_cursor`; pass it back as `cursor` (with the same other parameters) to get the next page. It is missing on the last page. Treat cursors as opaque. `totals` in the workout list always cover the whole range, on every page.

### MCP

AI assistants can use the same data through the Model Context Protocol at `/mcp` (Streamable HTTP), with the same bearer tokens. With `API_TOKEN` an assistant gets `list_workouts`, `get_workout`, `list_routines` and `get_routine`. With `API_WRITE_TOKEN` it also gets `create_routine` and `update_routine`, which follow the same rules as `POST` and `PUT` above. `list_workouts` and `list_routines` take the same `limit` and `cursor` arguments as the [paged](#paging) endpoints.

### Connecting

If n8n or the assistant runs on the same Docker network, call `http://gym-tracker:8080` directly. If it goes through the Cloudflare Tunnel, Access blocks it before it reaches the app: create an Access **service token**, add a policy with the **Service Auth** action that allows it, and send its `CF-Access-Client-Id` and `CF-Access-Client-Secret` headers alongside the bearer token (with `claude mcp add`, pass each as another `--header`).

## Usage Notes

- **Syncing**: The app syncs on start, shortly after each edit, when the phone comes back online, and when you return to it. If the same thing is edited on two devices, the newest edit wins.
- **Offline use**: You can log a whole workout with no signal. Changes are queued on the phone and sent the next time the server is reachable.
- **Deleting**: Deletes sync too, so removing a workout on one device removes it everywhere.
- **Updates**: Pull the new image with `docker compose pull && docker compose up -d`. The phone picks up the new version within a launch or two.

## Releasing

Push a version tag. The **Release** workflow runs the checks, builds the image for linux/amd64 and linux/arm64, and pushes it to `ghcr.io/agusespa/gym-tracker` as `:1.2.3`, `:1.2` and `:latest`:

```bash
git tag v1.2.3 && git push origin v1.2.3
```

To republish `main` without a new version, run the workflow from the Actions tab (**Release → Run workflow**). That pushes `:latest` and `:sha-<commit>`.
