# Development

## Components

- `main.go`: validated CLI, lifecycle, sampling, schedules and render cadence.
- `dashboard/options.go`: fixed selection and monotonic-time playlist rotation.
- `dashboard/render.go`: four views and cached displayed-value keys.
- `dashboard/health.go`: measured health and critical alert precedence.
- `dashboard/notify.go`: bounded local socket notifications and screenshots.
- `network/`: Linux/ZFS data collection and parsing.
- `wave/animator.go`: bounded timestamped history and elapsed-time scaling.
- `display/`: text/geometry, RGB565 conversion and compatible serial transfers.

## Rendering

One goroutine owns state and rendering. A reusable base image contains all
non-animated content. Keys use displayed precision and view-specific fields;
hidden CPU changes cannot invalidate a storage page. A settled unchanged frame
returns before pixel copying, conversion or transmission. The network page adds
two smoothstep curves; interpolation cannot overshoot adjacent observations.
Colored fills use straight alpha converted once per curve. Wave point arrays,
RGB565 data and packet buffers are reused.

A completed successful transfer is copied into one reusable screenshot buffer
under a mutex. Screenshot requests clone that buffer. Failed transfers invalidate
the device cache and cannot publish a misleading successful screenshot.

Rendering profiles request 1/4/8 FPS. CPU and measured partial-transfer duration
can reduce the rate. Full-page transitions do not permanently set the animation
budget. Critical changes, notifications and page changes can wake the renderer
independently of manual timing. No animation is performed for hidden views.

## Data cadence

| Source | Cadence |
|---|---|
| Network counters, CPU, operstate | 1 second while awake |
| Memory, ARC, kernel temperatures | 5 seconds while awake |
| IP, link speed, uptime | 30 seconds while awake |
| Shell sessions | 10 seconds |
| Pool capacity, drive inventory, ZFS health | 60 seconds, one sequential worker |
| Night schedule | 60 seconds |

ZFS/lsblk commands have two-second timeouts and run outside the render loop.
Temperature data comes from hwmon; no repeated SMART subprocesses or disk scans
are introduced. Physical slots map by PCI/by-path topology, not kernel sdX order.
This mapping is specific to the WTR MAX and should be verified on other hardware.

Unavailable reads are distinct from empty or healthy results. A failed health
refresh retains known critical faults, labelled as last known. Link operstate is
separate from speed metadata. Counter resets establish a new network baseline.
CPU and memory reads do not attempt to infer ZFS ARC health from RAM usage.

## Checks

`go test -race ./...`, `go vet ./...`, and a Linux amd64 build are required.
`go run . --preview build/previews` generates deterministic layout fixtures.
Inspect network, storage, system, drives, fault and unavailable images. Use
`go test ./dashboard ./display -run '^$' -bench . -benchmem` for local timing.
Hardware transfer and physical readability must be checked separately.

## Demo media

The README uses the same native renderer as the display, with synthetic data and
an RFC 5737 documentation address. No NAS connection is needed. Regenerate it
with Go and FFmpeg:

```sh
go run . --preview docs/previews
go run ./cmd/showcase -out build/showcase
ffmpeg -y -framerate 4 -i build/showcase/%03d.png -filter_complex \
  '[0:v]split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=none:diff_mode=rectangle' \
  -loop 0 docs/showcase.gif
```

The tour shows rotating pages, a fixed live network page, line-only waves and a
critical fault. It is a presentation fixture, not a hardware performance test.

## Releases

Main and pull requests run tests, race checks, vet and a Linux build. An explicit
`vMAJOR.MINOR.PATCH` tag runs the same checks and publishes the Linux amd64 binary
and SHA-256 checksum. Main pushes do not invent version numbers. Build artifacts
use `-trimpath` so developer filesystem paths are not embedded. Keep sample data
synthetic; do not commit deployment scripts, device logs or live data captures.
