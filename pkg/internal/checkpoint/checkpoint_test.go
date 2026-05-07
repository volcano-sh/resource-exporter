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

package checkpoint

import "testing"

func TestParseCPUManagerState(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantPolicy     string
		wantDefaultSet string
		wantErr        bool
	}{
		{
			name:           "valid static policy",
			input:          `{"policyName":"static","defaultCpuSet":"0-3,5","entries":{"pod-abc":{"container1":"4"}},"checksum":12345}`,
			wantPolicy:     "static",
			wantDefaultSet: "0-3,5",
		},
		{
			name:           "valid none policy",
			input:          `{"policyName":"none","defaultCpuSet":"0-7","checksum":0}`,
			wantPolicy:     "none",
			wantDefaultSet: "0-7",
		},
		{
			name:           "empty default cpu set",
			input:          `{"policyName":"static","defaultCpuSet":"","entries":{},"checksum":0}`,
			wantPolicy:     "static",
			wantDefaultSet: "",
		},
		{
			name:    "invalid json",
			input:   `not json at all`,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   ``,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cp, err := ParseCPUManagerState([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cp.PolicyName != tc.wantPolicy {
				t.Errorf("PolicyName = %q, want %q", cp.PolicyName, tc.wantPolicy)
			}
			if cp.DefaultCPUSet != tc.wantDefaultSet {
				t.Errorf("DefaultCPUSet = %q, want %q", cp.DefaultCPUSet, tc.wantDefaultSet)
			}
		})
	}
}
