# Web Frontend

Preact and plain CSS frontend bundled for Haepage, embedded into the Go binary via `go:embed`.

Can run in two modes:
1. **Static mode**: Standalone browser-only dashboard (no backend required).
2. **Server mode**: Connected to Go backend via `/api/ping`, `/api/services`, `/api/events` (SSE), etc.
