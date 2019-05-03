// +build !windows

/*
Copyright 2019 Gravitational, Inc.

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

import (
	"syscall"

	"github.com/gravitational/trace"
)

// PercentUsed returns percentage of disk space used. The percentage of disk
// space used is calculated from (total blocks - free blocks)/total blocks.
// The value is rounded to the nearest whole integer.
func PercentUsed(path string) (float64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	ratio := float64(stat.Blocks-stat.Bfree) / float64(stat.Blocks)
	return Round(ratio * 100), nil
}
