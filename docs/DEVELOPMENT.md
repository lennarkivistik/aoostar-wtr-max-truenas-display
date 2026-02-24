# Development Guide

This document describes the architecture and coding conventions for the `wtr-display` project.

## Project Structure

The project follows a modular architecture to separate concerns between hardware drivers, data monitoring, rendering logic, and application orchestration.

```
go-wtr-display/
├── main.go                 # CLI entry point and main loop
├── build.sh                # Cross-compilation script
├── dashboard/
│   ├── auto.go             # Smart Auto Mode (FPS/Power scaling)
│   ├── notify.go           # Unix socket notification server
│   ├── render.go           # UI compositing and layout
│   ├── scheduler.go        # Night mode scheduling
│   └── state.go            # Dashboard state, frame storage
├── display/
│   ├── display.go          # Serial port communication and protocol
│   ├── gauge.go            # Arc gauge rendering
│   ├── image.go            # RGB565 conversion and image transmission
│   ├── shapes.go           # Rectangles, circles, rounded rects
│   ├── text.go             # Font rendering with Go fonts
│   └── wave.go             # Wave/sparkline visualization
├── network/
│   ├── history.go          # Network stats history (ring buffer)
│   ├── stats.go            # Network throughput monitoring
│   └── system.go           # CPU, memory, ZFS, login detection (utmp)
├── wave/
│   └── animator.go         # Wave animation state machine
└── docs/
    ├── DESIGN.md            # UI/UX design concepts and color palette
    ├── DEVELOPMENT.md       # Architecture and coding conventions
    └── screenshot.png       # Display screenshot
```

## Architecture Overview

### 1. Rendering Loop
The application uses a **Layered Rendering** approach to minimize CPU usage:
-   **Static Layer (`bgImg`)**: Contains elements that update slowly (Background, Header, Gauges, Labels). Updated every 5s-30s.
-   **Dynamic Layer**: Contains elements that update rapidly (Waveform, Speed Numbers). Updated at 4-15 FPS.

On every animation frame:
1.  `display.CopyImage(frame, bgImg)`: Fast `copy(dst.Pix, src.Pix)` of the static background.
2.  Draw Waveform on top (uses `display.WavePoint` directly from `wave.Animator`).
3.  Draw Speed Numbers on top (with unit hysteresis to prevent flicker at boundaries).
4.  `state.SetLastFrame(frame)`: Bulk copy of pixel data for thread-safe screenshots.
5.  Send `frame` to display via USB.

### 2. Auto Mode
The `--auto` flag enables smart resource management:
-   **Idle**: Low traffic/load → Low FPS (4) to save CPU.
-   **Active**: High traffic → High FPS (15) for smooth visuals.
-   **Heavy Load**: High CPU usage → Min FPS (1) to prioritize system tasks.
-   **Night Mode**: Automatically turns off display between 23:00 and 10:00.

## Coding Conventions

### Performance
This app runs on a NAS/Router, so efficiency is critical.
*   **Avoid Allocations in Loop**: Do not allocate slices or huge structs inside the render loop (`RenderAndSend`). Use pre-allocated buffers.
*   **Direct Pixel Access**: Use `copy(dst.Pix, src.Pix)` for bulk image operations instead of per-pixel `Set()`/`At()`. Use optimized drawing functions (e.g., `drawVerticalLineAlpha`) for heavy operations like filling waves.
*   **Unify Types Across Packages**: Avoid creating duplicate structs in different packages that require per-frame conversion. Shared types live in `display/` and are imported by other packages.
*   **Prefer File Reads Over Exec**: Where possible, read system files directly (e.g., `/var/run/utmp` for login detection) instead of spawning processes. Process spawns are expensive relative to file reads.
*   **Thread-Safe Frame Storage**: `SetLastFrame` copies pixel data rather than storing a reference to the caller's buffer. This prevents partially-drawn frames from appearing in screenshots.
*   **Font Caching**: `TextRenderer` caches font faces. Reuse the renderer instance.

### Threading
*   **Single-Threaded Rendering**: The render loop is single-threaded. Multithreading adds overhead for 960x376 pixels.
*   **Tickers**: Use separate `time.Ticker` for data sampling (1s) and rendering (variable).

### Style
*   **Keep `main.go` Clean**: Put logic in `dashboard/` subpackages. `main.go` should only handle wiring and the main `select` loop.
*   **Explicit State**: Pass `*State` structs rather than using globals.

## Data Sources

All data is gathered non-invasively:

| Data | Source | Interval | Impact |
|------|--------|----------|--------|
| CPU Usage | `/proc/stat` | 1s | None (file read) |
| Memory | `/proc/meminfo` | 5s | None (file read) |
| Network Stats | `/sys/class/net/<iface>/statistics/` | 1s | None (file read) |
| ZFS Pools | `zpool list` command | 5s | Minimal (~10ms) |
| ZFS Status | `zpool status` command | 60s | Minimal (~20ms) |
| Logged-in Users | `/var/run/utmp` | 10s | None (file read) |
| TrueNAS Version | `/etc/version` or `/etc/os-release` | Once | None (file read) |
| IP Address | Go `net` package | 30s | None (syscall) |
| Link Speed | `/sys/class/net/<iface>/speed` | 30s | None (file read) |
| Uptime | `/proc/uptime` | 30s | None (file read) |

## Protocol Details

The display uses a proprietary protocol over USB serial:

- **Baud Rate**: 1,500,000 (ignored, USB bulk transfer)
- **Display On**: `AA 55 AA 55 0B 00 00 00`
- **Display Off**: `AA 55 AA 55 0A 00 00 00`
- **Image Format**: RGB565, sent in 47-byte chunks with offset headers

