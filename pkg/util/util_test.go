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

package util

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	"volcano.sh/apis/pkg/apis/nodeinfo/v1alpha1"
)

func TestFormatCPUs(t *testing.T) {
	testCases := []struct {
		name   string
		input  []int
		expect string
	}{
		{name: "empty", input: []int{}, expect: ""},
		{name: "single", input: []int{3}, expect: "3"},
		{name: "contiguous", input: []int{0, 1, 2, 3}, expect: "0-3"},
		{name: "mixed", input: []int{0, 1, 2, 5, 7, 8, 9}, expect: "0-2,5,7-9"},
		{name: "unsorted input gets sorted", input: []int{9, 1, 0, 8, 7, 5, 2}, expect: "0-2,5,7-9"},
		{name: "two singles", input: []int{3, 5}, expect: "3,5"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatCPUs(tc.input)
			if got != tc.expect {
				t.Fatalf("expected %q, got %q", tc.expect, got)
			}
		})
	}
}

func TestFormatRange(t *testing.T) {
	if got := formatRange(3, 3); got != "3" {
		t.Fatalf("expected \"3\", got %q", got)
	}
	if got := formatRange(2, 5); got != "2-5" {
		t.Fatalf("expected \"2-5\", got %q", got)
	}
}

func TestSortContainerAllocations(t *testing.T) {
	in := []v1alpha1.ContainerAllocation{
		{Name: "z"},
		{Name: "a"},
		{Name: "m"},
	}
	SortContainerAllocations(in)
	want := []string{"a", "m", "z"}
	for i, c := range in {
		if c.Name != want[i] {
			t.Fatalf("idx %d: expected %q, got %q", i, want[i], c.Name)
		}
	}
}

func TestSortPodAllocations(t *testing.T) {
	in := []v1alpha1.PodAllocation{
		{UID: "uid2", Namespace: "nsA", Name: "podB"},
		{UID: "uid1", Namespace: "nsB", Name: "podA"},
		{UID: "uid1", Namespace: "nsA", Name: "podZ"},
		{UID: "uid1", Namespace: "nsA", Name: "podA"},
	}
	SortPodAllocations(in)
	want := []struct{ uid, ns, name string }{
		{"uid1", "nsA", "podA"},
		{"uid1", "nsA", "podZ"},
		{"uid1", "nsB", "podA"},
		{"uid2", "nsA", "podB"},
	}
	for i, w := range want {
		if in[i].UID != w.uid || in[i].Namespace != w.ns || in[i].Name != w.name {
			t.Fatalf("idx %d: expected %+v, got UID=%q NS=%q Name=%q",
				i, w, in[i].UID, in[i].Namespace, in[i].Name)
		}
	}
}

func TestParseResourceList(t *testing.T) {
	t.Run("empty map returns nil", func(t *testing.T) {
		rl, err := ParseResourceList(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rl != nil {
			t.Fatalf("expected nil, got %v", rl)
		}
	})

	t.Run("valid cpu memory ephemeral", func(t *testing.T) {
		rl, err := ParseResourceList(map[string]string{
			string(v1.ResourceCPU):              "500m",
			string(v1.ResourceMemory):           "1Gi",
			string(v1.ResourceEphemeralStorage): "2Gi",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rl) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(rl))
		}
	})

	t.Run("negative quantity fails", func(t *testing.T) {
		_, err := ParseResourceList(map[string]string{
			string(v1.ResourceCPU): "-1",
		})
		if err == nil {
			t.Fatalf("expected error for negative quantity")
		}
	})

	t.Run("unsupported resource fails", func(t *testing.T) {
		_, err := ParseResourceList(map[string]string{
			"nvidia.com/gpu": "1",
		})
		if err == nil {
			t.Fatalf("expected error for unsupported resource")
		}
	})

	t.Run("invalid quantity fails", func(t *testing.T) {
		_, err := ParseResourceList(map[string]string{
			string(v1.ResourceCPU): "abc",
		})
		if err == nil {
			t.Fatalf("expected error for invalid quantity")
		}
	})
}
