# Repository Guidelines

JustPing is a distributed network-latency monitoring platform: a Go host server, a Go probe agent, and a React web UI.

## Project Structure

- `host/` — Go server. Entrypoint `host/cmd/server/main.go`; packages under `host/internal/` (`api`, `auth`, `db`, `model`, `ws`, `ui`).
- `agent/` — Go probe. Entrypoint `agent/cmd/agent/`; packages `internal/pinger`, `internal/client`, `internal/service`.
- `shared/protocol/` — WebSocket message types shared by host and agent.
- `web/` — Vite + React + TypeScript + Tailwind UI (`web/src/components`, `web/src/lib`). `@/` aliases `web/src`.
- `scripts/install.sh` — universal Linux agent installer (systemd, OpenRC, procd, runit, SysVinit).
- `.github/workflows/ci.yml` — builds binaries, Docker image, and tagged releases.

`host`, `agent`, and `shared` form a Go 1.22 workspace (`go.work`). Each is its own Go module with `replace` directives to `../shared`.

## Build, Test, and Development Commands

```bash
cd web && npm install && npm run dev     # UI dev server on :5173, proxies /api to :8080
cd web && npm run build && npm run lint  # tsc + Vite build; oxlint
cp -r web/dist/* host/internal/ui/dist/  # embed UI assets (dist is gitignored)
cd host && go build -o justping-host ./cmd/server
cd host && DATABASE_URL="postgres://postgres:postgres@localhost:5432/justping?sslmode=disable" ./justping-host
cd agent && go build -o justping-agent ./cmd/agent
docker compose up -d                     # full stack: PostgreSQL + host
```

Copy `.env.example` to `.env` before running locally. Use `go vet ./...` and `gofmt -l .` per Go module. Cross-compile with `CGO_ENABLED=0` and `GOWORK=off`.

## Coding Style & Naming Conventions

- Go: `gofmt` with tabs. Locals are `camelCase`, exports `PascalCase`, packages short lowercase nouns. Add doc comments to exported names and keep files small.
- TypeScript/React: 2-space indent, no semicolons, single quotes. Components are `PascalCase.tsx`; hooks and helpers use `camelCase`.
- Run `npm run lint` before committing UI changes.

## Testing Guidelines

No test suite exists yet. Use Go's `testing` package (`*_test.go`, `TestXxx` names) and run `go test ./...` in `host/`, `agent/`, or `shared/`. For the UI, prefer Vitest (`*.test.tsx`) with an `npm test` script. Prioritize protocol encode/decode, scoring logic, and auth middleware.

## Commit & Pull Request Guidelines

Commits follow Conventional Commits: `feat:`, `fix:`, `build(ci):`, `perf(docker):`. Use an imperative subject and reference issues (for example, `fix: address issues #13, #15`).

Pull requests must describe the change, link related issues, and name affected components. Include screenshots for UI changes plus build/lint/test output. Never commit secrets; `.env`, keys, and certificates are gitignored.
