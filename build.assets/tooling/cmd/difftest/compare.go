/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
