/*
Copyright 2020-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package alpnproxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestListenerMuxInjectConnection creates two listeners and checks if connection established to this listeners
// is handled by http server. Incoming connection accepted by secondListener is injected to http listener by
// ListenerMuxWrapper.HandleConnection function.
func TestListenerMuxInjectConnection(t *testing.T) {
	firstListener := mustCreateLocalListener(t)
	secondListener := mustCreateLocalListener(t)

	lx := NewMuxListenerWrapper(firstListener, secondListener)

	mustStartHTTPServer(t, lx)
	go func() {
		for {
			conn, err := secondListener.Accept()
			if err != nil {
				t.Logf("secondListener error:%v", err)
				return
			}
			go func() {
				err := lx.HandleConnection(context.Background(), conn)
				require.NoError(t, err)
			}()
		}
	}()

	mustSuccessfullyCallHTTPServer(t, firstListener.Addr().String())
	mustSuccessfullyCallHTTPServer(t, secondListener.Addr().String())

	err := lx.Close()
	require.NoError(t, err)
}
