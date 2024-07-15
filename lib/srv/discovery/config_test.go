/*
Copyright 2024 Gravitational, Inc.

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
