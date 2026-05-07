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

package cpuset

import "testing"

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		cpus []int
		want string
	}{
		{"empty", nil, ""},
		{"single", []int{3}, "3"},
		{"contiguous", []int{0, 1, 2, 3}, "0-3"},
		{"non-contiguous", []int{0, 2, 4}, "0,2,4"},
		{"mixed", []int{0, 1, 2, 5, 7, 8, 9}, "0-2,5,7-9"},
		{"unsorted input", []int{9, 1, 5, 0, 2, 8, 7}, "0-2,5,7-9"},
		{"duplicates", []int{1, 1, 2, 2, 3}, "1-3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(tc.cpus...)
			if got := s.String(); got != tc.want {
				t.Errorf("New(%v).String() = %q, want %q", tc.cpus, got, tc.want)
			}
		})
	}
}

func TestUnion(t *testing.T) {
	a := New(0, 1, 2)
	b := New(2, 3, 4)
	got := a.Union(b).String()
	want := "0-4"
	if got != want {
		t.Errorf("Union() = %q, want %q", got, want)
	}
}

func TestUnionEmpty(t *testing.T) {
	a := New()
	b := New(1, 2, 3)
	got := a.Union(b).String()
	want := "1-3"
	if got != want {
		t.Errorf("Union with empty = %q, want %q", got, want)
	}
}
