/*
Copyright 2022 Gravitational, Inc.

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

package cloud

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

// InstanceMetadata is an interface for fetching information from a cloud
// service's instance metadata.
type InstanceMetadata interface {
	// IsAvailable checks if instance metadata is available.
	IsAvailable(ctx context.Context) bool
	// GetTags gets all of the instance's tags.
	GetTags(ctx context.Context) (map[string]string, error)
	// GetHostname gets the hostname set by the cloud instance that Teleport
	// should use, if any.
	GetHostname(ctx context.Context) (string, error)
	// GetType gets the
	GetType() types.InstanceMetadataType
}

type IMDSClientConstructor func(ctx context.Context) (InstanceMetadata, error)

type IMDSProvider struct {
	Name        string
	Constructor IMDSClientConstructor
}

var (
	providers   []IMDSProvider
	providersMu sync.RWMutex
)

func getProviders() []IMDSProvider {
	providersMu.RLock()
	defer providersMu.RUnlock()
	p := append([]IMDSProvider{}, providers...)
	return p
}

func RegisterIMDSProvider(name string, constructor IMDSClientConstructor) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers = append(providers, IMDSProvider{
		Name:        name,
		Constructor: constructor,
	})
}

func DiscoverInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()

	c := make(chan InstanceMetadata)
	providers := getProviders()
	clients := make([]InstanceMetadata, 0, len(providers))
	for _, provider := range providers {
		client, err := provider.Constructor(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clients = append(clients, client)
	}

	for _, client := range clients {
		client := client
		go func() {
			if client.IsAvailable(ctx) {
				c <- client
			}
		}()
	}

	select {
	case client := <-c:
		return client, nil
	case <-ctx.Done():
		return nil, trace.NotFound("No instance metadata service found")
	}
}
