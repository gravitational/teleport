//go:build windows
// +build windows

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

package shell

import (
	"github.com/gravitational/trace"
)

// getLoginShell always return an error on Windows. This code his behind a
// build flag to allow cross compilation (Unix version uses CGO).
func getLoginShell(username string) (string, error) {
	return "", trace.BadParameter("login shell on Windows is not supported")
}
