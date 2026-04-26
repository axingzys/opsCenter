package service

import (
	"context"
	"strings"
)

type monitorInventoryDisk struct {
	Device     string `json:"device"`
	MountPoint string `json:"mountPoint"`
	FSType     string `json:"fstype"`
}

func (s *HostMonitorService) resolveMonitorDiskIODeviceSet(ctx context.Context, hostID uint) map[string]struct{} {
	item, err := s.getInventoryModel(ctx, hostID)
	if err != nil || item == nil || strings.TrimSpace(item.DisksJSON) == "" {
		return nil
	}

	var disks []monitorInventoryDisk
	decodeJSONString(item.DisksJSON, &disks)
	if len(disks) == 0 {
		return nil
	}

	devices := make(map[string]struct{}, len(disks))
	for _, disk := range disks {
		if shouldIgnoreMonitorInventoryDisk(disk) {
			continue
		}
		for _, candidate := range monitorDiskDeviceCandidates(disk.Device) {
			devices[candidate] = struct{}{}
		}
	}
	if len(devices) == 0 {
		return nil
	}
	return devices
}

func filterMonitorDiskIOSeries(series []prometheusMatrixSeries, allowed map[string]struct{}) []prometheusMatrixSeries {
	if len(allowed) == 0 {
		return series
	}

	filtered := make([]prometheusMatrixSeries, 0, len(series))
	for _, item := range series {
		matched := false
		for _, candidate := range monitorDiskDeviceCandidates(item.Metric["device"]) {
			if _, ok := allowed[candidate]; ok {
				matched = true
				break
			}
		}
		if matched {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		return series
	}
	return filtered
}

func shouldIgnoreMonitorInventoryDisk(disk monitorInventoryDisk) bool {
	return shouldIgnoreTrendDiskMetric(map[string]string{
		"device":      disk.Device,
		"mount_point": disk.MountPoint,
	})
}

func monitorDiskDeviceCandidates(value string) []string {
	normalized := normalizeMonitorDiskDeviceName(value)
	if normalized == "" {
		return nil
	}

	items := []string{normalized}
	if idx := strings.LastIndexAny(normalized, `/\`); idx >= 0 && idx+1 < len(normalized) {
		base := normalized[idx+1:]
		if base != "" && base != normalized {
			items = append(items, base)
		}
	}
	return uniqueMonitorDiskDeviceNames(items)
}

func normalizeMonitorDiskDeviceName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	value = strings.TrimRight(value, `\/`)
	value = strings.TrimPrefix(value, "/dev/")
	value = strings.TrimPrefix(value, `\\.\`)
	return value
}

func uniqueMonitorDiskDeviceNames(items []string) []string {
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
