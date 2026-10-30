package dashboard

import (
	"testing"
	"time"
)

func TestFixedViewNeverAdvances(t *testing.T) {
	o := DefaultOptions()
	if err := o.SetViews("storage", "network,system"); err != nil {
		t.Fatal(err)
	}
	o.Pool = "tank"
	pages := o.Pages([]string{"fastpool", "tank"})
	start := time.Unix(0, 0)
	for _, elapsed := range []time.Duration{0, time.Minute, 48 * time.Hour} {
		if got := o.PageAt(start, start.Add(elapsed), pages); got != (Page{ViewStorage, 1}) {
			t.Fatalf("page = %+v", got)
		}
	}
}

func TestRotationIncludesEveryPoolInOrder(t *testing.T) {
	o := DefaultOptions()
	if err := o.SetViews("", "network,storage,system"); err != nil {
		t.Fatal(err)
	}
	pages := o.Pages([]string{"a", "b", "c"})
	want := []Page{{ViewNetwork, 0}, {ViewStorage, 0}, {ViewStorage, 1}, {ViewStorage, 2}, {ViewSystem, 0}, {ViewNetwork, 0}}
	start := time.Unix(10, 0)
	for i, w := range want {
		if got := o.PageAt(start, start.Add(time.Duration(i)*o.Dwell), pages); got != w {
			t.Fatalf("step %d: %+v != %+v", i, got, w)
		}
	}
}

func TestInvalidOptions(t *testing.T) {
	for _, input := range []string{"", "network,bogus", "network,network"} {
		o := DefaultOptions()
		if o.SetViews("", input) == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	for _, change := range []func(*Options){
		func(o *Options) { o.Dwell = 0 }, func(o *Options) { o.WaveHistory = time.Hour },
		func(o *Options) { o.Profile = "fast" }, func(o *Options) { o.WaveStyle = "rainbow" },
		func(o *Options) { o.WaveScale = "bogus" }, func(o *Options) { o.FrameInterval = -time.Second },
	} {
		o := DefaultOptions()
		change(&o)
		if o.Validate() == nil {
			t.Fatalf("accepted %+v", o)
		}
	}
}
