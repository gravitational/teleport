//go:build unix

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

package socketpair

import (
	"os"

	"github.com/gravitational/trace"
)

// NewFDs creates a unix socket pair, returning the halves as files.
func NewFDs() (left, right *os.File, err error) {
	lfd, rfd, err := cloexecSocketpair()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return os.NewFile(lfd, "lsock"), os.NewFile(rfd, "rsock"), nil
}
