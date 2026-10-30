package network

import (
	"testing"
	"time"
)

func TestThroughputResetsBaselineWithoutUnsignedSpike(t *testing.T) {
	previous := &Stats{RxBytes: 10000, TxBytes: 5000, Time: time.Unix(0, 0)}
	reset := &Stats{RxBytes: 10, TxBytes: 6000, Time: time.Unix(1, 0)}
	if got := throughputBetween(reset, previous); got.RxBytesPerSec != 0 || got.TxBytesPerSec != 0 {
		t.Fatalf("reset spike: %+v", got)
	}
	next := &Stats{RxBytes: 110, TxBytes: 6200, Time: time.Unix(2, 0)}
	if got := throughputBetween(next, reset); got.RxBytesPerSec != 100 || got.TxBytesPerSec != 200 {
		t.Fatalf("failed to resume: %+v", got)
	}
}
