/*
Copyright 2023 Gravitational, Inc.

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

package mlock

import (
	"golang.org/x/sys/unix"
)

// LockMemory locks the process memory to prevent secrets from being exposed in a swap.
func LockMemory() error {
	return unix.Mlockall(unix.MCL_CURRENT | unix.MCL_FUTURE)
}
