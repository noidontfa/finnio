# Finnio

Live streaming platform with a Go control-plane API and a MediaMTX + FFmpeg media plane.

Create and manage streams over HTTP, publish via RTMP (or other MediaMTX protocols), and play back adaptive HLS (360p–1080p) from a master playlist.

## What's new in v2

v2 replaces the v1 **one-shot multi-rung FFmpeg ABR** (a single long-lived encode of the whole RTMP input) with a **segment-then-ABR** pipeline: source segments first, then per-segment encode workers over NATS JetStream.

| | v1 | v2 |
|---|---|---|
| Live encode | One FFmpeg process builds the full ladder from RTMP | FFmpeg `-c copy` segments → NATS job per segment → ABR workers encode each rung |
| Pipeline logic | Mostly `media/scripts/on_ready.sh` | Go packages + Cobra CLI (`media` binary) |
| Scaling | Single encode process per stream | Horizontal ABR consumers (`abr-consumer`) |
| Queue | None | NATS JetStream (`ABR` stream, `abr.requests`) |
| On-disk layout | `/hls/abr/{key}/v{rung}/…` from one encode | Source: `/hls/abr/{key}/seg_*.ts`; ABR: `/tmp/abr/{key}/{rung}/…` |
| Compose | `api` + `media` | `media` + `abr-consumer` + `nats` (API often on the host) |

### Added

- **`media` Go module** in the workspace (`go.work` → `api`, `media`, `shared`) with a Cobra CLI.
- **Source segmenter** (`segment`, `segment-rtmp`): FFmpeg `-c copy` into `playlist.m3u8` + `seg_%05d.ts`.
- **Segment watcher**: publishes each completed `.ts` (and a final `video_done`) to JetStream.
- **ABR encode path**: per-segment ladder encode (360p / 480p / 720p / 1080p), atomic `.tmp` → rename, continuous timestamps (`-copyts`).
- **Playlist assembly**: writes `master.m3u8` and per-rung `index.m3u8` (ENDLIST on `video_done`).
- **NATS JetStream** service and `NATS_URL` platform config.
- **`abr-consumer` Compose service**: runs `media abr-consumer` against shared `/hls` + `/tmp/abr` volumes.
- **Live HLS serving hardening** on `GET /hls/{key}/*`: no stale `304` on playlists; `503` + `Retry-After` while ABR is not ready yet.

### Changed

- **`on_ready.sh`**: notifies the API, then runs `media segment-rtmp` (no inline multi-variant FFmpeg ABR).
- **HLS output root for playback**: API `HLS_ABR_DIR` should point at the ABR output tree (Compose: `deployments/tmp/abr`), not the source-segment folder.
- **Variant dirs**: `360p/`, `480p/`, … (no `v` prefix).
- **Default ingress**: `rtmp://localhost:1935` (via shared platform config).
- **Media image**: builds and ships `/usr/local/bin/media` alongside MediaMTX + FFmpeg.

### Unchanged for viewers

Playback URL stays `GET /hls/{key}/master.m3u8` (and variant playlists / `.ts` under that key).

Design notes: [`docs/specs/segment-then-abr.md`](docs/specs/segment-then-abr.md).

## Architecture

```
Publisher (OBS / FFmpeg)
        │  RTMP :1935
        ▼
   ┌─────────┐  hooks / auth   ┌─────────┐
   │ MediaMTX │ ──────────────► │   API   │ ──► PostgreSQL
   └────┬────┘                 └────┬────┘
        │ on_ready                  │
        ▼                           ▼
┌────────────────────────────────┐  Serve ABR HLS
│  media module (Go CLI)         │  GET /hls/{key}/*
│                                │       ▲
│  segment-rtmp (-c copy)        │       │
│    → /hls/abr/{key}/seg_*.ts   │       │
│           │                    │       │
│           ▼                    │       │
│  NATS JetStream (abr.requests) │       │
│           │                    │       │
│           ▼                    │       │
│  abr-consumer(s)               │       │
│    encode → /tmp/abr/{key}/ ───┼───────┘
│    master.m3u8 + {rung}/…      │
└────────────────────────────────┘
```

