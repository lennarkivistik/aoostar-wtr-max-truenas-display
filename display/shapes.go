// Package display shapes.go provides drawing primitives for UI elements
// like progress bars, arc gauges, sparklines, and lines.
package display

import (
	"image"
	"image/color"
)

// setPixel sets a pixel directly without bounds checking (faster than img.Set)
// Caller must ensure x,y are within bounds
func setPixel(img *image.RGBA, x, y int, r, g, b, a uint8) {
	offset := (y-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)*4
	img.Pix[offset] = r
	img.Pix[offset+1] = g
	img.Pix[offset+2] = b
	img.Pix[offset+3] = a
}

// setPixelColor sets a pixel using a color.Color (slightly slower due to RGBA() call)
func setPixelColor(img *image.RGBA, x, y int, c color.Color) {
	r, g, b, a := c.RGBA()
	offset := (y-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)*4
	img.Pix[offset] = uint8(r >> 8)
	img.Pix[offset+1] = uint8(g >> 8)
	img.Pix[offset+2] = uint8(b >> 8)
	img.Pix[offset+3] = uint8(a >> 8)
}

// DrawLine draws a horizontal line
// OPTIMIZED: Uses direct pixel access
func DrawLine(img *image.RGBA, x1, y, x2 int, c color.Color) {
	bounds := img.Bounds()
	if y < bounds.Min.Y || y >= bounds.Max.Y {
		return
	}
	if x1 < bounds.Min.X {
		x1 = bounds.Min.X
	}
	if x2 >= bounds.Max.X {
		x2 = bounds.Max.X - 1
	}
	if x1 > x2 {
		return
	}

	r, g, b, a := c.RGBA()
	rb, gb, bb, ab := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)

	for x := x1; x <= x2; x++ {
		setPixel(img, x, y, rb, gb, bb, ab)
	}
}

// blendPixel mixes a color with the existing pixel color (alpha blending)
// OPTIMIZED: Uses direct slice access and integer math (no float64)
func blendPixel(img *image.RGBA, x, y int, c color.Color) {
	// Bounds check
	bounds := img.Bounds()
	if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
		return
	}

	r, g, b, a := c.RGBA()
	if a == 0 {
		return
	}

	// Direct pixel access
	offset := (y-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)*4
	pix := img.Pix

	// If fully opaque, just set directly
	if a == 0xFFFF {
		pix[offset] = uint8(r >> 8)
		pix[offset+1] = uint8(g >> 8)
		pix[offset+2] = uint8(b >> 8)
		pix[offset+3] = 255
		return
	}

	// Get existing pixel (8-bit values)
	bgR := uint32(pix[offset])
	bgG := uint32(pix[offset+1])
	bgB := uint32(pix[offset+2])
	bgA := uint32(pix[offset+3])

	// Convert source to 8-bit
	srcR := r >> 8
	srcG := g >> 8
	srcB := b >> 8
	srcA := a >> 8

	// Alpha blending using 8-bit math
	// out = src * srcAlpha + dst * (1 - srcAlpha)
	// Using 255 as max alpha, integer division
	invA := 255 - srcA

	// Blend each channel: (src * alpha + dst * (255-alpha)) / 255
	outR := (srcR*srcA + bgR*invA) / 255
	outG := (srcG*srcA + bgG*invA) / 255
	outB := (srcB*srcA + bgB*invA) / 255
	outA := srcA + (bgA*invA)/255

	pix[offset] = uint8(outR)
	pix[offset+1] = uint8(outG)
	pix[offset+2] = uint8(outB)
	pix[offset+3] = uint8(outA)
}

