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

package machineinfo

import (
	v1 "k8s.io/api/core/v1"

	"volcano.sh/resource-exporter/pkg/internal/capacity"
)

const (
	defaultCPUOnlinePath = "/sys/devices/system/cpu/online"
	defaultMeminfoPath   = "/proc/meminfo"
)

var gMachineCapacity v1.ResourceList

// InitializeMachineInfo reads machine capacity from sysfs/procfs.
func InitializeMachineInfo() error {
	cap, err := capacity.FromSysfs(defaultCPUOnlinePath, defaultMeminfoPath)
	if err != nil {
		return err
	}
	gMachineCapacity = cap
	return nil
}

// GetMachineCapacity returns the machine's resource capacity.
func GetMachineCapacity() v1.ResourceList {
	return gMachineCapacity
}
