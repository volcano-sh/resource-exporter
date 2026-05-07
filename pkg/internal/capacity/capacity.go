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

// Package capacity provides functions to determine node capacity from system information.
package capacity

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// FromSysfs reads machine capacity (CPU count and memory) directly from sysfs/procfs.
// cpuOnlinePath: typically /sys/devices/system/cpu/online
// meminfoPath: typically /proc/meminfo
func FromSysfs(cpuOnlinePath, meminfoPath string) (v1.ResourceList, error) {
	rl := make(v1.ResourceList)

	// Read CPU count from online CPUs
	cpuData, err := os.ReadFile(cpuOnlinePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CPU online: %w", err)
	}
	cpuCount, err := countCPUs(strings.TrimSpace(string(cpuData)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse CPU online: %w", err)
	}
	rl[v1.ResourceCPU] = *resource.NewQuantity(int64(cpuCount), resource.DecimalSI)

	// Read memory from /proc/meminfo
	memData, err := os.ReadFile(meminfoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read meminfo: %w", err)
	}
	memBytes, err := parseMemTotal(string(memData))
	if err != nil {
		return nil, fmt.Errorf("failed to parse meminfo: %w", err)
	}
	rl[v1.ResourceMemory] = *resource.NewQuantity(memBytes, resource.BinarySI)

	return rl, nil
}

// countCPUs parses a kernel CPU range string like "0-7" or "0-3,5,7-9" and returns the count.
func countCPUs(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	count := 0
	ranges := strings.Split(s, ",")
	for _, r := range ranges {
		parts := strings.Split(r, "-")
		if len(parts) == 1 {
			if _, err := strconv.Atoi(strings.TrimSpace(parts[0])); err != nil {
				return 0, err
			}
			count++
		} else if len(parts) == 2 {
			start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return 0, err
			}
			end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return 0, err
			}
			if end < start {
				return 0, fmt.Errorf("invalid CPU range: start %d > end %d", start, end)
			}
			count += end - start + 1
		}
	}
	return count, nil
}

// parseMemTotal extracts MemTotal from /proc/meminfo content and returns bytes.
func parseMemTotal(content string) (int64, error) {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("unexpected MemTotal format: %s", line)
			}
			kbytes, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, err
			}
			// Convert kB to bytes
			return kbytes * 1024, nil
		}
	}
	return 0, fmt.Errorf("MemTotal not found in meminfo")
}
