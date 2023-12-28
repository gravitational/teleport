//go:build bpf && !386

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
	"os"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var logger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentBPF)

const (
	kprobeProgPrefix     = "kprobe__"
	kretprobeProgPrefix  = "kretprobe__"
	tracepointProgPrefix = "tracepoint__"
	syscallsCategory     = "syscalls"
	syscallEnterPrefix   = "sys_enter_"
	syscallExitPrefix    = "sys_exit_"
	counterSuffix        = "_counter"
	doorbellSuffix       = "_doorbell"
)

var pageSize = os.Getpagesize()

const (
	// CommMax is the maximum length of a command from linux/sched.h.
	CommMax = 16

	// PathMax is the maximum length of a path from linux/limits.h.
	PathMax = 255

	// ArgvMax is the maximum length of the args vector.
	ArgvMax = 1024

	// eventArg is an exec event that holds the arguments to a function.
	eventArg = 0

	// eventRet holds the return value and other data about an event.
	eventRet = 1

	// chanSize is the size of the event channels.
	chanSize = 1024
)
