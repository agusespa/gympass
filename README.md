<p align="center"> <img src="web/static/icon-192.png" alt="Gym Tracker Icon" width="128"> </p>

# Gympass

A personal, offline-first web app for managing workout gym sessions.

### Features
- **Routines**: Reusable templates with exercises, sets and target reps or rep ranges. Start a workout from one with a single tap; each set starts from the weight and reps of your last session.
- **Workout logging**: Weight, reps and rest per set. Mark warm-ups, group exercises into supersets and reorder them as you go.
- **Running workouts**: A workout stays open with a live timer until you finish or discard it, even if you close the app.
- **Offline-first and synced**: Your data lives on the phone and syncs to your server, so nothing is lost if you lose or reset the phone.
- **Integrations**: Optionally send finished workouts to a webhook, or read them (and manage routines) through an API or MCP.

## Requirements

- A server with Docker and Docker Compose.
- A Cloudflare account with a domain, for the Cloudflare Tunnel and Access in front of the app.
- `sqlite3` on the server, for backups.

## Install

> [!IMPORTANT]
> The app has no login of its own, and that includes the sync endpoint, the API and MCP. It must only be reachable through a Cloudflare Tunnel protected by Cloudflare Access. Never add a `ports` section to the `gym-tracker` service or otherwise expose it: anything that can reach the container directly can read and change all your data.

The provided [`docker-compose.yml`](docker-compose.yml) runs the app (`ghcr.io/agusespa/gympass`) next to `cloudflared`, which connects out to Cloudflare, so no port is opened on the server or your router.

1. In Cloudflare Zero Trust, go to **Networks → Tunnels** and create a tunnel.
2. Add a public hostname to the tunnel (e.g. `gym.example.com`) with the service `http://gym-tracker:8080`.
3. Add an Access application for that hostname with a policy that only allows your own email address.
4. On the server, put `docker-compose.yml` in a folder and create a `.env` file next to it with the tunnel's token:

   ```
   TUNNEL_TOKEN=<your tunnel token>
   ```

5. Create the data folder. The container runs as UID 65532, so it must own it:

   ```bash
   mkdir -p data && sudo chown 65532:65532 data
   ```

6. Start it:

   ```bash
   docker compose up -d
   ```

7. Open `https://gym.example.com` on your iPhone in Safari, log in through Access and choose **Share → Add to Home Screen**.

Other containers on the same Docker network can reach `http://gym-tracker:8080` without going through Access, so only run services you trust there.

### Updating

```bash
docker compose pull && docker compose up -d
```

The phone picks up the new version within a launch or two.

## Configuration

Set these in the `environment` section of `docker-compose.yml`. All are optional.

