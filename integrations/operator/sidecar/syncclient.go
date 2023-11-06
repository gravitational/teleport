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

package sidecar

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

// ClientAccessor is a function that returns a SyncClient to be used in a
// reconciliation loop. As we need a new client each time tbot renews
// certificate, we cannot pass the client directly to the controllers.
// Controllers are given a ClientAccessor and are responsible for calling
// RLock() / RUnlock() on the SyncClient.
type ClientAccessor func(ctx context.Context) (*SyncClient, func(), error)

// SyncClient is a wrapper around client.Client that embeds an RWMutex to
// keep track of client usage and know when/if we can call client.Close().
type SyncClient struct {
	lock *sync.RWMutex
	*client.Client
}

// RetireClient waits for all SyncClient users to RUnlock(), then it closes the
// client. This function can be run asynchronously. If we can't close the
// client we're leaking goroutines and memory anyway.
func (tc *SyncClient) RetireClient() {
	if tc.Client == nil {
		return
	}
	log.Debug("Waiting for client users to exit")
	tc.lock.Lock()
	defer tc.lock.Unlock()

	log.Debug("Closing teleport client")
	err := tc.Close()
	if err != nil {
		log.Warnf("Failed to close teleport client: %s", err)
	}
}

func (tc *SyncClient) LockClient() func() {
	tc.lock.RLock()
	return func() {
		tc.lock.RUnlock()
	}
}

// NewSyncClient wraps an existing client.Client into a SyncClient.
func NewSyncClient(teleportClient *client.Client) *SyncClient {
	return &SyncClient{
		lock:   &sync.RWMutex{},
		Client: teleportClient,
	}
}
