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

package proxy

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

func TestNewClusterDetails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	getClusterDetailsConfig := func(c *clockwork.FakeClock) (clusterDetailsConfig, *clusterDetailsClientSet) {
		client := &clusterDetailsClientSet{}
		return clusterDetailsConfig{
			kubeCreds: &staticKubeCreds{
				kubeClient: client,
			},
			cluster: &types.KubernetesClusterV3{},
			log:     utils.NewSlogLoggerForTests(),
			clock:   c,
		}, client
	}

	t.Run("normal operation", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		config, client := getClusterDetailsConfig(clock)
		details, err := newClusterDetails(ctx, config)

		require.NoError(t, err)
		require.NotNil(t, details)
		require.Equal(t, 1, client.GetCalledTimes())

		clock.BlockUntil(1)

		// Advancing by short period doesn't cause another details refresh, since in normal state refresh interval
		// is long.
		clock.Advance(backoffRefreshStep + time.Second)
		clock.BlockUntil(1)
		require.Equal(t, 1, client.GetCalledTimes())

		// Advancing by the default interval period causes another details refresh.
		clock.Advance(defaultRefreshPeriod + time.Second)
		clock.BlockUntil(1)
		require.Equal(t, 2, client.GetCalledTimes())
	})

	t.Run("first time has failed, second time it's restored", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		config, client := getClusterDetailsConfig(clock)
		client.discoveryErr = errors.New("error")
		details, err := newClusterDetails(ctx, config)

		require.NoError(t, err)
		require.NotNil(t, details)
		require.True(t, details.isClusterOffline)
		require.Equal(t, 1, client.GetCalledTimes())

		clock.BlockUntil(1)

		client.discoveryErr = nil

		// Advancing by short interval causes details refresh because we're in a bad state, and trying to
		// refresh details more often.
		clock.Advance(backoffRefreshStep + time.Second)
		clock.BlockUntil(1)

		require.Equal(t, 2, client.GetCalledTimes())
		require.False(t, details.isClusterOffline)

		// After we've restored normal state advancing by short interval doesn't cause details refresh.
		clock.Advance(backoffRefreshStep + time.Second)
		clock.BlockUntil(1)
		require.Equal(t, 2, client.GetCalledTimes())

		// Advancing by the default interval period causes another details refresh.
		clock.Advance(defaultRefreshPeriod + time.Second)
		clock.BlockUntil(1)
		require.Equal(t, 3, client.GetCalledTimes())
	})

}

type clusterDetailsClientSet struct {
	kubernetes.Interface
	discovery.DiscoveryInterface

	discoveryErr error
	calledTimes  atomic.Int32
}

func (c *clusterDetailsClientSet) Discovery() discovery.DiscoveryInterface {
	return c
}

func (c *clusterDetailsClientSet) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	c.calledTimes.Add(1)
	if c.discoveryErr != nil {
		return nil, nil, c.discoveryErr
	}

	return nil, []*metav1.APIResourceList{
		&fakeAPIResource,
	}, nil
}

func (c *clusterDetailsClientSet) ServerVersion() (*version.Info, error) {
	return &version.Info{
		Major:      "1",
		Minor:      "29",
		GitVersion: "v1.29.0",
	}, nil
}

func (c *clusterDetailsClientSet) GetCalledTimes() int {
	return int(c.calledTimes.Load())
}
