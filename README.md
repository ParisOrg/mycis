# Controls Tracker (mycis)

Web application for tracking security control assessments against published frameworks (for example CIS Controls). Teams sign in, open assessments tied to a framework, score and prioritize individual controls, attach evidence and comments, and admins manage users and assessment lifecycle.

**Default product name in the UI:** `Controls Tracker` (override with `APP_NAME`).

---

## Features

- **Authentication** — Email and password sign-in with Gorilla session cookies; optional forced password change for new accounts.
- **Dashboard** — Overview of assessments with drill-down by assessment.
- **Frameworks** — Browse loaded frameworks, groups, and control definitions.
- **Assessments** — List and detail views; per-control status, score (1–5), priority, owners, reviewers, due dates, comments, and evidence links.
- **Filtering** — Assessment item lists support query-based filters (see the assessment detail UI).
- **Administration** (admin users only):
  - Create assessments (`/assessments/new`, POST create).
  - Bulk update items on an assessment.
  - Create users with temporary passwords (`/admin/users`).
- **CLI** — Database migrations, bootstrap admin user, seed framework YAML into Postgres.

---

## Tech stack

| Layer | Technology |
|--------|------------|
| Runtime | Go 1.26 |
| HTTP | [Echo v5](https://echo.labstack.com/) |
| Database | PostgreSQL 16, [pgx/v5](https://github.com/jackc/pgx), [golang-migrate](https://github.com/golang-migrate/migrate) |
| Queries | [sqlc](https://sqlc.dev/) (`db/queries` → `internal/db`) |
| Sessions | [gorilla/sessions](https://github.com/gorilla/sessions) (cookie store) |
| Front-end assets | Tailwind CSS v4, esbuild, [basecoat-css](https://basecoatui.com/) |
| Templates | `html/template`, layouts under `internal/http/templates` |

---

## Repository layout

```
cmd/app/              # Main binary: web server and CLI subcommands
internal/
  config/             # Environment configuration
  db/                 # sqlc-generated types and queries
  http/               # HTTP server, handlers, middleware, templates
  service/            # Application services
  seed/               # Framework YAML loading (used by seed-framework)
db/
  migrations/         # golang-migrate SQL files
  queries/            # sqlc query definitions
assets/               # Source CSS/JS (built into public/assets)
public/assets/        # Built static files (generated; do not hand-edit for workflow)
seed/frameworks/      # Framework YAML (e.g. CIS v8.1)
tools/generate-cis-yaml/  # Optional: CIS Excel workbook → YAML for seeding
```

---

## Prerequisites

- **Go** 1.26 or compatible toolchain (`go version`).
- **Node.js** and **npm** (for asset builds).
- **PostgreSQL** 16 (or run Postgres via Docker Compose).
- Optional but recommended for local development:
  - [**just**](https://github.com/casey/just) — task runner (`Justfile`).
  - [**Air**](https://github.com/air-verse/air) — live reload; configured in `.air.toml`.

---

## Configuration

Configuration is read from the process environment. If a file named `.env` exists in the working directory, [godotenv](https://github.com/joho/godotenv) loads it first.

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | **Yes** | PostgreSQL connection string (e.g. `postgres://user:pass@localhost:5432/mycis?sslmode=disable`). |
| `APP_SESSION_KEY` | **Yes** | Secret used to sign session cookies. **Must be at least 32 characters and cannot be one of the shipped example values.** |
| `APP_NAME` | No | Display name (default: `Controls Tracker`). |
| `APP_ADDR` | No | Listen address (default: `:8080`). |
| `APP_COOKIE_SECURE` | No | If `true`, session cookies are marked `Secure` (use behind HTTPS in production). Default: `false`. |

### Local example `.env`

```env
DATABASE_URL=postgres://mycis:mycis@localhost:5432/mycis?sslmode=disable
APP_SESSION_KEY=replace-me-with-a-32-byte-random-session-key
APP_ADDR=:8080
APP_COOKIE_SECURE=false
```

Replace the example `APP_SESSION_KEY` before starting the app. Startup rejects the example placeholders on purpose.

**Production:** Generate a long random `APP_SESSION_KEY`, set `APP_COOKIE_SECURE=true` when the site is served over HTTPS, and never commit real secrets.

---

## Run with Docker Compose

The stack includes Postgres and the Go application image built from the `Dockerfile`.

```bash
docker compose up --build
```

- **Application:** `http://localhost:8081` (Compose sets `APP_ADDR` to `:8081` and maps host `8081` → container `8081`).
- **Database:** In the base `docker-compose.yml`, Postgres is published on host port **5432**. If `docker-compose.override.yml` is present (as in this repo), the host port is **55432** instead (`55432:5432`) to avoid clashes with a local Postgres.

After the containers are up, apply migrations, create an admin, and seed the default framework. **Recommended:** run the CLI inside the running app container (Compose already sets `DATABASE_URL`; you must provide `APP_SESSION_KEY` via `.env` or your shell before startup):

```bash
docker compose exec app /app/app migrate
docker compose exec app /app/app create-admin -email admin@example.com -name "Admin User"
docker compose exec app /app/app seed-framework -slug cis-v8-1
docker compose exec app /app/app seed-framework -slug nist-csf-2-0
docker compose exec app /app/app seed-framework -slug iso-27001-2022
docker compose exec app /app/app seed-framework -slug iso-27002-2022
docker compose exec app /app/app seed-framework -slug soc2-2017
docker compose exec app /app/app seed-framework -slug nis2-2022
docker compose exec app /app/app seed-framework -slug dora-2025
docker compose exec app /app/app seed-framework -slug gdpr-2018
docker compose exec app /app/app seed-framework -slug eu-ai-act-2024
```

**Alternative (host binary):** build `./bin/mycis` locally, set `DATABASE_URL` to the **host** Postgres port (`5432` or `55432` if `docker-compose.override.yml` is active), set `APP_SESSION_KEY` to match Compose (or any valid 32+ character secret for DB-only commands), then run the same three commands against `./bin/mycis`.

> **Note:** `docker-compose.yml` no longer ships a reusable session secret. Set `APP_SESSION_KEY` in `.env` or the environment before running Compose.

---

## Local development (without Docker for the app)

### 1. Create database and role

Create a database and user matching your `DATABASE_URL` (example names: `mycis` / `mycis`).

### 2. Install dependencies

```bash
go mod download
npm install
```

### 3. Generate code and build assets

```bash
just build
```

Or manually:

```bash
sqlc generate
npm run build
go build -o bin/mycis ./cmd/app
```

### 4. Run migrations

```bash
go run ./cmd/app migrate
# or: ./bin/mycis migrate
```

Migrations live in `db/migrations` and are applied with `migrate.Up()` (no down command in the CLI).

If you previously ran an earlier local version of migration `000003_control_records`, recreate the database before migrating again. That migration was rewritten to reset the assessment/control-record model instead of preserving the earlier buggy shape.

### 5. Bootstrap data and admin

Seed one or more frameworks (YAML under `seed/frameworks/`):

```bash
just seed

# or seed individual frameworks:
go run ./cmd/app seed-framework -slug cis-v8-1
go run ./cmd/app seed-framework -slug nist-csf-2-0
go run ./cmd/app seed-framework -slug iso-27001-2022
go run ./cmd/app seed-framework -slug iso-27002-2022
go run ./cmd/app seed-framework -slug soc2-2017
go run ./cmd/app seed-framework -slug nis2-2022
go run ./cmd/app seed-framework -slug dora-2025
go run ./cmd/app seed-framework -slug gdpr-2018
go run ./cmd/app seed-framework -slug eu-ai-act-2024
```

Use `just seed-force` to refresh all shipped frameworks in place.

Create an admin user:

```bash
go run ./cmd/app create-admin -email you@example.com -name "Your Name"
```

Omit `-password` to receive a generated temporary password on stdout. To set a password explicitly:

```bash
go run ./cmd/app create-admin -email you@example.com -name "Your Name" -password 'your-secure-password'
```

### 6. Start the web server

```bash
just run
# or: go run ./cmd/app web
```

Open `http://localhost:8080` (or whatever you set in `APP_ADDR`). Sign in with the admin email and password; you may be redirected to change the password if the account was created with a temporary password.

### Live reload

```bash
just dev
```

Air rebuilds the binary when Go or asset sources change; the build step runs `just assets` first (see `.air.toml`).

---

## CLI reference

The same binary handles all commands:

```text
mycis [command] [flags]
```

If omitted, `command` defaults to `web`.

| Command | Purpose |
|---------|---------|
| `web` | Start the HTTP server. |
| `migrate` | Apply all pending SQL migrations from `db/migrations`. |
| `create-admin` | Create a user with admin privileges (see flags below). |
| `seed-framework` | Load framework definitions from `seed/frameworks/<slug>.yaml` into the database. |

### `create-admin` flags

| Flag | Meaning |
|------|---------|
| `-email` | Admin user email (required). |
| `-name` | Display name (required). |
| `-password` | Optional. If empty, a temporary password is generated and printed. |

### `seed-framework` flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-slug` | `cis-v8-1` | Slug matching `seed/frameworks/<slug>.yaml`. |
| `-force` | false | Re-seed an existing framework (upserts groups/items, deactivates removed ones). |

Available slugs shipped in this repository:

| Slug | Framework |
|------|-----------|
| `cis-v8-1` | CIS Controls v8.1 |
| `nist-csf-2-0` | NIST Cybersecurity Framework 2.0 |
| `iso-27001-2022` | ISO/IEC 27001:2022 |
| `iso-27002-2022` | ISO/IEC 27002:2022 |
| `soc2-2017` | SOC 2 Trust Service Criteria (2017) |
| `nis2-2022` | NIS2 Directive (2022) |
| `dora-2025` | Digital Operational Resilience Act (DORA) |
| `gdpr-2018` | General Data Protection Regulation (GDPR) |
| `eu-ai-act-2024` | EU Artificial Intelligence Act (2024) |

To add another framework, add a new YAML file under `seed/frameworks/` and run `seed-framework` with that slug.

---

## HTTP routes (summary)

| Method | Path | Auth | Notes |
|--------|------|------|--------|
| GET | `/` | — | Redirects to `/dashboard` if signed in, else `/login`. |
| GET/POST | `/login` | — | Sign-in form. |
| POST | `/logout` | Yes | End session. |
| GET/POST | `/change-password` | Yes | Required when `must_change_password` is set. |
| GET | `/dashboard` | Yes | Home dashboard. |
| GET | `/frameworks` | Yes | Framework browser. |
| GET | `/assessments` | Yes | Assessment list. |
| GET/POST | `/assessments/new` | Admin | New assessment form. |
| GET | `/assessments/{id}` | Yes | Assessment detail and items. |
| POST | `/assessments/{id}/bulk` | Admin | Bulk update selected items. |
| GET | `/items/{id}` | Yes | Single control item detail. |
| POST | `/items/{id}` | Yes | Update item fields. |
| POST | `/items/{id}/comments` | Yes | Add comment. |
| POST | `/items/{id}/evidence` | Yes | Add evidence link. |
| GET/POST | `/admin/users` | Admin | List users; create user (temporary password in flash). |
| GET | `/assets/*` | — | Static files from `public/assets`. |

---

## npm scripts

| Script | Description |
|--------|-------------|
| `npm run build` | Runs `build:css` and `build:js`. |
| `npm run build:css` | Tailwind CLI: `assets/css/app.css` → `public/assets/app.css` (minified). |
| `npm run build:js` | esbuild bundle: `assets/js/app.js` → `public/assets/app.js` (minified ESM). |

---

## sqlc

Query SQL lives in `db/queries`; generated Go is written to `internal/db` per `sqlc.yaml`.

```bash
sqlc generate
# or: just generate
```

Regenerate after changing queries or schema inputs referenced by sqlc.

---

## Optional: generate framework YAML from a CIS Excel workbook

The helper in `tools/generate-cis-yaml` converts a CIS Controls spreadsheet to the YAML format expected under `seed/frameworks/`.

```bash
go run ./tools/generate-cis-yaml -input /path/to/CIS-Controls-Workbook.xlsx
```

Use `-output`, `-slug`, `-label`, `-version`, and `-sheet` to customize output paths and metadata (see `main.go` flags).

After generating, run `seed-framework` with the matching `-slug` to load it into the database.

---

## Tests

```bash
go test ./...
# or: just test
```

---

## Troubleshooting

- **`DATABASE_URL is required`** — Export it or add it to `.env`.
- **`APP_SESSION_KEY must be at least 32 characters`** — Use a longer secret.
- **Empty frameworks list** — Run `seed-framework` for your YAML slug.
- **Cannot create assessment** — Only admin users can open `/assessments/new`; use `create-admin` or promote via database (not exposed in UI).
- **Port already in use** — Change `APP_ADDR` or stop the conflicting process.
- **Docker Postgres port** — Check whether `docker-compose.override.yml` maps `55432` vs `5432` and match `DATABASE_URL` accordingly.

---

## License

No license file is present in this repository; add one if you intend to distribute or open-source the project.
