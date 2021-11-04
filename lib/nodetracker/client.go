/*
Copyright 2021 Gravitational, Inc.

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

package nodetracker

import (
	"context"
	"errors"
	"sync"

	"github.com/gravitational/teleport/lib/nodetracker/api"

	"github.com/gravitational/trace"
)

var (
	cm     sync.RWMutex
	client api.Client = &noopClient{}

	noopError error = errors.New("nodetracker: noop error")
)

// SetClient sets the node tracker client interface
func SetClient(c api.Client) {
	cm.Lock()
	defer cm.Unlock()
	client = c
}

// GetClient returns the node tracker client interface
func GetClient() api.Client {
	cm.RLock()
	defer cm.RUnlock()
	return client
}

// noopClient is a no-op node tracker client that does nothing
type noopClient struct{}

// AddNode does nothing
func (c *noopClient) AddNode(
	ctx context.Context,
	nodeID string,
	proxyID string,
	clusterName string,
	addr string,
) error {
	return trace.NotImplemented(noopError.Error())
}

// RemoveNode does nothing
func (c *noopClient) RemoveNode(ctx context.Context, nodeID string) error {
	return trace.NotImplemented(noopError.Error())
}

// GetProxies does nothing
func (c *noopClient) GetProxies(ctx context.Context, nodeID string) ([]api.ProxyDetails, error) {
	return nil, trace.NotImplemented(noopError.Error())
}
