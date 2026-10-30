# Front-panel design

The panel is 960 x 376 pixels in a physically small enclosure. Pixels are not a
substitute for readable physical text size. Each page answers one question with
large numbers; secondary labels occupy much less space than the readings.

## Views

- Network: download left, upload right, 108px primary values and colored waves.
- Storage: one named pool, 140px utilization and 86px available capacity.
- System: CPU load, CPU temperature, available RAM and measured ARC size.
- Drives: six SATA bays and five NVMe bays, occupancy and sensor temperatures.

The header always names the page and shows measured status. A stable bottom
navigation row indicates the current view and AUTO/FIXED mode. Fixed mode never
advances pages, but readings stay live. Storage rotation includes every non-boot
pool; fixed storage selects one pool. A critical fault replaces page content.

## Visual language

Near-black background, pale text, cyan download/storage, blue upload/cache.
Amber means maintenance or incomplete data; red means a critical condition.
Drive outlines use cyan for occupancy, not green as an implied health claim.
Native geometry draws arrows; no unsupported symbol fonts or decorative dots.
Labels are measured and truncated at Unicode character boundaries.

Waves use flat translucent fills and bounded smooth interpolation. Adaptive
scaling is explicitly labelled. No gradients, full-screen fades, or constant
progress animations: unchanged RGB565 regions stay unchanged on the wire.

Preview PNGs are generated from demo data. Final acceptance includes viewing the
physical panel and checking real transfer latency, not only desktop screenshots.
