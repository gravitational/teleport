/*
Copyright 2017 Gravitational, Inc.

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

package utils

// CopyByteSlice returns a copy of the byte slice.
func CopyByteSlice(in []byte) []byte {
	if in == nil {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

// CopyByteSlices returns a copy of the byte slices.
func CopyByteSlices(in [][]byte) [][]byte {
	if in == nil {
		return nil
	}
	out := make([][]byte, len(in))
	for i := range in {
		out[i] = CopyByteSlice(in[i])
	}
	return out
}

// CopyStrings makes a deep copy of the passed in string slice and returns
// the copy.
func CopyStrings(in []string) []string {
	if in == nil {
		return nil
	}

	out := make([]string, len(in))
	copy(out, in)

	return out
}

// ReplaceInSlice replaces element old with new and returns a new slice.
func ReplaceInSlice(s []string, old string, new string) []string {
	out := make([]string, 0, len(s))

	for _, x := range s {
		if x == old {
			out = append(out, new)
		} else {
			out = append(out, x)
		}
	}

	return out
}
