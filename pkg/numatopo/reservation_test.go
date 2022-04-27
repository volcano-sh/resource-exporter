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
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	machineinfov1 "github.com/google/cadvisor/info/v1"
)

const (
	_1GBMem uint64 = 1024 * 1024 * 1024
)

var machineInfo = machineinfov1.MachineInfo{
	NumCores:       4,
	MemoryCapacity: 4 * _1GBMem,
}

func TestReservationCalculation(t *testing.T) {
	testCases := []struct {
		kletConfg            kubeletconfigv1beta1.KubeletConfiguration
		expectedResourceList map[v1.ResourceName]string
	}{
		{
			kubeletconfigv1beta1.KubeletConfiguration{
				KubeReserved: map[string]string{
					string(v1.ResourceCPU):    "200m",
					string(v1.ResourceMemory): "300Mi",
				},
				SystemReserved: map[string]string{
					string(v1.ResourceCPU):    "300m",
					string(v1.ResourceMemory): "1Gi",
				},
				EvictionHard: map[string]string{
					"memory.available": "1Gi",
				},
			},
			map[v1.ResourceName]string{
				v1.ResourceCPU:    "500m",
				v1.ResourceMemory: "2348Mi",
			},
		},
		{
			kubeletconfigv1beta1.KubeletConfiguration{
				KubeReserved: map[string]string{
					string(v1.ResourceCPU):    "500m",
					string(v1.ResourceMemory): "300Mi",
				},
				SystemReserved: map[string]string{
					string(v1.ResourceCPU):    "500m",
					string(v1.ResourceMemory): "1Gi",
				},
				EvictionHard: map[string]string{
					"memory.available": "50%",
				},
			},
			map[v1.ResourceName]string{
				v1.ResourceCPU:    "1",
				v1.ResourceMemory: fmt.Sprintf("%dMi", 2048+1324),
			},
		},
	}

	for _, tc := range testCases {
		reservation, err := calculateNodeResourceReservation(tc.kletConfg.KubeReserved, tc.kletConfg.SystemReserved, tc.kletConfg.EvictionHard, &machineInfo)
		if err != nil {
			t.Error(err)
			return
		}

		for metric, quan := range tc.expectedResourceList {
			pq, err := resource.ParseQuantity(quan)
			if err != nil {
				t.Errorf("failed to parseQuantity, err: %v", err)
				continue
			}
			if pq.Cmp(reservation[metric]) != 0 {
				t.Errorf("expect: %v, got: %v", pq, reservation[metric])
			}
		}
	}
}
