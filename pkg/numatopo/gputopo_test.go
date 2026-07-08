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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"volcano.sh/apis/pkg/apis/nodeinfo/v1alpha1"
)

func TestReadNUMANode(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{"valid numa 0", "0\n", 0},
		{"valid numa 1", "1\n", 1},
		{"negative numa (single socket)", "-1\n", 0},
		{"valid numa 3", "3\n", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := ioutil.TempDir("", "gputopo-test")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			numaFile := filepath.Join(tmpDir, "numa_node")
			if err := ioutil.WriteFile(numaFile, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got := readNUMANode(tmpDir)
			if got != tt.expected {
				t.Errorf("readNUMANode() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestReadNUMANodeMissingFile(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "gputopo-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	got := readNUMANode(tmpDir)
	if got != 0 {
		t.Errorf("readNUMANode(missing file) = %d, want 0", got)
	}
}

func TestIsNvidiaGPU(t *testing.T) {
	tests := []struct {
		name     string
		vendor   string
		class    string
		expected bool
	}{
		{"nvidia 3d controller", nvidiaPCIVendor, gpuPCIClass3D, true},
		{"nvidia vga", nvidiaPCIVendor, gpuPCIClassVGA, true},
		{"nvidia non-gpu", nvidiaPCIVendor, "0x068000", false},
		{"amd gpu", "0x1002", gpuPCIClass3D, false},
		{"intel non-gpu", "0x8086", "0x060000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := ioutil.TempDir("", "gputopo-test")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			if err := ioutil.WriteFile(filepath.Join(tmpDir, "vendor"), []byte(tt.vendor), 0644); err != nil {
				t.Fatal(err)
			}
			if err := ioutil.WriteFile(filepath.Join(tmpDir, "class"), []byte(tt.class), 0644); err != nil {
				t.Fatal(err)
			}

			got := isNvidiaGPU(tmpDir)
			if got != tt.expected {
				t.Errorf("isNvidiaGPU() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReadDeviceModel(t *testing.T) {
	// Test with label file present
	t.Run("with label", func(t *testing.T) {
		tmpDir, err := ioutil.TempDir("", "gputopo-test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		if err := ioutil.WriteFile(filepath.Join(tmpDir, "label"), []byte("NVIDIA A100-SXM4-80GB"), 0644); err != nil {
			t.Fatal(err)
		}

		got := readDeviceModel(tmpDir)
		if got != "NVIDIA A100-SXM4-80GB" {
			t.Errorf("readDeviceModel() = %q, want %q", got, "NVIDIA A100-SXM4-80GB")
		}
	})

	// Test fallback to vendor:device
	t.Run("without label", func(t *testing.T) {
		tmpDir, err := ioutil.TempDir("", "gputopo-test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		if err := ioutil.WriteFile(filepath.Join(tmpDir, "vendor"), []byte("0x10de"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := ioutil.WriteFile(filepath.Join(tmpDir, "device"), []byte("0x20b5"), 0644); err != nil {
			t.Fatal(err)
		}

		got := readDeviceModel(tmpDir)
		if got != "0x10de:0x20b5" {
			t.Errorf("readDeviceModel() = %q, want %q", got, "0x10de:0x20b5")
		}
	})
}

func TestGPUNumaInfoInterface(t *testing.T) {
	info := NewGPUNumaInfo()

	if info.Name() != "gpu" {
		t.Errorf("Name() = %q, want %q", info.Name(), "gpu")
	}

	// Empty GPU list should return empty resource info
	resInfo := info.GetResourceInfoMap()
	if resInfo.Capacity != 0 {
		t.Errorf("GetResourceInfoMap().Capacity = %d, want 0", resInfo.Capacity)
	}

	// Empty GPU list should return empty topo detail
	detail := info.GetResTopoDetail()
	gpuDetail, ok := detail.(map[string]v1alpha1.GPUInfo)
	if !ok {
		t.Fatal("GetResTopoDetail() did not return map[string]v1alpha1.GPUInfo")
	}
	if len(gpuDetail) != 0 {
		t.Errorf("GetResTopoDetail() returned %d entries, want 0", len(gpuDetail))
	}
}

func TestGPUNumaInfoWithDevices(t *testing.T) {
	info := &GPUNumaInfo{
		gpuDevices: map[int]v1alpha1.GPUInfo{
			0: {NUMANodeID: 0, BusID: "0000:3b:00.0", DeviceModel: "A100"},
			1: {NUMANodeID: 0, BusID: "0000:86:00.0", DeviceModel: "A100"},
			2: {NUMANodeID: 1, BusID: "0000:af:00.0", DeviceModel: "A100"},
			3: {NUMANodeID: 1, BusID: "0000:d8:00.0", DeviceModel: "A100"},
		},
	}

	resInfo := info.GetResourceInfoMap()
	if resInfo.Capacity != 4 {
		t.Errorf("Capacity = %d, want 4", resInfo.Capacity)
	}

	detail := info.GetResTopoDetail()
	gpuDetail, ok := detail.(map[string]v1alpha1.GPUInfo)
	if !ok {
		t.Fatal("GetResTopoDetail() type assertion failed")
	}
	if len(gpuDetail) != 4 {
		t.Errorf("GPUDetail has %d entries, want 4", len(gpuDetail))
	}

	gpu0, exists := gpuDetail["0"]
	if !exists {
		t.Fatal("GPU 0 not found in detail")
	}
	if gpu0.NUMANodeID != 0 {
		t.Errorf("GPU 0 NUMANodeID = %d, want 0", gpu0.NUMANodeID)
	}
	if gpu0.BusID != "0000:3b:00.0" {
		t.Errorf("GPU 0 BusID = %q, want %q", gpu0.BusID, "0000:3b:00.0")
	}

	gpu2, exists := gpuDetail["2"]
	if !exists {
		t.Fatal("GPU 2 not found in detail")
	}
	if gpu2.NUMANodeID != 1 {
		t.Errorf("GPU 2 NUMANodeID = %d, want 1", gpu2.NUMANodeID)
	}
}

func createFakePCIDevice(t *testing.T, baseDir, bdf, vendor, class, numaNode string) {
	t.Helper()
	devDir := filepath.Join(baseDir, bdf)
	if err := os.MkdirAll(devDir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"vendor":    vendor,
		"class":     class,
		"numa_node": numaNode,
	} {
		if err := ioutil.WriteFile(filepath.Join(devDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiscoverGPUs_FakeSysfs(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "fake-pci")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Save and restore the real path
	origPath := pciDevicesPath
	pciDevicesPath = tmpDir
	defer func() { pciDevicesPath = origPath }()

	// 4 NVIDIA GPUs across 2 NUMA nodes, plus some non-GPU devices
	createFakePCIDevice(t, tmpDir, "0000:3b:00.0", nvidiaPCIVendor, gpuPCIClass3D, "0")
	createFakePCIDevice(t, tmpDir, "0000:86:00.0", nvidiaPCIVendor, gpuPCIClass3D, "0")
	createFakePCIDevice(t, tmpDir, "0000:af:00.0", nvidiaPCIVendor, gpuPCIClass3D, "1")
	createFakePCIDevice(t, tmpDir, "0000:d8:00.0", nvidiaPCIVendor, gpuPCIClassVGA, "1")
	// Non-GPU NVIDIA device (e.g. NVSwitch bridge)
	createFakePCIDevice(t, tmpDir, "0000:00:1f.0", nvidiaPCIVendor, "0x068000", "0")
	// Intel network card
	createFakePCIDevice(t, tmpDir, "0000:01:00.0", "0x8086", "0x020000", "0")

	gpus := discoverGPUs()

	if len(gpus) != 4 {
		t.Fatalf("discoverGPUs() found %d GPUs, want 4", len(gpus))
	}

	// GPUs should be indexed 0-3, sorted by BDF
	// 0000:3b -> index 0, 0000:86 -> index 1, 0000:af -> index 2, 0000:d8 -> index 3
	expected := map[int]struct {
		numa int
		bdf  string
	}{
		0: {0, "0000:3b:00.0"},
		1: {0, "0000:86:00.0"},
		2: {1, "0000:af:00.0"},
		3: {1, "0000:d8:00.0"},
	}

	for idx, want := range expected {
		got, ok := gpus[idx]
		if !ok {
			t.Errorf("GPU index %d not found", idx)
			continue
		}
		if got.NUMANodeID != want.numa {
			t.Errorf("GPU %d: NUMANodeID = %d, want %d", idx, got.NUMANodeID, want.numa)
		}
		if got.BusID != want.bdf {
			t.Errorf("GPU %d: BusID = %q, want %q", idx, got.BusID, want.bdf)
		}
	}
}

func TestDiscoverGPUs_EmptyDir(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "fake-pci-empty")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	origPath := pciDevicesPath
	pciDevicesPath = tmpDir
	defer func() { pciDevicesPath = origPath }()

	gpus := discoverGPUs()
	if len(gpus) != 0 {
		t.Errorf("discoverGPUs() on empty dir found %d GPUs, want 0", len(gpus))
	}
}

func TestDiscoverGPUs_NoNvidiaGPUs(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "fake-pci-no-gpu")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	origPath := pciDevicesPath
	pciDevicesPath = tmpDir
	defer func() { pciDevicesPath = origPath }()

	// Intel NIC and AMD GPU — neither is NVIDIA
	createFakePCIDevice(t, tmpDir, "0000:01:00.0", "0x8086", "0x020000", "0")
	createFakePCIDevice(t, tmpDir, "0000:0a:00.0", "0x1002", gpuPCIClass3D, "0")

	gpus := discoverGPUs()
	if len(gpus) != 0 {
		t.Errorf("discoverGPUs() found %d GPUs, want 0", len(gpus))
	}
}

func TestGetAllocatedGPUs_EmptyPath(t *testing.T) {
	got := getAllocatedGPUs("", 4)
	if len(got) != 0 {
		t.Errorf("empty path should return empty map, got %v", got)
	}
}

func TestGetAllocatedGPUs_MissingFile(t *testing.T) {
	got := getAllocatedGPUs("/nonexistent/path", 4)
	if len(got) != 0 {
		t.Errorf("missing file should return empty map, got %v", got)
	}
}

func TestGetAllocatedGPUs_ValidCheckpoint(t *testing.T) {
	checkpoint := `{
		"Data": {
			"PodDeviceEntries": [
				{
					"ResourceName": "nvidia.com/gpu",
					"DeviceIDs": ["GPU-aaa", "GPU-bbb"]
				}
			],
			"RegisteredDevices": {
				"nvidia.com/gpu": ["GPU-aaa", "GPU-bbb", "GPU-ccc", "GPU-ddd"]
			}
		}
	}`

	tmpFile, err := ioutil.TempFile("", "checkpoint")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(checkpoint); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	got := getAllocatedGPUs(tmpFile.Name(), 4)
	if len(got) != 2 {
		t.Fatalf("expected 2 allocated, got %d", len(got))
	}
	if !got[0] || !got[1] {
		t.Errorf("expected GPU 0 and 1 allocated, got %v", got)
	}
	if got[2] || got[3] {
		t.Errorf("expected GPU 2 and 3 free, got %v", got)
	}
}

func TestGetAllocatedGPUs_NoGPUEntries(t *testing.T) {
	checkpoint := `{
		"Data": {
			"PodDeviceEntries": [
				{
					"ResourceName": "other.device/foo",
					"DeviceIDs": ["DEV-111"]
				}
			],
			"RegisteredDevices": {
				"nvidia.com/gpu": ["GPU-aaa", "GPU-bbb"]
			}
		}
	}`

	tmpFile, err := ioutil.TempFile("", "checkpoint")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(checkpoint); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	got := getAllocatedGPUs(tmpFile.Name(), 2)
	if len(got) != 0 {
		t.Errorf("no GPU allocations expected, got %v", got)
	}
}

func TestGetResourceInfoMap_WithAllocated(t *testing.T) {
	info := &GPUNumaInfo{
		gpuDevices: map[int]v1alpha1.GPUInfo{
			0: {NUMANodeID: 0},
			1: {NUMANodeID: 0},
			2: {NUMANodeID: 1},
			3: {NUMANodeID: 1},
		},
		allocatedGPUs: map[int]bool{
			0: true,
			2: true,
		},
	}

	resInfo := info.GetResourceInfoMap()
	if resInfo.Capacity != 4 {
		t.Errorf("Capacity = %d, want 4", resInfo.Capacity)
	}
	if resInfo.Allocatable != "1,3" {
		t.Errorf("Allocatable = %q, want %q", resInfo.Allocatable, "1,3")
	}
}

func TestGetResourceInfoMap_NoneAllocated(t *testing.T) {
	info := &GPUNumaInfo{
		gpuDevices: map[int]v1alpha1.GPUInfo{
			0: {NUMANodeID: 0},
			1: {NUMANodeID: 1},
		},
		allocatedGPUs: map[int]bool{},
	}

	resInfo := info.GetResourceInfoMap()
	if resInfo.Capacity != 2 {
		t.Errorf("Capacity = %d, want 2", resInfo.Capacity)
	}
	if resInfo.Allocatable != "0,1" {
		t.Errorf("Allocatable = %q, want %q", resInfo.Allocatable, "0,1")
	}
}

func TestGetResourceInfoMap_AllAllocated(t *testing.T) {
	info := &GPUNumaInfo{
		gpuDevices: map[int]v1alpha1.GPUInfo{
			0: {NUMANodeID: 0},
			1: {NUMANodeID: 1},
		},
		allocatedGPUs: map[int]bool{
			0: true,
			1: true,
		},
	}

	resInfo := info.GetResourceInfoMap()
	if resInfo.Capacity != 2 {
		t.Errorf("Capacity = %d, want 2", resInfo.Capacity)
	}
	if resInfo.Allocatable != "" {
		t.Errorf("Allocatable = %q, want empty", resInfo.Allocatable)
	}
}
