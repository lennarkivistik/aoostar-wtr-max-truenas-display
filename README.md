# WTR Display - TrueNAS Network Monitor

[Display Data](screenshot.png)

A Go application to display TrueNAS system statistics on the WTR MAX / AOOSTAR front panel LCD display.

> **Note**: This project is specifically built for **TrueNAS SCALE** running on AOOSTAR WTR MAX hardware. It's tailored to my personal use case. If you need different features or want to adapt it for other systems, please fork this repository.

## Acknowledgments

This project was made possible thanks to [zehnm/aoostar-rs](https://github.com/zehnm/aoostar-rs) and their reverse engineering efforts of the AOOSTAR display protocol. Their work documenting the proprietary serial protocol, image format, and control commands enabled this Go implementation. Thank you!

## Features

- **Network Throughput**: Real-time upload/download speeds with animated wave visualization
- **System Information**: TrueNAS version, IP address, interface name, link speed, uptime
- **CPU Usage**: Live CPU utilization with color-coded arc gauge
- **Memory Usage**: RAM utilization percentage with arc gauge
- **ZFS Pool Status**: Auto-discovers pools, shows usage with color-coded gauges
- **Notifications**: External scripts can send notifications to the display
- **Screenshots**: Capture current display to PNG file
- **Login Indicator**: Red dot shows when users are logged into shell
- **ZFS Maintenance Alerts**: Automatic notifications for scrub, resilver, degraded pools
- **Night Mode**: Schedule display off/on times
- **Auto Mode**: Smart FPS scaling based on activity

## Display Specifications

- **Resolution**: 960 x 376 pixels
- **Connection**: USB UART (VID:PID `0416:90A1`)
- **Protocol**: Proprietary serial protocol with RGB565 image data
- **Device**: Typically `/dev/ttyACM0` on Linux

## Installation

### Build from Source

```bash
# Build for Linux x86_64 (TrueNAS)
./build.sh linux amd64 v0.5.0

# Output: build/linux-amd64/wtr-display-linux-amd64-v0.5.0
```

### Deploy to TrueNAS

```bash
# Copy binary to TrueNAS
scp build/linux-amd64/wtr-display-linux-amd64-v0.5.0 root@truenas:/path/to/destination/

# On TrueNAS, make executable
chmod +x wtr-display-linux-amd64-v0.5.0
```

## Usage

### Basic Commands

```bash
# Turn display on
./wtr-display --on

# Turn display off
./wtr-display --off

# List available network interfaces
./wtr-display --list-ifaces

# Show version
./wtr-display --version
```

### Monitor Mode (Main Feature)

```bash
# Monitor network interface (auto-detect display)
./wtr-display --iface eth0

# With specific device
./wtr-display --device /dev/ttyACM0 --iface eth0

# With night mode (display off 23:00-10:00)
./wtr-display --iface eth0 --off-time 23:00 --on-time 10:00

# Auto mode (smart FPS + default night mode 23:00-10:00)
./wtr-display --iface eth0 --auto

# Custom animation settings
./wtr-display --iface eth0 --wave-fps 8 --samples 10
```

### Notifications

Send notifications to a running display instance:

```bash
# Info notification (default 10 seconds)
./wtr-display --notify "Backup complete"

# Warning notification (5 seconds)
./wtr-display --notify "Scrub starting" --notify-type warning --notify-duration 5

# Danger notification (persistent until replaced)
./wtr-display --notify "Drive failure!" --notify-type danger --notify-duration 0
```

Or via Unix socket directly:
```bash
echo '{"message":"Hello","theme":"warning","duration":10}' | nc -U /tmp/wtr-display.sock
```

### Screenshots

Capture the current display to a PNG file:

```bash
# Save screenshot from running display
./wtr-display --screenshot /tmp/display.png
```

### All Options

| Flag | Description | Default |
|------|-------------|---------|
| `--device` | Serial device path | Auto-detect |
| `--on` | Turn display on and exit | - |
| `--off` | Turn display off and exit | - |
| `--iface` | Network interface to monitor | - |
| `--list-ifaces` | List available interfaces | - |
| `--version` | Show version | - |
| `--wave-fps` | Animation frames per second | 4 |
| `--samples` | Sparkline history samples | 5 |
| `--frame-interval` | Override update interval (e.g., `500ms`) | - |
| `--auto` | Smart FPS + default night mode | false |
| `--off-time` | Night mode start (HH:MM) | - |
| `--on-time` | Night mode end (HH:MM) | - |
| `--no-cache` | Disable frame caching | false |
| `--notify` | Send notification to running display | - |
| `--notify-type` | Notification type: `info`, `warning`, `danger` | info |
| `--notify-duration` | Notification duration in seconds (0=persistent) | 10 |
| `--screenshot` | Save screenshot from running display (PNG) | - |

## Display Layout

```
+------------------------------------------------------------------------------+
| TrueNAS SCALE 24.10.0          192.168.1.100          eth0 10G  *  5d 12h    |
|------------------------------------------------------------------------------|
|                                                                              |
|      [wave visualization]                    [wave visualization]            |
|                                                                              |
|      V  452.3        Mb/s                    ^  234.1        Mb/s            |
|                                                                              |
|------------------------------------------------------------------------------|
|       (78)           (62)           (45)           (12)                      |
|        CPU            RAM          fastpool        tank                      |
+------------------------------------------------------------------------------+
|  *  <- Red dot when user logged in
```

### Notification Box (overlays center)
```
              +---------------------------+
              |   Scrub: tank 45.2%       |  <- Yellow (warning)
              +---------------------------+

              +---------------------------+
              |   DEGRADED: tank          |  <- Red (danger)
              +---------------------------+
```

### Color Coding

- **Download**: Teal/Cyan
- **Upload**: Blue
- **Gauges** (CPU/RAM/Pools):
  - Green: < 75%
  - Yellow: 75-90%
  - Red: > 90%
- **Notifications**:
  - Info: White background, black text
  - Warning: Yellow background, black text
  - Danger: Red background, white text
- **Login Indicator**: Bright red dot (bottom-left)

## Architecture

```
go-wtr-display/
├── main.go                 # CLI entry point and main loop
├── display/
│   ├── display.go          # Serial port communication and protocol
│   ├── image.go            # RGB565 conversion and image transmission
│   ├── text.go             # Font rendering with Go fonts
│   ├── shapes.go           # Rectangles, circles, rounded rects
│   ├── gauge.go            # Arc gauge rendering
│   └── wave.go             # Wave/sparkline visualization
├── dashboard/
│   ├── state.go            # Dashboard state and notification types
│   ├── render.go           # UI rendering functions
│   ├── scheduler.go        # Night mode scheduling
│   ├── auto.go             # Auto FPS calculation
│   └── notify.go           # Unix socket notification server
├── network/
│   ├── stats.go            # Network throughput monitoring
│   ├── history.go          # Network stats history
│   └── system.go           # CPU, memory, ZFS, login detection
├── wave/
│   └── animator.go         # Wave animation state machine
└── build.sh                # Cross-compilation script
```

## Data Sources

All data is gathered non-invasively:

| Data | Source | Interval | Impact |
|------|--------|----------|--------|
| CPU Usage | `/proc/stat` | 1s | None (file read) |
| Memory | `/proc/meminfo` | 5s | None (file read) |
| Network Stats | `/sys/class/net/<iface>/statistics/` | 1s | None (file read) |
| ZFS Pools | `zpool list` command | 5s | Minimal (~10ms) |
| ZFS Status | `zpool status` command | 60s | Minimal (~20ms) |
| Logged-in Users | `who` command | 10s | Minimal (~5ms) |
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

## Requirements

- Go 1.21+ (for building)
- Linux system with ZFS (for pool monitoring)
- WTR MAX / AOOSTAR device with front panel display

## License

Apache License, Version 2.0 (LICENSE-APACHE or http://www.apache.org/licenses/LICENSE-2.0)
