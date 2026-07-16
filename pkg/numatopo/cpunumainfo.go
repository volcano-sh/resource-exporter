/*
Copyright 2021 The Volcano Authors.

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
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"time"

	"k8s.io/klog/v2"
	podresv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
	cpustate "k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/state"
	"k8s.io/utils/cpuset"

	"volcano.sh/apis/pkg/apis/nodeinfo/v1alpha1"

	"volcano.sh/resource-exporter/pkg/args"
	"volcano.sh/resource-exporter/pkg/util"
)

const resourceCPU = "cpu"

// CPUNumaInfo is the object to maintain the cpu information
type CPUNumaInfo struct {
	NUMANodes   []int
	NUMA2CpuCap map[int]int
	cpu2NUMA    map[int]int
	cpuDetail   map[int]v1alpha1.CPUInfo

	NUMA2FreeCpus  map[int][]int
	podAllocations []v1alpha1.PodAllocation
}

// NewCPUNumaInfo init CPUNumaInfo struct object
func NewCPUNumaInfo() *CPUNumaInfo {
	numaInfo := &CPUNumaInfo{
		NUMA2CpuCap:   make(map[int]int),
		cpu2NUMA:      make(map[int]int),
		cpuDetail:     make(map[int]v1alpha1.CPUInfo),
		NUMA2FreeCpus: make(map[int][]int),
	}

	return numaInfo
}

// Name return the name of NumaInfo
func (info *CPUNumaInfo) Name() string {
	return resourceCPU
}

func getNumaOnline(onlinePath string) []int {
	data, err := ioutil.ReadFile(onlinePath)
	if err != nil {
		klog.Errorf("Read numa online file failed, err=%v.", err)
		return []int{}
	}

	nodeList, apiErr := util.Parse(string(data))
	if apiErr != nil {
		klog.Errorf("Parse numa online file failed, err=%v.", apiErr)
		return []int{}
	}

	return nodeList
}

func (info *CPUNumaInfo) cpu2numa(cpuid int) int {
	return info.cpu2NUMA[cpuid]
}

func getNumaNodeCpuCap(nodePath string, nodeID int) []int {
	cpuPath := filepath.Join(nodePath, fmt.Sprintf("node%d", nodeID), "cpulist")
	data, err := ioutil.ReadFile(cpuPath)
	if err != nil {
		klog.Errorf("Read node%d cpulist file failed, err: %v", nodeID, err)
		return nil
	}

	cpuList, apiErr := util.Parse(string(data))
	if apiErr != nil {
		klog.Errorf("Parse node%d cpulist file failed, err: %v", nodeID, apiErr)
		return nil
	}

	return cpuList
}

// getFreeCPUListAndPodAllocationsByManagerState returns a list of free (unallocated) CPU IDs and a list of pod cpu allocations by reading the cpu_manager_state file
func getFreeCPUListAndPodAllocationsByManagerState(cpuMngState string) ([]int, []v1alpha1.PodAllocation, error) {
	data, err := ioutil.ReadFile(cpuMngState)
	if err != nil {
		return nil, nil, fmt.Errorf("read cpu_manager_state failed, err: %w", err)
	}

	var checkpoint cpustate.CPUManagerCheckpoint
	if err := checkpoint.UnmarshalCheckpoint(data); err != nil {
		return nil, nil, fmt.Errorf("unmarshal cpu_manager_state failed, err: %w", err)
	}

	cpuList, apiErr := util.Parse(checkpoint.DefaultCPUSet)
	if apiErr != nil {
		return nil, nil, fmt.Errorf("parse cpu_manager_state.defaultCPUSet failed, err: %w", apiErr)
	}

	podAllocations := make([]v1alpha1.PodAllocation, 0, len(checkpoint.Entries))
	for podUID, containerMap := range checkpoint.Entries {
		podAlloc := v1alpha1.PodAllocation{UID: podUID}
		for containerName, allocatedCPUStr := range containerMap {
			if allocatedCPUStr != "" {
				podAlloc.ContainerAllocations = append(podAlloc.ContainerAllocations, v1alpha1.ContainerAllocation{
					Name:        containerName,
					Allocations: map[string]string{resourceCPU: allocatedCPUStr},
				})
			}
		}
		if len(podAlloc.ContainerAllocations) > 0 {
			util.SortContainerAllocations(podAlloc.ContainerAllocations)
			podAllocations = append(podAllocations, podAlloc)
		}
	}
	util.SortPodAllocations(podAllocations)

	klog.V(2).Infof("Collected %s PodAllocations for %d pods: %v", resourceCPU, len(podAllocations), podAllocations)
	return cpuList, podAllocations, nil
}

// GetFreeCPUListAndPodAllocationsByPodResources returns a list of free (unallocated) CPU IDs and a list of pod cpu allocations by calling the PodResources API.
func GetFreeCPUListAndPodAllocationsByPodResources(cpu2NUMA map[int]int) ([]int, []v1alpha1.PodAllocation, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("PodResourcesListerClient is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.List(ctx, &podresv1.ListPodResourcesRequest{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list pod resources, err: %w", err)
	}

	if resp == nil {
		return nil, nil, fmt.Errorf("received nil response from PodResourcesListerClient")
	}

	// Collecting IDs of Used CPUs
	usedCPUs := make(map[int]bool)
	podAllocations := make([]v1alpha1.PodAllocation, 0, len(resp.PodResources))
	for _, pod := range resp.PodResources {
		if pod == nil {
			continue
		}
		podAlloc := v1alpha1.PodAllocation{Name: pod.Name, Namespace: pod.Namespace}

		for _, container := range pod.Containers {
			if container == nil {
				continue
			}
			var allocatedCPUIDs []int
			for _, cpuID := range container.CpuIds {
				readID := int(cpuID)
				if _, exist := cpu2NUMA[readID]; exist {
					usedCPUs[readID] = true
					allocatedCPUIDs = append(allocatedCPUIDs, readID)
				}
			}

			allocatedCPUStr := util.FormatCPUs(allocatedCPUIDs)
			if allocatedCPUStr != "" {
				podAlloc.ContainerAllocations = append(podAlloc.ContainerAllocations, v1alpha1.ContainerAllocation{
					Name:        container.Name,
					Allocations: map[string]string{resourceCPU: allocatedCPUStr},
				})
			}
		}

		if len(podAlloc.ContainerAllocations) > 0 {
			util.SortContainerAllocations(podAlloc.ContainerAllocations)
			podAllocations = append(podAllocations, podAlloc)
		}
	}

	// Traverse cpu2NUMA to find the IDs of unused CPUs
	var freeCPUs []int
	for cpuID := range cpu2NUMA {
		if !usedCPUs[cpuID] {
			freeCPUs = append(freeCPUs, cpuID)
		}
	}

	sort.Ints(freeCPUs)
	util.SortPodAllocations(podAllocations)

	klog.V(2).Infof("the all cpu ids are: %v, the used CPUs are: %v, the free CPUs are: %v", cpu2NUMA, usedCPUs, freeCPUs)
	klog.V(2).Infof("Collected %s PodAllocations for %d pods: %v", resourceCPU, len(podAllocations), podAllocations)
	return freeCPUs, podAllocations, nil
}

func (info *CPUNumaInfo) numaCapUpdate(numaPath string) {
	for _, node := range info.NUMANodes {
		cpuList := getNumaNodeCpuCap(numaPath, node)
		info.NUMA2CpuCap[node] = len(cpuList)

		for _, cpu := range cpuList {
			info.cpu2NUMA[cpu] = node
		}
	}
}

func (info *CPUNumaInfo) numaAllocUpdate(cpuMngState string, enableGetCpuIDByPodResourceList bool) error {
	freeCPUList := make([]int, 0)
	var err error
	if enableGetCpuIDByPodResourceList {
		freeCPUList, info.podAllocations, err = GetFreeCPUListAndPodAllocationsByPodResources(info.cpu2NUMA)
	} else {
		freeCPUList, info.podAllocations, err = getFreeCPUListAndPodAllocationsByManagerState(cpuMngState)
	}
	if err != nil {
		// Preserve the previous valid state by aborting the update on failure;
		// otherwise an empty free CPU list would overwrite the allocatable CPUs
		// in the custom resource.
		return err
	}

	for _, cpuid := range freeCPUList {
		numaID := info.cpu2numa(cpuid)
		info.NUMA2FreeCpus[numaID] = append(info.NUMA2FreeCpus[numaID], cpuid)
	}

	return nil
}

// Update returns the latest cpu numa info
// if data is changed , return the latest , otherwise nil
func (info *CPUNumaInfo) Update(opt *args.Argument) NumaInfo {
	cpuNumaBasePath := filepath.Join(opt.DevicePath, "node")
	newInfo := NewCPUNumaInfo()
	newInfo.NUMANodes = getNumaOnline(filepath.Join(cpuNumaBasePath, "online"))
	newInfo.numaCapUpdate(cpuNumaBasePath)
	if err := newInfo.numaAllocUpdate(opt.CPUMngState, opt.EnableGetCpuIDByPodResourceList); err != nil {
		klog.Errorf("Failed to update NUMA allocation: %v", err)
		return nil
	}
	newInfo.cpuDetail = newInfo.getAllCPUTopoInfo(opt.DevicePath)
	if !reflect.DeepEqual(newInfo, info) {
		return newInfo
	}

	return nil
}

func (info *CPUNumaInfo) getAllCPUTopoInfo(devicePath string) map[int]v1alpha1.CPUInfo {
	cpuTopoInfo := make(map[int]v1alpha1.CPUInfo)
	for cpuID, numaID := range info.cpu2NUMA {
		coreID, socketID, err := getCoreIDSocketIDForCpu(devicePath, cpuID)
		if err != nil {
			klog.Errorf("Get cpu detail failed, err=<%v>", err)
			return nil
		}

		cpuTopoInfo[cpuID] = v1alpha1.CPUInfo{
			NUMANodeID: numaID,
			CoreID:     coreID,
			SocketID:   socketID,
		}
	}

	return cpuTopoInfo
}

func getCoreIDSocketIDForCpu(devicePath string, cpuID int) (coreID, socketID int, err error) {
	topoPath := filepath.Join(devicePath, fmt.Sprintf("cpu/cpu%d", cpuID), "topology")
	corePath := filepath.Join(topoPath, "core_id")
	data, err := ioutil.ReadFile(corePath)
	if err != nil {
		return 0, 0, fmt.Errorf("cpu %d read core_id file failed", cpuID)
	}

	tmpData, apiErr := util.Parse(string(data))
	if apiErr != nil {
		return 0, 0, fmt.Errorf("cpu %d core_id parse failed", cpuID)
	}

	coreID = tmpData[0]

	socketPath := filepath.Join(topoPath, "physical_package_id")
	data, err = ioutil.ReadFile(socketPath)
	if err != nil {
		return 0, 0, fmt.Errorf("cpu %d read scoket_id file failed", cpuID)
	}

	tmpData, apiErr = util.Parse(string(data))
	if apiErr != nil {
		return 0, 0, fmt.Errorf("cpu %d scoket_id parse failed", cpuID)
	}

	socketID = tmpData[0]

	return coreID, socketID, nil
}

// GetResourceInfoMap return the cpu topology info
func (info *CPUNumaInfo) GetResourceInfoMap() v1alpha1.ResourceInfo {
	sets := cpuset.New()
	var cap = 0

	for _, freeCpus := range info.NUMA2FreeCpus {
		tmp := cpuset.New(freeCpus...)
		sets = sets.Union(tmp)
	}

	for numaID := range info.NUMA2CpuCap {
		cap += info.NUMA2CpuCap[numaID]
	}

	return v1alpha1.ResourceInfo{
		Allocatable: sets.String(),
		Capacity:    cap,
	}
}

// GetResTopoDetail return the cpu capability topology info
func (info *CPUNumaInfo) GetResTopoDetail() interface{} {
	allCPUTopoInfo := make(map[string]v1alpha1.CPUInfo)

	for cpuID, cpuInfo := range info.cpuDetail {
		allCPUTopoInfo[strconv.Itoa(cpuID)] = cpuInfo
	}

	return allCPUTopoInfo
}

// GetPodAllocations returns the pod allocation info
func (info *CPUNumaInfo) GetPodAllocations() []v1alpha1.PodAllocation {
	return info.podAllocations
}
