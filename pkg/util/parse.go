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
	"strconv"
	"strings"
)

// Parse string, such as "1,2-7,9,10-13,14"
func Parse(s string) ([]int, error) {
	var result []int

	// Handle empty string.
	if s == "" {
		return []int{}, nil
	}

	s = strings.Trim(s, "\n")
	if s == "" {
		return []int{}, nil
	}

	// Split CPU list string:
	// "0-5,34,46-48 => ["0-5", "34", "46-48"]
	ranges := strings.Split(s, ",")
	for _, r := range ranges {
		boundaries := strings.Split(r, "-")
		if len(boundaries) == 1 {
			// Handle ranges that consist of only one element like "34".
			elem, err := strconv.Atoi(boundaries[0])
			if err != nil {
				return []int{}, err
			}
			result = append(result, elem)
		} else if len(boundaries) == 2 {
			// Handle multi-element ranges like "0-5".
			start, err := strconv.Atoi(boundaries[0])
			if err != nil {
				return []int{}, err
			}
			end, err := strconv.Atoi(boundaries[1])
			if err != nil {
				return []int{}, err
			}
			// Add all elements to the result.
			// e.g. "0-5", "46-48" => [0, 1, 2, 3, 4, 5, 46, 47, 48].
			for e := start; e <= end; e++ {
				result = append(result, e)
			}
		}
	}

	return result, nil
}
