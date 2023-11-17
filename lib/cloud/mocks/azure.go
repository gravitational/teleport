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

package mocks

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/lib/cloud/azure"
)

// AKSClusterEntry is an entry in the AKSMock.Clusters list.
type AKSClusterEntry struct {
	azure.ClusterCredentialsConfig
	Config *rest.Config
	TTL    time.Duration
}

// AKSMock implements the azure.AKSClient interface for tests.
type AKSMock struct {
	azure.AKSClient
	Clusters []AKSClusterEntry
	Notify   chan struct{}
	Clock    clockwork.Clock
}

func (a *AKSMock) ClusterCredentials(ctx context.Context, cfg azure.ClusterCredentialsConfig) (*rest.Config, time.Time, error) {
	defer func() {
		a.Notify <- struct{}{}
	}()
	for _, cluster := range a.Clusters {
		if cluster.ClusterCredentialsConfig.ResourceGroup == cfg.ResourceGroup &&
			cluster.ClusterCredentialsConfig.ResourceName == cfg.ResourceName &&
			cluster.ClusterCredentialsConfig.TenantID == cfg.TenantID {
			return cluster.Config, a.Clock.Now().Add(cluster.TTL), nil
		}
	}
	return nil, time.Now(), trace.NotFound("cluster not found")
}
