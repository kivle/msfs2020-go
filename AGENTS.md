# Repository Guidelines

## Project Structure & Module Organization
- Root Go module `github.com/lian/msfs2020-go`; flight-sim bindings live in `simconnect/` (Go bindings + generated `bindata.go` for SimConnect headers).
- `vfrmap/` is the Windows data server binary; HTTP is a JSON placeholder, and real-time data flows through WebSocket handlers in `vfrmap/websockets/`.
- `examples/request_data/` mirrors the official SimConnect sample; use it as a minimal integration sanity check.
- Release artifacts (`vfrmap.exe`, `SimConnect.dll`) are kept in the repo for convenience—do not overwrite without intent.

## Build, Run, and Development Commands
- Standard toolchain: Go 1.14+; ensure `go` is on PATH.
- Fast compile check: `go build ./...` (verifies both `simconnect` and `vfrmap`).
- Build Windows binary from any platform: `GOOS=windows GOARCH=amd64 go build github.com/lian/msfs2020-go/vfrmap`.
- Full release build with embedded assets/version stamps (requires `go-bindata`): `./build-vfrmap.sh`.
- Local run while developing the map: `go run ./vfrmap` then open `http://localhost:9000`.

## Coding Style & Naming Conventions
- Format with `gofmt` (or `goimports`) before sending changes; prefer idiomatic Go naming (`Thing`, `thing`, `ThingWithContext`).
- Keep exported symbols documented when they are part of the `simconnect` API; align new enums/structs with SDK names for traceability.
- Avoid global state in new code paths; pass context/config explicitly where feasible.

## Testing Guidelines
- There is no automated test suite today; add `*_test.go` alongside code and name tests `TestXxx` using table-driven cases when possible.
- For integration changes, exercise `go run ./examples/request_data` against a running MSFS instance and note observed behavior.
- Include manual test notes (inputs/outputs, simulator version) in PRs until coverage grows.

## Commit & Pull Request Guidelines
- Commit messages follow short, imperative summaries (e.g., "Wait for SimConnect before exit"); group related edits together.
- PRs should describe the change, risks, and simulator setup used for validation; link issues when applicable.
- Add screenshots or GIFs for UI tweaks under `vfrmap/html/`, and list all commands executed (build/test/manual steps).

## Security & Configuration Tips
- Do not commit personal simulator configs or credentials; only the bundled `SimConnect.dll` and generated assets belong in-tree.
- When regenerating assets with `go-bindata`, verify outputs remain deterministic and avoid embedding secrets in HTML/JS.
