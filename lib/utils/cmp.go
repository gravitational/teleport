/*
Copyright 2021 Gravitational, Inc.

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

// MinInt64 returns the numerically lesser of two signed 64-bit integers.
func MinInt64(x, y int64) int64 {
	if x > y {
		return y
	}
	return x
}

// MaxInt64 returns the numerically greater of two signed 64-bit integers.
func MaxInt64(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}
