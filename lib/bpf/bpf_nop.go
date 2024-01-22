//go:build !bpf || 386
// +build !bpf 386

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

import "github.com/gravitational/teleport/lib/service/servicecfg"

// Service is used on non-Linux systems as a NOP service that allows the
// caller to open and close sessions that do nothing on systems that don't
// support eBPF.
type Service struct {
}

// New returns a new NOP service. Note this function does nothing.
func New(_ *servicecfg.BPFConfig) (BPF, error) {
	return &NOP{}, nil
}

// SystemHasBPF returns true if the binary was build with support for BPF
// compiled in.
func SystemHasBPF() bool {
	return false
}
