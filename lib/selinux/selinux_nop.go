//go:build !selinux && cgo

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

package selinux

import (
	"github.com/gravitational/trace"
)

// InBuild returns true if the binary was built with SELinux support.
func InBuild() bool {
	return true
}

// CheckConfiguration returns an error if SELinux is not configured to
// enforce the SSH service correctly.
func CheckConfiguration() error {
	return trace.Errorf("SELinux was enabled but this Teleport binary was built without SELinux support")
}

// UserContext returns the SELinux context that should be used when
// creating processes as a certain user.
func UserContext(login string) (string, error) {
	return "", nil
}
