# Finnio

Live streaming platform with a Go control-plane API and a MediaMTX + FFmpeg media plane.

Create and manage streams over HTTP, publish via RTMP (or other MediaMTX protocols), and play back adaptive HLS (360p–1080p) from a master playlist.

## Architecture

```
Publisher (OBS / FFmpeg)
        │  RTMP :1935
        ▼
   ┌─────────┐  hooks / auth   ┌─────────┐
   │ MediaMTX │ ──────────────► │   API   │ ──► PostgreSQL
   └────┬────┘                 └────┬────┘
        │ FFmpeg ABR ladder         │
        ▼                           ▼
   /hls/abr/{key}/            Serve ABR HLS
   master.m3u8                GET /hls/{key}/*
```

| Path | Role |
|------|------|
| `api/` | Stream management HTTP API (Chi + fx) |
| `media/` | MediaMTX config, ABR scripts, Docker image |
| `shared/` | Shared platform config helpers |
| `deployments/` | Docker Compose and runtime volumes |
| `tmp/` | Local playback helper and sample assets |

## Tech stack

| Layer | Technology |
|-------|------------|
| Language | Go 1.26 (workspace: `go.work`) |
| HTTP | [chi](https://github.com/go-chi/chi) |
| DI | [uber/fx](https://uber-go.github.io/fx/) |
| Database | PostgreSQL + [pgx](https://github.com/jackc/pgx) |
| SQL | [sqlc](https://sqlc.dev/) (codegen) + [goose](https://github.com/pressly/goose) (migrations) |
| Docs | [swag](https://github.com/swaggo/swag) → Swagger UI at `/swagger/index.html` |
| Validation | [go-playground/validator](https://github.com/go-playground/validator) |
| Media server | [MediaMTX](https://github.com/bluenviron/mediamtx) (RTMP, RTSP, HLS, WebRTC, SRT) |
| Transcoding | FFmpeg (multi-bitrate HLS ladder) |
| Hot reload | [Air](https://github.com/air-verse/air) |
| Orchestration | Docker Compose |

## Prerequisites

- **Go** 1.26+ (`go version`)
- **Docker** and **Docker Compose** (for the full stack)
- **PostgreSQL** 14+ (for local API development outside Compose)
- Optional: **FFmpeg** / OBS if you publish streams from the host

## Quick start (Docker Compose)

From the repo root:

```bash
cd deployments
docker compose up --build
```

Services:

| Service | Ports | Purpose |
|---------|-------|---------|
| `api` | `5555` | Control API, health, Swagger, ABR HLS proxy |
| `media` | `1935` (RTMP), `8554` (RTSP), `8888` (HLS), `9996` (playback), `9997` (MediaMTX API) | Ingest + remux |

Health check: `GET http://localhost:5555/healthz`  
Swagger: http://localhost:5555/swagger/index.html

HLS and recordings are written under `deployments/tmp/`.

## Local API development

### 1. PostgreSQL

Create a database matching the defaults (or set `DATABASE_URL`):

```text
postgres://root:root@127.0.0.1:5432/dev_finnio
```

Goose settings live in `api/.env`:

```env
GOOSE_DRIVER=postgres
GOOSE_DBSTRING=postgres://root:root@127.0.0.1:5432/dev_finnio
GOOSE_MIGRATION_DIR=./db/migrations
```

### 2. Migrate

```bash
cd api
# load GOOSE_* from .env, then:
go tool goose up
```

### 3. Install / sync tools

Tools are declared in `api/go.mod` (`tool` block). From `api/`:

```bash
make setup   # optional: install swag, goose, sqlc, air globally
# or use go tool …
go tool sqlc generate
go tool swag init -g cmd/server/main.go -o gen/swagger --parseDependency --parseInternal
```

### 4. Run

```bash
cd api
make run     # generate swagger + sqlc, build, run once
# or hot reload:
make dev     # air → make build-dev on change
```

Default listen address: `:5555`.

Useful env vars (API / platform):

| Variable | Default | Description |
|----------|---------|-------------|
| `API_addr` | `:5555` | HTTP listen address |
| `DATABASE_URL` | `postgres://root:root@127.0.0.1:5432/dev_finnio` | Postgres DSN |
| `INGRESS_URL` | `rtmp://localhost:5554` | Base RTMP URL returned for publish |
| `PUBLIC_URL` | `http://localhost:5555` | Public base URL for HLS links |
| `HLS_ABR_DIR` | `tmp/hls/abr` | On-disk ABR playlist root |
| `DATA_DIR` | `data` | API data directory |

For Compose, MediaMTX also accepts `ABR_DISABLE_360P` / `480P` / `720P` / `1080P` (`"1"` to skip a ladder rung).

## Development process

Typical loop when working on Finnio:

### One-time setup

1. Clone the repo and ensure Go 1.26+, Docker, and PostgreSQL are available.
2. Copy env sample and adjust if needed:

   ```bash
   cp api/.env.sample api/.env
   ```

3. Create the Postgres database (`dev_finnio`) and apply migrations (`go tool goose up` from `api/`).
4. From `api/`, run `make setup` once if you prefer global tool binaries; otherwise `go tool …` is enough.

### Day-to-day

1. **Start dependencies**
   - Full stack: `cd deployments && docker compose up --build`
   - API-only: keep Postgres running; optionally run only the `media` service if you need ingest/ABR.

2. **Run the API with hot reload**

   ```bash
   cd api
   make dev
   ```

   Air watches `.go` files and rebuilds via `make build-dev` (which regenerates Swagger + sqlc, then builds).

3. **Make the change in the right place**

   | You are changing… | Edit | Then |
   |-------------------|------|------|
   | HTTP routes / handlers / services | `api/internal/httpapi/…` | Air rebuilds automatically |
   | Request/response shapes or Swagger comments | handlers + `cmd/server/main.go` annotations | `make swagger` (or wait for Air rebuild) |
   | SQL queries | `api/db/queries/*.sql` | `make sqlc-generate` (or Air rebuild) |
   | Schema | add a goose migration under `api/db/migrations/` | `go tool goose up`, then update queries + regenerate sqlc |
   | Media hooks / ABR ladder | `media/scripts/`, `media/config/mediamtx.yml` | restart the `media` container / Compose service |
   | Shared env/platform helpers | `shared/platform/` | rebuild API |

4. **Verify**
   - Health: `curl http://localhost:5555/healthz`
   - API docs: http://localhost:5555/swagger/index.html
   - Unit tests: `cd api && make test`
   - End-to-end: create a stream → publish RTMP → open `/hls/{key}/master.m3u8` (see [Example flow](#example-flow))

5. **Before you push**
   - Run migrations on a clean DB if you added SQL.
   - Ensure generated artifacts are up to date (`make swagger sqlc-generate` / `make build`).
   - `go test ./...` from `api/` (and `shared/` if you touched it).
   - Keep secrets out of git (`api/.env` is gitignored; commit `.env.sample` only).

### Useful Makefile targets (`api/`)

| Target | What it does |
|--------|----------------|
| `make setup` | Install swag, goose, sqlc, air |
| `make swagger` | Regenerate OpenAPI under `gen/swagger` |
| `make sqlc-generate` | Regenerate typed DB code under `gen/db` |
| `make build` | swagger + sqlc + compile to `bin/api` |
| `make run` | build and run once |
| `make dev` | Air hot reload |
| `make test` | `go test ./...` |
| `make clean` | remove `bin/` |

Generated code (`api/gen/`, `api/bin/`) is gitignored — always regenerate locally (or via `make build` / Air) before running.

## Go workspace

This repo is a multi-module workspace:

```
go.work → ./api, ./shared
```

With `go.work` present, local `shared` is used without publishing. Common commands from the repo root:

```bash
go build ./api/...
go test ./api/...
go test ./shared/...
```

## API overview

Base URL: `http://localhost:5555`

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Liveness |
| `GET`/`POST` | `/streams` | List / create streams |
| `GET`/`PUT`/`DELETE` | `/streams/{id}` | Read / update / delete by ID |
| `POST` | `/streams/{key}/ready` … `/end` | Lifecycle (`ready`, `go-live`, `pause`, `resume`, `end`) |
| `GET` | `/streams/{key}/ingress` | RTMP ingress URL for the stream key |
| `GET` | `/hls/{key}/*` | Serve ABR HLS (e.g. `…/master.m3u8`) |
| `POST` | `/mediamtx/auth` | MediaMTX HTTP auth |
| `POST` | `/mediamtx/hooks/{event}` | MediaMTX lifecycle hooks |

Stream statuses: `idle` → `ready` → `live` / `paused` → `ended` (or `failed`).

## Dependencies

### Direct (runtime / app)

Declared in `api/go.mod`:

- `github.com/go-chi/chi/v5` — HTTP router
- `github.com/go-playground/validator/v10` — request validation
- `github.com/google/uuid` — stream keys
- `github.com/jackc/pgx/v5` — PostgreSQL driver / pool
- `github.com/swaggo/http-swagger` / `github.com/swaggo/swag` — OpenAPI UI + codegen
- `go.uber.org/fx` — dependency injection / lifecycle

`shared` has no third-party `require`s; it only exposes platform env loading.

### Dev tools (`go tool` / Makefile)

- `github.com/air-verse/air` — live reload
- `github.com/pressly/goose/v3` — migrations
- `github.com/sqlc-dev/sqlc` — typed SQL → Go
- `github.com/swaggo/swag` — Swagger generation

### Media / infra

- MediaMTX `1.x` (`bluenviron/mediamtx`)
- FFmpeg (ABR encoding in the media container)
- PostgreSQL
- Docker / Alpine-based images under `api/Dockerfile` and `media/Dockerfile`

## Example flow

1. Create a stream: `POST /streams` with `{"name":"demo"}`.
2. Read `ingress_url` (or `GET /streams/{key}/ingress`).
3. Publish with OBS/FFmpeg to that RTMP URL (MediaMTX path = stream key).
4. On publish, MediaMTX runs `media/scripts/on_ready.sh` → notifies the API and builds `/hls/abr/{key}/master.m3u8`.
5. Play via `GET /hls/{key}/master.m3u8` or open `tmp/playback.html` against your public URL.

## License

API Swagger metadata currently marks the API as Apache 2.0; add a root `LICENSE` if you intend to publish the project under that (or another) license.
