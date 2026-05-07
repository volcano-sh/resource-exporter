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

// Package cpuset provides a minimal CPU set implementation for formatting
// CPU lists as kernel-style range strings (e.g. "0-3,5,7-9").
package cpuset

import (
	"fmt"
	"sort"
	"strings"
)

// CPUSet represents a set of CPU IDs.
type CPUSet struct {
	cpus map[int]struct{}
}

// New creates a CPUSet from the given CPU IDs.
func New(cpus ...int) CPUSet {
	s := CPUSet{cpus: make(map[int]struct{}, len(cpus))}
	for _, c := range cpus {
		s.cpus[c] = struct{}{}
	}
	return s
}

// Union returns a new CPUSet containing all elements from both sets.
func (s CPUSet) Union(other CPUSet) CPUSet {
	result := New()
	for c := range s.cpus {
		result.cpus[c] = struct{}{}
	}
	for c := range other.cpus {
		result.cpus[c] = struct{}{}
	}
	return result
}

// String returns the CPU set as a kernel-style range string (e.g. "0-3,5,7-9").
func (s CPUSet) String() string {
	if len(s.cpus) == 0 {
		return ""
	}

	sorted := make([]int, 0, len(s.cpus))
	for c := range s.cpus {
		sorted = append(sorted, c)
	}
	sort.Ints(sorted)

	var ranges []string
	start := sorted[0]
	end := sorted[0]

	for i := 1; i < len(sorted); i++ {
		if sorted[i] == end+1 {
			end = sorted[i]
		} else {
			ranges = append(ranges, formatRange(start, end))
			start = sorted[i]
			end = sorted[i]
		}
	}
	ranges = append(ranges, formatRange(start, end))

	return strings.Join(ranges, ",")
}

func formatRange(start, end int) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}
