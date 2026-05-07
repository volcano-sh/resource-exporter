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

// Package checkpoint provides parsing of kubelet's cpu_manager_state file.
// The file is a JSON checkpoint with at minimum a "defaultCpuSet" field.
package checkpoint

import "encoding/json"

// CPUManagerCheckpoint represents the kubelet CPU manager state checkpoint.
type CPUManagerCheckpoint struct {
	PolicyName    string                       `json:"policyName"`
	DefaultCPUSet string                       `json:"defaultCpuSet"`
	Entries       map[string]map[string]string `json:"entries,omitempty"`
	Checksum      uint64                       `json:"checksum"`
}

// ParseCPUManagerState parses the cpu_manager_state JSON file content and
// returns the checkpoint data.
func ParseCPUManagerState(data []byte) (*CPUManagerCheckpoint, error) {
	cp := &CPUManagerCheckpoint{}
	if err := json.Unmarshal(data, cp); err != nil {
		return nil, err
	}
	return cp, nil
}
