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
