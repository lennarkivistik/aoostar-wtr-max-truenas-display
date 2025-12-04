// Package display text.go provides text rendering capabilities using Go's built-in fonts.
// It uses golang.org/x/image/font for high-quality, scalable text rendering.
package display

import (
	"fmt"
	"image"
	"image/color"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// faceCacheKey identifies a specific font configuration
type faceCacheKey struct {
	size float64
	bold bool
}

// TextRenderer handles text rendering with support for regular and bold fonts.
// It uses Go's embedded fonts (Go Regular and Go Bold) which are clean,
// modern sans-serif fonts suitable for dashboard displays.
type TextRenderer struct {
	regularFont *opentype.Font
	boldFont    *opentype.Font

	// Cache for font faces to avoid expensive re-creation
	faceCache map[faceCacheKey]font.Face
	mu        sync.Mutex
}

// NewTextRenderer creates a new text renderer with embedded Go fonts.
// The fonts are parsed once and reused for all text operations.
func NewTextRenderer() *TextRenderer {
	regular, _ := opentype.Parse(goregular.TTF)
	bold, _ := opentype.Parse(gobold.TTF)

	return &TextRenderer{
		regularFont: regular,
		boldFont:    bold,
		faceCache:   make(map[faceCacheKey]font.Face),
	}
}

// getFace retrieves or creates a cached font.Face at the specified point size.
// DPI is set to 72 which gives 1:1 point-to-pixel ratio.
func (t *TextRenderer) getFace(size float64, bold bool) font.Face {
	key := faceCacheKey{size, bold}

	t.mu.Lock()
	if face, ok := t.faceCache[key]; ok {
		t.mu.Unlock()
		return face
	}
	t.mu.Unlock()

	f := t.regularFont
	if bold {
		f = t.boldFont
	}

	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		fmt.Printf("Error creating font face: %v\n", err)
		return nil
	}

	t.mu.Lock()
	t.faceCache[key] = face
	t.mu.Unlock()

	return face
}

// DrawText renders text at the specified (x, y) position.
// The y coordinate represents the baseline of the text.
// Use bold=true for emphasized text like values and headers.
func (t *TextRenderer) DrawText(img *image.RGBA, x, y int, text string, c color.Color, size float64, bold bool) {
	face := t.getFace(size, bold)
	if face == nil {
		return
	}

	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	drawer.DrawString(text)
}

// DrawTextCentered renders text horizontally centered on the image.
// Useful for titles and single-line centered content.
// The y coordinate is the text baseline.
func (t *TextRenderer) DrawTextCentered(img *image.RGBA, y int, text string, c color.Color, size float64, bold bool) {
	face := t.getFace(size, bold)
	if face == nil {
		return
	}

	bounds := img.Bounds()
	advance := font.MeasureString(face, text)
	x := (bounds.Dx() - advance.Ceil()) / 2

	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	drawer.DrawString(text)
}

// MeasureText calculates the width of text in pixels without rendering it.
// Useful for layout calculations, such as positioning elements after text.
func (t *TextRenderer) MeasureText(text string, size float64, bold bool) int {
	face := t.getFace(size, bold)
	if face == nil {
		return 0
	}

	advance := font.MeasureString(face, text)
	return advance.Ceil()
}

