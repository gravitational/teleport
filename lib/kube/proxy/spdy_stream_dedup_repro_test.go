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

package proxy

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// On a request with tty=true, stdin=true, stdout=true, expectedStreams==4 (error + stdin + stdout + resize).
// A correct waitForStreams must NOT return a successful proxy with resizeStream==nil
// when a resize stream was counted in expectedStreams.
func TestRegression_WaitForStreams_RejectsDuplicateStream(t *testing.T) {
	handler := &v4ProtocolHandler{}

	const expectedStreams = 4 // mirrors tty+stdin+stdout+(implicit error)
	streams := make(chan streamAndReply, expectedStreams)
	for _, st := range []string{
		StreamTypeError,
		StreamTypeStdin,
		StreamTypeStdout,
		StreamTypeStdin, // duplicate; would-be resize
	} {
		hdr := http.Header{}
		hdr.Set(StreamType, st)
		reply := make(chan struct{})
		close(reply) // immediate, so waitStreamReply increments receivedStreams right away
		streams <- streamAndReply{
			Stream:    &fakeSPDYStream{headers: hdr},
			replySent: reply,
		}
	}

	_, err := handler.waitForStreams(t.Context(), streams, expectedStreams, nil)
	require.Error(t, err, "waitForStreams should return an error when a duplicate stream type arrives")
}

// Calling kubeProxyClientStreams.resizeQueue() with a nil sizeQueue must not panic.
func TestRegression_KubeProxyClientStreams_ResizeQueue_NilSafe(t *testing.T) {
	c := &kubeProxyClientStreams{ /* sizeQueue is nil */ }

	ch := c.resizeQueue()
	require.NotNil(t, ch)
}
