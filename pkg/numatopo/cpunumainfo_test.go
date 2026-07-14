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
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"google.golang.org/grpc"
	podresv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
	cpustate "k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/state"
)

// ---------------------------------------------------------------------------
// helpers for the cpu_manager_state backend
// ---------------------------------------------------------------------------

func writeCheckpointFile(t *testing.T, path string, cp *cpustate.CPUManagerCheckpoint) {
	t.Helper()
	blob, err := cp.MarshalCheckpoint()
	if err != nil {
		t.Fatalf("marshal checkpoint: %v", err)
	}
	if err := os.WriteFile(path, blob, 0o600); err != nil {
		t.Fatalf("write checkpoint file: %v", err)
	}
}

func newCheckpoint(defaultCPUSet string, entries map[string]map[string]string) *cpustate.CPUManagerCheckpoint {
	cp := cpustate.NewCPUManagerCheckpoint()
	cp.PolicyName = "static"
	cp.DefaultCPUSet = defaultCPUSet
	for podUID, containers := range entries {
		if cp.Entries[podUID] == nil {
			cp.Entries[podUID] = make(map[string]string)
		}
		for cname, val := range containers {
			cp.Entries[podUID][cname] = val
		}
	}
	return cp
}

// ---------------------------------------------------------------------------
// getFreeCPUListAndPodAllocationsByManagerState
// ---------------------------------------------------------------------------

func TestGetFreeCPUListAndPodAllocationsByManagerState(t *testing.T) {
	t.Run("normal: returns free cpus and sorted pod allocations", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "cpu_manager_state")
		writeCheckpointFile(t, statePath, newCheckpoint("4-7", map[string]map[string]string{
			"uid-b": {"c1": "0-1", "c2": ""},
			"uid-a": {"c1": "2-3"},
		}))

		freeCpus, podAllocs, err := getFreeCPUListAndPodAllocationsByManagerState(statePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := []int{4, 5, 6, 7}; !reflect.DeepEqual(freeCpus, want) {
			t.Fatalf("freeCpus: expected %v, got %v", want, freeCpus)
		}

		if len(podAllocs) != 2 {
			t.Fatalf("expected 2 pod allocations, got %d", len(podAllocs))
		}
		// Sorted by UID
		if podAllocs[0].UID != "uid-a" || podAllocs[1].UID != "uid-b" {
			t.Fatalf("unexpected pod order: %v %v", podAllocs[0].UID, podAllocs[1].UID)
		}
		// containers with empty cpu string are filtered out
		if len(podAllocs[1].ContainerAllocations) != 1 {
			t.Fatalf("expected 1 container for uid-b (empty one dropped), got %d",
				len(podAllocs[1].ContainerAllocations))
		}
		if podAllocs[1].ContainerAllocations[0].Name != "c1" {
			t.Fatalf("expected c1, got %q", podAllocs[1].ContainerAllocations[0].Name)
		}
		if got := podAllocs[1].ContainerAllocations[0].Allocations[resourceCPU]; got != "0-1" {
			t.Fatalf("expected \"0-1\", got %q", got)
		}
	})

	t.Run("containers within a pod are sorted by name", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "cpu_manager_state")
		writeCheckpointFile(t, statePath, newCheckpoint("0", map[string]map[string]string{
			"uid-x": {"zContainer": "1", "aContainer": "2", "mContainer": "3"},
		}))

		_, podAllocs, err := getFreeCPUListAndPodAllocationsByManagerState(statePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(podAllocs) != 1 || len(podAllocs[0].ContainerAllocations) != 3 {
			t.Fatalf("unexpected allocation shape: %+v", podAllocs)
		}
		got := []string{
			podAllocs[0].ContainerAllocations[0].Name,
			podAllocs[0].ContainerAllocations[1].Name,
			podAllocs[0].ContainerAllocations[2].Name,
		}
		want := []string{"aContainer", "mContainer", "zContainer"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	})

	t.Run("no entries: empty pod allocations, free CPU = defaultCPUSet", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "cpu_manager_state")
		writeCheckpointFile(t, statePath, newCheckpoint("0-3", nil))

		freeCpus, podAllocs, err := getFreeCPUListAndPodAllocationsByManagerState(statePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := []int{0, 1, 2, 3}; !reflect.DeepEqual(freeCpus, want) {
			t.Fatalf("expected %v, got %v", want, freeCpus)
		}
		if len(podAllocs) != 0 {
			t.Fatalf("expected 0 pod allocations, got %d", len(podAllocs))
		}
	})

	t.Run("missing file: returns error", func(t *testing.T) {
		_, _, err := getFreeCPUListAndPodAllocationsByManagerState(filepath.Join(t.TempDir(), "nope"))
		if err == nil {
			t.Fatalf("expected error for missing file")
		}
	})

	t.Run("invalid JSON: returns error", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "cpu_manager_state")
		if err := os.WriteFile(statePath, []byte("{not-json"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		_, _, err := getFreeCPUListAndPodAllocationsByManagerState(statePath)
		if err == nil {
			t.Fatalf("expected error for invalid JSON")
		}
	})

	t.Run("invalid defaultCPUSet: returns error", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "cpu_manager_state")
		writeCheckpointFile(t, statePath, newCheckpoint("abc", nil))
		_, _, err := getFreeCPUListAndPodAllocationsByManagerState(statePath)
		if err == nil {
			t.Fatalf("expected error for invalid defaultCPUSet")
		}
	})
}

