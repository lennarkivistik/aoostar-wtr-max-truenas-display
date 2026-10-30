package display

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"time"
)

const (
	// ChunkSize is the number of RGB565 bytes sent per chunk.
	// This value (47) was determined from reverse-engineering the original protocol.
	// Total image: 960x376x2 = 721,920 bytes / 47 = 15,360 chunks exactly.
	ChunkSize = 47
)

// Protocol headers for image transmission.
// All headers start with magic bytes AA 55 AA 55.
var (
	// CmdImageStart signals the beginning of an image transfer
	CmdImageStart = []byte{0xAA, 0x55, 0xAA, 0x55, 0x05, 0x00, 0x00, 0x00, 0x04, 0x00, 0x0F, 0x2F, 0x00, 0x04, 0x0B, 0x00}
	// CmdImageEnd signals the end of image transfer, display will refresh
	CmdImageEnd = []byte{0xAA, 0x55, 0xAA, 0x55, 0x06, 0x00, 0x00, 0x00}
	// ChunkHeader prefixes each data chunk (followed by 4-byte offset + 47 bytes data)
	ChunkHeader = []byte{0xAA, 0x55, 0xAA, 0x55, 0x08, 0x00, 0x00, 0x00}
)

// FrameStats reports render-to-device transmission cost for one frame.
type FrameStats struct {
	ChangedChunks int
	TotalChunks   int
	BytesWritten  int
	Writes        int
	ConvertTime   time.Duration
	DiffTime      time.Duration
	SendTime      time.Duration
}

// RGBToRGB565 converts a 24-bit RGB888 color to 16-bit RGB565 format.
// RGB565 packs colors as: RRRRRGGG GGGBBBBB (5 bits red, 6 bits green, 5 bits blue).
// This halves memory usage compared to RGB888 while maintaining acceptable color depth.
func RGBToRGB565(r, g, b uint8) uint16 {
	return uint16(r&0xF8)<<8 | uint16(g&0xFC)<<3 | uint16(b)>>3
}

// ImageToRGB565 converts a Go image.Image to a byte slice in RGB565 format.
// The output is in little-endian byte order as required by the display.
// Each pixel becomes 2 bytes, so the output size is width * height * 2.
// OPTIMIZED: Uses direct pixel access for RGBA images.
func ImageToRGB565(img image.Image) []byte {
	return imageToRGB565Into(img, nil)
}

func (d *Display) imageToRGB565(img image.Image) []byte {
	return imageToRGB565Into(img, &d.rgb565Buffer)
}

func imageToRGB565Into(img image.Image, reusable *[]byte) []byte {
	// Try to use fast path for RGBA images (most common case)
	if rgba, ok := img.(*image.RGBA); ok {
		return imageRGBAToRGB565Into(rgba, reusable)
	}

	// Fallback for other image types
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	neededSize := width * height * 2
	data := ensureRGB565Buffer(reusable, neededSize)

	// Convert each pixel row by row, left to right
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			r, g, b, _ := c.RGBA()
			rgb565 := RGBToRGB565(uint8(r>>8), uint8(g>>8), uint8(b>>8))

			offset := (y*width + x) * 2
			binary.LittleEndian.PutUint16(data[offset:], rgb565)
		}
	}

	return data[:neededSize]
}

// imageRGBAToRGB565 is the fast path for RGBA images.
// Uses direct slice access (~10x faster than img.At()).
func imageRGBAToRGB565(img *image.RGBA) []byte {
	return imageRGBAToRGB565Into(img, nil)
}

func imageRGBAToRGB565Into(img *image.RGBA, reusable *[]byte) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	stride := img.Stride
	pix := img.Pix

	neededSize := width * height * 2
	data := ensureRGB565Buffer(reusable, neededSize)

	// Direct pixel access: img.Pix is [R, G, B, A, R, G, B, A, ...]
	// Each row starts at y*stride, each pixel is 4 bytes
	outOffset := 0
	for y := 0; y < height; y++ {
		rowStart := (bounds.Min.Y+y-img.Rect.Min.Y)*stride + (bounds.Min.X-img.Rect.Min.X)*4
		for x := 0; x < width; x++ {
			pixOffset := rowStart + x*4
			r := pix[pixOffset]
			g := pix[pixOffset+1]
			b := pix[pixOffset+2]

			// RGB565: RRRRRGGG GGGBBBBB
			rgb565 := uint16(r&0xF8)<<8 | uint16(g&0xFC)<<3 | uint16(b)>>3
			data[outOffset] = byte(rgb565)
			data[outOffset+1] = byte(rgb565 >> 8)
			outOffset += 2
		}
	}

	return data[:neededSize]
}

func ensureRGB565Buffer(reusable *[]byte, neededSize int) []byte {
	if reusable == nil {
		return make([]byte, neededSize)
	}
	if cap(*reusable) < neededSize {
		*reusable = make([]byte, neededSize)
	}
	return (*reusable)[:neededSize]
}

