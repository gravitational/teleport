/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

// Multi-arch setup, as mentioned in https://github.com/cilium/ebpf/issues/305
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64 -cflags "-D__TARGET_ARCH_x86" -tags bpf -type data_t command ../../bpf/enhancedrecording/command.bpf.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64 -cflags "-D__TARGET_ARCH_x86" -tags bpf -type data_t -no-global-types disk ../../bpf/enhancedrecording/disk.bpf.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64 -cflags "-D__TARGET_ARCH_x86" -tags bpf -type ipv4_data_t -type ipv6_data_t network ../../bpf/enhancedrecording/network.bpf.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target arm64 -cflags "-D__TARGET_ARCH_arm64" -tags bpf -type data_t command ../../bpf/enhancedrecording/command.bpf.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target arm64 -cflags "-D__TARGET_ARCH_arm64" -tags bpf -no-global-types -type data_t disk ../../bpf/enhancedrecording/disk.bpf.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target arm64 -cflags "-D__TARGET_ARCH_arm64" -tags bpf -type ipv4_data_t -type ipv6_data_t network ../../bpf/enhancedrecording/network.bpf.c
