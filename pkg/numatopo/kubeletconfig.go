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
	"reflect"

	machineinfov1 "github.com/google/cadvisor/info/v1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/kubernetes/pkg/kubelet/cadvisor"
	"k8s.io/kubernetes/pkg/kubelet/eviction"
	"sigs.k8s.io/yaml"

	"volcano.sh/apis/pkg/apis/nodeinfo/v1alpha1"
	"volcano.sh/resource-exporter/pkg/machineinfo"
	"volcano.sh/resource-exporter/pkg/util"
)

type kubeletConfig struct {
	topoPolicy  map[v1alpha1.PolicyName]string
	resReserved map[string]string
}

var config = &kubeletConfig{
	topoPolicy:  make(map[v1alpha1.PolicyName]string),
	resReserved: make(map[string]string),
}

// GetPolicy return the topology manager policy on kubelet
func GetPolicy() map[v1alpha1.PolicyName]string {
	return config.topoPolicy
}

// GetResReserved return the reserved info about all resource
func GetResReserved() map[string]string {
	return config.resReserved
}

// GetKubeletConfigFromLocalFile get kubelet configuration from kubelet config file
func GetKubeletConfigFromLocalFile(kubeletConfigPath string) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
	kubeletBytes, err := ioutil.ReadFile(kubeletConfigPath)
	if err != nil {
		return nil, err
	}

	kConfig := &kubeletconfigv1beta1.KubeletConfiguration{}
	if err = yaml.Unmarshal(kubeletBytes, kConfig); err != nil {
		return nil, err
	}
	return kConfig, nil
}

// TryUpdatingResourceReservation try to update reservation based on opt.ResReserved and kubelet configuration
func TryUpdatingResourceReservation(klConfig *kubeletconfigv1beta1.KubeletConfiguration, optResReserved map[string]string) bool {
	var isChange bool = false
	policy := make(map[v1alpha1.PolicyName]string)
	policy[v1alpha1.CPUManagerPolicy] = klConfig.CPUManagerPolicy
	policy[v1alpha1.TopologyManagerPolicy] = klConfig.TopologyManagerPolicy

	if !reflect.DeepEqual(config.topoPolicy, policy) {
		for key := range config.topoPolicy {
			config.topoPolicy[key] = policy[key]
		}
		isChange = true
	}

	// TODO taking memory into consideration when memory topology is added into numa info
	var cpuReserved string
	if _, ok := optResReserved[string(v1.ResourceCPU)]; ok {
		cpuReserved = optResReserved[string(v1.ResourceCPU)]
	} else {
		// machine info is guaranteed at starting
		mi := machineinfo.GetMachineInfo()
		ReservedRes, err := calculateNodeResourceReservation(klConfig.KubeReserved, klConfig.SystemReserved, klConfig.EvictionHard, mi)
		klog.Infof("%+v", ReservedRes)
		// err won't happen regularly, unless there wrong configurations on kubelet, which would also lead to stop kubelet.
		// so let just take the default value as 0
		if err != nil {
			cpuReserved = resource.NewQuantity(0, resource.DecimalSI).String()
			klog.Warningf("failed to calculate cpu reservation, err: %v", err)
		} else {
			cpuReserved = ReservedRes.Cpu().String()
		}
	}

	if config.resReserved[string(v1.ResourceCPU)] != cpuReserved {
		config.resReserved[string(v1.ResourceCPU)] = cpuReserved
		isChange = true
	}

	return isChange
}

func calculateNodeResourceReservation(kubeReserved, systemReserved, evictionHard map[string]string, mInfo *machineinfov1.MachineInfo) (v1.ResourceList, error) {
	kubeRes, err := util.ParseResourceList(kubeReserved)
	if err != nil {
		return nil, fmt.Errorf("failed to parse KubeReserved, err: %v", err)
	}

	systemRes, err := util.ParseResourceList(systemReserved)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SystemReserved, err: %v", err)
	}

	hardEvictionThresholds, err := eviction.ParseThresholdConfig([]string{}, evictionHard, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse hard eviction, err: %v", err)
	}

	var (
		evictionReservation v1.ResourceList
		machineCapacity     v1.ResourceList
	)

	machineCapacity = cadvisor.CapacityFromMachineInfo(mInfo)
	evictionReservation = util.HardEvictionReservation(hardEvictionThresholds, machineCapacity)

	result := make(v1.ResourceList)
	for metric := range machineCapacity {
		value := resource.NewQuantity(0, resource.DecimalSI)
		if kubeRes != nil {
			value.Add(kubeRes[metric])
		}
		if systemRes != nil {
			value.Add(systemRes[metric])
		}
		if evictionReservation != nil {
			value.Add(evictionReservation[metric])
		}
		if !value.IsZero() {
			result[metric] = *value
		}
	}

	return result, nil
}

func init() {
	config.topoPolicy[v1alpha1.CPUManagerPolicy] = "none"
	config.topoPolicy[v1alpha1.TopologyManagerPolicy] = "none"
}
