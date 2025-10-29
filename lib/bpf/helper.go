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
	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var logger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentBPF)

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
)
