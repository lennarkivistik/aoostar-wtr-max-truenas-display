# Rotating and fixed display views

## Objective and accepted scope
Make the 960 x 376 AOOSTAR panel readable at its physical size. Preserve the
existing work in Git and restore the colored network wave. Default to a rotating
playlist; `--view` pins one live view. Static mode means no page changes, not
frozen measurements. Critical faults still take precedence.

## Contract
- Views: network, storage, system, drives. Default order follows that list.
- `--view NAME` pins a view; `--views LIST` selects ordered rotating views.
- `--view-duration 10s` controls dwell. `--pool NAME` selects storage content.
- `--profile eco|balanced|smooth` bounds animation; default balanced.
- `--wave-style filled|line|off`, `--wave-history 20s`, and
  `--wave-scale adaptive|link` control the network visualization.
- Existing notification, screenshot, device, benchmark and timing flags remain.
- Preserve legacy `--auto` scheduling; add `--night-mode off` to override it.
- Add hardware-free `--preview DIR` for reproducible layout review.
- Unknown or failed telemetry must never render as healthy. Bay occupancy is
  explicitly distinct from disk health. Alerts cannot be hidden by info notices.

## Implementation order
1. Validated options, playlist selection and time-based bounded wave history.
2. Truthful telemetry and alert priority; four large native Go-rendered views.
3. CLI/runtime integration with cached rendering, bounded animation and polling.
4. Fixture previews, race tests, Linux build, benchmarks and documentation.

## Structure and style
Use dashboard/ for selection/rendering, network/ for Linux data, wave/ for the
animator and main.go for CLI and orchestration. Go tests live beside source.
No new dependencies. Use explicit structs and gofmt, for example:

```go
if err := options.Validate(); err != nil {
    return err
}
```

## Verification and boundaries
Run `go test ./...`, `go test -race ./...`, `go vet ./...` and
`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...`.
Test fixed selection, rotation, invalid CLI input, fault priority, stale data,
counter resets and frame-rate-independent scaling. Render and inspect every
view, missing data, long labels and faults. Benchmark filled waves and no-op
frames. Preserve 47-byte protocol chunks and existing rollback binaries.
For deployment, preserve the previous binary and startup script, then verify
live screenshots and resource use. A hardware-free preview does not establish
physical transfer speed or successful boot startup.