| Path | Role |
|------|------|
| `api/` | Stream management HTTP API (Chi + fx) |
| `media/` | MediaMTX config, Go CLI (`segment`, `abr-consumer`, …), Docker image |
| `shared/` | Shared platform config (`INGRESS_URL`, `NATS_URL`, …) |
| `deployments/` | Docker Compose and runtime volumes (`tmp/hls`, `tmp/abr`) |
| `docs/specs/` | Feature specs (e.g. segment-then-ABR) |
| `tmp/` | Local playback helpers and sample assets |

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
| Media CLI | [Cobra](https://github.com/spf13/cobra) (`media` binary) |
| Queue | [NATS](https://nats.io/) JetStream (ABR job queue) |
| Transcoding | FFmpeg (source segment copy + per-segment ABR ladder) |
| Hot reload | [Air](https://github.com/air-verse/air) |
| Orchestration | Docker Compose |

## Prerequisites

- **Go** 1.26+ (`go version`)
- **Docker** and **Docker Compose** (for media / NATS / ABR workers)
- **PostgreSQL** 14+ (for local API development)
- Optional: **FFmpeg** / OBS if you publish streams from the host

## Quick start (Docker Compose)

From the repo root:

```bash
cd deployments
docker compose up --build
```

Services (current Compose layout):

| Service | Ports | Purpose |
|---------|-------|---------|
| `media` | `1935` (RTMP), `8554` (RTSP), `8888` (HLS), `9996` (playback), `9997` (MediaMTX API) | Ingest; `on_ready` runs `segment-rtmp` |
| `abr-consumer` | — | JetStream workers; encode segments → `/tmp/abr/{key}/` |
| `nats` | `4222` (client), `8222` (monitoring) | JetStream (`-js`) |

The `api` service is typically commented out in Compose while you run the API on the host (`cd api && make run` / `make dev`) with:

```bash
HLS_ABR_DIR=../deployments/tmp/abr
```

Point MediaMTX at that host API via `deployments/.env` if needed:

```env
API_BASE_URL=http://host.docker.internal:5555
MTX_AUTHHTTPADDRESS=http://host.docker.internal:5555/mediamtx/auth
```

Then recreate media: `docker compose up -d --force-recreate media`.

Health check (API on host): `GET http://localhost:5555/healthz`  
Swagger: http://localhost:5555/swagger/index.html

Volumes:

| Host path | Role |
|-----------|------|
| `deployments/tmp/hls/` | Source segments (`/hls/abr/{key}/seg_*.ts` in containers) |
| `deployments/tmp/abr/` | ABR output served by the API (`{key}/master.m3u8`, `{key}/360p/…`) |

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
make run     # generate swagger + sqlc, build, run with HLS_ABR_DIR=../deployments/tmp/abr
# or hot reload:
make dev     # air → make build-dev on change
```

Default listen address: `:5555`.

Useful env vars:

| Variable | Default | Description |
|----------|---------|-------------|
| `API_addr` | `:5555` | HTTP listen address |
| `DATABASE_URL` | `postgres://root:root@127.0.0.1:5432/dev_finnio` | Postgres DSN |
| `INGRESS_URL` | `rtmp://localhost:1935` | Base RTMP URL returned for publish |
| `PUBLIC_URL` / `EGRESS_URL` | `http://localhost:5555` | Public base URL for HLS links |
| `HLS_ABR_DIR` | `tmp/hls/abr` (override to `../deployments/tmp/abr` for Compose ABR) | On-disk ABR playlist root |
| `NATS_URL` | `nats://localhost:4222` | NATS / JetStream (media plane) |
| `DATA_DIR` | `data` | API data directory |

Compose knobs for ABR workers: `ABR_CONSUMERS` (default `2`), `ABR_OUT_FOLDER` (default `/tmp/abr`).

## Media CLI

Build from `media/`:

```bash
cd media
make build   # → bin/media
```

| Command | Purpose |
|---------|---------|
| `segment -i <file> -o <dir> -t 2 -s <id>` | File → source segments (`-c copy`) |
| `segment-rtmp -i rtmp://… -o <dir> -t 2 -s <id>` | Live RTMP → source segments; publish ABR jobs to NATS |
| `abr -c <n> -i <seg.ts> -o <dir>` | Dev helper: start consumers + publish one segment job |
| `abr-consumer -c <n> -o <dir>` | Long-running JetStream workers (Compose entrypoint) |

Useful Makefile targets (`media/`): `segment`, `segment-rtmp`, `segment-rtmp-test`, `abr`, `abr-consumer`, `play` (local HLS player via `tmp/play.py`).

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
   - Media plane: `cd deployments && docker compose up --build` (`media`, `abr-consumer`, `nats`)
   - API: `cd api && make run` or `make dev` with `HLS_ABR_DIR` pointing at `deployments/tmp/abr`

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
   | Segment / ABR / NATS | `media/internal/…`, `media/internal/cmd/` | `cd media && make build` / restart `media` + `abr-consumer` |
   | Media hooks | `media/scripts/`, `media/config/mediamtx.yml` | recreate the `media` Compose service |
   | Shared env/platform helpers | `shared/platform/` | rebuild API and/or media |

4. **Verify**
   - Health: `curl http://localhost:5555/healthz`
   - API docs: http://localhost:5555/swagger/index.html
   - Unit tests: `cd api && make test`; `cd media && make test`
   - End-to-end: create a stream → publish RTMP → open `/hls/{key}/master.m3u8` (see [Example flow](#example-flow))

5. **Before you push**
   - Run migrations on a clean DB if you added SQL.
   - Ensure generated artifacts are up to date (`make swagger sqlc-generate` / `make build`).
   - `go test ./...` from `api/`, `media/`, and `shared/` as touched.
   - Keep secrets out of git (`api/.env` is gitignored; commit `.env.sample` only).

### Useful Makefile targets (`api/`)

| Target | What it does |
|--------|----------------|
| `make setup` | Install swag, goose, sqlc, air |
| `make swagger` | Regenerate OpenAPI under `gen/swagger` |
| `make sqlc-generate` | Regenerate typed DB code under `gen/db` |
| `make build` | swagger + sqlc + compile to `bin/api` |
| `make run` | build and run once (`HLS_ABR_DIR=../deployments/tmp/abr`) |
| `make dev` | Air hot reload |
| `make test` | `go test ./...` |
| `make clean` | remove `bin/` |

Generated code (`api/gen/`, `api/bin/`) is gitignored — always regenerate locally (or via `make build` / Air) before running.

## Go workspace

This repo is a multi-module workspace:

```
go.work → ./api, ./media, ./shared
```

With `go.work` present, local `shared` / `media` are used without publishing. Common commands from the repo root:

```bash
go build ./api/...
go build ./media/...
go test ./api/...
go test ./media/...
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

`media` (selected):

- `github.com/spf13/cobra` — CLI
- `github.com/nats-io/nats.go` — JetStream client
- `github.com/fsnotify/fsnotify` — segment folder watcher

`shared` exposes platform env loading (`INGRESS_URL`, `EGRESS_URL`, `DATABASE_URL`, `NATS_URL`).

### Dev tools (`go tool` / Makefile)

- `github.com/air-verse/air` — live reload
- `github.com/pressly/goose/v3` — migrations
- `github.com/sqlc-dev/sqlc` — typed SQL → Go
- `github.com/swaggo/swag` — Swagger generation

### Media / infra

- MediaMTX `1.x` (`bluenviron/mediamtx`)
- FFmpeg (segment copy + ABR encode in the media image)
- NATS 2.x with JetStream
- PostgreSQL
- Docker / Alpine-based images under `api/Dockerfile` and `media/Dockerfile`

## Example flow

1. Start Compose (`media`, `abr-consumer`, `nats`) and the API with `HLS_ABR_DIR=../deployments/tmp/abr`.
2. Create a stream: `POST /streams` with `{"name":"demo"}`.
3. Read `ingress_url` (or `GET /streams/{key}/ingress`).
4. Publish with OBS/FFmpeg to that RTMP URL (MediaMTX path = stream key).
5. On publish, MediaMTX runs `on_ready.sh` → API ready hook → `media segment-rtmp` writes `/hls/abr/{key}/seg_*.ts` and publishes ABR jobs.
6. `abr-consumer` encodes into `deployments/tmp/abr/{key}/{360p,480p,720p,1080p}/` and writes `master.m3u8`.
7. Play via `GET /hls/{key}/master.m3u8` or open `tmp/playback.html` / `make play` against the ABR folder.

## License

API Swagger metadata currently marks the API as Apache 2.0; add a root `LICENSE` if you intend to publish the project under that (or another) license.
