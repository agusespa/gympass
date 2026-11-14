<p align="center"> <img src="web/static/icon-192.png" alt="Gym Tracker Icon" width="128"> </p>

# Gym Tracker

A personal, offline-first workout logger PWA for iPhone, installed via **Add to Home Screen**. Log your sets at the gym without signal; everything syncs to your own server when you're back online.

### Features
- **Routines**: Build reusable templates with exercises, sets, target reps and weight, and start a workout from one with a single tap.
- **Workout logging**: Track weight, reps and rest per set. Mark sets as warm-ups, group exercises into supersets, and reorder them as you go.
- **Running workouts**: A workout stays open with a live timer until you finish or discard it, even if you close the app.
- **Finish screen**: A session summary (exercises, sets, reps, volume) with the option to save the workout as a new routine.
- **History**: Every finished workout in the Profile tab.
- **Offline-first**: Data lives on the phone (IndexedDB) and a service worker caches the app, so it works with no connection.
- **Sync**: Changes sync to a Go + SQLite server, newest edit wins.
- **Webhook**: Optionally send each finished workout to n8n (e.g. to log it in Notion).

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
- `WEBHOOK_SECRET`: Sent as `Authorization: Bearer <secret>` (n8n: Header Auth).
- `WEBHOOK_DELAY`: Wait after the last change before sending, so a burst of edits becomes one send (default: `30s`).

Set them in the `environment` section of `docker-compose.yml`, or in a `.env` file next to it.

## Secure Hosting

The app has no built-in authentication, so host it on a local server and expose it with a Cloudflare Tunnel. The tunnel (`cloudflared`) makes an outbound-only connection to Cloudflare's network, so you can reach the app from anywhere without opening ports on your router.

Point the tunnel at `http://localhost:8080`, then add an Access Application in the Cloudflare Dashboard with a Policy that only allows your own email address.

> [!TIP]
> When the Access session expires, the app keeps your local changes and shows a banner asking you to reload. The reload goes through the Access login and the kept changes sync afterwards.

## Data Persistence

The application uses a **Bind Mount** so your data persists on the host machine.

- **Host Path**: `./data` (relative to your `docker-compose.yml`)
- **Mount Point**: `/app/data` (inside the container)
- **Database File**: `./data/gym.db`

The phone also keeps a full copy in IndexedDB, so the server can be offline for a while without losing anything.

## Backups & Recovery

The database runs in **Write-Ahead Logging (WAL)** mode, which allows safe "hot" backups while the application is running.

### Atomic Backups to NAS

Use `sqlite3` on the host for a consistent backup (copying the file directly can miss data still in the WAL):

```bash
sqlite3 ./data/gym.db ".backup '/path/to/your/nas/gym_backup.db'"
```

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

## Webhook

Set `WEBHOOK_URL` and the server POSTs each **finished** workout there (e.g. an n8n Webhook node feeding Notion). A running workout is never sent, and neither is one you discard.

- Sends go through an outbox table in SQLite, so they survive restarts. Failed sends are retried with backoff (30 s doubling up to 1 h).
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

`totals` count working sets only. `started_at` and `duration_seconds` are left out for workouts created before start times were recorded. A deletion is `{ "event": "workout.deleted", "workout": { "id": "…" } }`.

## How Sync Works

Every record has a client-generated UUID, an `updated_at` timestamp (ms, set by the client) and a `deleted` flag. Deleted records are kept as tombstones so the delete syncs.

The client sends `{ cursor, changes }` to `POST /api/sync` and receives `{ cursor, changes }`:

- **Newest wins:** the server stores an incoming row only if its `updated_at` is newer than the stored one. The client applies the same rule to the rows it receives.
- **Server cursor:** every row the server stores gets a server-side `seq` number. The client asks for rows with `seq > cursor`, so a wrong phone clock can't cause changes to be missed.
- **When it syncs:** on app start, shortly after each edit, when the phone comes back online, and when the app returns to the foreground.

## Development

- **Frontend:** SvelteKit (Svelte 5, TypeScript), built as a static SPA. IndexedDB via Dexie; a service worker for offline use.
- **Backend:** Go and SQLite (`modernc.org/sqlite`, pure Go). It serves the embedded frontend and `POST /api/sync`.

Requires Go 1.27.4+, Node 22+, and Docker for the image.

| Target | What it does |
|---|---|
| `make dev` | Go API on :8080 and the Vite dev server on :5173 (proxies `/api`) |
| `make run` | Builds the frontend, then runs the production server on :8080 |
| `make test` | Go tests |
| `make e2e` | Builds everything, then runs the Playwright tests |
| `make docker` | Builds the multi-arch image (linux/amd64, linux/arm64) |
| `make verify` | Runs the full check script (also used by the pre-push hook) |
| `make hooks` | Enables the pre-push hook |

```
cmd/gym-tracker/     main: env config, HTTP server
internal/store/      SQLite schema and sync logic (+ tests)
internal/api/        /api/sync and static file serving (+ tests)
internal/webhook/    sends finished workouts from the outbox to WEBHOOK_URL (+ tests)
internal/ui/         embeds the built frontend from internal/ui/dist
web/                 SvelteKit app (db.ts, repo.ts, sync.svelte.ts, service-worker/)
e2e/                 Playwright tests
```

To publish the image:

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/agusespa/gym-tracker:latest --push .
```
