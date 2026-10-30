package network

import (
	"fmt"
	"testing"
)

func TestParseZFSPoolsUsesUsableDatasetCapacity(t *testing.T) {
	const tib uint64 = 1024 * 1024 * 1024 * 1024
	used := 444 * tib / 10
	avail := 556 * tib / 10
	input := []byte(fmt.Sprintf("tank\t%d\t%d\nfastpool\t%d\t%d\nboot-pool\t%d\t%d\n",
		used, avail,
		uint64(620)*1024*1024*1024, uint64(1428)*1024*1024*1024,
		uint64(10)*1024*1024*1024, uint64(10)*1024*1024*1024,
	))

	pools := parseZFSPools(input)
	if len(pools) != 3 {
		t.Fatalf("len(pools) = %d, want 3", len(pools))
	}
	tank := pools[0]
	if tank.Name != "tank" {
		t.Fatalf("pool name = %q, want tank", tank.Name)
	}
	if tank.Used != "44.4T" || tank.Free != "55.6T" || tank.Size != "100T" {
		t.Fatalf("tank capacity = used %q free %q size %q, want 44.4T 55.6T 100T", tank.Used, tank.Free, tank.Size)
	}
	if tank.UsedPercent < 44.39 || tank.UsedPercent > 44.41 {
		t.Fatalf("tank UsedPercent = %.4f, want about 44.4", tank.UsedPercent)
	}
}

func TestParseZFSPoolsIgnoresRawZpoolListShape(t *testing.T) {
	input := []byte("tank\t100T\t44.4T\t55.6T\t44%\n")
	if pools := parseZFSPools(input); len(pools) != 0 {
		t.Fatalf("parseZFSPools accepted raw zpool list output: %#v", pools)
	}
}

func TestParseZFSPoolsSkipsChildDatasetsAndBadLines(t *testing.T) {
	input := []byte("tank/data\t1024\t1024\nbad\tone\t1024\ntank\t1024\t3072\n")
	pools := parseZFSPools(input)
	if len(pools) != 1 {
		t.Fatalf("len(pools) = %d, want 1: %#v", len(pools), pools)
	}
	if pools[0].Name != "tank" || pools[0].Used != "1.0K" || pools[0].Size != "4.0K" {
		t.Fatalf("pool = %#v, want root tank usable bytes", pools[0])
	}
}

func TestParseDiskSlotsUsesStableChassisPathOrder(t *testing.T) {
	input := []byte(`{
		"blockdevices": [
			{"name":"sda","type":"disk","model":"Example SATA disk","serial":"DEMO-SATA-4","size":"16.4T","tran":"sata","path":"/dev/sda"},
			{"name":"sdb","type":"disk","model":"Example SATA disk","serial":"DEMO-SATA-2","size":"16.4T","tran":"sata","path":"/dev/sdb"},
			{"name":"sdc","type":"disk","model":"Example SATA disk","serial":"DEMO-SATA-3","size":"16.4T","tran":"sata","path":"/dev/sdc"},
			{"name":"sdd","type":"disk","model":"MassStorageClass","serial":"DEMO-USB-1","size":"0B","tran":"usb","path":"/dev/sdd"},
			{"name":"sde","type":"disk","model":"Example SATA disk","serial":"DEMO-SATA-1","size":"16.4T","tran":"sata","path":"/dev/sde"},
			{"name":"nvme0n1","type":"disk","model":"Example NVMe disk","serial":"DEMO-NVME-1","size":"1.8T","tran":"nvme","path":"/dev/nvme0n1"}
		]
	}`)
	pathTargets := map[string]string{
		"pci-0000:67:00.0-ata-6.0": "sde",
		"pci-0000:67:00.0-ata-5.0": "sdb",
		"pci-0000:67:00.0-ata-4.0": "sdc",
		"pci-0000:67:00.0-ata-3.0": "sda",
		"pci-0000:6a:00.0-nvme-1":  "nvme0n1",
	}

	slots, err := ParseDiskSlots(input, pathTargets)
	if err != nil {
		t.Fatalf("ParseDiskSlots returned error: %v", err)
	}

	want := []struct {
		kind   string
		bay    int
		device string
		serial string
	}{
		{"sata", 1, "sde", "DEMO-SATA-1"},
		{"sata", 2, "sdb", "DEMO-SATA-2"},
		{"sata", 3, "sdc", "DEMO-SATA-3"},
		{"sata", 4, "sda", "DEMO-SATA-4"},
		{"nvme", 1, "nvme0n1", "DEMO-NVME-1"},
	}

	for _, tt := range want {
		slot := findSlot(t, slots, tt.kind, tt.bay)
		if !slot.Present || slot.Device != tt.device || slot.Serial != tt.serial {
			t.Fatalf("%s slot %d = %#v, want device=%s serial=%s", tt.kind, tt.bay, slot, tt.device, tt.serial)
		}
	}
	for _, bay := range []int{5, 6} {
		slot := findSlot(t, slots, "sata", bay)
		if slot.Present {
			t.Fatalf("sata slot %d present = true, want empty: %#v", bay, slot)
		}
	}
}

