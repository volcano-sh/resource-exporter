/*
Copyright 2018 The Kubernetes Authors.
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

package util

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	evictionapi "k8s.io/kubernetes/pkg/kubelet/eviction/api"
)

func TestHardEvictionReservation(t *testing.T) {
	capacity := v1.ResourceList{
		v1.ResourceMemory:           resource.MustParse("10Gi"),
		v1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
	}

	t.Run("no thresholds returns nil", func(t *testing.T) {
		if got := HardEvictionReservation(nil, capacity); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("non less-than operator is ignored", func(t *testing.T) {
		thresholds := []evictionapi.Threshold{
			{
				Signal:   evictionapi.SignalMemoryAvailable,
				Operator: evictionapi.ThresholdOperator("GreaterThan"),
				Value:    evictionapi.ThresholdValue{Percentage: 0.1},
			},
		}
		got := HardEvictionReservation(thresholds, capacity)
		if _, ok := got[v1.ResourceMemory]; ok {
			t.Errorf("expected no memory reservation, got %v", got)
		}
	})

	t.Run("memory available threshold reserves memory", func(t *testing.T) {
		value := evictionapi.ThresholdValue{Percentage: 0.1}
		thresholds := []evictionapi.Threshold{
			{Signal: evictionapi.SignalMemoryAvailable, Operator: evictionapi.OpLessThan, Value: value},
		}
		got := HardEvictionReservation(thresholds, capacity)
		memCapacity := capacity[v1.ResourceMemory]
		want := evictionapi.GetThresholdQuantity(value, &memCapacity)
		if q, ok := got[v1.ResourceMemory]; !ok || q.Cmp(*want) != 0 {
			t.Errorf("expected memory reservation %v, got %v", want, got[v1.ResourceMemory])
		}
	})

	t.Run("node fs available threshold reserves ephemeral storage", func(t *testing.T) {
		value := evictionapi.ThresholdValue{Percentage: 0.1}
		thresholds := []evictionapi.Threshold{
			{Signal: evictionapi.SignalNodeFsAvailable, Operator: evictionapi.OpLessThan, Value: value},
		}
		got := HardEvictionReservation(thresholds, capacity)
		storageCapacity := capacity[v1.ResourceEphemeralStorage]
		want := evictionapi.GetThresholdQuantity(value, &storageCapacity)
		if q, ok := got[v1.ResourceEphemeralStorage]; !ok || q.Cmp(*want) != 0 {
			t.Errorf("expected ephemeral storage reservation %v, got %v", want, got[v1.ResourceEphemeralStorage])
		}
	})

	t.Run("unrecognized signal is ignored", func(t *testing.T) {
		thresholds := []evictionapi.Threshold{
			{
				Signal:   evictionapi.SignalImageFsAvailable,
				Operator: evictionapi.OpLessThan,
				Value:    evictionapi.ThresholdValue{Percentage: 0.1},
			},
		}
		got := HardEvictionReservation(thresholds, capacity)
		if len(got) != 0 {
			t.Errorf("expected no reservations, got %v", got)
		}
	})
}
