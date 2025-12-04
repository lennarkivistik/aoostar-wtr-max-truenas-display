# WTR Display Design Concepts

## Physical Constraints

| Property | Value |
|----------|-------|
| Resolution | 960 × 376 pixels |
| Physical size | 6.57cm × 2.56cm |
| Pixel density | ~146 px/cm |
| Viewing distance | ~1 meter |
| Display brightness | Dim LCD |

### Readability Requirements

At 1m viewing distance, ergonomic guidelines suggest:
- **Minimum text height**: 3-4mm = **44-60 pixels**
- **Comfortable text height**: 5-6mm = **73-88 pixels**
- **Large/primary text**: 8-10mm = **117-146 pixels**

### Design Principles for This Display

1. **High contrast** - Bright elements (white/yellow) on dark background
2. **Minimal text** - Use colors and shapes to convey meaning
3. **Large numbers** - Primary data should be huge
4. **Color coding** - Green/yellow/red for status at a glance
5. **Remove clutter** - No interface name, minimal labels

---

## Design Concept A: "Compact Speed Focus"

Network speeds as primary focus, stats as a minimal bottom bar.

```
┌────────────────────────────────────────────────────────────────────────────────┐
│  TrueNAS SCALE 24.10.0                                      192.168.1.100      │
│                                                                                │
│              ▼ 452        ▲ 234  Mbps                                          │
│                                                                                │
│  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  │
│  CPU 78%        RAM 62%        fastpool 45% 2T        tank 12% 8T              │
└────────────────────────────────────────────────────────────────────────────────┘
```

**Pros**: Clean, speeds very prominent  
**Cons**: Bottom bar text might be too small

---

## Design Concept B: "Circular Gauges"

Uses pie/donut charts for visual status - highly visible even when dim.

```
┌────────────────────────────────────────────────────────────────────────────────┐
│  TrueNAS 24.10                              ▼ 452    ▲ 234                     │
│                                                Mbps                            │
│                                                                                │
│      ◐           ◐           ◐           ◐                                     │
│     78%         62%         45%         12%                                    │
│     CPU         RAM        fast        tank                                    │
└────────────────────────────────────────────────────────────────────────────────┘
```

**Pros**: Gauges visible from distance, color-coded status at a glance  
**Cons**: More complex to render, less space for speed numbers

---

## Design Concept C: "Two-Column Layout"

Left side for speeds (primary), right side for system stats.

```
┌────────────────────────────────────────────────────────────────────────────────┐
│  TrueNAS SCALE 24.10.0 • 192.168.1.100                                         │
│                                                                                │
│         ▼ 452                              CPU ████████░░ 78%                  │
│              Mbps                          RAM ██████░░░░ 62%                  │
│         ▲ 234                             fast ████░░░░░░ 45%                  │
│              Mbps                         tank █░░░░░░░░░ 12%                  │
└────────────────────────────────────────────────────────────────────────────────┘
```

**Pros**: Everything visible, good use of width  
**Cons**: Speeds smaller than ideal

---

## Design Concept D: "Maximum Speed" (Recommended)

Speeds are HUGE and centered. Minimal stats below as colored bars only.

```
┌────────────────────────────────────────────────────────────────────────────────┐
│  TrueNAS 24.10 • 192.168.1.100                                                 │
│                                                                                │
│                   ▼ 452     ▲ 234                                              │
│                        Mbps                                                    │
│                                                                                │
│  ██████78   ████62   ███45   █12                                               │
│  CPU        RAM      fast    tank                                              │
└────────────────────────────────────────────────────────────────────────────────┘
```

Font sizes:
- Header: 32px (readable, not primary)
- Speed numbers: **140px** (main attraction)
- "Mbps": 48px
- Stats: 36px bars + 28px labels

**Pros**: Speeds immediately visible from 2+ meters, stats as visual bars  
**Cons**: Stats are secondary (but that's fine for a NAS)

---

## Design Concept E: "Arc Dashboard"

Semi-circular arc gauges for a modern dashboard look.

```
┌────────────────────────────────────────────────────────────────────────────────┐
│                           TrueNAS 24.10                                        │
│                                                                                │
│            ▼ 452                    ▲ 234                                      │
│              Mbps                     Mbps                                     │
│                                                                                │
│    ╭───╮  78%    ╭───╮  62%    ╭───╮  45%    ╭───╮  12%                        │
│    ╰───╯  CPU    ╰───╯  RAM    ╰───╯ fast    ╰───╯ tank                        │
└────────────────────────────────────────────────────────────────────────────────┘
```

Arc gauges would be bright yellow/green filled on dark gray background.

**Pros**: Very visual, modern look, easy to read from distance  
**Cons**: Complex rendering, uses vertical space

---

## Color Palette

| Element | Color | Hex |
|---------|-------|-----|
| Background | Dark Navy | #0F172A |
| Primary text | White | #FFFFFF |
| Download | Teal | #00BCD4 |
| Upload | Blue | #3B82F6 |
| Good (0-74%) | Green | #22C55E |
| Warning (75-89%) | Yellow | #FACC15 |
| Critical (90%+) | Red | #EF4444 |
| Gauge background | Dark Gray | #1E293B |
| Secondary text | Gray | #94A3B8 |

---

## Recommended: Concept D with Arc Elements

Combining the best of D and E:

```
┌────────────────────────────────────────────────────────────────────────────────┐
│  TrueNAS 24.10                                            192.168.1.100        │
│                                                                                │
│                    ▼ 452      ▲ 234                                            │
│                         Mbps                                                   │
│                                                                                │
│     ╭──╮         ╭──╮         ╭──╮         ╭──╮                                 │
│     │78│         │62│         │45│         │12│                                 │
│     ╰──╯         ╰──╯         ╰──╯         ╰──╯                                 │
└────────────────────────────────────────────────────────────────────────────────┘
```

### Layout Breakdown

| Row | Height | Content |
|-----|--------|---------|
| Header | 50px | "TrueNAS 24.10" left, IP right (gray, 28px) |
| Speeds | 180px | Huge centered speeds (140px numbers) |
| Gauges | 146px | 4 arc gauges with percentages inside |

### Gauge Design

Each gauge is a 120px diameter semi-circle:
- Dark gray background arc (270° sweep)
- Colored fill arc based on percentage
- Large percentage number in center (48px, white)
- Small label below (24px, gray)

---

## Polling Strategy

```go
// Fast updates (network, CPU) - every 1 second
// Slow updates (RAM, pools) - every 5 seconds
// Static (version, IP) - once at startup
```

This reduces CPU usage by ~80% for the slow-changing data.

