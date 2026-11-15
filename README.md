<p align="center"> <img src="web/static/icon-192.png" alt="Gym Tracker Icon" width="128"> </p>

# Gym Tracker

A personal, offline-first workout logger PWA for iPhone, installed via **Add to Home Screen**. Log your sets at the gym without signal; everything syncs to your own server when you're back online.

### Features
- **Routines**: Build reusable templates with exercises, sets, target reps and weight, and start a workout from one with a single tap.
- **Workout logging**: Track weight, reps and rest per set. Mark sets as warm-ups, group exercises into supersets, and reorder them as you go.
- **Running workouts**: A workout stays open with a live timer until you finish or discard it, even if you close the app.
- **Finish screen**: A session summary (exercises, sets, reps, volume) with the option to save the workout as a new routine.
- **History**: Every finished workout in the Profile tab.
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
- `API_TOKEN`: Enables the [read API](#read-api); requests must send `Authorization: Bearer <token>`. The read API is off when unset. Use a long random value, e.g. `openssl rand -hex 32`.

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

`totals` count working sets only. `started_at` and `duration_seconds` are left out for workouts created before start times were recorded. A deletion is `{ "event": "workout.deleted", "workout": { "id": "…" } }`.

## Read API

Set `API_TOKEN` to let automations (e.g. an n8n schedule building a weekly summary) read finished workouts. Every request needs `Authorization: Bearer <API_TOKEN>`; without it the server answers `401`. Running, discarded and deleted workouts are never returned.

| Endpoint | Returns |
|---|---|
| `GET /api/workouts?from=YYYY-MM-DD&to=YYYY-MM-DD` | Finished workouts whose `date` is in the range (both ends included, at most 366 days), oldest first, plus totals for the range. |
| `GET /api/workouts/{id}` | One finished workout, or `404`. |

Each workout has the same shape as `workout` in the [webhook](#webhook) payload. The list looks like this:

```json
{
  "from": "2026-11-09", "to": "2026-11-15",
  "totals": { "workouts": 3, "working_sets": 54, "reps": 412, "volume_kg": 24310.5, "duration_seconds": 12600 },
  "workouts": [{ "id": "…", "name": "Push", "date": "2026-11-10", "…": "…" }]
}
```

`totals` count working sets only. `duration_seconds` adds up the workouts that have a recorded start time. `date` is the day the phone logged the workout on, so pick the range in your own time zone; in n8n, `{{ $now.minus({ days: 6 }).toFormat('yyyy-MM-dd') }}` to `{{ $now.toFormat('yyyy-MM-dd') }}` is the last seven days.

If n8n runs on the same Docker network, call `http://gym-tracker:8080` directly. If it goes through the Cloudflare Tunnel, Access blocks it before it reaches the app: create an Access **service token**, add a policy with the **Service Auth** action that allows it, and send its `CF-Access-Client-Id` and `CF-Access-Client-Secret` headers alongside the bearer token.

## Usage Notes

- **Syncing**: The app syncs on start, shortly after each edit, when the phone comes back online, and when you return to it. If the same thing is edited on two devices, the newest edit wins.
- **Offline use**: You can log a whole workout with no signal. Changes are queued on the phone and sent the next time the server is reachable.
- **Deleting**: Deletes sync too, so removing a workout on one device removes it everywhere.
- **Updates**: Pull the new image with `docker compose pull && docker compose up -d`. The phone picks up the new version within a launch or two.