| Variable | Default | Purpose |
|---|---|---|
| `DB_PATH` | `/app/data/gym.db` | Path to the SQLite database inside the container. |
| `PORT` | `8080` | Port the app listens on. If you change it, update the tunnel's service URL too. |
| `WEBHOOK_URL` | unset | Endpoint to POST finished workouts to. The webhook is off when unset. See [Webhook](#webhook). |
| `WEBHOOK_SECRET` | unset | Sent to the webhook as `Authorization: Bearer <secret>`. |
| `WEBHOOK_DELAY` | `30s` | Wait after the last change before sending, so a burst of edits becomes one send. Go duration format, e.g. `1m`. |

## Data and backups

Your data is in `./data/gym.db` on the server (mounted at `/app/data` in the container). The phone also keeps a full copy, so the server can be offline for a while without losing anything. Server backups only contain changes the phones have already synced.

The database runs in WAL mode, so you can back it up while the app runs. Use `sqlite3` on the server, since copying the file directly can miss recent changes:

```bash
sqlite3 ./data/gym.db ".backup '/path/to/backups/gym_backup.db'"
```

For a nightly backup, add a crontab entry on the server:

```bash
0 3 * * * sqlite3 /path/to/gym-tracker/data/gym.db ".backup '/path/to/backups/gym_backup_$(date +\%Y\%m\%d).db'"
```

To restore:

1. `docker compose down`
2. `rm -f ./data/gym.db-wal ./data/gym.db-shm && cp /path/to/backups/gym_backup_20260101.db ./data/gym.db`
3. `sudo chown 65532:65532 ./data/gym.db`
4. `docker compose up -d`
5. `docker compose exec gym-tracker /app/gym-tracker healthcheck`

Restoring an older database can resend pending webhooks, so the webhook receiver should upsert by workout ID.

## Monitoring

| Endpoint | Purpose |
|---|---|
| `GET /api/healthz` | Liveness: `200` with `{"status":"ok"}` while the server responds. |
| `GET /api/readyz` | Readiness: `200` with `{"status":"ready"}` when the database works, otherwise `503` with `{"status":"not_ready"}`. |
| `GET /metrics` | Prometheus metrics. |

Use these endpoints rather than `/`, which always returns the app with `200`.

The container has a built-in Docker health check (every 30 seconds) that calls `/api/readyz`. Check it with `docker compose ps`, or run it by hand with `docker compose exec gym-tracker /app/gym-tracker healthcheck`. Docker does not restart an unhealthy container by itself.

To scrape metrics, point Prometheus at `gym-tracker:8080/metrics` on the same Docker network. The metrics are:

- `gym_tracker_sync_requests_total{code="200|400|500"}`: syncs by response code.
- `gym_tracker_sync_request_duration_seconds`: sync duration histogram.
- `gym_tracker_sync_rejected_rows_total`: changes a sync refused. The server log names the first refused row of each sync.

For example, to alert on server errors:

```promql
sum(rate(gym_tracker_sync_requests_total{code="500"}[5m])) > 0
```

## Webhook

Set `WEBHOOK_URL` and the server POSTs each **finished** workout there. Running and discarded workouts are never sent.

- Pending sends survive restarts. Failed sends are retried with backoff (30 s doubling up to 1 h).
- Editing a finished workout sends the whole workout again. Deleting one that was already delivered sends `workout.deleted`. Upsert on `workout.id` in the receiver.

```json
{
  "event": "workout.saved",
  "sent_at": "2026-11-14T18:31:02Z",
  "workout": {
    "id": "…", "name": "Push", "date": "2026-11-14",
    "started_at": "2026-11-14T17:17:30Z", "finished_at": "2026-11-14T18:30:00Z", "duration_seconds": 4350,
    "exercises": [{ "name": "Bench", "superset_id": null,
      "sets": [{ "reps": 8, "weight_kg": 82.5, "warmup": false, "rest_seconds": 120 }] }],
    "totals": { "exercises": 1, "working_sets": 1, "reps": 8 }
  }
}
```

`totals` count working sets only. A deletion is `{ "event": "workout.deleted", "workout": { "id": "…" } }`.

## API

Lets automations and AI assistants read your workouts (e.g. an n8n schedule building a weekly summary) and manage your routines. Like the app, it has no login of its own and relies on Cloudflare Access (see [Connecting](#connecting)). Requests that change data are refused (`403`) when a browser marks them as coming from another site; scripts, n8n and MCP clients aren't affected.

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
  "totals": { "workouts": 3, "working_sets": 54, "reps": 412, "duration_seconds": 12600 },
  "workouts": [{ "id": "…", "name": "Push", "date": "2026-11-10", "…": "…" }],
  "next_cursor": "…"
}
```

`totals` count working sets only. `duration_seconds` adds up the workouts that have a recorded start time. `date` is the day the phone logged the workout on, so pick the range in your own time zone; in n8n, `{{ $now.minus({ days: 6 }).toFormat('yyyy-MM-dd') }}` to `{{ $now.toFormat('yyyy-MM-dd') }}` is the last seven days.

### Routines

| Endpoint | Does |
|---|---|
| `GET /api/routines` | `{"routines": [...], "next_cursor": "…"}`, sorted by name. [Paged](#paging). |
| `GET /api/routines/{id}` | One routine, or `404`. |
| `POST /api/routines` | Creates a routine and returns it with `201`. Any ids in the body are ignored. |
| `PUT /api/routines/{id}` | Makes the routine look exactly like the body and returns the result. |

There is no delete; remove routines in the app.

```json
{
  "id": "…", "name": "Push",
  "exercises": [
    { "id": "…", "exercise_id": "bench-press", "name": "Bench Press", "superset_id": null,
      "sets": [{ "id": "…", "reps": 8, "warmup": false, "rest_seconds": 120,
                 "duration_seconds": 0, "rep_min": 8, "rep_max": 12 }] }
  ]
}
```

To change a routine, `GET` it, edit the JSON and `PUT` it back:

- Exercises and sets that keep their `id` are updated, ones without an `id` are added, and ones you leave out are deleted. An `id` must belong to that routine (a set's to that exercise), otherwise the request fails with `400`.
- Order in the array is the order in the app.
- An exercise is chosen by `name`, ignoring case, from the built-in list (see [Exercises](#exercises)). Any other name fails with `400`. Reads also return `exercise_id`, which is ignored on write.
- Each set needs `reps` or, for a timed exercise such as a dead hang, `duration_seconds` (0–36000). `warmup` defaults to `false` and `rest_seconds` to `120` (0–3600). Unknown fields are rejected, so a typo like `rest` fails instead of being dropped.
- Routines plan no weights. A workout started from a routine takes each set's weight and reps from the last session of that exercise, so progression happens in the workouts, not here.
- `rep_max` makes `reps` the bottom of a rep range: `"reps": 8, "rep_max": 12` plans 8–12 reps. Leave it out for a fixed count. Reads return the range as `rep_min` and `rep_max`; `rep_min` is ignored on write.
- Adjacent exercises with the same `superset_id` form a superset. Any string works as a label for a new one.
- Changes reach your phone on its next sync, like an edit made in the app. If the same set is edited in the app around the same time, the newest edit wins.

### Exercises

`GET /api/exercises` returns `{"exercises": [...]}`: every exercise there is, sorted by name. The list is built in (`internal/store/catalog.json`); there are no custom exercises.

```json
{ "id": "bench-press", "name": "Bench Press", "measure": "reps", "equipment": "barbell" }
```

`measure` is `reps` or `time`. `equipment` is `barbell`, `dumbbell`, `machine`, `plate`, `cable`, `kettlebell` or `bodyweight`.

### Paging

List endpoints return at most `limit` items (1–200, default 50). While more remain, the response has a `next_cursor`; pass it back as `cursor` (with the same other parameters) to get the next page. `totals` in the workout list always cover the whole range.

### MCP

AI assistants can use the same data through the Model Context Protocol at `/mcp` (Streamable HTTP). The tools are `list_workouts`, `get_workout`, `list_exercises`, `list_routines`, `get_routine`, `create_routine` and `update_routine`, with the same rules as the endpoints above.

### Connecting

Services on the same Docker network can call `http://gym-tracker:8080` directly, with full access. Through the tunnel, Access blocks requests before they reach the app: create an Access **service token**, add a policy with the **Service Auth** action that allows it, and send its `CF-Access-Client-Id` and `CF-Access-Client-Secret` headers with every request.

For Claude Code:

```bash
claude mcp add --transport http gym-tracker https://gym.example.com/mcp --header "CF-Access-Client-Id: <client id>" --header "CF-Access-Client-Secret: <client secret>"
```

Assistants like Claude (web, desktop and phone) and ChatGPT connect from their own cloud and log in with OAuth, so they can't send a service token. Let Cloudflare Access be the OAuth server instead:

1. In Zero Trust, edit the Access application and, under **Advanced settings**, turn on **Managed OAuth**. Under **Allowed redirect URIs**, add your assistant's OAuth callback, or it can't register ("redirect_uri is not allowed"). For Claude, add `https://claude.ai/api/mcp/auth_callback` and `https://claude.com/api/mcp/auth_callback`.
2. Add `https://gym.example.com/mcp` as a custom connector in your assistant (in Claude: **Settings → Connectors → Add custom connector**, OAuth fields left empty). You log in through your normal Access policy.
