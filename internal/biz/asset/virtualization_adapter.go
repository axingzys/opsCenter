// Copyright (c) 2026 DYCloud J.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package asset

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type virtualizationProviderAdapter interface {
	Collect(ctx context.Context, platform *VirtualizationPlatform) (*virtualizationSyncSnapshot, error)
	PowerAction(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost, action string) error
	ListSnapshots(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost) ([]*virtualizationGuestSnapshotRecord, error)
	CreateSnapshot(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost, req *VirtualizationGuestSnapshotCreateRequest) error
	RollbackSnapshot(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost, snapshotName string) error
	DeleteSnapshot(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost, snapshotName string) error
	BuildConsoleLink(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost) (*virtualizationGuestConsoleLinkRecord, error)
}

type virtualizationSyncSnapshot struct {
	Clusters []*virtualizationClusterSnapshot
	Hosts    []*virtualizationHostSnapshot
	Guests   []*virtualizationGuestSnapshot
}

type virtualizationClusterSnapshot struct {
	ExternalID string
	Name       string
	Datacenter string
	Status     string
	HostCount  int
	GuestCount int
}

type virtualizationHostSnapshot struct {
	ExternalID        string
	ClusterExternalID string
	Name              string
	ManagementIP      string
	CPUModel          string
	CPUCores          int
	MemoryTotalMB     int64
	MemoryUsedMB      int64
	GuestCount        int
	Status            string
}

type virtualizationGuestSnapshot struct {
	ExternalID        string
	ClusterExternalID string
	HostExternalID    string
	Name              string
	OSType            string
	PowerState        string
	CPUCount          int
	MemoryMB          int64
	PrimaryIP         string
	ToolsStatus       string
}

type virtualizationGuestSnapshotRecord struct {
	Name          string
	Description   string
	CreatedAt     *time.Time
	ParentName    string
	Current       bool
	IncludeMemory bool
	Quiesced      bool
	PowerState    string
	Children      int
}

type virtualizationGuestConsoleLinkRecord struct {
	Mode          string
	URL           string
	RequiresLogin bool
	Message       string
	ExpiresAt     *time.Time
}

func newVirtualizationAdapter(provider string) (virtualizationProviderAdapter, error) {
	switch strings.TrimSpace(provider) {
	case VirtualizationProviderESXi:
		return &esxiAdapter{}, nil
	case VirtualizationProviderPVE:
		return &pveAdapter{}, nil
	default:
		return nil, fmt.Errorf("不支持的虚拟化平台类型: %s", provider)
	}
}

type esxiAdapter struct{}

func (a *esxiAdapter) Collect(ctx context.Context, platform *VirtualizationPlatform) (*virtualizationSyncSnapshot, error) {
	endpoint, err := normalizeVirtualizationEndpoint(platform.Endpoint, platform.Port)
	if err != nil {
		return nil, err
	}

	if endpoint.Path == "" || endpoint.Path == "/" {
		endpoint.Path = "/sdk"
	}
	endpoint.User = url.UserPassword(platform.Username, platform.Password)

	client, err := govmomi.NewClient(ctx, endpoint, platform.InsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("连接 VMware 失败: %w", err)
	}

	finder := find.NewFinder(client.Client, true)
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("获取 VMware 数据中心失败: %w", err)
	}
	if len(datacenters) == 0 {
		return nil, fmt.Errorf("未发现 VMware 数据中心")
	}

	snapshot := &virtualizationSyncSnapshot{
		Clusters: make([]*virtualizationClusterSnapshot, 0),
		Hosts:    make([]*virtualizationHostSnapshot, 0),
		Guests:   make([]*virtualizationGuestSnapshot, 0),
	}

	for _, dc := range datacenters {
		dcName, err := a.loadDatacenterName(ctx, dc)
		if err != nil {
			return nil, err
		}
		part, err := a.collectDatacenter(ctx, client, dc.Reference(), dcName)
		if err != nil {
			return nil, err
		}
		snapshot.Clusters = append(snapshot.Clusters, part.Clusters...)
		snapshot.Hosts = append(snapshot.Hosts, part.Hosts...)
		snapshot.Guests = append(snapshot.Guests, part.Guests...)
	}

	return snapshot, nil
}

func (a *esxiAdapter) PowerAction(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost, action string) error {
	client, err := a.newClient(ctx, platform)
	if err != nil {
		return err
	}

	vm, err := a.findVirtualMachine(ctx, client, guest)
	if err != nil {
		return err
	}

	switch action {
	case VirtualizationGuestPowerActionOn:
		task, taskErr := vm.PowerOn(ctx)
		if taskErr != nil {
			return fmt.Errorf("VMware 开机失败: %w", taskErr)
		}
		if waitErr := task.Wait(ctx); waitErr != nil {
			return fmt.Errorf("VMware 开机任务执行失败: %w", waitErr)
		}
		return nil
	case VirtualizationGuestPowerActionOff:
		task, taskErr := vm.PowerOff(ctx)
		if taskErr != nil {
			return fmt.Errorf("VMware 关机失败: %w", taskErr)
		}
		if waitErr := task.Wait(ctx); waitErr != nil {
			return fmt.Errorf("VMware 关机任务执行失败: %w", waitErr)
		}
		return nil
	case VirtualizationGuestPowerActionReboot:
		if rebootErr := vm.RebootGuest(ctx); rebootErr == nil {
			return nil
		}
		task, taskErr := vm.Reset(ctx)
		if taskErr != nil {
			return fmt.Errorf("VMware 重启失败: %w", taskErr)
		}
		if waitErr := task.Wait(ctx); waitErr != nil {
			return fmt.Errorf("VMware 重启任务执行失败: %w", waitErr)
		}
		return nil
	default:
		return fmt.Errorf("不支持的电源操作: %s", action)
	}
}