// drawLineBresenham draws a line between two points using Bresenham's algorithm.
// Works for lines at any angle, not just horizontal.
// OPTIMIZED: Uses direct pixel access
func drawLineBresenham(img *image.RGBA, x1, y1, x2, y2 int, c color.Color) {
	r, g, b, a := c.RGBA()
	rb, gb, bb, ab := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)

	bounds := img.Bounds()
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)
	sx := 1
	sy := 1
	if x1 > x2 {
		sx = -1
	}
	if y1 > y2 {
		sy = -1
	}
	err := dx - dy

	for {
		// Draw with bounds check
		if x1 >= bounds.Min.X && x1 < bounds.Max.X && y1 >= bounds.Min.Y && y1 < bounds.Max.Y {
			setPixel(img, x1, y1, rb, gb, bb, ab)
		}
		// Thicker line
		if x1 >= bounds.Min.X && x1 < bounds.Max.X && y1+1 >= bounds.Min.Y && y1+1 < bounds.Max.Y {
			setPixel(img, x1, y1+1, rb, gb, bb, ab)
		}

		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// DrawFilledRect draws a filled rectangle.
// OPTIMIZED: Uses row copy like FillImage for better cache performance
func DrawFilledRect(img *image.RGBA, x, y, width, height int, c color.Color) {
	bounds := img.Bounds()

	// Clamp to image bounds
	if x < bounds.Min.X {
		width -= bounds.Min.X - x
		x = bounds.Min.X
	}
	if y < bounds.Min.Y {
		height -= bounds.Min.Y - y
		y = bounds.Min.Y
	}
	if x+width > bounds.Max.X {
		width = bounds.Max.X - x
	}
	if y+height > bounds.Max.Y {
		height = bounds.Max.Y - y
	}
	if width <= 0 || height <= 0 {
		return
	}

	// Convert color to bytes once
	r, g, b, a := c.RGBA()
	rb, gb, bb, ab := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)

	pix := img.Pix
	stride := img.Stride

	// Fill first row
	rowStart := (y-img.Rect.Min.Y)*stride + (x-img.Rect.Min.X)*4
	for px := 0; px < width; px++ {
		offset := rowStart + px*4
		pix[offset] = rb
		pix[offset+1] = gb
		pix[offset+2] = bb
		pix[offset+3] = ab
	}

	// Copy first row to subsequent rows
	firstRow := pix[rowStart : rowStart+width*4]
	for py := 1; py < height; py++ {
		destStart := rowStart + py*stride
		copy(pix[destStart:destStart+width*4], firstRow)
	}
}

// DrawFilledCircle draws a filled circle.
// Uses squared distance comparison to avoid sqrt for performance.
func DrawFilledCircle(img *image.RGBA, cx, cy, radius int, c color.Color) {
	bounds := img.Bounds()
	radiusSq := radius * radius

	r, g, b, a := c.RGBA()
	rb, gb, bb, ab := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)

	for py := cy - radius; py <= cy+radius; py++ {
		if py < bounds.Min.Y || py >= bounds.Max.Y {
			continue
		}
		dy := py - cy
		dySq := dy * dy

		for px := cx - radius; px <= cx+radius; px++ {
			if px < bounds.Min.X || px >= bounds.Max.X {
				continue
			}
			dx := px - cx
			if dx*dx+dySq <= radiusSq {
				setPixel(img, px, py, rb, gb, bb, ab)
			}
		}
	}
}

// DrawRoundedRect draws a filled rectangle with rounded corners.
// The radius parameter controls the corner roundness.
func DrawRoundedRect(img *image.RGBA, x, y, width, height, radius int, c color.Color) {
	bounds := img.Bounds()

	// Clamp radius to half the smallest dimension
	maxRadius := width / 2
	if height/2 < maxRadius {
		maxRadius = height / 2
	}
	if radius > maxRadius {
		radius = maxRadius
	}

	r, g, b, a := c.RGBA()
	rb, gb, bb, ab := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)

	radiusSq := radius * radius

	// Draw the rectangle row by row
	for py := 0; py < height; py++ {
		screenY := y + py
		if screenY < bounds.Min.Y || screenY >= bounds.Max.Y {
			continue
		}

		// Determine the horizontal span for this row
		rowStart := x
		rowEnd := x + width

		// Check if we're in a corner region (top or bottom)
		inTopCorner := py < radius
		inBottomCorner := py >= height-radius

		if inTopCorner || inBottomCorner {
			// Calculate the y-distance from the corner center
			var cornerCenterY int
			if inTopCorner {
				cornerCenterY = y + radius
			} else {
				cornerCenterY = y + height - radius - 1
			}
			dy := screenY - cornerCenterY
			dySq := dy * dy

			// For each pixel in this row, check if it's inside the rounded corner
			for px := x; px < x+width; px++ {
				if px < bounds.Min.X || px >= bounds.Max.X {
					continue
				}

				// Check left corner
				inLeftCorner := px < x+radius
				// Check right corner
				inRightCorner := px >= x+width-radius

				shouldDraw := true

				if inLeftCorner {
					cornerCenterX := x + radius
					dx := px - cornerCenterX
					if dx*dx+dySq > radiusSq {
						shouldDraw = false
					}
				} else if inRightCorner {
					cornerCenterX := x + width - radius - 1
					dx := px - cornerCenterX
					if dx*dx+dySq > radiusSq {
						shouldDraw = false
					}
				}

				if shouldDraw {
					setPixel(img, px, screenY, rb, gb, bb, ab)
				}
			}
		} else {
			// Middle section - full width horizontal line
			for px := rowStart; px < rowEnd; px++ {
				if px >= bounds.Min.X && px < bounds.Max.X {
					setPixel(img, px, screenY, rb, gb, bb, ab)
				}
			}
		}
	}
}
