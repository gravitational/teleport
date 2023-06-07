//go:build bpf && !386
// +build bpf,!386

/*
 *
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package bpf

import (
	"unsafe"

	"github.com/aquasecurity/libbpfgo"
	"github.com/gravitational/trace"
)

const monitoredCGroups = "monitored_cgroups"

type session struct {
	module *libbpfgo.Module
}

// startSession registers the given cgroup in the BPF module. Only registered
// cgroups will return events to the userspace.
func (s *session) startSession(cgroupID uint64) error {
	cgroupMap, err := s.module.GetMap(monitoredCGroups)
	if err != nil {
		return trace.Wrap(err)
	}

	dummyVal := 0
	err = cgroupMap.Update(unsafe.Pointer(&cgroupID), unsafe.Pointer(&dummyVal))
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// endSession removes the previously registered cgroup from the BPF module.
func (s *session) endSession(cgroupID uint64) error {
	cgroupMap, err := s.module.GetMap(monitoredCGroups)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := cgroupMap.DeleteKey(unsafe.Pointer(&cgroupID)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
