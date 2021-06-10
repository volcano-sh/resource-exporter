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
	"io/ioutil"
	"reflect"

	"sigs.k8s.io/yaml"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	"volcano.sh/apis/pkg/apis/nodeinfo/v1alpha1"
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

	kubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{}
	if err := yaml.Unmarshal(kubeletBytes, kubeletConfig); err != nil {
		return nil, err
	}
	return kubeletConfig, nil
}

// GetkubeletConfig get kubelet configuration from kubelet config file
func GetkubeletConfig(confPath string, resReserved map[string]string) bool {
	klConfig, err := GetKubeletConfigFromLocalFile(confPath)
	if err != nil {
		klog.Errorf("Get topology Manager Policy failed, err: %v", err)
		return false
	}

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

	var cpuReserved string
	if _, ok := resReserved[string(v1.ResourceCPU)]; ok {
		cpuReserved = resReserved[string(v1.ResourceCPU)]
	} else {
		cpuReserved = klConfig.KubeReserved[string(v1.ResourceCPU)]
	}

	if config.resReserved[string(v1.ResourceCPU)] != cpuReserved {
		config.resReserved[string(v1.ResourceCPU)] = cpuReserved
		isChange = true
	}

	return isChange
}

func init() {
	config.topoPolicy[v1alpha1.CPUManagerPolicy] = "none"
	config.topoPolicy[v1alpha1.TopologyManagerPolicy] = "none"
}
