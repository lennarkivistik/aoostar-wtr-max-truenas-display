package dashboard

import (
	"testing"
	"time"
)

func BenchmarkRenderNetwork(b *testing.B) {
	o := DefaultOptions()
	now := time.Unix(1000, 0)
	s := DemoState(now, o.WaveHistory)
	r := NewFrameRenderer()
	p := Page{View: ViewNetwork}
	r.Frame(s, p, o, now, true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Frame(s, p, o, now, false)
	}
}

func BenchmarkRenderUnchangedStorage(b *testing.B) {
	o := DefaultOptions()
	now := time.Unix(1000, 0)
	s := DemoState(now, o.WaveHistory)
	r := NewFrameRenderer()
	p := Page{View: ViewStorage}
	r.Frame(s, p, o, now, true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Frame(s, p, o, now, false)
	}
}
