# Finance Backend Skeleton

Go (Gin + GORM) skeleton for the finance system. Business logic is intentionally omitted; this only sets up the project layout.

## Structure
- `cmd/server` – entrypoint.
- `internal/config` – env-based configuration.
- `internal/db` – GORM connection helpers (PostgreSQL/MySQL).
- `internal/router` – Gin router setup.
- `internal/handler/health` – sample health endpoint.
- `frontend/` – placeholder directory for the future SPA/FE project.

## Quick start
1) Set environment variables (see `.env.example`), e.g.:
   ```bash
   export APP_ENV=development
   export HTTP_PORT=8080
   export DB_DRIVER=postgres
   export DB_HOST=127.0.0.1
   export DB_PORT=5432
   export DB_USER=finance
   export DB_PASSWORD=finance
   export DB_NAME=finance
   ```
2) Run the server:
   ```bash
   go run ./cmd/server
   ```
3) Test health:
   ```bash
   curl http://localhost:8080/api/health
   ```

## Notes
- `DB_DRIVER` supports `postgres` or `mysql`.
- Connection pool defaults are set in `internal/db/db.go`; tweak as needed.
- Add your own modules (services/repositories/handlers) under `internal/`.

## Hot reload (Air)
- Install (requires network): `go install github.com/cosmtrek/air@latest` (set `GOPROXY` if需要国内镜像，如 `export GOPROXY=https://goproxy.cn,direct`).
- Config file: `.air.toml` (already included).
- Run with hot reload:
  ```bash
  air
  ```
