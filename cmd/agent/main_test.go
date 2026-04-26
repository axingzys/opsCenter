package main

import "testing"

func TestBuildMountedDiskIODeviceSet(t *testing.T) {
	disks := []diskInfo{
		{Device: "/dev/dm-0", MountPoint: "/", Fstype: "ext4"},
		{Device: "/dev/sda2", MountPoint: "/boot", Fstype: "ext4"},
		{Device: "/dev/loop0", MountPoint: "/snap/core20/1", Fstype: "squashfs"},
		{Device: "efivarfs", MountPoint: "/sys/firmware/efi/efivars", Fstype: "efivarfs"},
	}

	items := buildMountedDiskIODeviceSet(disks)
	if len(items) != 2 {
		t.Fatalf("expected 2 mounted devices, got %d", len(items))
	}
	if _, ok := items["dm-0"]; !ok {
		t.Fatalf("expected dm-0 in mounted device set")
	}
	if _, ok := items["sda2"]; !ok {
		t.Fatalf("expected sda2 in mounted device set")
	}
	if _, ok := items["loop0"]; ok {
		t.Fatalf("did not expect loop0 in mounted device set")
	}
}

func TestNormalizeMetricDiskDeviceName(t *testing.T) {
	if got := normalizeMetricDiskDeviceName(`/dev/dm-0`); got != "dm-0" {
		t.Fatalf("unexpected linux device normalization: %q", got)
	}
	if got := normalizeMetricDiskDeviceName(`C:\\`); got != "c:" {
		t.Fatalf("unexpected windows device normalization: %q", got)
	}
}
