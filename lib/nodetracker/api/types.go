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

package api

import (
	"context"
)

// Tracker describes high level node tracker operations
type Tracker interface {
	AddNode(ctx context.Context, nodeID string, proxyID string, clusterName string, addr string)
	RemoveNode(ctx context.Context, nodeID string)
	GetProxies(ctx context.Context, nodeID string) []ProxyDetails
	Stop()
}

// Client describes high level node tracker client operations
type Client interface {
	AddNode(ctx context.Context, nodeID string, proxyID string, clusterName string, addr string) error
	RemoveNode(ctx context.Context, nodeID string) error
	GetProxies(ctx context.Context, nodeID string) ([]ProxyDetails, error)
}

// Server describes high level node tracker server operations
type Server interface {
	Start() error
	Stop()
}
