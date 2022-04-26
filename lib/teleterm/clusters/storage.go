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

package clusters

import (
	"context"
	"net"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"

	"github.com/gravitational/trace"
)

// NewStorage creates an instance of Cluster profile storage.
func NewStorage(cfg Config) (*Storage, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Storage{Config: cfg}, nil
}

// ReadAll reads clusters from profiles
func (s *Storage) ReadAll() ([]*Cluster, error) {
	pfNames, err := profile.ListProfileNames(s.Dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusters := make([]*Cluster, 0, len(pfNames))
	for _, name := range pfNames {
		cluster, err := s.fromProfile(name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

// GetByName returns a cluster by name
func (s *Storage) GetByName(clusterName string) (*Cluster, error) {
	cluster, err := s.fromProfile(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cluster, nil
}

// GetByURI returns a cluster by URI
func (s *Storage) GetByURI(clusterURI string) (*Cluster, error) {
	URI := uri.New(clusterURI)
	rootClusterName := URI.GetRootClusterName()
	leafClusterName := URI.GetLeafClusterName()

	cluster, err := s.fromProfile(rootClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if leafClusterName != "" {
		cluster.clusterClient.SiteName = leafClusterName
	}

	cluster.URI = URI

	return cluster, nil
}

// Remove removes a cluster
func (s *Storage) Remove(ctx context.Context, clusterName string) error {
	if err := profile.RemoveProfile(s.Dir, clusterName); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Add adds a cluster
func (s *Storage) Add(ctx context.Context, webProxyAddress string) (*Cluster, error) {
	profiles, err := profile.ListProfileNames(s.Dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName := parseName(webProxyAddress)
	for _, pname := range profiles {
		if pname == clusterName {
			return nil, trace.BadParameter("cluster %v already exists", clusterName)
		}
	}

	cluster, err := s.addCluster(ctx, s.Dir, webProxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cluster, nil
}

// addCluster adds a new cluster
func (s *Storage) addCluster(ctx context.Context, dir, webProxyAddress string) (*Cluster, error) {
	if webProxyAddress == "" {
		return nil, trace.BadParameter("cluster address is missing")
	}

	if dir == "" {
		return nil, trace.BadParameter("cluster directory is missing")
	}

	cfg := client.MakeDefaultConfig()
	cfg.WebProxyAddr = webProxyAddress
	cfg.HomePath = s.Dir
	cfg.KeysDir = s.Dir
	cfg.InsecureSkipVerify = s.InsecureSkipVerify

	clusterName := parseName(webProxyAddress)
	clusterURI := uri.NewClusterURI(clusterName)
	clusterClient, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// verify that cluster is reachable
	_, err = clusterClient.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cfg.SaveProfile(s.Dir, false); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Cluster{
		URI:           clusterURI,
		Name:          clusterName,
		clusterClient: clusterClient,
		dir:           s.Dir,
		clock:         s.Clock,
		Log:           s.Log.WithField("cluster", clusterURI),
	}, nil
}

// fromProfile creates a new cluster from its profile
func (s *Storage) fromProfile(clusterName string) (*Cluster, error) {
	if clusterName == "" {
		return nil, trace.BadParameter("cluster name is missing")
	}

	cfg := client.MakeDefaultConfig()
	if err := cfg.LoadProfile(s.Dir, clusterName); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.KeysDir = s.Dir
	cfg.HomePath = s.Dir
	cfg.InsecureSkipVerify = s.InsecureSkipVerify

	clusterClient, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	status := &client.ProfileStatus{}

	// load profile status if key exists
	_, err = clusterClient.LocalAgent().GetKey(clusterName)
	if err != nil {
		s.Log.WithError(err).Infof("Unable to load the keys for cluster %v.", clusterName)
	}

	if err == nil && cfg.Username != "" {
		status, err = client.ReadProfileStatus(s.Dir, clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := clusterClient.LoadKeyForCluster(status.Cluster); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	clusterURI := uri.NewClusterURI(clusterName)
	return &Cluster{
		URI:           clusterURI,
		Name:          clusterName,
		clusterClient: clusterClient,
		dir:           s.Dir,
		clock:         s.Clock,
		status:        *status,
		Log:           s.Log.WithField("cluster", clusterURI),
	}, nil
}

// parseName gets cluster name from cluster web proxy address
func parseName(webProxyAddress string) string {
	clusterName, _, err := net.SplitHostPort(webProxyAddress)
	if err != nil {
		clusterName = webProxyAddress
	}

	return clusterName
}

// Storage is the cluster storage
type Storage struct {
	Config
}
