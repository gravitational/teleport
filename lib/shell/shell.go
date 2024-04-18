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
	"github.com/sirupsen/logrus"
)

const (
	DefaultShell = "/bin/sh"
)

// GetLoginShell determines the login shell for a given username.
func GetLoginShell(username string) (string, error) {
	var err error
	var shellcmd string

	shellcmd, err = getLoginShell(username)
	if err != nil {
		if trace.IsNotFound(err) {
			logrus.Warnf("No shell specified for %v, using default %v.", username, DefaultShell)
			return DefaultShell, nil
		}
		return "", trace.Wrap(err)
	}

	return shellcmd, nil
}