// ---------------------------------------------------------------------------
// numaAllocUpdate
// ---------------------------------------------------------------------------

// newInfoWithNUMA builds a CPUNumaInfo whose cpu2NUMA map is populated and
// whose NUMA2FreeCpus map is initialized. NUMA2FreeCpus is seeded with the
// given prior values so the "failure must not overwrite" contract can be
// exercised.
func newInfoWithNUMA(cpu2NUMA map[int]int, priorFreeCpus map[int][]int) *CPUNumaInfo {
	info := NewCPUNumaInfo()
	info.cpu2NUMA = cpu2NUMA
	for numaID, cpus := range priorFreeCpus {
		info.NUMA2FreeCpus[numaID] = append([]int(nil), cpus...)
	}
	return info
}

func TestNumaAllocUpdate(t *testing.T) {
	cpu2NUMA := map[int]int{0: 0, 1: 0, 2: 0, 3: 1, 4: 1}

	t.Run("manager_state backend success: buckets free cpus per NUMA and records allocations", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "cpu_manager_state")
		// defaultCPUSet = free = {0,1,3}; entries use {2},{4}
		writeCheckpointFile(t, statePath, newCheckpoint("0-1,3", map[string]map[string]string{
			"uid-a": {"c0": "2"},
			"uid-b": {"c0": "4"},
		}))

		info := newInfoWithNUMA(cpu2NUMA, nil)
		err := info.numaAllocUpdate(statePath, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Free CPUs 0,1 on NUMA 0; CPU 3 on NUMA 1. Order is insertion-based
		// (0,1 from the defaultCPUSet parse, 3 last), so we compare as sets.
		if got, want := info.NUMA2FreeCpus[0], []int{0, 1}; !reflect.DeepEqual(got, want) {
			t.Fatalf("NUMA0 free: expected %v, got %v", want, got)
		}
		if got, want := info.NUMA2FreeCpus[1], []int{3}; !reflect.DeepEqual(got, want) {
			t.Fatalf("NUMA1 free: expected %v, got %v", want, got)
		}
		if len(info.podAllocations) != 2 {
			t.Fatalf("expected 2 pod allocations, got %d", len(info.podAllocations))
		}
	})

	t.Run("manager_state backend failure: does not overwrite prior NUMA2FreeCpus", func(t *testing.T) {
		// Missing file -> backend returns error. Prior free state must survive.
		info := newInfoWithNUMA(cpu2NUMA, map[int][]int{
			0: {0, 1},
			1: {3, 4},
		})
		err := info.numaAllocUpdate(filepath.Join(t.TempDir(), "missing"), false)
		if err == nil {
			t.Fatalf("expected error from manager_state backend")
		}
		if got, want := info.NUMA2FreeCpus[0], []int{0, 1}; !reflect.DeepEqual(got, want) {
			t.Fatalf("NUMA0 free must be preserved on failure: expected %v, got %v", want, got)
		}
		if got, want := info.NUMA2FreeCpus[1], []int{3, 4}; !reflect.DeepEqual(got, want) {
			t.Fatalf("NUMA1 free must be preserved on failure: expected %v, got %v", want, got)
		}
		if info.podAllocations != nil {
			t.Fatalf("podAllocations must remain nil on failure, got %v", info.podAllocations)
		}
	})

	t.Run("podresources backend success: buckets free cpus per NUMA", func(t *testing.T) {
		resp := &podresv1.ListPodResourcesResponse{
			PodResources: []*podresv1.PodResources{
				{Name: "pod-a", Namespace: "ns", Containers: []*podresv1.ContainerResources{
					{Name: "c0", CpuIds: []int64{0, 2}}, // uses NUMA0 cpus 0,2 -> free on NUMA0: {1}
				}},
				{Name: "pod-b", Namespace: "ns", Containers: []*podresv1.ContainerResources{
					{Name: "c0", CpuIds: []int64{4}}, // uses NUMA1 cpu 4 -> free on NUMA1: {3}
				}},
			},
		}
		withClient(t, &fakePodResourcesClient{resp: resp})

		info := newInfoWithNUMA(cpu2NUMA, nil)
		if err := info.numaAllocUpdate("", true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := info.NUMA2FreeCpus[0], []int{1}; !reflect.DeepEqual(got, want) {
			t.Fatalf("NUMA0 free: expected %v, got %v", want, got)
		}
		if got, want := info.NUMA2FreeCpus[1], []int{3}; !reflect.DeepEqual(got, want) {
			t.Fatalf("NUMA1 free: expected %v, got %v", want, got)
		}
		if len(info.podAllocations) != 2 {
			t.Fatalf("expected 2 pod allocations, got %d", len(info.podAllocations))
		}
	})

	t.Run("podresources backend failure: does not overwrite prior NUMA2FreeCpus", func(t *testing.T) {
		withClient(t, &fakePodResourcesClient{err: errors.New("rpc broken")})

		info := newInfoWithNUMA(cpu2NUMA, map[int][]int{
			0: {0, 1},
			1: {3, 4},
		})
		err := info.numaAllocUpdate("", true)
		if err == nil {
			t.Fatalf("expected error from podresources backend")
		}
		if got, want := info.NUMA2FreeCpus[0], []int{0, 1}; !reflect.DeepEqual(got, want) {
			t.Fatalf("NUMA0 free must be preserved on failure: expected %v, got %v", want, got)
		}
		if got, want := info.NUMA2FreeCpus[1], []int{3, 4}; !reflect.DeepEqual(got, want) {
			t.Fatalf("NUMA1 free must be preserved on failure: expected %v, got %v", want, got)
		}
	})

	t.Run("podresources nil client: failure preserves prior state", func(t *testing.T) {
		withClient(t, nil)
		info := newInfoWithNUMA(cpu2NUMA, map[int][]int{0: {0}})
		if err := info.numaAllocUpdate("", true); err == nil {
			t.Fatalf("expected error when client is nil")
		}
		if got, want := info.NUMA2FreeCpus[0], []int{0}; !reflect.DeepEqual(got, want) {
			t.Fatalf("NUMA0 free must be preserved: expected %v, got %v", want, got)
		}
	})

	t.Run("empty freeCPUList clears NUMA2FreeCpus bucket when no prior state", func(t *testing.T) {
		// Manager-state with defaultCPUSet that parses to nothing is hard (Parse is
		// lenient), so use the podresources path with no pods: all CPUs free.
		withClient(t, &fakePodResourcesClient{resp: &podresv1.ListPodResourcesResponse{}})

		info := newInfoWithNUMA(cpu2NUMA, nil)
		if err := info.numaAllocUpdate("", true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// All CPUs free -> NUMA0 {0,1,2}, NUMA1 {3,4}. Both buckets populated.
		if got, want := info.NUMA2FreeCpus[0], []int{0, 1, 2}; !reflect.DeepEqual(got, want) {
			t.Fatalf("NUMA0 free: expected %v, got %v", want, got)
		}
		if got, want := info.NUMA2FreeCpus[1], []int{3, 4}; !reflect.DeepEqual(got, want) {
			t.Fatalf("NUMA1 free: expected %v, got %v", want, got)
		}
	})
}

// ---------------------------------------------------------------------------
// helpers for the podresources backend
// ---------------------------------------------------------------------------

// fakePodResourcesClient is a minimal PodResourcesListerClient fake that only
// implements List. Get/GetAllocatableResources are not exercised by the
// function under test.
type fakePodResourcesClient struct {
	resp *podresv1.ListPodResourcesResponse
	err  error
}

func (f *fakePodResourcesClient) List(ctx context.Context, _ *podresv1.ListPodResourcesRequest, _ ...grpc.CallOption) (*podresv1.ListPodResourcesResponse, error) {
	return f.resp, f.err
}

func (f *fakePodResourcesClient) GetAllocatableResources(ctx context.Context, _ *podresv1.AllocatableResourcesRequest, _ ...grpc.CallOption) (*podresv1.AllocatableResourcesResponse, error) {
	return nil, errors.New("not implemented")
}

func (f *fakePodResourcesClient) Get(ctx context.Context, _ *podresv1.GetPodResourcesRequest, _ ...grpc.CallOption) (*podresv1.GetPodResourcesResponse, error) {
	return nil, errors.New("not implemented")
}

// withClient swaps the package-level client var for the duration of a test and
// restores the previous value on cleanup.
func withClient(t *testing.T, c podresv1.PodResourcesListerClient) {
	t.Helper()
	prev := client
	client = c
	t.Cleanup(func() { client = prev })
}

// Sanity guard: the fake must satisfy the PodResourcesListerClient interface.
var _ podresv1.PodResourcesListerClient = (*fakePodResourcesClient)(nil)

// ---------------------------------------------------------------------------
// GetFreeCPUListAndPodAllocationsByPodResources
// ---------------------------------------------------------------------------

func TestGetFreeCPUListAndPodAllocationsByPodResources(t *testing.T) {
	// NUMA map: cpus 0-3 on NUMA 0, cpus 4-7 on NUMA 1.
	cpu2NUMA := map[int]int{0: 0, 1: 0, 2: 0, 3: 0, 4: 1, 5: 1, 6: 1, 7: 1}

	t.Run("nil client returns error", func(t *testing.T) {
		withClient(t, nil)
		_, _, err := GetFreeCPUListAndPodAllocationsByPodResources(cpu2NUMA)
		if err == nil {
			t.Fatalf("expected error when client is nil")
		}
	})

	t.Run("List error propagates", func(t *testing.T) {
		withClient(t, &fakePodResourcesClient{err: errors.New("rpc broken")})
		_, _, err := GetFreeCPUListAndPodAllocationsByPodResources(cpu2NUMA)
		if err == nil {
			t.Fatalf("expected error when List fails")
		}
	})

	t.Run("nil response returns error", func(t *testing.T) {
		withClient(t, &fakePodResourcesClient{resp: nil})
		_, _, err := GetFreeCPUListAndPodAllocationsByPodResources(cpu2NUMA)
		if err == nil {
			t.Fatalf("expected error when response is nil")
		}
	})

	t.Run("normal: free cpus sorted, allocations sorted, unknown cpus ignored", func(t *testing.T) {
		resp := &podresv1.ListPodResourcesResponse{
			PodResources: []*podresv1.PodResources{
				{
					Name:      "pod-b",
					Namespace: "ns",
					Containers: []*podresv1.ContainerResources{
						{Name: "c1", CpuIds: []int64{0, 1}},
						{Name: "c0", CpuIds: []int64{2}},
					},
				},
				{
					Name:      "pod-a",
					Namespace: "ns",
					Containers: []*podresv1.ContainerResources{
						{Name: "c1", CpuIds: []int64{4, 5}},
						// CPU 99 is outside cpu2NUMA and must be ignored.
						{Name: "c2", CpuIds: []int64{99}},
					},
				},
			},
		}
		withClient(t, &fakePodResourcesClient{resp: resp})

		freeCpus, podAllocs, err := GetFreeCPUListAndPodAllocationsByPodResources(cpu2NUMA)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Used CPUs: 0,1,2,4,5 -> Free: 3,6,7
		if want := []int{3, 6, 7}; !reflect.DeepEqual(freeCpus, want) {
			t.Fatalf("freeCpus: expected %v, got %v", want, freeCpus)
		}

		// Sorted by namespace then name.
		if len(podAllocs) != 2 {
			t.Fatalf("expected 2 pod allocations, got %d", len(podAllocs))
		}
		if podAllocs[0].Name != "pod-a" || podAllocs[1].Name != "pod-b" {
			t.Fatalf("unexpected pod order: %q %q", podAllocs[0].Name, podAllocs[1].Name)
		}

		// pod-b: containers sorted by name (c0, c1). FormatCPUs applied.
		b := podAllocs[1]
		if len(b.ContainerAllocations) != 2 {
			t.Fatalf("expected 2 containers, got %d", len(b.ContainerAllocations))
		}
		if b.ContainerAllocations[0].Name != "c0" || b.ContainerAllocations[1].Name != "c1" {
			t.Fatalf("containers not sorted: %q %q",
				b.ContainerAllocations[0].Name, b.ContainerAllocations[1].Name)
		}
		if got := b.ContainerAllocations[1].Allocations[resourceCPU]; got != "0-1" {
			t.Fatalf("expected \"0-1\", got %q", got)
		}

		// pod-a: c2 had only unknown cpu (99), so it must be dropped (no container allocation).
		a := podAllocs[0]
		if len(a.ContainerAllocations) != 1 {
			t.Fatalf("expected 1 container (c2 dropped), got %d", len(a.ContainerAllocations))
		}
		if a.ContainerAllocations[0].Name != "c1" {
			t.Fatalf("expected c1, got %q", a.ContainerAllocations[0].Name)
		}
		if got := a.ContainerAllocations[0].Allocations[resourceCPU]; got != "4-5" {
			t.Fatalf("expected \"4-5\", got %q", got)
		}
	})

	t.Run("nil pod and nil container are skipped", func(t *testing.T) {
		resp := &podresv1.ListPodResourcesResponse{
			PodResources: []*podresv1.PodResources{
				nil,
				{
					Name:      "pod-a",
					Namespace: "ns",
					Containers: []*podresv1.ContainerResources{
						nil,
						{Name: "c0", CpuIds: []int64{0}},
					},
				},
			},
		}
		withClient(t, &fakePodResourcesClient{resp: resp})

		freeCpus, podAllocs, err := GetFreeCPUListAndPodAllocationsByPodResources(cpu2NUMA)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := []int{1, 2, 3, 4, 5, 6, 7}; !reflect.DeepEqual(freeCpus, want) {
			t.Fatalf("freeCpus: expected %v, got %v", want, freeCpus)
		}
		if len(podAllocs) != 1 || podAllocs[0].Name != "pod-a" {
			t.Fatalf("unexpected pod allocations: %+v", podAllocs)
		}
		if len(podAllocs[0].ContainerAllocations) != 1 {
			t.Fatalf("expected 1 container (nil dropped), got %d", len(podAllocs[0].ContainerAllocations))
		}
	})

	t.Run("empty cpu2NUMA: no free cpus, no allocations", func(t *testing.T) {
		resp := &podresv1.ListPodResourcesResponse{
			PodResources: []*podresv1.PodResources{
				{Name: "pod-a", Namespace: "ns", Containers: []*podresv1.ContainerResources{
					{Name: "c0", CpuIds: []int64{0, 1, 2}},
				}},
			},
		}
		withClient(t, &fakePodResourcesClient{resp: resp})

		freeCpus, podAllocs, err := GetFreeCPUListAndPodAllocationsByPodResources(map[int]int{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(freeCpus) != 0 {
			t.Fatalf("expected 0 free cpus, got %v", freeCpus)
		}
		// All CPUs are outside cpu2NUMA, so no container allocation recorded.
		if len(podAllocs) != 0 {
			t.Fatalf("expected 0 pod allocations, got %d", len(podAllocs))
		}
	})

	t.Run("all cpus allocated: free cpus empty", func(t *testing.T) {
		resp := &podresv1.ListPodResourcesResponse{
			PodResources: []*podresv1.PodResources{
				{Name: "pod-a", Namespace: "ns", Containers: []*podresv1.ContainerResources{
					{Name: "c0", CpuIds: []int64{0, 1, 2, 3, 4, 5, 6, 7}},
				}},
			},
		}
		withClient(t, &fakePodResourcesClient{resp: resp})

		freeCpus, _, err := GetFreeCPUListAndPodAllocationsByPodResources(cpu2NUMA)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(freeCpus) != 0 {
			t.Fatalf("expected 0 free cpus, got %v", freeCpus)
		}
	})
}

// Ensure sort import stays referenced if future helpers use it directly.
var _ = sort.Slice
