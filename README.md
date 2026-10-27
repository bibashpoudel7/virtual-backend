# Virtual Tour Backend (Go)

REST API for the virtual tour platform: tours, scenes, hotspots, overlays (including 3D GLB models),
play tours, properties, contacts and payments.

**Stack:** Go 1.25 · Gin · GORM · PostgreSQL · Cloudflare R2

## Prerequisites

- **Go 1.25+**. Check with `go version`. Install from https://go.dev/dl or with `brew install go`.
- **PostgreSQL 13+**. Run it locally or use Docker (see below).
- **Optional:** [Air](https://github.com/air-verse/air) for hot reload. Install it with `make install-air`.

## Quick start

```bash
cd backend

# 1. Configure the environment
cp .env.example .env        # then fill in the values (see "Environment variables")

# 2. Download dependencies
go mod download

# 3. Start the server
go run ./cmd/api
```

The server listens on `http://localhost:5555`, or on `PORT` if that's set.

On startup the server:
1. Loads `.env` from the current directory, so run it from `backend/`.
2. Connects to **two** databases: the virtual tour DB (`POSTGRES_DSN`) and the main Nimto DB (`MAIN_POSTGRES_DSN`).
   The server exits if either one is unreachable.
3. Runs GORM auto-migration for the tour tables: tours, scenes, hotspots, overlays and play tours.

## Run modes

| Command | What it does |
|---|---|
| `go run ./cmd/api` | Run once without hot reload |
| `make dev` or `./dev.sh` | Hot reload with Air (`dev.sh` installs Air if it's missing) |
| `make build` | Build the binary to `bin/api` |
| `make run` | Build, then run `bin/api` |
| `make test` | Run `go test -v ./...` |
| `make fmt` / `make lint` | Format the code, or lint it (needs `golangci-lint`) |
| `make help` | List all targets |

## Database

### Local Postgres with Docker
```bash
# from the repo root (virtual/)
docker compose up -d postgres
# DSN: postgres://postgres:postgres@localhost:5432/virtual_tour?sslmode=disable
```

### Or with Homebrew
```bash
brew install postgresql@16 && brew services start postgresql@16
createdb virtual
# DSN: postgres://<your-user>@localhost:5432/virtual?sslmode=disable
```

### SQL migrations
Tables are created automatically on startup. Schema changes beyond that live in `migrations/` as numbered
`NNN_name.up.sql` / `NNN_name.down.sql` files, which are tracked in a migrations table.

```bash
make migrate                               # apply pending migrations
go run ./cmd/migrate -check                # list pending migrations without running them
make migrate-down                          # roll back the last migration
go run ./cmd/migrate -rollback 2           # roll back the last 2 migrations
```

To add a migration, create the next numbered pair, for example `009_add_x.up.sql` and `009_add_x.down.sql`.

## Environment variables

Set these in `backend/.env`. See `.env.example` for the full list.

| Variable | Required | Description |
|---|---|---|
| `PORT` | no | HTTP port (default `5555`) |
| `POSTGRES_DSN` | yes | Virtual tour database: tours, scenes, hotspots, overlays |
| `MAIN_POSTGRES_DSN` | yes | Main Nimto database: users, companies, properties (read-only) |
| `JWT_SECRET` | yes | Secret used to validate auth tokens. Must match the main app. |
| `CORS_ORIGINS` | no | Comma-separated allowed origins (default `*`), e.g. `http://localhost:3000` |
| `R2_ENDPOINT`, `R2_REGION`, `R2_BUCKET`, `R2_ACCESS_KEY`, `R2_SECRET_KEY`, `R2_ACCOUNT_ID`, `R2_USE_PATH_STYLE` | for uploads | Cloudflare R2 storage |
| `CF_IMAGES_ACCOUNT_ID`, `CF_IMAGES_API_TOKEN` | optional | Cloudflare Images direct upload |
| `REDIS_ADDR` | optional | Redis address (default `localhost:6379`) |
| `AUTH_JWKS_URL` | optional | JWKS URL for token validation |
| `FFMPEG_BIN` | optional | Path to ffmpeg (default `/usr/bin/ffmpeg`; on macOS with Homebrew, `/opt/homebrew/bin/ffmpeg`) |
| `EMAIL_ADDRESS`, `EMAIL_PASSWORD`, `EMAIL_HOST`, `EMAIL_PORT` | for contact form | SMTP settings for contact form emails |

> Never commit `.env`. Use `.env.example` for placeholders only.

## Project structure

```
backend/
├── cmd/
│   ├── api/main.go          # HTTP server entry point
│   └── migrate/main.go      # SQL migration runner
├── internal/
│   ├── auth/                # JWT auth middleware
│   ├── config/              # env-based config
│   ├── db/                  # DB connections + auto-migrate
│   ├── http/                # Gin router, CORS, middleware
│   ├── models/              # GORM models (Tour, Scene, Hotspot, Overlay, PlayTour, …)
│   ├── tours/               # tours / scenes / hotspots / overlays / play tours (handler → service → repo)
│   ├── properties/          # property lookups (main DB)
│   ├── contact/             # contact form + email
│   ├── payment/             # payment / tour limits
│   ├── users/               # user lookups (main DB)
│   ├── email/               # SMTP email service
│   └── migrate/             # migration engine
├── migrations/              # numbered .up.sql / .down.sql files
├── scripts/                 # Python helpers (equirect → cubemap)
├── utils/jwt/               # JWT helpers
├── openapi.yaml             # API spec
└── Makefile
```

## API overview

All routes are prefixed with `/api`. **Protected** routes need `Authorization: Bearer <token>`.

**Public**
```
GET  /api/tours/public
GET  /api/tours/:id/public
GET  /api/tours/:id/scenes/public
GET  /api/scenes/:sceneId/hotspots/public
GET  /api/scenes/:sceneId/overlays/public
GET  /api/tours/:id/play-tours/public
GET  /api/properties/:propertyId/tour
GET  /api/properties/:propertyId/tour-details
POST /api/contact
POST /api/auth/validate
```

**Protected: tours**
```
GET    /api/tours                     POST /api/tours
GET    /api/tours/featured            GET  /api/tours/by-property/:propertyID
GET    /api/tours/:id                 PUT  /api/tours/:id         DELETE /api/tours/:id
PUT    /api/tours/:id/publish         PUT  /api/tours/:id/feature
GET    /api/tours/:id/scenes          POST /api/tours/:id/scenes
GET|POST          /api/tours/:id/scenes/:sceneId/hotspots
PUT|DELETE        /api/tours/:id/scenes/:sceneId/hotspots/:hotspotId
GET|POST          /api/tours/:id/play-tours
```

**Protected: scenes, hotspots, overlays**
```
GET|PUT|DELETE    /api/scenes/:sceneId
POST              /api/scenes/:sceneId/update-images
GET|POST          /api/scenes/:sceneId/hotspots
PUT|DELETE        /api/scenes/:sceneId/hotspots/:hotspotId
GET|POST          /api/scenes/:sceneId/overlays
PUT|DELETE        /api/scenes/:sceneId/overlays/:overlayId
GET|PUT|DELETE    /api/play-tours/:id
```

**Protected: other**
```
GET  /api/properties/approved | /details/:propertyId | /company-info
GET  /api/contacts            GET /api/contacts/:id     PATCH /api/contacts/:id/status
GET  /api/payment/check-limit POST /api/payment/create-payment  POST /api/payment/process/:paymentId
```

Overlays accept any `kind` with a JSON `payload`. The frontend uses `text`, `image`, `video` and `model`
(3D GLB; see `../GLB_MODELS.md`).

## Testing the API

```bash
# Public endpoint (no auth)
curl http://localhost:5555/api/tours/public

# Protected endpoint
curl -H "Authorization: Bearer <token>" http://localhost:5555/api/tours
```

## Running with the frontend

```bash
# terminal 1
cd backend && go run ./cmd/api

# terminal 2
cd frontend && yarn install && yarn dev     # http://localhost:3000
```

Point the frontend's API base URL at `http://localhost:5555` in `frontend/.env`, and include
`http://localhost:3000` in `CORS_ORIGINS`.

## Troubleshooting

| Problem | Fix |
|---|---|
| `failed to connect to virtual database` | Postgres isn't running or `POSTGRES_DSN` is wrong |
| `failed to connect to nimto database` | `MAIN_POSTGRES_DSN` is unreachable (check the VPN, firewall or credentials) |
| `No .env file found` | You're not running from `backend/`, or `.env` is missing |
| `address already in use` | Another process is using the port: `lsof -i :5555`, then stop it or change `PORT` |
| `air: command not found` | Run `make install-air` and make sure `$(go env GOPATH)/bin` is on your `PATH` |
| 401 on protected routes | The token is missing or invalid, or `JWT_SECRET` doesn't match the issuing app |
