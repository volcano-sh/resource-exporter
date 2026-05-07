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

package capacity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCountCPUs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"empty", "", 0, false},
		{"single cpu", "0", 1, false},
		{"simple range", "0-7", 8, false},
		{"multiple ranges", "0-3,5,7-9", 8, false},
		{"single values", "0,2,4,6", 4, false},
		{"reversed range", "7-3", 0, true},
		{"non-numeric", "abc", 0, true},
		{"partial non-numeric", "0-abc", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := countCPUs(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("countCPUs(%q) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("countCPUs(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("countCPUs(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseMemTotal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{
			name:  "standard meminfo",
			input: "MemTotal:       16384000 kB\nMemFree:         8192000 kB\n",
			want:  16384000 * 1024,
		},
		{
			name:  "4GB",
			input: "MemTotal:       4194304 kB\nMemAvailable:   3000000 kB\n",
			want:  4194304 * 1024,
		},
		{
			name:    "missing memtotal",
			input:   "MemFree:   8192000 kB\n",
			wantErr: true,
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMemTotal(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("parseMemTotal() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFromSysfs(t *testing.T) {
	dir := t.TempDir()

	cpuFile := filepath.Join(dir, "cpu_online")
	memFile := filepath.Join(dir, "meminfo")

	if err := os.WriteFile(cpuFile, []byte("0-3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memFile, []byte("MemTotal:       8388608 kB\nMemFree:       4000000 kB\n"), 0644); err != nil {
		t.Fatal(err)
	}

	rl, err := FromSysfs(cpuFile, memFile)
	if err != nil {
		t.Fatalf("FromSysfs() error: %v", err)
	}

	cpuQty := rl["cpu"]
	if cpuQty.Value() != 4 {
		t.Errorf("CPU = %v, want 4", cpuQty.Value())
	}

	memQty := rl["memory"]
	expectedMem := int64(8388608 * 1024)
	if memQty.Value() != expectedMem {
		t.Errorf("Memory = %v, want %v", memQty.Value(), expectedMem)
	}
}

func TestFromSysfsMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := FromSysfs(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "also_nonexistent"))
	if err == nil {
		t.Error("expected error for missing files, got nil")
	}
}
