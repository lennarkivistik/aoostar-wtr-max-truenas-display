package network

import "testing"

func TestZFSStatusReadsContinuationAndLeafErrors(t *testing.T) {
	got := parseZFSStatus([]byte(`  pool: tank
 state: DEGRADED
  scan: resilver in progress since today
        12.5% done, 01:00:00 to go
config:
        NAME STATE READ WRITE CKSUM
        tank DEGRADED 0 0 3
          mirror-0 DEGRADED 0 0 3
            disk-a ONLINE 0 0 3
            disk-b UNAVAIL 0 0 0
errors: No known data errors
`))
	if got == nil || got.ScanProgress != "12.5%" || got.ErrorCount != 3 || !got.ResilverInProgress || got.PoolStates["tank"] != "DEGRADED" {
		t.Fatalf("status = %+v", got)
	}
}

func TestZFSStatusDoesNotTreatMissingDataAsHealthy(t *testing.T) {
	if parseZFSStatus([]byte("no pools available")) != nil {
		t.Fatal("missing pools reported as healthy")
	}
	got := parseZFSStatus([]byte("pool: tank\nstate: UNAVAIL\nerrors: Permanent errors have been detected"))
	if len(got.DegradedPools) != 1 || got.ErrorCount == 0 {
		t.Fatalf("missed fault: %+v", got)
	}
}

func TestARCRequiresMeasuredSize(t *testing.T) {
	if parseARC([]byte("garbage")) != nil {
		t.Fatal("invented ARC data")
	}
	got := parseARC([]byte("size 4 2147483648\nc 4 4294967296\n"))
	if got == nil || got.SizeGB != 2 || got.TargetGB != 4 {
		t.Fatalf("ARC = %+v", got)
	}
}

func TestResilverKeepsItsOwnPoolAndProgress(t *testing.T) {
	got := parseZFSStatus([]byte("pool: first\nstate: ONLINE\nscan: resilver in progress\n 42% done\nconfig:\npool: second\nstate: ONLINE\nscan: scrub in progress\n 70% done\nconfig:"))
	if got.ActivePool != "first" || got.ScanProgress != "42%" || !got.ResilverInProgress || !got.ScrubInProgress {
		t.Fatalf("mixed pool scan: %+v", got)
	}
}
