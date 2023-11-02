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
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strconv"

	"k8s.io/klog"
	cpustate "k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/state"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"

	"volcano.sh/apis/pkg/apis/nodeinfo/v1alpha1"

	"volcano.sh/resource-exporter/pkg/args"
	"volcano.sh/resource-exporter/pkg/util"
)

// CPUNumaInfo is the object to maintain the cpu information
type CPUNumaInfo struct {
	NUMANodes   []int
	NUMA2CpuCap map[int]int
	cpu2NUMA    map[int]int
	cpuDetail   map[int]v1alpha1.CPUInfo

	NUMA2FreeCpus map[int][]int
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

// Name return function name
func (info *CPUNumaInfo) Name() string {
	return "cpu"
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

func getFreeCPUList(cpuMngState string) []int {
	data, err := ioutil.ReadFile(cpuMngState)
	if err != nil {
		klog.Errorf("Read cpu_manager_state failed, err: %v", err)
		return nil
	}

	checkpoint := cpustate.NewCPUManagerCheckpoint()
	checkpoint.UnmarshalCheckpoint(data)

	cpuList, apiErr := util.Parse(checkpoint.DefaultCPUSet)
	if apiErr != nil {
		klog.Errorf("Parse cpu_manager_state failed, err: %v", err)
		return nil
	}

	return cpuList
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

func (info *CPUNumaInfo) numaAllocUpdate(cpuMngState string) {
	freeCPUList := getFreeCPUList(cpuMngState)
	for _, cpuid := range freeCPUList {
		numaID := info.cpu2numa(cpuid)
		info.NUMA2FreeCpus[numaID] = append(info.NUMA2FreeCpus[numaID], cpuid)
	}
}

// Update returns the latest cpu numa info
// if data is changed , return the latest , otherwise nil
func (info *CPUNumaInfo) Update(opt *args.Argument) NumaInfo {
	cpuNumaBasePath := filepath.Join(opt.DevicePath, "node")
	newInfo := NewCPUNumaInfo()
	newInfo.NUMANodes = getNumaOnline(filepath.Join(cpuNumaBasePath, "online"))
	newInfo.numaCapUpdate(cpuNumaBasePath)
	newInfo.numaAllocUpdate(opt.CPUMngState)
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
	sets := cpuset.NewCPUSet()
	var cap = 0

	for _, freeCpus := range info.NUMA2FreeCpus {
		tmp := cpuset.NewCPUSet(freeCpus...)
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