func (a *esxiAdapter) ListSnapshots(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost) ([]*virtualizationGuestSnapshotRecord, error) {
	client, err := a.newClient(ctx, platform)
	if err != nil {
		return nil, err
	}

	vm, err := a.findVirtualMachine(ctx, client, guest)
	if err != nil {
		return nil, err
	}

	var vmObj mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"snapshot"}, &vmObj); err != nil {
		return nil, fmt.Errorf("读取 VMware 快照失败: %w", err)
	}
	if vmObj.Snapshot == nil || len(vmObj.Snapshot.RootSnapshotList) == 0 {
		return []*virtualizationGuestSnapshotRecord{}, nil
	}

	currentRef := ""
	if vmObj.Snapshot.CurrentSnapshot != nil {
		currentRef = vmObj.Snapshot.CurrentSnapshot.Value
	}

	result := make([]*virtualizationGuestSnapshotRecord, 0)
	a.flattenVMwareSnapshots(vmObj.Snapshot.RootSnapshotList, "", currentRef, &result)
	return result, nil
}

func (a *esxiAdapter) CreateSnapshot(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost, req *VirtualizationGuestSnapshotCreateRequest) error {
	client, err := a.newClient(ctx, platform)
	if err != nil {
		return err
	}

	vm, err := a.findVirtualMachine(ctx, client, guest)
	if err != nil {
		return err
	}

	task, err := vm.CreateSnapshot(ctx, strings.TrimSpace(req.Name), strings.TrimSpace(req.Description), req.IncludeMemory, req.Quiesce)
	if err != nil {
		return fmt.Errorf("VMware 创建快照失败: %w", err)
	}
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("VMware 创建快照任务执行失败: %w", err)
	}
	return nil
}

func (a *esxiAdapter) RollbackSnapshot(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost, snapshotName string) error {
	client, err := a.newClient(ctx, platform)
	if err != nil {
		return err
	}

	vm, err := a.findVirtualMachine(ctx, client, guest)
	if err != nil {
		return err
	}

	task, err := vm.RevertToSnapshot(ctx, strings.TrimSpace(snapshotName), false)
	if err != nil {
		return fmt.Errorf("VMware 快照回滚失败: %w", err)
	}
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("VMware 快照回滚任务执行失败: %w", err)
	}
	return nil
}

func (a *esxiAdapter) DeleteSnapshot(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost, snapshotName string) error {
	client, err := a.newClient(ctx, platform)
	if err != nil {
		return err
	}

	vm, err := a.findVirtualMachine(ctx, client, guest)
	if err != nil {
		return err
	}

	task, err := vm.RemoveSnapshot(ctx, strings.TrimSpace(snapshotName), false, nil)
	if err != nil {
		return fmt.Errorf("VMware 删除快照失败: %w", err)
	}
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("VMware 删除快照任务执行失败: %w", err)
	}
	return nil
}

func (a *esxiAdapter) BuildConsoleLink(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost) (*virtualizationGuestConsoleLinkRecord, error) {
	client, err := a.newClient(ctx, platform)
	if err != nil {
		return nil, err
	}

	vm, err := a.findVirtualMachine(ctx, client, guest)
	if err != nil {
		return nil, err
	}

	manager := session.NewManager(client.Client)
	cloneTicket, err := manager.AcquireCloneTicket(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取 VMware 控制台会话失败: %w", err)
	}

	baseURL := client.Client.URL()
	hostName := baseURL.Hostname()
	if hostName == "" {
		return nil, fmt.Errorf("无法识别 VMware 控制台主机地址")
	}

	if client.Client.ServiceContent.Setting != nil {
		optionManager := object.NewOptionManager(client.Client, *client.Client.ServiceContent.Setting)
		if options, queryErr := optionManager.Query(ctx, "VirtualCenter.FQDN"); queryErr == nil && len(options) > 0 {
			optionValue := strings.TrimSpace(fmt.Sprint(options[0].GetOptionValue().Value))
			if optionValue != "" && optionValue != "<nil>" {
				hostName = optionValue
			}
		}
	}

	var certInfo object.HostCertificateInfo
	if err := certInfo.FromURL(baseURL, nil); err != nil {
		return nil, fmt.Errorf("读取 VMware 控制台证书指纹失败: %w", err)
	}

	consoleURL := *baseURL
	consoleURL.Path = "/ui/webconsole.html"
	consoleURL.RawQuery = url.Values{
		"vmId":          []string{vm.Reference().Value},
		"vmName":        []string{guest.Name},
		"serverGuid":    []string{client.Client.ServiceContent.About.InstanceUuid},
		"host":          []string{hostName},
		"sessionTicket": []string{cloneTicket},
		"thumbprint":    []string{certInfo.ThumbprintSHA1},
	}.Encode()

	return &virtualizationGuestConsoleLinkRecord{
		Mode:          "html5",
		URL:           consoleURL.String(),
		RequiresLogin: false,
		Message:       "已生成 VMware HTML5 控制台链接",
	}, nil
}

func (a *esxiAdapter) newClient(ctx context.Context, platform *VirtualizationPlatform) (*govmomi.Client, error) {
	endpoint, err := normalizeVirtualizationEndpoint(platform.Endpoint, platform.Port)
	if err != nil {
		return nil, err
	}
	if endpoint.Path == "" || endpoint.Path == "/" {
		endpoint.Path = "/sdk"
	}
	endpoint.User = url.UserPassword(platform.Username, platform.Password)

	client, err := govmomi.NewClient(ctx, endpoint, platform.InsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("连接 VMware 失败: %w", err)
	}
	return client, nil
}

