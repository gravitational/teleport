//go:build terraformprovider

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

package terminal

import (
	"io"

	"github.com/gravitational/trace"
)

type Terminal struct {
	signalEmitter

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func New(stdin io.Reader, stdout, stderr io.Writer) (*Terminal, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (t *Terminal) Stdin() io.Reader {
	return t.stdin
}

func (t *Terminal) Stdout() io.Writer {
	return t.stdout
}

func (t *Terminal) Stderr() io.Writer {
	return t.stderr
}

func (t *Terminal) InitRaw(input bool) error {
	return trace.NotImplemented("not implemented")
}

func (t *Terminal) IsAttached() bool {
	return false
}

func (t *Terminal) Size() (width int16, height int16, err error) {
	return 0, 0, trace.NotImplemented("not implemented")
}

func (t *Terminal) Resize(width, height int16) error {
	return trace.NotImplemented("not implemented")
}

func (t *Terminal) Close() error {
	return trace.NotImplemented("not implemented")
}
