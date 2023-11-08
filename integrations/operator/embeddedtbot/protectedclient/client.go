/*
Copyright 2023 Gravitational, Inc.

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

package protectedclient

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client"
)

// Note: this whole part of the code handles client reloading.
// In the next versions, tbot will be able to issue Client with automatically
// refreshed certificates. We will be able to remove this whole section and
// pass a plain client to the controllers.

// Accessor is a function that returns a ProtectedClient to be used in a
// reconciliation loop. As we need a new client each time tbot renews
// certificate, we cannot pass the client directly to the controllers.
// Controllers are given a Accessor and are responsible for calling
// the release() function when they are done with the client.
type Accessor func(ctx context.Context) (client *ProtectedClient, release func(), err error)

// ProtectedClient is a wrapper around client.Client that embeds a WaitGroup to
// keep track of client usage and know when/if we can call client.Close().
type ProtectedClient struct {
	useCounter *sync.WaitGroup
	*client.Client
}

// RetireClient waits for all ProtectedClient users to release the client
// before closing it. This function can be run asynchronously. If we
// can't close the client we're leaking goroutines and memory anyway.
func (tc *ProtectedClient) RetireClient() {
	if tc.Client == nil {
		return
	}
	log.Debug("Waiting for client users to exit")
	tc.useCounter.Wait()

	log.Debug("Closing teleport client")
	err := tc.Close()
	if err != nil {
		log.Warnf("Failed to close teleport client: %s", err)
	}
}

// lockClient registers that the caller is using the client and
// prevents client deletion. It returns a function that must be called to
// release the client once the reconciliation is done.
func (tc *ProtectedClient) lockClient() func() {
	tc.useCounter.Add(1)
	return func() {
		tc.useCounter.Done()
	}
}

// NewClient wraps an existing client.Client into a ProtectedClient.
func NewClient(teleportClient *client.Client) *ProtectedClient {
	return &ProtectedClient{
		useCounter: &sync.WaitGroup{},
		Client:     teleportClient,
	}
}
