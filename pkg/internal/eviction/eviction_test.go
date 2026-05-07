/*
Copyright 2018 The Kubernetes Authors.
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

package eviction

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestParseThresholdConfig(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]string
		wantLen int
		wantErr bool
	}{
		{"nil map", nil, 0, false},
		{"empty map", map[string]string{}, 0, false},
		{
			"valid absolute",
			map[string]string{"memory.available": "100Mi"},
			1, false,
		},
		{
			"valid percentage",
			map[string]string{"memory.available": "10%"},
			1, false,
		},
		{
			"multiple signals",
			map[string]string{"memory.available": "100Mi", "nodefs.available": "10%"},
			2, false,
		},
		{
			"invalid quantity",
			map[string]string{"memory.available": "not-a-quantity"},
			0, true,
		},
		{
			"negative percentage",
			map[string]string{"memory.available": "-10%"},
			0, true,
		},
		{
			"over 100 percentage",
			map[string]string{"memory.available": "150%"},
			0, true,
		},
		{
			"zero percentage is valid",
			map[string]string{"memory.available": "0%"},
			1, false,
		},
		{
			"100 percentage is valid",
			map[string]string{"nodefs.available": "100%"},
			1, false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			thresholds, err := ParseThresholdConfig(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(thresholds) != tc.wantLen {
				t.Errorf("got %d thresholds, want %d", len(thresholds), tc.wantLen)
			}
		})
	}
}

func TestGetThresholdQuantity(t *testing.T) {
	tests := []struct {
		name     string
		value    ThresholdValue
		capacity resource.Quantity
		want     int64
	}{
		{
			"absolute value",
			ThresholdValue{Quantity: resource.NewQuantity(1024*1024*100, resource.BinarySI)},
			*resource.NewQuantity(1024*1024*1024*4, resource.BinarySI),
			1024 * 1024 * 100,
		},
		{
			"10 percent of 4Gi",
			ThresholdValue{Percentage: 0.1},
			*resource.NewQuantity(1024*1024*1024*4, resource.BinarySI),
			1024 * 1024 * 1024 * 4 / 10,
		},
		{
			"50 percent of 8Gi",
			ThresholdValue{Percentage: 0.5},
			*resource.NewQuantity(1024*1024*1024*8, resource.BinarySI),
			1024 * 1024 * 1024 * 4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GetThresholdQuantity(tc.value, &tc.capacity)
			if got.Value() != tc.want {
				t.Errorf("GetThresholdQuantity() = %d, want %d", got.Value(), tc.want)
			}
		})
	}
}

func TestHardEvictionReservation(t *testing.T) {
	capacity := v1.ResourceList{
		v1.ResourceMemory:           *resource.NewQuantity(1024*1024*1024*4, resource.BinarySI),
		v1.ResourceEphemeralStorage: *resource.NewQuantity(1024*1024*1024*100, resource.BinarySI),
	}

	thresholds := []Threshold{
		{Signal: SignalMemoryAvailable, Operator: OpLessThan, Value: ThresholdValue{Quantity: resource.NewQuantity(1024*1024*1024, resource.BinarySI)}},
		{Signal: SignalNodeFsAvailable, Operator: OpLessThan, Value: ThresholdValue{Percentage: 0.1}},
	}

	result := HardEvictionReservation(thresholds, capacity)

	memReserved := result[v1.ResourceMemory]
	if memReserved.Value() != 1024*1024*1024 {
		t.Errorf("memory reservation = %d, want %d", memReserved.Value(), 1024*1024*1024)
	}

	storageReserved := result[v1.ResourceEphemeralStorage]
	expectedStorage := int64(1024 * 1024 * 1024 * 10)
	if storageReserved.Value() != expectedStorage {
		t.Errorf("storage reservation = %d, want %d", storageReserved.Value(), expectedStorage)
	}
}

func TestHardEvictionReservationEmpty(t *testing.T) {
	result := HardEvictionReservation(nil, v1.ResourceList{})
	if result != nil {
		t.Errorf("expected nil for empty thresholds, got %v", result)
	}
}
