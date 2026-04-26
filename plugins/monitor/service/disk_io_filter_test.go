package service

import "testing"

func TestFilterMonitorDiskIOSeries(t *testing.T) {
	series := []prometheusMatrixSeries{
		{Metric: map[string]string{"device": "dm-0"}},
		{Metric: map[string]string{"device": "sda"}},
		{Metric: map[string]string{"device": "sda3"}},
		{Metric: map[string]string{"device": "sda2"}},
	}

	filtered := filterMonitorDiskIOSeries(series, map[string]struct{}{
		"dm-0": {},
		"sda2": {},
	})

	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered series, got %d", len(filtered))
	}
	if filtered[0].Metric["device"] != "dm-0" {
		t.Fatalf("expected dm-0, got %q", filtered[0].Metric["device"])
	}
	if filtered[1].Metric["device"] != "sda2" {
		t.Fatalf("expected sda2, got %q", filtered[1].Metric["device"])
	}
}

func TestMonitorDiskDeviceCandidates(t *testing.T) {
	items := monitorDiskDeviceCandidates(`/dev/sda2`)
	if len(items) == 0 || items[0] != "sda2" {
		t.Fatalf("unexpected normalized candidates: %#v", items)
	}

	windows := monitorDiskDeviceCandidates(`D:\\`)
	if len(windows) == 0 || windows[0] != "d:" {
		t.Fatalf("unexpected windows candidates: %#v", windows)
	}
}
