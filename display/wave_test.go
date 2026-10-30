package display

import (
	"image/color"
	"testing"
)

var benchWavePoints = []WavePoint{
	{X: -80, Y: 0.10},
	{X: 0, Y: 0.30},
	{X: 120, Y: 0.20},
	{X: 260, Y: 0.80},
	{X: 420, Y: 0.40},
	{X: 620, Y: 0.70},
	{X: 820, Y: 0.35},
	{X: 1040, Y: 0.55},
}

func BenchmarkDrawTimestampedWaveFilled(b *testing.B) {
	benchmarkDrawTimestampedWave(b, color.NRGBA{34, 211, 238, 60})
}

func BenchmarkDrawTimestampedWaveLineOnly(b *testing.B) {
	benchmarkDrawTimestampedWave(b, nil)
}

func benchmarkDrawTimestampedWave(b *testing.B, fill color.Color) {
	img := NewDisplayImage()
	line := color.RGBA{34, 211, 238, 255}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FillImage(img, color.RGBA{15, 23, 42, 255})
		DrawTimestampedWave(img, 120, 100, Width, benchWavePoints, line, fill)
	}
}

func TestWaveFillUsesStraightAlpha(t *testing.T) {
	img := NewDisplayImage()
	FillImage(img, color.RGBA{10, 20, 30, 255})
	DrawTimestampedWave(img, 0, 100, Width, []WavePoint{{X: 0, Y: 0.8}, {X: Width, Y: 0.8}}, color.White, color.NRGBA{40, 180, 220, 64})
	got := img.RGBAAt(100, 50)
	want := color.RGBA{uint8((40*64 + 10*191) / 255), uint8((180*64 + 20*191) / 255), uint8((220*64 + 30*191) / 255), 255}
	if got != want {
		t.Fatalf("fill = %v, want %v", got, want)
	}
}
