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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/cloud/imds/aws"
	"github.com/gravitational/teleport/lib/cloud/imds/azure"
)

const (
	// discoverInstanceMetadataTimeout is the maximum amount of time allowed
	// to discover an instance metadata service. The timeout is short to
	// minimize Teleport's startup time when it isn't running on any cloud
	// instance. Checking for instance metadata typically takes less than 30ms.
	discoverInstanceMetadataTimeout = 500 * time.Millisecond
)

type imConstructor func(ctx context.Context) (imds.Client, error)

var providers = map[types.InstanceMetadataType]imConstructor{
	types.InstanceMetadataTypeEC2:   initEC2,
	types.InstanceMetadataTypeAzure: initAzure,
}

func initEC2(ctx context.Context) (imds.Client, error) {
	im, err := aws.NewInstanceMetadataClient(ctx)
	return im, trace.Wrap(err)
}

func initAzure(ctx context.Context) (imds.Client, error) {
	return azure.NewInstanceMetadataClient(), nil
}

// DiscoverInstanceMetadata checks which cloud instance type Teleport is
// running on, if any.
func DiscoverInstanceMetadata(ctx context.Context) (imds.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, discoverInstanceMetadataTimeout)
	defer cancel()

	c := make(chan imds.Client)
	clients := make([]imds.Client, 0, len(providers))
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
