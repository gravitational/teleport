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

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
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
		cluster, err := s.fromProfile(name, "")
		if err != nil {
			return nil, trace.Wrap(err)
		}

		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

// GetByURI returns a cluster by URI
func (s *Storage) GetByURI(clusterURI uri.ResourceURI) (*Cluster, error) {
	profileName := clusterURI.GetProfileName()
	leafClusterName := clusterURI.GetLeafClusterName()

	cluster, err := s.fromProfile(profileName, leafClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cluster, nil
}

// GetByResourceURI returns a cluster by a URI of its resource. Accepts both root and leaf cluster
// resources and will return a root or leaf cluster accordingly.
func (s *Storage) GetByResourceURI(resourceURI uri.ResourceURI) (*Cluster, error) {
	cluster, err := s.GetByURI(resourceURI.GetClusterURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cluster, nil
}

// ResolveCluster is an alias for GetByResourceURI.
func (s *Storage) ResolveCluster(resourceURI uri.ResourceURI) (*Cluster, error) {
	cluster, err := s.GetByResourceURI(resourceURI)
	return cluster, trace.Wrap(err)
}

// Remove removes a cluster
func (s *Storage) Remove(ctx context.Context, profileName string) error {
	if err := profile.RemoveProfile(s.Dir, profileName); err != nil {
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
			cluster, err := s.fromProfile(clusterName, "")
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return cluster, nil
		}
	}

	cluster, err := s.addCluster(ctx, s.Dir, webProxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cluster, nil
}

// addCluster adds a new cluster. This makes the underlying profile .yaml file to be saved to the
// tsh home dir without logging in the user yet. Adding a cluster makes it show up in the UI as the
// list of clusters depends on the profiles in the home dir of tsh.
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

	profileName := parseName(webProxyAddress)
	clusterURI := uri.NewClusterURI(profileName)
	clusterClient, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Ping verifies that the cluster is reachable. It also updates a couple of TeleportClient fields
	// automatically based on the ping response – those fields are then saved to the profile file.
	pingResponse, err := clusterClient.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Do the ALPN handshake test to decide if connection upgrades are required
	// for TLS Routing. Only do the test once Ping verifies the cluster is
	// reachable.
	clusterClient.TLSRoutingConnUpgradeRequired = apiclient.IsALPNConnUpgradeRequired(ctx, webProxyAddress, s.InsecureSkipVerify)

	if err := clusterClient.SaveProfile(false); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Cluster{
		URI: clusterURI,
		// The cluster name cannot be deduced from the web proxy address alone. The name of the cluster
		// might be different than the address of the proxy.
		Name:          pingResponse.ClusterName,
		ProfileName:   profileName,
		clusterClient: clusterClient,
		dir:           s.Dir,
		clock:         s.Clock,
		Log:           s.Log.WithField("cluster", clusterURI),
	}, nil
}

// fromProfile creates a new cluster from its profile
func (s *Storage) fromProfile(profileName, leafClusterName string) (*Cluster, error) {
	if profileName == "" {
		return nil, trace.BadParameter("cluster name is missing")
	}

	clusterNameForKey := profileName
	clusterURI := uri.NewClusterURI(profileName)

	profileStore := client.NewFSProfileStore(s.Dir)

	cfg := client.MakeDefaultConfig()
	if err := cfg.LoadProfile(profileStore, profileName); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.KeysDir = s.Dir
	cfg.HomePath = s.Dir
	cfg.InsecureSkipVerify = s.InsecureSkipVerify

	if leafClusterName != "" {
		clusterNameForKey = leafClusterName
		clusterURI = clusterURI.AppendLeafCluster(leafClusterName)
		cfg.SiteName = leafClusterName
	}

	clusterClient, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	status := &client.ProfileStatus{}

	// load profile status if key exists
	_, err = clusterClient.LocalAgent().GetKey(clusterNameForKey)
	if err != nil {
		if trace.IsNotFound(err) {
			s.Log.Infof("No keys found for cluster %v.", clusterNameForKey)
		} else {
			return nil, trace.Wrap(err)
		}
	}
	if err == nil && cfg.Username != "" {
		status, err = clusterClient.ProfileStatus()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := clusterClient.LoadKeyForCluster(context.Background(), status.Cluster); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &Cluster{
		URI:           clusterURI,
		Name:          clusterClient.SiteName,
		ProfileName:   profileName,
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
