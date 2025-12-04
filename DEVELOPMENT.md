# Development Guide

This document describes the architecture and coding conventions for the `wtr-display` project.

## Project Structure

The project follows a modular architecture to separate concerns between hardware drivers, data monitoring, rendering logic, and application orchestration.

```
go-wtr-display/
├── cmd/                (Optional future expansion)
├── dashboard/          # High-level application logic
│   ├── auto.go         # Smart Auto Mode (FPS/Power scaling) logic
│   ├── render.go       # Compositing and layout (draws the UI)
│   ├── scheduler.go    # Night mode / On-Off scheduling
│   └── state.go        # Central data structure (State)
├── display/            # Low-level display driver & graphics primitives
│   ├── display.go      # Serial communication & commands
│   ├── image.go        # RGB565 protocol & buffer management
│   ├── shapes.go       # Basic drawing primitives (Line, Rect)
│   ├── text.go         # Font rendering engine
│   ├── wave.go         # Waveform rendering logic
│   └── gauge.go        # Arc gauge rendering logic
├── network/            # System data providers
│   ├── stats.go        # Network interface statistics
│   └── system.go       # CPU, Memory, Disk usage
├── wave/               # Wave math & animation state
│   └── animator.go     # Continuous wave logic (not rendering)
└── main.go             # Entry point: Flags, Tickers, Event Loop
```

## Architecture Overview

### 1. Rendering Loop
The application uses a **Layered Rendering** approach to minimize CPU usage:
-   **Static Layer (`bgImg`)**: Contains elements that update slowly (Background, Header, Gauges, Labels). Updated every 5s-30s.
-   **Dynamic Layer**: Contains elements that update rapidly (Waveform, Speed Numbers). Updated at 4-15 FPS.

On every animation frame:
1.  `display.CopyImage(frame, bgImg)`: Fast memory copy of the static background.
2.  Draw Waveform on top.
3.  Draw Speed Numbers on top.
4.  Send `frame` to display via USB.

### 2. Auto Mode
The `--auto` flag enables smart resource management:
-   **Idle**: Low traffic/load → Low FPS (4) to save CPU.
-   **Active**: High traffic → High FPS (15) for smooth visuals.
-   **Heavy Load**: High CPU usage → Min FPS (1) to prioritize system tasks.
-   **Night Mode**: Automatically turns off display between 23:00 and 10:00.

## Coding Conventions

### Performance
This app runs on a NAS/Router, so efficiency is critical.
*   **Avoid Allocations in Loop**: Do not allocate slices or huge structs inside the render loop (`renderAndSend`). Use pre-allocated buffers.
*   **Direct Pixel Access**: Use optimized drawing functions (e.g., `drawVerticalLineAlpha`) instead of generic `Set()` for heavy operations like filling waves.
*   **Font Caching**: `TextRenderer` caches font faces. Reuse the renderer instance.

### Threading
*   **Single-Threaded Rendering**: The render loop is single-threaded. Multithreading adds overhead for 960x376 pixels.
*   **Tickers**: Use separate `time.Ticker` for data sampling (1s) and rendering (variable).

### Style
*   **Keep `main.go` Clean**: Put logic in `dashboard/` subpackages. `main.go` should only handle wiring and the main `select` loop.
*   **Explicit State**: Pass `*State` structs rather than using globals.

