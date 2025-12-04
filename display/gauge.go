// Package display shapes.go provides drawing primitives for UI elements
// like progress bars, arc gauges, sparklines, and lines.
package display

import (
	"image"
	"image/color"
	"math"
)

// DrawArcGauge draws a circular arc gauge (like a speedometer).
// The gauge is a thick arc that shows a percentage value.
// Parameters:
//   - cx, cy: center point of the arc
//   - radius: outer radius of the arc
//   - thickness: width of the arc stroke
//   - percent: value to display (0-100)
//   - fillColor: color of the filled portion
//   - bgColor: color of the unfilled background arc
//   - startAngle: starting angle in degrees (0 = right, 90 = down)
//   - sweepAngle: total sweep angle in degrees (e.g., 270 for 3/4 circle)
//
// OPTIMIZED: Uses squared distances (no sqrt), direct pixel access
func DrawArcGauge(img *image.RGBA, cx, cy, radius, thickness int, percent float64, fillColor, bgColor color.Color, startAngle, sweepAngle float64) {
	// Clamp percent
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	innerRadius := radius - thickness
	if innerRadius < 0 {
		innerRadius = 0
	}

	// Use squared radii to avoid sqrt in inner loop
	innerRadiusSq := float64(innerRadius * innerRadius)
	radiusSq := float64(radius * radius)

	// Convert angles to radians
	startRad := startAngle * math.Pi / 180
	endRad := startRad + (sweepAngle * math.Pi / 180)
	fillEndRad := startRad + (sweepAngle * math.Pi / 180 * percent / 100)

	// Pre-convert colors to bytes
	fr, fg, fb, fa := fillColor.RGBA()
	fillR, fillG, fillB, fillA := uint8(fr>>8), uint8(fg>>8), uint8(fb>>8), uint8(fa>>8)
	br, bg_, bb, ba := bgColor.RGBA()
	bgR, bgG, bgB, bgA := uint8(br>>8), uint8(bg_>>8), uint8(bb>>8), uint8(ba>>8)

	bounds := img.Bounds()

	// Draw each pixel in the bounding box
	for py := cy - radius; py <= cy+radius; py++ {
		if py < bounds.Min.Y || py >= bounds.Max.Y {
			continue
		}
		dy := float64(py - cy)
		dySq := dy * dy

		for px := cx - radius; px <= cx+radius; px++ {
			if px < bounds.Min.X || px >= bounds.Max.X {
				continue
			}
			dx := float64(px - cx)
			distSq := dx*dx + dySq

			// Check if point is within the arc ring (using squared distances)
			if distSq >= innerRadiusSq && distSq <= radiusSq {
				// Calculate angle of this point (atan2 gives -PI to PI)
				angle := math.Atan2(dy, dx)

				// Check if angle is within the arc range
				if isAngleInRange(angle, startRad, endRad) {
					if isAngleInRange(angle, startRad, fillEndRad) {
						setPixel(img, px, py, fillR, fillG, fillB, fillA)
					} else {
						setPixel(img, px, py, bgR, bgG, bgB, bgA)
					}
				}
			}
		}
	}
}

// isAngleInRange checks if an angle is within a range, handling wrap-around.
// All angles in radians. Handles cases where end > 2*PI or start/end cross 0.
func isAngleInRange(angle, start, end float64) bool {
	// Normalize angle to [0, 2*PI)
	twoPi := 2 * math.Pi
	for angle < 0 {
		angle += twoPi
	}
	for angle >= twoPi {
		angle -= twoPi
	}

	// Normalize start to [0, 2*PI)
	for start < 0 {
		start += twoPi
	}
	for start >= twoPi {
		start -= twoPi
	}

	// Calculate normalized end
	normalizedEnd := end
	for normalizedEnd < 0 {
		normalizedEnd += twoPi
	}
	for normalizedEnd >= twoPi {
		normalizedEnd -= twoPi
	}

	// If the arc wraps around (end < start after normalization)
	if end-start >= twoPi {
		// Full circle or more
		return true
	}

	if normalizedEnd < start {
		// Arc wraps around 0
		return angle >= start || angle <= normalizedEnd
	}

	// Normal case: start <= angle <= end
	return angle >= start && angle <= normalizedEnd
}

// DrawArcGaugeSimple draws an arc gauge with common defaults.
// Opens at the bottom (135° start, 270° sweep) which is a typical gauge style.
func DrawArcGaugeSimple(img *image.RGBA, cx, cy, radius, thickness int, percent float64, fillColor, bgColor color.Color) {
	DrawArcGauge(img, cx, cy, radius, thickness, percent, fillColor, bgColor, 135, 270)
}