// SendImage converts and transmits an image to the display.
//
// The transmission protocol works as follows:
//  1. Send image start command
//  2. Send image data in 47-byte chunks, each prefixed with header + offset
//  3. Send image end command (triggers display refresh)
//
// With frame caching enabled (default), only changed chunks are sent.
// This dramatically speeds up partial screen updates (e.g., number changes).
// A full 960x376 image has 15,360 chunks, but typical dashboard updates
// only change ~500-2000 chunks, making updates 5-10x faster.
func (d *Display) SendImage(img image.Image) (FrameStats, error) {
	var stats FrameStats

	// Convert image to RGB565 format
	convertStart := time.Now()
	rgb565Data := d.imageToRGB565(img)
	stats.ConvertTime = time.Since(convertStart)

	diffStart := time.Now()
	dirtyChunks := d.findDirtyChunks(rgb565Data)
	stats.DiffTime = time.Since(diffStart)
	stats.ChangedChunks = len(dirtyChunks)
	stats.TotalChunks = chunkCount(len(rgb565Data))

	if d.enableCache && d.prevFrame != nil && len(dirtyChunks) == 0 {
		return stats, nil
	}

	sendStart := time.Now()

	if err := d.sendCounted(CmdImageStart, &stats); err != nil {
		return stats, fmt.Errorf("failed to send image start: %w", err)
	}

	// Send image data in chunks (with optional differential updates)
	// Each chunk: [8-byte header] [4-byte offset LE] [47 bytes RGB565 data]
	for _, offset := range dirtyChunks {
		end := offset + ChunkSize
		if end > len(rgb565Data) {
			end = len(rgb565Data)
		}
		chunk := rgb565Data[offset:end]

		if err := d.sendChunkPacket(offset, chunk, &stats); err != nil {
			return stats, fmt.Errorf("failed to send chunk at offset %d: %w", offset, err)
		}
	}

	// Signal end of transfer - this triggers the display to show the new image
	if err := d.sendAndDrainCounted(CmdImageEnd, &stats); err != nil {
		return stats, fmt.Errorf("failed to send image end: %w", err)
	}
	stats.SendTime = time.Since(sendStart)

	// Cache current frame for next comparison
	if d.enableCache {
		if d.prevFrame == nil || len(d.prevFrame) != len(rgb565Data) {
			d.prevFrame = make([]byte, len(rgb565Data))
		}
		copy(d.prevFrame, rgb565Data)
	}

	return stats, nil
}

func chunkCount(byteLen int) int {
	if byteLen == 0 {
		return 0
	}
	return (byteLen + ChunkSize - 1) / ChunkSize
}

func (d *Display) findDirtyChunks(rgb565Data []byte) []int {
	d.dirtyChunks = d.dirtyChunks[:0]
	for offset := 0; offset < len(rgb565Data); offset += ChunkSize {
		end := offset + ChunkSize
		if end > len(rgb565Data) {
			end = len(rgb565Data)
		}

		if d.enableCache && d.prevFrame != nil && end <= len(d.prevFrame) &&
			chunksEqual(d.prevFrame[offset:end], rgb565Data[offset:end]) {
			continue
		}

		d.dirtyChunks = append(d.dirtyChunks, offset)
	}
	return d.dirtyChunks
}

func (d *Display) sendChunkPacket(offset int, chunk []byte, stats *FrameStats) error {
	packetLen := len(ChunkHeader) + 4 + len(chunk)
	if cap(d.packetBuffer) < packetLen {
		d.packetBuffer = make([]byte, len(ChunkHeader)+4+ChunkSize)
	}
	packet := d.packetBuffer[:packetLen]
	copy(packet, ChunkHeader)
	binary.LittleEndian.PutUint32(packet[len(ChunkHeader):], uint32(offset))
	copy(packet[len(ChunkHeader)+4:], chunk)

	return d.sendCounted(packet, stats)
}

func (d *Display) sendCounted(data []byte, stats *FrameStats) error {
	if err := d.send(data); err != nil {
		return err
	}
	stats.BytesWritten += len(data)
	stats.Writes++
	return nil
}

func (d *Display) sendAndDrainCounted(data []byte, stats *FrameStats) error {
	if err := d.sendCounted(data, stats); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if err := d.port.Drain(); err != nil {
			if i < 2 {
				continue
			}
			return fmt.Errorf("failed to drain port: %w", err)
		}
		break
	}
	return nil
}

// chunksEqual compares two byte slices for equality.
// Uses Go's optimized bytes.Equal (SIMD on supported platforms).
func chunksEqual(a, b []byte) bool {
	return bytes.Equal(a, b)
}

// CopyImage copies the content of src to dst efficiently.
// Both images must be the same size.
func CopyImage(dst, src *image.RGBA) {
	copy(dst.Pix, src.Pix)
}

// NewDisplayImage creates a new RGBA image buffer sized for the display (960x376).
// This is the starting point for rendering - draw on this image, then send it.
func NewDisplayImage() *image.RGBA {
	return image.NewRGBA(image.Rect(0, 0, Width, Height))
}

// FillImage fills an entire image with a solid color.
// Typically used to set the background before drawing other elements.
// OPTIMIZED: Uses direct slice access instead of img.Set().
func FillImage(img *image.RGBA, c color.Color) {
	// Convert color to RGBA bytes once
	r, g, b, a := c.RGBA()
	rb := uint8(r >> 8)
	gb := uint8(g >> 8)
	bb := uint8(b >> 8)
	ab := uint8(a >> 8)

	pix := img.Pix
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	stride := img.Stride

	// Fill first row
	rowStart := (bounds.Min.Y-img.Rect.Min.Y)*stride + (bounds.Min.X-img.Rect.Min.X)*4
	for x := 0; x < width; x++ {
		offset := rowStart + x*4
		pix[offset] = rb
		pix[offset+1] = gb
		pix[offset+2] = bb
		pix[offset+3] = ab
	}

	// Copy first row to all subsequent rows (much faster than pixel-by-pixel)
	firstRow := pix[rowStart : rowStart+width*4]
	for y := 1; y < height; y++ {
		destStart := rowStart + y*stride
		copy(pix[destStart:destStart+width*4], firstRow)
	}
}
