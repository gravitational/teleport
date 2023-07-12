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

package cmd

import (
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
)

type StorageByResourceURI interface {
	GetByResourceURI(uri.ResourceURI) (*clusters.Cluster, error)
}

// ProviderManagerConfig is the config for ProviderManager.
type ProviderManagerConfig struct {
	Storage StorageByResourceURI
	Execer  dbcmd.Execer
}

// CheckAndSetDefaults checks and sets the defaults.
func (c *ProviderManagerConfig) CheckAndSetDefaults() error {
	if c.Storage == nil {
		return trace.BadParameter("missing storage")
	}
	if c.Execer == nil {
		c.Execer = dbcmd.SystemExecer{}
	}
	return nil
}

// ProviderManager manages gateway command providers.
type ProviderManager struct {
	cfg ProviderManagerConfig

	dbProvider       gateway.CLICommandProvider
	dbProviderOnce   sync.Once
	kubeProvider     gateway.CLICommandProvider
	kubeProviderOnce sync.Once
}

// NewProviderManager returns a ProviderManager.
func NewProviderManager(cfg ProviderManagerConfig) (*ProviderManager, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &ProviderManager{
		cfg: cfg,
	}, nil
}

// Get returns a gateway.CLICommandProvider based on gateway's target URI.
func (m *ProviderManager) Get(targetURI uri.ResourceURI) (gateway.CLICommandProvider, error) {
	switch {
	case targetURI.IsDB():
		return m.getDBProvider(), nil

	case targetURI.IsKube():
		return m.getKubeProvider(), nil

	default:
		return nil, trace.NotImplemented("gateway not supported for %v", targetURI)
	}
}

func (m *ProviderManager) getDBProvider() gateway.CLICommandProvider {
	m.dbProviderOnce.Do(func() {
		m.dbProvider = NewDBCLICommandProvider(m.cfg.Storage, m.cfg.Execer)
	})
	return m.dbProvider
}
func (m *ProviderManager) getKubeProvider() gateway.CLICommandProvider {
	m.kubeProviderOnce.Do(func() {
		m.kubeProvider = NewKubeCLICommandProvider()
	})
	return m.kubeProvider
}
