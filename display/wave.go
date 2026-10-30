package display

import (
	"image"
	"image/color"
)

// WavePoint represents a point with X in wave-space and Y as normalized 0-1.
type WavePoint struct {
	X float64 // X position in wave coordinates (pixels from right edge)
	Y float64 // Normalized value 0.0-1.0
}

// Point represents a generic 2D point for calculations
type Point struct {
	X, Y float64
}

// drawVerticalLineAlpha draws a vertical line with alpha blending.
// OPTIMIZED: Calculates offset once and increments by stride.
func drawVerticalLineAlpha(img *image.RGBA, x, yStart, yEnd int, c color.Color) {
	drawVerticalLineNRGBA(img, x, yStart, yEnd, color.NRGBAModel.Convert(c).(color.NRGBA))
}

func drawVerticalLineNRGBA(img *image.RGBA, x, yStart, yEnd int, straight color.NRGBA) {
	bounds := img.Bounds()
	if x < bounds.Min.X || x >= bounds.Max.X {
		return
	}
	if yStart < bounds.Min.Y {
		yStart = bounds.Min.Y
	}
	if yEnd > bounds.Max.Y {
		yEnd = bounds.Max.Y
	}
	if yStart >= yEnd {
		return
	}

	srcR, srcG, srcB, srcA := uint32(straight.R), uint32(straight.G), uint32(straight.B), uint32(straight.A)

	if srcA == 0 {
		return
	}

	invA := 255 - srcA
	stride := img.Stride
	pix := img.Pix

	offset := (yStart-img.Rect.Min.Y)*stride + (x-img.Rect.Min.X)*4

	for y := yStart; y < yEnd; y++ {
		bgR := uint32(pix[offset])
		bgG := uint32(pix[offset+1])
		bgB := uint32(pix[offset+2])
		bgA := uint32(pix[offset+3])

		outR := (srcR*srcA + bgR*invA) / 255
		outG := (srcG*srcA + bgG*invA) / 255
		outB := (srcB*srcA + bgB*invA) / 255
		outA := srcA + (bgA*invA)/255

		pix[offset] = uint8(outR)
		pix[offset+1] = uint8(outG)
		pix[offset+2] = uint8(outB)
		pix[offset+3] = uint8(outA)

		offset += stride
	}
}

// DrawTimestampedWave draws a wave from timestamped points.
func DrawTimestampedWave(img *image.RGBA, screenY, screenHeight, waveWidth int, points []WavePoint, lineColor, fillColor color.Color) {
	if len(points) < 2 {
		return
	}

	screenWidth := Width

	lr, lg, lb, la := lineColor.RGBA()
	lineR, lineG, lineB := uint8(lr>>8), uint8(lg>>8), uint8(lb>>8)
	lineA := la

	var fillR, fillG, fillB uint8
	var fillA uint32
	var fillStraight color.NRGBA
	hasFill := fillColor != nil
	if hasFill {
		fillStraight = color.NRGBAModel.Convert(fillColor).(color.NRGBA)
		fr, fg, fb, fa := fillColor.RGBA()
		fillR, fillG, fillB = uint8(fr>>8), uint8(fg>>8), uint8(fb>>8)
		fillA = fa
	}

	for i := 0; i < len(points)-1; i++ {
		p1 := points[i]
		p2 := points[i+1]

		p1X := int(p1.X)
		p2X := int(p2.X)
		startX := p1X
		endX := p2X

		if startX < 0 {
			startX = 0
		}
		if endX > screenWidth {
			endX = screenWidth
		}
		if startX >= endX {
			continue
		}

		segmentWidth := p2X - p1X
		if segmentWidth <= 0 {
			continue
		}

		for px := startX; px < endX; px++ {
			t := float64(px-p1X) / float64(segmentWidth)
			// Smoothstep cannot invent peaks or negative troughs between samples.
			curveY := p1.Y + (p2.Y-p1.Y)*t*t*(3-2*t)

			if curveY < 0 {
				curveY = 0
			}
			if curveY > 1 {
				curveY = 1
			}

			screenYpos := screenY + screenHeight - int(curveY*float64(screenHeight))

			if screenYpos >= screenY && screenYpos < screenY+screenHeight {
				if lineA == 0xFFFF {
					setPixel(img, px, screenYpos, lineR, lineG, lineB, 255)
				} else {
					blendPixel(img, px, screenYpos, lineColor)
				}
				if screenYpos+1 < screenY+screenHeight {
					if lineA == 0xFFFF {
						setPixel(img, px, screenYpos+1, lineR, lineG, lineB, 255)
					} else {
						blendPixel(img, px, screenYpos+1, lineColor)
					}
				}
			}

			if hasFill && curveY > 0.02 {
				fillStart := screenYpos + 2
				if fillStart < screenY {
					fillStart = screenY
				}
				fillEnd := screenY + screenHeight

				if fillA == 0xFFFF {
					for py := fillStart; py < fillEnd; py++ {
						setPixel(img, px, py, fillR, fillG, fillB, 255)
					}
				} else if fillA > 0 {
					drawVerticalLineNRGBA(img, px, fillStart, fillEnd, fillStraight)
				}
			}
		}
	}
}

