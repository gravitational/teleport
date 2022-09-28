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
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/trace"
)

const (
	// discoverInstanceMetadataTimeout is the maximum amount of time allowed
	// to discover an instance metadata service. The timeout is short to
	// minimize Teleport's startup time when it isn't running on any cloud
	// instance. Checking for instance metadata typically takes less than 30ms.
	discoverInstanceMetadataTimeout = 500 * time.Millisecond
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
	// GetType gets the cloud instance type.
	GetType() types.InstanceMetadataType
}

type imConstructor func(ctx context.Context) (InstanceMetadata, error)

var providers = map[types.InstanceMetadataType]imConstructor{
	types.InstanceMetadataTypeEC2:   initEC2,
	types.InstanceMetadataTypeAzure: initAzure,
}

func initEC2(ctx context.Context) (InstanceMetadata, error) {
	im, err := aws.NewInstanceMetadataClient(ctx)
	return im, trace.Wrap(err)
}

func initAzure(ctx context.Context) (InstanceMetadata, error) {
	return azure.NewInstanceMetadataClient(), nil
}

// DiscoverInstanceMetadata checks which cloud instance type Teleport is
// running on, if any.
func DiscoverInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	ctx, cancel := context.WithTimeout(ctx, discoverInstanceMetadataTimeout)
	defer cancel()

	c := make(chan InstanceMetadata)
	clients := make([]InstanceMetadata, 0, len(providers))
	for _, constructor := range providers {
		im, err := constructor(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clients = append(clients, im)
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
		return nil, trace.NotFound("no instance metadata service found")
	}
}
