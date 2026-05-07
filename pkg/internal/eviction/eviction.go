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

// Package eviction provides parsing of kubelet eviction threshold configuration.
package eviction

import (
	"fmt"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Signal defines the type for eviction signal names.
type Signal string

const (
	// SignalMemoryAvailable is the signal for available memory.
	SignalMemoryAvailable Signal = "memory.available"
	// SignalNodeFsAvailable is the signal for available node filesystem space.
	SignalNodeFsAvailable Signal = "nodefs.available"
)

// ThresholdOperator is the operator used to compare against a threshold.
type ThresholdOperator int

const (
	// OpLessThan means the threshold is triggered when the value is less than.
	OpLessThan ThresholdOperator = iota
)

// ThresholdValue represents either a percentage or a quantity.
type ThresholdValue struct {
	Quantity   *resource.Quantity
	Percentage float64
}

// Threshold defines an eviction threshold.
type Threshold struct {
	Signal   Signal
	Operator ThresholdOperator
	Value    ThresholdValue
}

// ParseThresholdConfig parses the eviction hard threshold map from kubelet config.
// The map format is signal => value, e.g. {"memory.available": "100Mi", "nodefs.available": "10%"}
func ParseThresholdConfig(evictionHard map[string]string) ([]Threshold, error) {
	if len(evictionHard) == 0 {
		return nil, nil
	}

	var thresholds []Threshold
	for signal, value := range evictionHard {
		threshold, err := parseThreshold(Signal(signal), value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse eviction threshold for %s=%s: %w", signal, value, err)
		}
		thresholds = append(thresholds, threshold)
	}
	return thresholds, nil
}

func parseThreshold(signal Signal, value string) (Threshold, error) {
	t := Threshold{
		Signal:   signal,
		Operator: OpLessThan,
	}

	if strings.HasSuffix(value, "%") {
		pctStr := strings.TrimSuffix(value, "%")
		pct, err := strconv.ParseFloat(pctStr, 64)
		if err != nil {
			return t, err
		}
		t.Value.Percentage = pct / 100.0
	} else {
		q, err := resource.ParseQuantity(value)
		if err != nil {
			return t, err
		}
		t.Value.Quantity = &q
	}

	return t, nil
}

// GetThresholdQuantity calculates the actual quantity for a threshold given the capacity.
func GetThresholdQuantity(value ThresholdValue, capacity *resource.Quantity) *resource.Quantity {
	if value.Quantity != nil {
		return value.Quantity
	}
	// Percentage-based
	capBytes := capacity.Value()
	thresholdBytes := int64(float64(capBytes) * value.Percentage)
	q := resource.NewQuantity(thresholdBytes, resource.BinarySI)
	return q
}

// HardEvictionReservation returns a ResourceList that includes reservation of resources
// based on hard eviction thresholds.
func HardEvictionReservation(thresholds []Threshold, capacity v1.ResourceList) v1.ResourceList {
	if len(thresholds) == 0 {
		return nil
	}
	ret := v1.ResourceList{}
	for _, threshold := range thresholds {
		if threshold.Operator != OpLessThan {
			continue
		}
		switch threshold.Signal {
		case SignalMemoryAvailable:
			memoryCapacity := capacity[v1.ResourceMemory]
			value := GetThresholdQuantity(threshold.Value, &memoryCapacity)
			ret[v1.ResourceMemory] = *value
		case SignalNodeFsAvailable:
			storageCapacity := capacity[v1.ResourceEphemeralStorage]
			value := GetThresholdQuantity(threshold.Value, &storageCapacity)
			ret[v1.ResourceEphemeralStorage] = *value
		}
	}
	return ret
}