func (a *esxiAdapter) flattenVMwareSnapshots(nodes []types.VirtualMachineSnapshotTree, parentName, currentRef string, result *[]*virtualizationGuestSnapshotRecord) {
	for _, node := range nodes {
		createdAt := node.CreateTime
		record := &virtualizationGuestSnapshotRecord{
			Name:          node.Name,
			Description:   strings.TrimSpace(node.Description),
			CreatedAt:     &createdAt,
			ParentName:    parentName,
			Current:       node.Snapshot.Value == currentRef,
			IncludeMemory: node.State == types.VirtualMachinePowerStatePoweredOn,
			Quiesced:      node.Quiesced,
			PowerState:    mapVMwarePowerState(node.State),
			Children:      len(node.ChildSnapshotList),
		}
		*result = append(*result, record)
		a.flattenVMwareSnapshots(node.ChildSnapshotList, node.Name, currentRef, result)
	}
}

func (a *esxiAdapter) findVirtualMachine(ctx context.Context, client *govmomi.Client, guest *VirtualizationGuest) (*object.VirtualMachine, error) {
	externalID := strings.TrimSpace(guest.ExternalID)
	if externalID == "" {
		return nil, fmt.Errorf("虚机缺少外部ID，无法执行电源操作")
	}

	searchIndex := object.NewSearchIndex(client.Client)
	if looksLikeUUID(externalID) {
		ref, err := searchIndex.FindByUuid(ctx, nil, externalID, true, nil)
		if err != nil {
			return nil, fmt.Errorf("按 UUID 查找 VMware 虚机失败: %w", err)
		}
		if ref != nil {
			if vm, ok := ref.(*object.VirtualMachine); ok {
				return vm, nil
			}
		}
	}

	if strings.HasPrefix(externalID, "vm-") {
		return object.NewVirtualMachine(client.Client, types.ManagedObjectReference{
			Type:  "VirtualMachine",
			Value: externalID,
		}), nil
	}

	return nil, fmt.Errorf("无法定位 VMware 虚机: %s", externalID)
}

func (a *esxiAdapter) loadDatacenterName(ctx context.Context, dc *object.Datacenter) (string, error) {
	var dcObj mo.Datacenter
	if err := dc.Properties(ctx, dc.Reference(), []string{"name"}, &dcObj); err != nil {
		return "", fmt.Errorf("读取 VMware 数据中心名称失败: %w", err)
	}
	return dcObj.Name, nil
}

func (a *esxiAdapter) collectDatacenter(ctx context.Context, client *govmomi.Client, container types.ManagedObjectReference, datacenterName string) (*virtualizationSyncSnapshot, error) {
	manager := view.NewManager(client.Client)
	cv, err := manager.CreateContainerView(ctx, container, []string{
		"ClusterComputeResource",
		"ComputeResource",
		"HostSystem",
		"VirtualMachine",
	}, true)
	if err != nil {
		return nil, fmt.Errorf("创建 VMware 视图失败: %w", err)
	}
	defer func() { _ = cv.Destroy(ctx) }()

	var (
		clusterObjs []mo.ClusterComputeResource
		computeObjs []mo.ComputeResource
		hostObjs    []mo.HostSystem
		vmObjs      []mo.VirtualMachine
	)

	if err := cv.Retrieve(ctx, []string{"ClusterComputeResource"}, []string{"name", "host"}, &clusterObjs); err != nil {
		return nil, fmt.Errorf("读取 VMware 集群失败: %w", err)
	}
	if err := cv.Retrieve(ctx, []string{"ComputeResource"}, []string{"name", "host"}, &computeObjs); err != nil {
		return nil, fmt.Errorf("读取 VMware 计算资源失败: %w", err)
	}
	if err := cv.Retrieve(ctx, []string{"HostSystem"}, []string{
		"name",
		"parent",
		"runtime.powerState",
		"summary.hardware.cpuModel",
		"summary.hardware.numCpuCores",
		"summary.hardware.memorySize",
		"summary.managementServerIp",
		"summary.quickStats.overallMemoryUsage",
		"vm",
	}, &hostObjs); err != nil {
		return nil, fmt.Errorf("读取 VMware 宿主机失败: %w", err)
	}
	if err := cv.Retrieve(ctx, []string{"VirtualMachine"}, []string{
		"name",
		"runtime.powerState",
		"runtime.host",
		"config.hardware.numCPU",
		"config.hardware.memoryMB",
		"config.guestFullName",
		"guest.ipAddress",
		"guest.toolsRunningStatus",
		"summary.config.uuid",
	}, &vmObjs); err != nil {
		return nil, fmt.Errorf("读取 VMware 虚机失败: %w", err)
	}

	snapshot := &virtualizationSyncSnapshot{
		Clusters: make([]*virtualizationClusterSnapshot, 0, len(clusterObjs)+len(computeObjs)),
		Hosts:    make([]*virtualizationHostSnapshot, 0, len(hostObjs)),
		Guests:   make([]*virtualizationGuestSnapshot, 0, len(vmObjs)),
	}

	clusterMap := make(map[string]*virtualizationClusterSnapshot)
	addCluster := func(externalID, name string) {
		if externalID == "" {
			return
		}
		if _, ok := clusterMap[externalID]; ok {
			return
		}
		item := &virtualizationClusterSnapshot{
			ExternalID: externalID,
			Name:       name,
			Datacenter: datacenterName,
			Status:     "normal",
		}
		clusterMap[externalID] = item
		snapshot.Clusters = append(snapshot.Clusters, item)
	}

	for _, cluster := range clusterObjs {
		addCluster(cluster.Reference().Value, cluster.Name)
	}
	for _, compute := range computeObjs {
		addCluster(compute.Reference().Value, compute.Name)
	}

	hostClusterMap := make(map[string]string)
	for _, host := range hostObjs {
		clusterExternalID := host.Parent.Value
		if clusterExternalID == "" {
			clusterExternalID = fmt.Sprintf("dc-%s-standalone", datacenterName)
			addCluster(clusterExternalID, "Standalone Hosts")
		}
		hostExternalID := host.Reference().Value
		hostClusterMap[hostExternalID] = clusterExternalID

		totalMB := int64(0)
		if host.Summary.Hardware.MemorySize > 0 {
			totalMB = host.Summary.Hardware.MemorySize / (1024 * 1024)
		}

		usedMB := int64(host.Summary.QuickStats.OverallMemoryUsage)
		item := &virtualizationHostSnapshot{
			ExternalID:        hostExternalID,
			ClusterExternalID: clusterExternalID,
			Name:              host.Name,
			ManagementIP:      host.Summary.ManagementServerIp,
			CPUModel:          host.Summary.Hardware.CpuModel,
			CPUCores:          int(host.Summary.Hardware.NumCpuCores),
			MemoryTotalMB:     totalMB,
			MemoryUsedMB:      usedMB,
			GuestCount:        len(host.Vm),
			Status:            mapVMwareHostState(host.Runtime.PowerState),
		}
		snapshot.Hosts = append(snapshot.Hosts, item)

		if cluster, ok := clusterMap[clusterExternalID]; ok {
			cluster.HostCount++
			cluster.GuestCount += len(host.Vm)
		}
	}

	for _, vm := range vmObjs {
		hostExternalID := ""
		if vm.Runtime.Host != nil {
			hostExternalID = vm.Runtime.Host.Value
		}
		clusterExternalID := hostClusterMap[hostExternalID]

		externalID := vm.Reference().Value
		if strings.TrimSpace(vm.Summary.Config.Uuid) != "" {
			externalID = strings.TrimSpace(vm.Summary.Config.Uuid)
		}

		item := &virtualizationGuestSnapshot{
			ExternalID:        externalID,
			ClusterExternalID: clusterExternalID,
			HostExternalID:    hostExternalID,
			Name:              vm.Name,
			OSType:            vm.Config.GuestFullName,
			PowerState:        mapVMwarePowerState(vm.Runtime.PowerState),
			CPUCount:          int(vm.Config.Hardware.NumCPU),
			MemoryMB:          int64(vm.Config.Hardware.MemoryMB),
			PrimaryIP:         vm.Guest.IpAddress,
			ToolsStatus:       string(vm.Guest.ToolsRunningStatus),
		}
		snapshot.Guests = append(snapshot.Guests, item)
	}

	return snapshot, nil
}

