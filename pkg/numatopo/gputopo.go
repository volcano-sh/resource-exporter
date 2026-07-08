/*
Copyright 2026 The Volcano Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package numatopo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"k8s.io/klog"

	"volcano.sh/apis/pkg/apis/nodeinfo/v1alpha1"

	"volcano.sh/resource-exporter/pkg/args"
)

// pciDevicesPath is a var so tests can override it with a fake sysfs tree.
var pciDevicesPath = "/sys/bus/pci/devices"

const (
	gpuPCIClassVGA  = "0x030000" // VGA-compatible controller
	gpuPCIClass3D   = "0x030200" // 3D controller (e.g. A100, H100)
	nvidiaPCIVendor = "0x10de"
)

type checkpointData struct {
	Data checkpointDataInner `json:"Data"`
}

type checkpointDataInner struct {
	PodDeviceEntries  []podDeviceEntry  `json:"PodDeviceEntries"`
	RegisteredDevices map[string][]string `json:"RegisteredDevices"`
}

type podDeviceEntry struct {
	ResourceName string   `json:"ResourceName"`
	DeviceIDs    []string `json:"DeviceIDs"`
}

const nvidiaGPUResourceName = "nvidia.com/gpu"

// GPUNumaInfo is the object to maintain the gpu information
type GPUNumaInfo struct {
	gpuDevices    map[int]v1alpha1.GPUInfo
	allocatedGPUs map[int]bool
}

// NewGPUNumaInfo init GPUNumaInfo struct object
func NewGPUNumaInfo() *GPUNumaInfo {
	return &GPUNumaInfo{
		gpuDevices:    make(map[int]v1alpha1.GPUInfo),
		allocatedGPUs: make(map[int]bool),
	}
}

// Name return function name
func (info *GPUNumaInfo) Name() string {
	return "gpu"
}

// Update returns the latest gpu numa info
// if data is changed , return the latest , otherwise nil
func (info *GPUNumaInfo) Update(opt *args.Argument) NumaInfo {
	newInfo := NewGPUNumaInfo()
	newInfo.gpuDevices = discoverGPUs()
	newInfo.allocatedGPUs = getAllocatedGPUs(opt.DevicePluginCheckpoint, len(newInfo.gpuDevices))

	if !reflect.DeepEqual(newInfo.gpuDevices, info.gpuDevices) ||
		!reflect.DeepEqual(newInfo.allocatedGPUs, info.allocatedGPUs) {
		return newInfo
	}

	return nil
}

// GetResourceInfoMap return the gpu topology info
func (info *GPUNumaInfo) GetResourceInfoMap() v1alpha1.ResourceInfo {
	count := len(info.gpuDevices)
	indices := make([]int, 0, count)
	for idx := range info.gpuDevices {
		if !info.allocatedGPUs[idx] {
			indices = append(indices, idx)
		}
	}
	sort.Ints(indices)

	parts := make([]string, 0, len(indices))
	for _, idx := range indices {
		parts = append(parts, strconv.Itoa(idx))
	}

	return v1alpha1.ResourceInfo{
		Allocatable: strings.Join(parts, ","),
		Capacity:    count,
	}
}

// GetResTopoDetail return the gpu capability topology info
func (info *GPUNumaInfo) GetResTopoDetail() interface{} {
	gpuDetail := make(map[string]v1alpha1.GPUInfo, len(info.gpuDevices))
	for idx, gpuInfo := range info.gpuDevices {
		gpuDetail[strconv.Itoa(idx)] = gpuInfo
	}
	return gpuDetail
}

func discoverGPUs() map[int]v1alpha1.GPUInfo {
	gpuDevices := make(map[int]v1alpha1.GPUInfo)

	entries, err := ioutil.ReadDir(pciDevicesPath)
	if err != nil {
		klog.Errorf("Failed to read PCI devices directory %s: %v", pciDevicesPath, err)
		return gpuDevices
	}

	// Sort by BDF for stable GPU indexing across reboots.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	gpuIndex := 0
	for _, entry := range entries {
		bdf := entry.Name()
		devicePath := filepath.Join(pciDevicesPath, bdf)

		if !isNvidiaGPU(devicePath) {
			continue
		}

		numaNode := readNUMANode(devicePath)
		deviceModel := readDeviceModel(devicePath)

		gpuDevices[gpuIndex] = v1alpha1.GPUInfo{
			NUMANodeID:  numaNode,
			BusID:       bdf,
			DeviceModel: deviceModel,
		}

		klog.V(4).Infof("Discovered GPU %d: BDF=%s NUMA=%d Model=%s", gpuIndex, bdf, numaNode, deviceModel)
		gpuIndex++
	}

	klog.Infof("Discovered %d NVIDIA GPU(s)", len(gpuDevices))
	return gpuDevices
}

func isNvidiaGPU(devicePath string) bool {
	vendor := readSysfsString(filepath.Join(devicePath, "vendor"))
	if vendor != nvidiaPCIVendor {
		return false
	}

	class := readSysfsString(filepath.Join(devicePath, "class"))
	return class == gpuPCIClassVGA || class == gpuPCIClass3D
}

func readNUMANode(devicePath string) int {
	numaPath := filepath.Join(devicePath, "numa_node")
	data, err := ioutil.ReadFile(numaPath)
	if err != nil {
		klog.V(4).Infof("Failed to read numa_node for %s, defaulting to 0: %v", devicePath, err)
		return 0
	}

	val, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		klog.V(4).Infof("Failed to parse numa_node for %s, defaulting to 0: %v", devicePath, err)
		return 0
	}

	if val < 0 {
		return 0
	}

	return val
}

func readDeviceModel(devicePath string) string {
	label := readSysfsString(filepath.Join(devicePath, "label"))
	if label != "" {
		return label
	}

	vendor := readSysfsString(filepath.Join(devicePath, "vendor"))
	device := readSysfsString(filepath.Join(devicePath, "device"))
	if vendor != "" && device != "" {
		return fmt.Sprintf("%s:%s", vendor, device)
	}

	return "unknown"
}

func readSysfsString(path string) string {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func getAllocatedGPUs(checkpointPath string, totalGPUs int) map[int]bool {
	allocated := make(map[int]bool)
	if checkpointPath == "" {
		return allocated
	}

	data, err := ioutil.ReadFile(checkpointPath)
	if err != nil {
		klog.V(4).Infof("Failed to read device plugin checkpoint %s: %v", checkpointPath, err)
		return allocated
	}

	var cp checkpointData
	if err := json.Unmarshal(data, &cp); err != nil {
		klog.Errorf("Failed to parse device plugin checkpoint: %v", err)
		return allocated
	}

	registered := cp.Data.RegisteredDevices[nvidiaGPUResourceName]
	if len(registered) == 0 {
		return allocated
	}

	devToIndex := make(map[string]int, len(registered))
	for i, devID := range registered {
		if i < totalGPUs {
			devToIndex[devID] = i
		}
	}

	for _, entry := range cp.Data.PodDeviceEntries {
		if entry.ResourceName != nvidiaGPUResourceName {
			continue
		}
		for _, devID := range entry.DeviceIDs {
			if idx, ok := devToIndex[devID]; ok {
				allocated[idx] = true
			}
		}
	}

	klog.V(4).Infof("GPU allocation: %d total, %d allocated, %d free",
		totalGPUs, len(allocated), totalGPUs-len(allocated))

	return allocated
}