// DrawWavePacket draws a wave at a specific horizontal position.
func DrawWavePacket(img *image.RGBA, progress float64, screenY, screenHeight, packetWidth int, values []float64, lineColor, fillColor color.Color) {
	if len(values) < 3 {
		return
	}

	screenWidth := Width
	startX := screenWidth + packetWidth/2
	endX := -packetWidth - packetWidth/2
	currentX := int(float64(startX) + progress*float64(endX-startX))

	waveStartX := currentX - packetWidth/2
	waveEndX := currentX + packetWidth/2

	drawStartX := waveStartX
	drawEndX := waveEndX
	if drawStartX < 0 {
		drawStartX = 0
	}
	if drawEndX > screenWidth {
		drawEndX = screenWidth
	}
	if drawStartX >= drawEndX {
		return
	}

	maxVal := 0.0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal <= 0.0001 {
		maxVal = 1
	}
	maxVal = maxVal * 1.1

	var points []Point
	xStep := float64(packetWidth) / float64(len(values)-1)

	for i, v := range values {
		px := float64(waveStartX) + float64(i)*xStep
		py := float64(screenY+screenHeight) - (v/maxVal)*float64(screenHeight)
		points = append(points, Point{px, py})
	}

	for px := drawStartX; px < drawEndX; px++ {
		localX := float64(px - waveStartX)
		relX := localX / xStep
		idx := int(relX)
		if idx < 0 {
			idx = 0
		}
		if idx >= len(points)-1 {
			idx = len(points) - 2
		}
		t := relX - float64(idx)
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}

		p0 := points[max(0, idx-1)]
		p1 := points[idx]
		p2 := points[min(len(points)-1, idx+1)]
		p3 := points[min(len(points)-1, idx+2)]

		curveY := catmullRom(p0.Y, p1.Y, p2.Y, p3.Y, t)
		startY := int(curveY)
		endY := screenY + screenHeight

		if startY >= screenY && startY < screenY+screenHeight {
			blendPixel(img, px, startY, lineColor)
			blendPixel(img, px, startY+1, lineColor)
		}

		if fillColor != nil {
			for py := startY + 2; py < endY; py++ {
				if py >= screenY && py < screenY+screenHeight {
					blendPixel(img, px, py, fillColor)
				}
			}
		}
	}
}

// DrawSmoothWave draws a fluid, curved wave graph with opacity fill.
func DrawSmoothWave(img *image.RGBA, x, y, width, height int, values []float64, lineColor color.Color, fillColor color.Color) {
	if len(values) < 2 {
		return
	}

	minVal, maxVal := getMinMax(values)
	maxVal = maxVal * 1.1
	valueRange := maxVal - minVal
	if valueRange <= 0.0001 {
		valueRange = 1
	}

	var points []Point
	xStep := float64(width) / float64(len(values)-1)

	for i, v := range values {
		px := float64(x) + float64(i)*xStep
		py := float64(y+height) - ((v - minVal) / valueRange * float64(height))
		points = append(points, Point{px, py})
	}

	for px := 0; px < width; px++ {
		relX := float64(px) / xStep
		idx := int(relX)
		if idx >= len(points)-1 {
			idx = len(points) - 2
		}
		t := relX - float64(idx)

		p0 := points[max(0, idx-1)]
		p1 := points[idx]
		p2 := points[idx+1]
		p3 := points[min(len(points)-1, idx+2)]

		curveY := catmullRom(p0.Y, p1.Y, p2.Y, p3.Y, t)
		currentX := x + px
		startY := int(curveY)
		endY := y + height

		if startY >= y && startY < y+height {
			blendPixel(img, currentX, startY, lineColor)
			blendPixel(img, currentX, startY+1, lineColor)
		}

		if fillColor != nil {
			for py := startY + 2; py < endY; py++ {
				blendPixel(img, currentX, py, fillColor)
			}
		}
	}
}

// Helpers

func catmullRom(p0, p1, p2, p3, t float64) float64 {
	v0 := (p2 - p0) * 0.5
	v1 := (p3 - p1) * 0.5
	t2 := t * t
	t3 := t * t2
	return (2*p1-2*p2+v0+v1)*t3 + (-3*p1+3*p2-2*v0-v1)*t2 + v0*t + p1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getMinMax(values []float64) (float64, float64) {
	minVal, maxVal := values[0], values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == minVal {
		maxVal += 1.0
	}
	return minVal, maxVal
}
