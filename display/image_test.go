package display

import (
	"image"
	"image/color"
	"testing"
	"time"

	"go.bug.st/serial"
)

type fakePort struct {
	writes int
	bytes  int
}

func (f *fakePort) SetMode(*serial.Mode) error                           { return nil }
func (f *fakePort) Read([]byte) (int, error)                             { return 0, nil }
func (f *fakePort) Drain() error                                         { return nil }
func (f *fakePort) ResetInputBuffer() error                              { return nil }
func (f *fakePort) ResetOutputBuffer() error                             { return nil }
func (f *fakePort) SetDTR(bool) error                                    { return nil }
func (f *fakePort) SetRTS(bool) error                                    { return nil }
func (f *fakePort) GetModemStatusBits() (*serial.ModemStatusBits, error) { return nil, nil }
func (f *fakePort) SetReadTimeout(time.Duration) error                   { return nil }
func (f *fakePort) Close() error                                         { return nil }
func (f *fakePort) Break(time.Duration) error                            { return nil }

func (f *fakePort) Write(p []byte) (int, error) {
	f.writes++
	f.bytes += len(p)
	return len(p), nil
}

func newTestDisplay() (*Display, *fakePort) {
	port := &fakePort{}
	return &Display{
		port:        port,
		enableCache: true,
	}, port
}

func solidImage(c color.Color) *image.RGBA {
	img := NewDisplayImage()
	FillImage(img, c)
	return img
}

func TestSendImageSkipsNoopFrame(t *testing.T) {
	d, port := newTestDisplay()
	img := solidImage(color.RGBA{R: 10, G: 20, B: 30, A: 255})

	first, err := d.SendImage(img)
	if err != nil {
		t.Fatalf("first SendImage failed: %v", err)
	}
	if first.ChangedChunks != first.TotalChunks {
		t.Fatalf("first frame changed chunks = %d, want %d", first.ChangedChunks, first.TotalChunks)
	}
	writesAfterFirst := port.writes
	bytesAfterFirst := port.bytes

	second, err := d.SendImage(img)
	if err != nil {
		t.Fatalf("second SendImage failed: %v", err)
	}
	if second.ChangedChunks != 0 {
		t.Fatalf("noop changed chunks = %d, want 0", second.ChangedChunks)
	}
	if second.Writes != 0 || second.BytesWritten != 0 {
		t.Fatalf("noop wrote %d bytes in %d writes, want none", second.BytesWritten, second.Writes)
	}
	if port.writes != writesAfterFirst || port.bytes != bytesAfterFirst {
		t.Fatalf("port changed on noop: writes %d->%d bytes %d->%d",
			writesAfterFirst, port.writes, bytesAfterFirst, port.bytes)
	}
}

func TestSendImageWritesChunkPacketsSeparately(t *testing.T) {
	d, _ := newTestDisplay()
	img := solidImage(color.RGBA{R: 1, G: 2, B: 3, A: 255})

	stats, err := d.SendImage(img)
	if err != nil {
		t.Fatalf("SendImage failed: %v", err)
	}
	if stats.TotalChunks != Width*Height*2/ChunkSize {
		t.Fatalf("total chunks = %d, want %d", stats.TotalChunks, Width*Height*2/ChunkSize)
	}
	wantWrites := stats.TotalChunks + 2 // start + each chunk packet + end
	if stats.Writes != wantWrites {
		t.Fatalf("writes = %d, want %d", stats.Writes, wantWrites)
	}
}

func TestSendImageNoCacheAlwaysSends(t *testing.T) {
	d, _ := newTestDisplay()
	d.SetCacheEnabled(false)
	img := solidImage(color.RGBA{R: 1, G: 2, B: 3, A: 255})

	first, err := d.SendImage(img)
	if err != nil {
		t.Fatalf("first SendImage failed: %v", err)
	}
	second, err := d.SendImage(img)
	if err != nil {
		t.Fatalf("second SendImage failed: %v", err)
	}
	if first.ChangedChunks != first.TotalChunks {
		t.Fatalf("first changed chunks = %d, want %d", first.ChangedChunks, first.TotalChunks)
	}
	if second.ChangedChunks != second.TotalChunks {
		t.Fatalf("second changed chunks = %d, want %d", second.ChangedChunks, second.TotalChunks)
	}
	if second.Writes == 0 || second.BytesWritten == 0 {
		t.Fatalf("no-cache second frame wrote no bytes")
	}
}

func BenchmarkImageToRGB565(b *testing.B) {
	img := solidImage(color.RGBA{R: 12, G: 34, B: 56, A: 255})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ImageToRGB565(img)
	}
}

func BenchmarkDisplayImageToRGB565Reuse(b *testing.B) {
	d, _ := newTestDisplay()
	img := solidImage(color.RGBA{R: 12, G: 34, B: 56, A: 255})
	_ = d.imageToRGB565(img)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.imageToRGB565(img)
	}
}

func BenchmarkSendImageNoop(b *testing.B) {
	d, _ := newTestDisplay()
	img := solidImage(color.RGBA{R: 12, G: 34, B: 56, A: 255})
	if _, err := d.SendImage(img); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := d.SendImage(img); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSendImageFullFrameCompatibleWrites(b *testing.B) {
	d, _ := newTestDisplay()
	d.SetCacheEnabled(false)
	img := solidImage(color.RGBA{R: 12, G: 34, B: 56, A: 255})
	if _, err := d.SendImage(img); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			FillImage(img, color.RGBA{R: 12, G: 34, B: 56, A: 255})
		} else {
			FillImage(img, color.RGBA{R: 13, G: 35, B: 57, A: 255})
		}
		if _, err := d.SendImage(img); err != nil {
			b.Fatal(err)
		}
	}
}
