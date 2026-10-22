/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package events

import (
	"io"

	"filippo.io/age"
	"github.com/gravitational/trace"
)

// NewTestAgeWriter returns a writer that starts an age stream to recipient on
// the first write, like the Auth Service's writer does. The slice reserves its
// part header before the first event is written, so an age header written up
// front would land in the wrong place.
func NewTestAgeWriter(writer io.WriteCloser, recipient age.Recipient) io.WriteCloser {
	return &testAgeWriter{writer: writer, recipient: recipient}
}

type testAgeWriter struct {
	writer    io.WriteCloser
	recipient age.Recipient
	started   bool
}

func (w *testAgeWriter) Write(data []byte) (int, error) {
	if !w.started {
		encrypter, err := age.Encrypt(w.writer, w.recipient)
		if err != nil {
			return 0, trace.Wrap(err)
		}
		w.writer = encrypter
		w.started = true
	}
	return w.writer.Write(data)
}

func (w *testAgeWriter) Close() error {
	return w.writer.Close()
}
