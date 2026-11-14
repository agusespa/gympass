# gym-tracker

A personal, offline-first workout logger for iPhone, installed via **Add to Home Screen**.

- **Frontend:** SvelteKit (Svelte 5, TypeScript), built as a static SPA. Data lives in IndexedDB (Dexie); a service worker caches the app so it works without signal.
- **Backend:** Go and SQLite (`modernc.org/sqlite`, pure Go). It serves the embedded frontend and `POST /api/sync`.
- **Auth:** none built in. The app runs behind a Cloudflare Tunnel with an Access policy.

## Layout

```
cmd/gym-tracker/     main: env config, HTTP server
internal/store/      SQLite schema and sync logic (+ tests)
internal/api/        /api/sync and static file serving (+ tests)
internal/ui/         embeds the built frontend from internal/ui/dist
web/                 SvelteKit app (db.ts, repo.ts, sync.svelte.ts, service-worker/)
e2e/                 Playwright tests
```

## Sync

Every record has a client-generated UUID, an `updated_at` timestamp (ms, set by the client) and a `deleted` flag. Deleted records are kept as tombstones so the delete syncs.

The client sends `{ cursor, changes }` and receives `{ cursor, changes }`:

- **Newest wins:** the server stores an incoming row only if its `updated_at` is newer than the stored one. The client applies the same rule to the rows it receives.
- **Server cursor:** every row the server stores gets a server-side `seq` number. The client asks for rows with `seq > cursor`, so a wrong phone clock can't cause changes to be missed.
- **When it syncs:** on app start, shortly after each edit, when the phone comes back online, and when the app returns to the foreground.
- **Expired Access session:** if `/api/sync` answers with a redirect, a 401/403 or non-JSON, local changes are kept and a banner asks you to reload. The reload goes through Cloudflare Access login, and the kept changes sync afterwards.

## Development

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

The server reads `DB_PATH` (default `data/gym.db`) and `PORT` (default `8080`).

## Deployment

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/agusespa/gym-tracker:latest --push .
```

On the host, next to `docker-compose.yml`:

```bash
mkdir -p data && sudo chown 65532:65532 data   # the image runs as distroless "nonroot"
docker compose up -d
```

Point the Cloudflare Tunnel at `http://localhost:8080` and protect the hostname with an Access policy.
