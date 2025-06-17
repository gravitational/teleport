//go:build !linux

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
	"log/slog"

	"github.com/gravitational/trace"
)

const errPlatformNotSupportedMsg = "platform not supported"

// ModuleSource returns the source of the SELinux SSH module.
func ModuleSource() string {
	return ""
}

// FileContexts returns file contexts for the SELinux SSH module.
func FileContexts(dataDir, configPath string) (string, error) {
	return "", trace.Errorf(errPlatformNotSupportedMsg)
}

// CheckConfiguration returns an error if SELinux is not configured to
// enforce the SSH service correctly.
func CheckConfiguration(ensureEnforced bool, logger *slog.Logger) error {
	return trace.Errorf(errPlatformNotSupportedMsg)
}

// UserContext returns the SELinux context that should be used when
// creating processes as a certain user.
func UserContext(login string) (string, error) {
	return "", trace.Errorf(errPlatformNotSupportedMsg)
}
