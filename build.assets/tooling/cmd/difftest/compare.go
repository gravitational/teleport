/*
Copyright 2022 Gravitational, Inc.

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

package main

// CompareResult represents the result of comparison of two method sets
type CompareResult struct {
	Unchanged []Method
	New       []Method
	Changed   []Method
}

// NewCompareResult returns zero CompareResult
func NewCompareResult() CompareResult {
	return CompareResult{
		Unchanged: make([]Method, 0),
		New:       make([]Method, 0),
		Changed:   make([]Method, 0),
	}
}

// HasNew returns true if there are new methods in change set
func (r CompareResult) HasNew() bool {
	return len(r.New) > 0
}

// HasChanged returns true if there are changed methods in change set
func (r CompareResult) HasChanged() bool {
	return len(r.Changed) > 0
}

// compare compares two method sets
func compare(forkPoint []Method, head []Method) CompareResult {
	r := NewCompareResult()

	for _, h := range head {
		m, ok := findExistingMethod(forkPoint, h)
		if !ok {
			r.New = append(r.New, h)
			continue
		}

		if h.Name == m.Name {
			if h.SHA1 == m.SHA1 {
				r.Unchanged = append(r.Unchanged, m)
			} else {
				r.Changed = append(r.Changed, m)
			}
		}
	}

	return r
}

// findExistingMethod returns true if h method is found in the current state and adds it to
func findExistingMethod(forkPoint []Method, h Method) (Method, bool) {
	for _, p := range forkPoint {
		if h.Name == p.Name {
			return p, true
		}
	}

	return Method{}, false
}
