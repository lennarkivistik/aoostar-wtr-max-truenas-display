# WTR Display v1.11.0

The WTR MAX panel now rotates through large, readable network, storage, system
and drive views. Every data pool gets its own storage page. Use `--view` to pin
one live page, or `--views` and `--view-duration` to customize rotation.

- Colored network waves return, with filled, line and off styles; adaptive or
  link-speed scaling; bounded history; and eco, balanced or smooth profiles.
- Hidden and unchanged views avoid rendering and transfers. Wave timing adapts
  to CPU load and measured display transfer cost.
- Memory availability, ARC size, CPU temperature and available disk temperatures
  come from measured telemetry. Missing data is explicit; bay occupancy is not
  presented as SMART health. Critical faults override pages and notifications.
- Network counter resets, multi-pool scan progress, screenshot ownership and
  duplicate-instance detection are handled. Local notification requests are
  bounded. The local IPC socket remains writable by local users for compatibility.
- Existing notification, screenshot, scheduling and benchmark commands remain.
  `--auto` keeps the legacy night schedule; `--night-mode off` overrides it.
- Hardware-free preview generation and a synthetic animated tour document the
  layouts without publishing live system details. Builds omit local source paths.

## Install or upgrade

Download the Linux amd64 binary and `SHA256SUMS` into the same directory, run
`sha256sum -c SHA256SUMS`, and make the binary executable. Preserve the previous
binary and startup script, stop the running monitor, and update the binary path
in `start.sh`. A Post Init script must launch the monitor in the background;
see the README for the complete launcher and fixed-view examples.

The monitor requires access to the AOOSTAR serial display and Linux/ZFS tools.
The physical bay mapping targets the WTR MAX. No system account or group changes
are performed by the application.
