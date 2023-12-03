//go:build bpf && !386
// +build bpf,!386

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