func mapVMwarePowerState(state types.VirtualMachinePowerState) string {
	switch state {
	case types.VirtualMachinePowerStatePoweredOn:
		return "powered_on"
	case types.VirtualMachinePowerStateSuspended:
		return "suspended"
	default:
		return "powered_off"
	}
}

func mapVMwareHostState(state types.HostSystemPowerState) string {
	switch state {
	case types.HostSystemPowerStatePoweredOn:
		return "online"
	case types.HostSystemPowerStateStandBy:
		return "maintenance"
	default:
		return "offline"
	}
}

type pveAdapter struct{}

type pveAuthSession struct {
	Ticket    string
	CSRFToken string
}

type pveTicketResponse struct {
	Data struct {
		Ticket              string `json:"ticket"`
		CSRFPreventionToken string `json:"CSRFPreventionToken"`
	} `json:"data"`
}

type pveResourcesResponse struct {
	Data []pveResource `json:"data"`
}

type pveGuestConfigResponse struct {
	Data pveGuestConfig `json:"data"`
}

type pveSnapshotListResponse struct {
	Data []pveSnapshotItem `json:"data"`
}

type pveGuestConfig struct {
	Name   string `json:"name"`
	OSType string `json:"ostype"`
	Tags   string `json:"tags"`
}

type pveSnapshotItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SnapTime    int64  `json:"snaptime"`
	VMState     int    `json:"vmstate"`
	Parent      string `json:"parent"`
	Running     int    `json:"running"`
}

