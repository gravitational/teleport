/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package discovery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

func TestConfigCheckAndSetDefaults(t *testing.T) {
	type fakeSVCs struct {
		events.Emitter
		authclient.DiscoveryAccessPoint
		kubernetes.Clientset
	}

	tests := []struct {
		name                        string
		cfgChange                   func(*Config)
		errAssertFunc               require.ErrorAssertionFunc
		postCheckAndSetDefaultsFunc func(*testing.T, *Config)
	}{
		{
			name:          "success without kubernetes matchers",
			errAssertFunc: require.NoError,
			cfgChange:     func(c *Config) {},
			postCheckAndSetDefaultsFunc: func(t *testing.T, c *Config) {
				require.NotNil(t, c.CloudClients)
				require.NotNil(t, c.AWSConfigProvider)
				require.NotNil(t, c.AWSDatabaseFetcherFactory)
				require.NotNil(t, c.Log)
				require.NotNil(t, c.clock)
				require.NotNil(t, c.TriggerFetchC)
				require.Equal(t, 5*time.Minute, c.PollInterval)
			},
		},
		{
			name:          "not running in kube cluster w/ kubernetes matchers",
			errAssertFunc: require.Error,
			cfgChange: func(c *Config) {
				c.Matchers = Matchers{
					Kubernetes: []types.KubernetesMatcher{
						{
							Types: []string{"svc"},
						},
					},
				}
			},
			postCheckAndSetDefaultsFunc: func(t *testing.T, c *Config) {},
		},
		{
			name:          "running in kube cluster w/ kubernetes matchers",
			errAssertFunc: require.NoError,
			cfgChange: func(c *Config) {
				c.KubernetesClient = &fakeSVCs{}
				c.Matchers = Matchers{
					Kubernetes: []types.KubernetesMatcher{
						{
							Types: []string{"svc"},
						},
					},
				}
			},
			postCheckAndSetDefaultsFunc: func(t *testing.T, c *Config) {},
		},
		{
			name:          "missing matchers & discovery group",
			errAssertFunc: require.Error,
			cfgChange: func(c *Config) {
				c.Matchers = Matchers{}
				c.DiscoveryGroup = ""
			},
			postCheckAndSetDefaultsFunc: func(t *testing.T, c *Config) {},
		},
		{
			name:          "missing emitter",
			errAssertFunc: require.Error,
			cfgChange: func(c *Config) {
				c.Emitter = nil
			},
			postCheckAndSetDefaultsFunc: func(t *testing.T, c *Config) {},
		},
		{
			name:          "missing access point",
			errAssertFunc: require.Error,
			cfgChange: func(c *Config) {
				c.AccessPoint = nil
			},
			postCheckAndSetDefaultsFunc: func(t *testing.T, c *Config) {},
		},
		{
			name:          "missing cluster features",
			errAssertFunc: require.Error,
			cfgChange: func(c *Config) {
				c.ClusterFeatures = nil
			},
			postCheckAndSetDefaultsFunc: func(t *testing.T, c *Config) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Matchers: Matchers{
					AWS: []types.AWSMatcher{
						{
							Types: []string{"svc"},
						},
					},
				},
				Emitter:     &fakeSVCs{},
				AccessPoint: &fakeSVCs{},
				ClusterFeatures: func() proto.Features {
					return proto.Features{}
				},
				DiscoveryGroup: "test",
			}
			tt.cfgChange(cfg)
			err := cfg.CheckAndSetDefaults()
			tt.errAssertFunc(t, err)

			tt.postCheckAndSetDefaultsFunc(t, cfg)
		})
	}
}