func TestParseDiskSlotsReturnsAllChassisSlotsWhenEmpty(t *testing.T) {
	slots, err := ParseDiskSlots([]byte(`{"blockdevices":[]}`), nil)
	if err != nil {
		t.Fatalf("ParseDiskSlots returned error: %v", err)
	}
	if len(slots) != 11 {
		t.Fatalf("len(slots) = %d, want 11", len(slots))
	}
	for _, tt := range []struct {
		kind  string
		count int
	}{
		{"sata", 6},
		{"nvme", 5},
	} {
		for bay := 1; bay <= tt.count; bay++ {
			slot := findSlot(t, slots, tt.kind, bay)
			if slot.Present {
				t.Fatalf("%s slot %d present = true, want empty", tt.kind, bay)
			}
		}
	}
}

func TestReadDiskByPathTargetsIgnoresUSBMassStorage(t *testing.T) {
	if !ignoredDiskByPaths["pci-0000:03:00.4-usbv3-0:1.4:1.0-scsi-0:0:0:0"] {
		t.Fatal("usbv3 mass storage path is not ignored")
	}
	if !ignoredDiskByPaths["pci-0000:03:00.4-usb-0:1.4:1.0-scsi-0:0:0:0"] {
		t.Fatal("usb mass storage path is not ignored")
	}
}

func TestParseDiskSlotsDoesNotUseSerialToLocateSlots(t *testing.T) {
	input := []byte(`{
		"blockdevices": [
			{"name":"sdb","type":"disk","model":"Example SATA disk","serial":"DEMO-SATA-2","size":"16.4T","tran":"sata","path":"/dev/sdb"}
		]
	}`)
	pathTargets := map[string]string{
		"pci-0000:67:00.0-ata-6.0": "sdb",
	}

	slots, err := ParseDiskSlots(input, pathTargets)
	if err != nil {
		t.Fatalf("ParseDiskSlots returned error: %v", err)
	}
	slot1 := findSlot(t, slots, "sata", 1)
	if !slot1.Present || slot1.Serial != "DEMO-SATA-2" {
		t.Fatalf("sata slot 1 = %#v, want present path-mapped disk", slot1)
	}
	slot2 := findSlot(t, slots, "sata", 2)
	if slot2.Present {
		t.Fatalf("sata slot 2 present = true, serial must not locate a slot: %#v", slot2)
	}
}

func findSlot(t *testing.T, slots []DiskSlot, kind string, bay int) DiskSlot {
	t.Helper()
	for _, slot := range slots {
		if slot.Kind == kind && slot.Bay == bay {
			return slot
		}
	}
	t.Fatalf("missing %s slot %d in %#v", kind, bay, slots)
	return DiskSlot{}
}