type pveResource struct {
	Type     string  `json:"type"`
	ID       string  `json:"id"`
	Node     string  `json:"node"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	CPU      float64 `json:"cpu"`
	MaxCPU   int     `json:"maxcpu"`
	MaxMem   int64   `json:"maxmem"`
	Mem      int64   `json:"mem"`
	Template int     `json:"template"`
	VMID     int     `json:"vmid"`
	IP       string  `json:"ip"`
	OSType   string  `json:"ostype"`
	Tags     string  `json:"tags"`
}

type pveHTTPError struct {
	Stage      string
	StatusCode int
	Body       string
}

func (e *pveHTTPError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("%s: HTTP %d", e.Stage, e.StatusCode)
	}
	return fmt.Sprintf("%s: %s", e.Stage, body)
}

func (e *pveHTTPError) IsAuthFailure() bool {
	if e == nil {
		return false
	}
	if e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(e.Body))
	return strings.Contains(msg, "authentication failure") || strings.Contains(msg, "invalid credentials")
}

func (a *pveAdapter) Collect(ctx context.Context, platform *VirtualizationPlatform) (*virtualizationSyncSnapshot, error) {
	baseURL, err := normalizeVirtualizationEndpoint(platform.Endpoint, platform.Port)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: platform.InsecureSkipVerify,
			},
		},
	}

	auth, err := a.loginWithFallback(ctx, client, baseURL, platform.Username, platform.Password)
	if err != nil {
		return nil, err
	}

	resources, err := a.listResources(ctx, client, baseURL, auth)
	if err != nil {
		return nil, err
	}

	resources = a.enrichGuestResources(ctx, client, baseURL, auth, resources)

	return a.buildSnapshot(platform, resources), nil
}

func (a *pveAdapter) PowerAction(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost, action string) error {
	if guest == nil {
		return fmt.Errorf("虚机不存在")
	}
	nodeName, vmType, vmid, err := a.resolveGuestTarget(guest, host)
	if err != nil {
		return err
	}

	baseURL, err := normalizeVirtualizationEndpoint(platform.Endpoint, platform.Port)
	if err != nil {
		return err
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: platform.InsecureSkipVerify,
			},
		},
	}

	auth, err := a.loginWithFallback(ctx, client, baseURL, platform.Username, platform.Password)
	if err != nil {
		return err
	}

	actionPath, err := mapPVEPowerAction(vmType, action)
	if err != nil {
		return err
	}
	return a.executeAction(ctx, client, baseURL, auth, nodeName, vmType, vmid, actionPath)
}

func (a *pveAdapter) ListSnapshots(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost) ([]*virtualizationGuestSnapshotRecord, error) {
	nodeName, vmType, vmid, err := a.resolveGuestTarget(guest, host)
	if err != nil {
		return nil, err
	}

	baseURL, client, auth, err := a.newSession(ctx, platform)
	if err != nil {
		return nil, err
	}

	items, err := a.fetchSnapshots(ctx, client, baseURL, auth, nodeName, vmType, vmid)
	if err != nil {
		return nil, err
	}

	result := make([]*virtualizationGuestSnapshotRecord, 0, len(items))
	currentParent := ""
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "current" {
			currentParent = strings.TrimSpace(item.Parent)
			continue
		}
		var createdAt *time.Time
		if item.SnapTime > 0 {
			t := time.Unix(item.SnapTime, 0)
			createdAt = &t
		}
		result = append(result, &virtualizationGuestSnapshotRecord{
			Name:          strings.TrimSpace(item.Name),
			Description:   strings.TrimSpace(item.Description),
			CreatedAt:     createdAt,
			ParentName:    strings.TrimSpace(item.Parent),
			Current:       strings.TrimSpace(item.Name) == currentParent,
			IncludeMemory: item.VMState == 1,
			PowerState:    mapPVESnapshotPowerState(item),
		})
	}
	return result, nil
}

func (a *pveAdapter) CreateSnapshot(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost, req *VirtualizationGuestSnapshotCreateRequest) error {
	nodeName, vmType, vmid, err := a.resolveGuestTarget(guest, host)
	if err != nil {
		return err
	}

	baseURL, client, auth, err := a.newSession(ctx, platform)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("snapname", strings.TrimSpace(req.Name))
	if desc := strings.TrimSpace(req.Description); desc != "" {
		form.Set("description", desc)
	}
	if vmType == "qemu" && req.IncludeMemory {
		form.Set("vmstate", "1")
	}
	if vmType == "qemu" && req.Quiesce {
		form.Set("quiesce", "1")
	}

	return a.executeActionWithForm(ctx, client, baseURL, auth, http.MethodPost, a.snapshotPath(nodeName, vmType, vmid, ""), form)
}

func (a *pveAdapter) RollbackSnapshot(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost, snapshotName string) error {
	nodeName, vmType, vmid, err := a.resolveGuestTarget(guest, host)
	if err != nil {
		return err
	}

	baseURL, client, auth, err := a.newSession(ctx, platform)
	if err != nil {
		return err
	}
	return a.executeActionWithForm(ctx, client, baseURL, auth, http.MethodPost, a.snapshotPath(nodeName, vmType, vmid, url.PathEscape(strings.TrimSpace(snapshotName))+"/rollback"), nil)
}

func (a *pveAdapter) DeleteSnapshot(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost, snapshotName string) error {
	nodeName, vmType, vmid, err := a.resolveGuestTarget(guest, host)
	if err != nil {
		return err
	}

	baseURL, client, auth, err := a.newSession(ctx, platform)
	if err != nil {
		return err
	}
	return a.executeActionWithForm(ctx, client, baseURL, auth, http.MethodDelete, a.snapshotPath(nodeName, vmType, vmid, url.PathEscape(strings.TrimSpace(snapshotName))), nil)
}

func (a *pveAdapter) BuildConsoleLink(ctx context.Context, platform *VirtualizationPlatform, guest *VirtualizationGuest, host *VirtualizationHost) (*virtualizationGuestConsoleLinkRecord, error) {
	nodeName, vmType, vmid, err := a.resolveGuestTarget(guest, host)
	if err != nil {
		return nil, err
	}

	baseURL, client, auth, err := a.newSession(ctx, platform)
	if err != nil {
		return nil, err
	}
	if err := a.ensureGuestExists(ctx, client, baseURL, auth, nodeName, vmType, vmid); err != nil {
		return nil, err
	}

	consoleURL := *baseURL
	consoleURL.Path = "/"

	params := url.Values{
		"node":   []string{nodeName},
		"vmid":   []string{strconv.Itoa(vmid)},
		"vmname": []string{guest.Name},
	}
	switch vmType {
	case "qemu":
		params.Set("console", "kvm")
		params.Set("novnc", "1")
		params.Set("resize", "off")
	case "lxc":
		params.Set("console", "lxc")
		params.Set("xtermjs", "1")
	default:
		return nil, fmt.Errorf("不支持的 PVE 虚机类型: %s", vmType)
	}
	consoleURL.RawQuery = params.Encode()

	message := "PVE 控制台依赖平台站点登录态；若浏览器未登录 PVE，会先进入 PVE 登录页，登录后再进入目标控制台。"
	return &virtualizationGuestConsoleLinkRecord{
		Mode:          "platform",
		URL:           consoleURL.String(),
		RequiresLogin: true,
		Message:       message,
	}, nil
}

func (a *pveAdapter) newSession(ctx context.Context, platform *VirtualizationPlatform) (*url.URL, *http.Client, *pveAuthSession, error) {
	baseURL, err := normalizeVirtualizationEndpoint(platform.Endpoint, platform.Port)
	if err != nil {
		return nil, nil, nil, err
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: platform.InsecureSkipVerify,
			},
		},
	}
	auth, err := a.loginWithFallback(ctx, client, baseURL, platform.Username, platform.Password)
	if err != nil {
		return nil, nil, nil, err
	}
	return baseURL, client, auth, nil
}

func (a *pveAdapter) fetchSnapshots(ctx context.Context, client *http.Client, baseURL *url.URL, auth *pveAuthSession, nodeName, vmType string, vmid int) ([]pveSnapshotItem, error) {
	endpoint := *baseURL
	endpoint.Path = a.snapshotPath(nodeName, vmType, vmid, "")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.AddCookie(&http.Cookie{Name: "PVEAuthCookie", Value: auth.Ticket, Path: "/"})

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取 PVE 快照列表失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, &pveHTTPError{
			Stage:      "获取 PVE 快照列表失败",
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
		}
	}

	var payload pveSnapshotListResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析 PVE 快照列表响应失败: %w", err)
	}
	return payload.Data, nil
}

func (a *pveAdapter) ensureGuestExists(ctx context.Context, client *http.Client, baseURL *url.URL, auth *pveAuthSession, nodeName, vmType string, vmid int) error {
	endpoint := *baseURL
	endpoint.Path = fmt.Sprintf("/api2/json/nodes/%s/%s/%d/config", url.PathEscape(nodeName), url.PathEscape(vmType), vmid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.AddCookie(&http.Cookie{Name: "PVEAuthCookie", Value: auth.Ticket, Path: "/"})

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("校验 PVE 控制台目标失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return &pveHTTPError{
			Stage:      "校验 PVE 控制台目标失败",
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
		}
	}
	return nil
}

func (a *pveAdapter) loginWithFallback(ctx context.Context, client *http.Client, baseURL *url.URL, username, password string) (*pveAuthSession, error) {
	candidates := pveUsernameCandidates(username)
	var (
		auth        *pveAuthSession
		err         error
		lastAuthErr error
	)
	for _, candidate := range candidates {
		auth, err = a.login(ctx, client, baseURL, candidate, password)
		if err == nil {
			return auth, nil
		}
		if isPVEAuthFailure(err) {
			lastAuthErr = err
			continue
		}
		return nil, err
	}
	if lastAuthErr == nil {
		lastAuthErr = fmt.Errorf("PVE 认证失败")
	}
	if len(candidates) > 1 {
		return nil, fmt.Errorf("%w（已尝试账号: %s）", lastAuthErr, strings.Join(candidates, ", "))
	}
	return nil, lastAuthErr
}

func (a *pveAdapter) login(ctx context.Context, client *http.Client, baseURL *url.URL, username, password string) (*pveAuthSession, error) {
	endpoint := *baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/api2/json/access/ticket"

	form := url.Values{}
	form.Set("username", username)
	form.Set("password", password)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 PVE 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, &pveHTTPError{
			Stage:      "PVE 认证失败",
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
		}
	}

	var payload pveTicketResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析 PVE 认证响应失败: %w", err)
	}
	if strings.TrimSpace(payload.Data.Ticket) == "" {
		return nil, fmt.Errorf("PVE 认证失败: ticket 为空")
	}
	return &pveAuthSession{
		Ticket:    payload.Data.Ticket,
		CSRFToken: payload.Data.CSRFPreventionToken,
	}, nil
}

func (a *pveAdapter) listResources(ctx context.Context, client *http.Client, baseURL *url.URL, auth *pveAuthSession) ([]pveResource, error) {
	endpoint := *baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/api2/json/cluster/resources"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.AddCookie(&http.Cookie{
		Name:  "PVEAuthCookie",
		Value: auth.Ticket,
		Path:  "/",
	})

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取 PVE 资源失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, &pveHTTPError{
			Stage:      "获取 PVE 资源失败",
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
		}
	}

	var payload pveResourcesResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析 PVE 资源响应失败: %w", err)
	}
	return payload.Data, nil
}

func (a *pveAdapter) enrichGuestResources(ctx context.Context, client *http.Client, baseURL *url.URL, auth *pveAuthSession, resources []pveResource) []pveResource {
	if len(resources) == 0 {
		return resources
	}

	enriched := make([]pveResource, len(resources))
	copy(enriched, resources)

	for i := range enriched {
		resource := &enriched[i]
		if (resource.Type != "qemu" && resource.Type != "lxc") || resource.Template == 1 || resource.VMID == 0 || strings.TrimSpace(resource.Node) == "" {
			continue
		}
		if strings.TrimSpace(resource.OSType) != "" && strings.TrimSpace(resource.Tags) != "" {
			continue
		}

		cfg, err := a.getGuestConfig(ctx, client, baseURL, auth, *resource)
		if err != nil {
			continue
		}
		resource.Name = firstNonEmpty([]string{resource.Name, cfg.Name})
		resource.OSType = firstNonEmpty([]string{resource.OSType, cfg.OSType})
		resource.Tags = firstNonEmpty([]string{resource.Tags, cfg.Tags})
	}

	return enriched
}

func (a *pveAdapter) getGuestConfig(ctx context.Context, client *http.Client, baseURL *url.URL, auth *pveAuthSession, resource pveResource) (*pveGuestConfig, error) {
	endpoint := *baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") +
		"/api2/json/nodes/" + url.PathEscape(resource.Node) +
		"/" + url.PathEscape(resource.Type) +
		"/" + strconv.Itoa(resource.VMID) + "/config"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.AddCookie(&http.Cookie{
		Name:  "PVEAuthCookie",
		Value: auth.Ticket,
		Path:  "/",
	})

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, &pveHTTPError{
			Stage:      "读取 PVE 虚机配置失败",
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
		}
	}

	var payload pveGuestConfigResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析 PVE 虚机配置响应失败: %w", err)
	}
	return &payload.Data, nil
}

func (a *pveAdapter) executeAction(ctx context.Context, client *http.Client, baseURL *url.URL, auth *pveAuthSession, nodeName, vmType string, vmid int, actionPath string) error {
	endpoint := *baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") +
		"/api2/json/nodes/" + url.PathEscape(nodeName) +
		"/" + url.PathEscape(vmType) +
		"/" + strconv.Itoa(vmid) + "/status/" + actionPath

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.AddCookie(&http.Cookie{
		Name:  "PVEAuthCookie",
		Value: auth.Ticket,
		Path:  "/",
	})
	if strings.TrimSpace(auth.CSRFToken) != "" {
		req.Header.Set("CSRFPreventionToken", auth.CSRFToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("执行 PVE 电源操作失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return &pveHTTPError{
			Stage:      "PVE 电源操作失败",
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
		}
	}
	return nil
}

func (a *pveAdapter) executeActionWithForm(ctx context.Context, client *http.Client, baseURL *url.URL, auth *pveAuthSession, method, path string, form url.Values) error {
	endpoint := *baseURL
	endpoint.Path = path

	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return err
	}
	req.AddCookie(&http.Cookie{
		Name:  "PVEAuthCookie",
		Value: auth.Ticket,
		Path:  "/",
	})
	if strings.TrimSpace(auth.CSRFToken) != "" {
		req.Header.Set("CSRFPreventionToken", auth.CSRFToken)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("执行 PVE 快照操作失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return &pveHTTPError{
			Stage:      "PVE 快照操作失败",
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(respBody)),
		}
	}
	return nil
}

func (a *pveAdapter) snapshotPath(nodeName, vmType string, vmid int, suffix string) string {
	base := strings.TrimRight("/api2/json/nodes/"+url.PathEscape(nodeName)+"/"+url.PathEscape(vmType)+"/"+strconv.Itoa(vmid)+"/snapshot", "/")
	if strings.TrimSpace(suffix) == "" {
		return base
	}
	return base + "/" + strings.TrimLeft(suffix, "/")
}

func (a *pveAdapter) resolveGuestTarget(guest *VirtualizationGuest, host *VirtualizationHost) (string, string, int, error) {
	externalID := strings.TrimSpace(guest.ExternalID)
	if externalID == "" {
		return "", "", 0, fmt.Errorf("虚机缺少外部ID，无法执行电源操作")
	}

	parts := strings.Split(externalID, "/")
	if len(parts) != 2 {
		return "", "", 0, fmt.Errorf("无法识别 PVE 虚机标识: %s", externalID)
	}
	vmType := strings.TrimSpace(parts[0])
	if vmType != "qemu" && vmType != "lxc" {
		return "", "", 0, fmt.Errorf("不支持的 PVE 虚机类型: %s", vmType)
	}
	vmid, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || vmid <= 0 {
		return "", "", 0, fmt.Errorf("无效的 PVE VMID: %s", parts[1])
	}

	nodeName := ""
	if host != nil {
		externalHostID := strings.TrimSpace(host.ExternalID)
		if strings.HasPrefix(externalHostID, "node/") {
			nodeName = strings.TrimPrefix(externalHostID, "node/")
		}
		if nodeName == "" {
			nodeName = strings.TrimSpace(host.Name)
		}
	}
	if nodeName == "" {
		return "", "", 0, fmt.Errorf("缺少宿主节点信息，请先同步平台后再执行电源操作")
	}
	return nodeName, vmType, vmid, nil
}

func mapPVEPowerAction(vmType, action string) (string, error) {
	switch action {
	case VirtualizationGuestPowerActionOn:
		return "start", nil
	case VirtualizationGuestPowerActionOff:
		return "shutdown", nil
	case VirtualizationGuestPowerActionReboot:
		return "reboot", nil
	default:
		return "", fmt.Errorf("不支持的电源操作: %s", action)
	}
}

func mapPVESnapshotPowerState(item pveSnapshotItem) string {
	if item.Running == 1 {
		return "powered_on"
	}
	if item.VMState == 1 {
		return "powered_on"
	}
	return "powered_off"
}

func (a *pveAdapter) buildSnapshot(platform *VirtualizationPlatform, resources []pveResource) *virtualizationSyncSnapshot {
	clusterExternalID := "pve-cluster"
	clusterName := platform.Name
	for _, resource := range resources {
		if resource.Type == "cluster" {
			if name := strings.TrimSpace(resource.Name); name != "" {
				clusterName = name
			}
			if id := strings.TrimSpace(resource.ID); id != "" {
				clusterExternalID = id
			}
			break
		}
	}

	snapshot := &virtualizationSyncSnapshot{
		Clusters: []*virtualizationClusterSnapshot{
			{
				ExternalID: clusterExternalID,
				Name:       clusterName,
				Datacenter: "pve",
				Status:     "normal",
			},
		},
		Hosts:  make([]*virtualizationHostSnapshot, 0),
		Guests: make([]*virtualizationGuestSnapshot, 0),
	}

	hostGuestCount := make(map[string]int)
	for _, resource := range resources {
		if (resource.Type == "qemu" || resource.Type == "lxc") && resource.Template == 0 {
			hostExternalID := buildPVEHostExternalID(resource.Node)
			hostGuestCount[hostExternalID]++
		}
	}

	for _, resource := range resources {
		switch resource.Type {
		case "node":
			hostExternalID := strings.TrimSpace(resource.ID)
			if hostExternalID == "" {
				hostExternalID = buildPVEHostExternalID(resource.Node)
			}

			item := &virtualizationHostSnapshot{
				ExternalID:        hostExternalID,
				ClusterExternalID: clusterExternalID,
				Name:              firstNonEmpty([]string{resource.Node, resource.Name, hostExternalID}),
				ManagementIP:      "",
				CPUModel:          "",
				CPUCores:          resource.MaxCPU,
				MemoryTotalMB:     bytesToMB(resource.MaxMem),
				MemoryUsedMB:      bytesToMB(resource.Mem),
				GuestCount:        hostGuestCount[hostExternalID],
				Status:            mapPVEHostStatus(resource.Status),
			}
			snapshot.Hosts = append(snapshot.Hosts, item)
		case "qemu", "lxc":
			if resource.Template == 1 {
				continue
			}

			hostExternalID := buildPVEHostExternalID(resource.Node)
			externalID := strings.TrimSpace(resource.ID)
			if externalID == "" && resource.VMID > 0 {
				externalID = resource.Type + "/" + strconv.Itoa(resource.VMID)
			}
			if externalID == "" {
				continue
			}

			cpuCount := resource.MaxCPU
			if cpuCount == 0 && resource.CPU > 0 {
				cpuCount = int(math.Ceil(resource.CPU))
			}

			item := &virtualizationGuestSnapshot{
				ExternalID:        externalID,
				ClusterExternalID: clusterExternalID,
				HostExternalID:    hostExternalID,
				Name:              firstNonEmpty([]string{resource.Name, externalID}),
				OSType:            mapPVEOsType(resource.OSType),
				PowerState:        mapPVEPowerStatus(resource.Status),
				CPUCount:          cpuCount,
				MemoryMB:          bytesToMB(resource.MaxMem),
				PrimaryIP:         firstValidIPAddress(resource.IP, extractPrimaryIPFromPVETags(resource.Tags)),
				ToolsStatus:       "",
			}
			snapshot.Guests = append(snapshot.Guests, item)
		}
	}

	if len(snapshot.Clusters) > 0 {
		snapshot.Clusters[0].HostCount = len(snapshot.Hosts)
		snapshot.Clusters[0].GuestCount = len(snapshot.Guests)
	}

	return snapshot
}

func buildPVEHostExternalID(node string) string {
	node = strings.TrimSpace(node)
	if node == "" {
		return "node/unknown"
	}
	return "node/" + node
}

func mapPVEPowerStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return "powered_on"
	case "paused":
		return "suspended"
	default:
		return "powered_off"
	}
}

func mapPVEHostStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online":
		return "online"
	case "maintenance":
		return "maintenance"
	default:
		return "offline"
	}
}

func mapPVEOsType(osType string) string {
	switch strings.ToLower(strings.TrimSpace(osType)) {
	case "":
		return ""
	case "l24":
		return "Linux 2.4"
	case "l26":
		return "Linux"
	case "other":
		return "Other"
	case "solaris":
		return "Solaris"
	case "wxp":
		return "Windows XP"
	case "w2k":
		return "Windows 2000"
	case "w2k3":
		return "Windows 2003"
	case "w2k8":
		return "Windows 2008"
	case "wvista":
		return "Windows Vista"
	case "win7":
		return "Windows 7"
	case "win8":
		return "Windows 8/2012/2012r2"
	case "win10":
		return "Windows 10/2016/2019"
	case "win11":
		return "Windows 11/2022/2025"
	default:
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(osType)), "win") {
			return "Windows"
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(osType)), "l") {
			return "Linux"
		}
		return strings.TrimSpace(osType)
	}
}

func extractPrimaryIPFromPVETags(tags string) string {
	normalized := strings.NewReplacer(";", ",", " ", ",", "\n", ",", "\t", ",").Replace(tags)
	for _, raw := range strings.Split(normalized, ",") {
		token := strings.TrimSpace(raw)
		token = strings.TrimPrefix(token, "ip:")
		token = strings.TrimPrefix(token, "ip=")
		token = strings.TrimSpace(token)
		if net.ParseIP(token) != nil {
			return token
		}
	}
	return ""
}

func firstValidIPAddress(candidates ...string) string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" && net.ParseIP(candidate) != nil {
			return candidate
		}
	}
	return ""
}

func pveUsernameCandidates(username string) []string {
	base := strings.TrimSpace(username)
	if base == "" {
		return []string{""}
	}
	if strings.Contains(base, "@") {
		return []string{base}
	}

	seen := make(map[string]struct{}, 3)
	result := make([]string, 0, 3)
	appendCandidate := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		result = append(result, candidate)
	}

	appendCandidate(base + "@pam")
	appendCandidate(base + "@pve")
	appendCandidate(base)
	return result
}

func isPVEAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *pveHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.IsAuthFailure()
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "authentication failure") || strings.Contains(msg, "invalid credentials") || strings.Contains(msg, "ticket 为空")
}

func looksLikeUUID(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) == 36 && strings.Count(value, "-") == 4
}

func bytesToMB(n int64) int64 {
	if n <= 0 {
		return 0
	}
	return n / (1024 * 1024)
}

func normalizeVirtualizationEndpoint(endpoint string, port int) (*url.URL, error) {
	raw := strings.TrimSpace(endpoint)
	if raw == "" {
		return nil, fmt.Errorf("平台地址不能为空")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("平台地址格式不正确")
	}
	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("平台地址格式不正确")
	}
	if port <= 0 {
		port = 443
	}
	if parsed.Port() == "" {
		parsed.Host = parsed.Hostname() + ":" + strconv.Itoa(port)
	}
	return parsed, nil
}
