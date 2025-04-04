/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package clusters

import (
	"context"
	"encoding/json"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	dtauthn "github.com/gravitational/teleport/lib/devicetrust/authn"
	dtenroll "github.com/gravitational/teleport/lib/devicetrust/enroll"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

// NewStorage creates an instance of Cluster profile storage.
func NewStorage(cfg Config) (*Storage, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Storage{Config: cfg}, nil
}

// ListProfileNames returns just the names of profiles in s.Dir.
func (s *Storage) ListProfileNames() ([]string, error) {
	return s.ClientStore.ListProfiles()
}

// ListRootClusters reads root clusters from profiles.
func (s *Storage) ListRootClusters() ([]*Cluster, error) {
	pfNames, err := s.ClientStore.ListProfiles()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusters := make([]*Cluster, 0, len(pfNames))
	for _, name := range pfNames {
		cluster, _, err := s.fromProfile(name, "")
		if cluster == nil {
			return nil, trace.Wrap(err)
		}

		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

// GetByURI returns a cluster by URI. Assumes the URI has been successfully parsed and is of a
// cluster.
//
// clusterClient being returned as the second return value is a stopgap in an effort to make
// clusters.Cluster a regular struct with no extra behavior and a much smaller interface.
// https://github.com/gravitational/teleport/issues/13278
func (s *Storage) GetByURI(clusterURI uri.ResourceURI) (*Cluster, *client.TeleportClient, error) {
	profileName := clusterURI.GetProfileName()
	leafClusterName := clusterURI.GetLeafClusterName()

	cluster, clusterClient, err := s.fromProfile(profileName, leafClusterName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return cluster, clusterClient, nil
}

// GetByResourceURI returns a cluster by a URI of its resource. Accepts both root and leaf cluster
// resources and will return a root or a leaf cluster accordingly.
//
// clusterClient being returned as the second return value is a stopgap in an effort to make
// clusters.Cluster a regular struct with no extra behavior and a much smaller interface.
// https://github.com/gravitational/teleport/issues/13278
func (s *Storage) GetByResourceURI(resourceURI uri.ResourceURI) (*Cluster, *client.TeleportClient, error) {
	cluster, clusterClient, err := s.GetByURI(resourceURI.GetClusterURI())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return cluster, clusterClient, nil
}

// ResolveCluster is an alias for GetByResourceURI.
//
// clusterClient being returned as the second return value is a stopgap in an effort to make
// clusters.Cluster a regular struct with no extra behavior and a much smaller interface.
// https://github.com/gravitational/teleport/issues/13278
func (s *Storage) ResolveCluster(resourceURI uri.ResourceURI) (*Cluster, *client.TeleportClient, error) {
	cluster, clusterClient, err := s.GetByResourceURI(resourceURI)
	return cluster, clusterClient, trace.Wrap(err)
}

// Remove removes a cluster
func (s *Storage) Remove(ctx context.Context, profileName string) error {
	return s.ClientStore.DeleteProfile(profileName)
}

// Add adds a cluster
//
// clusterClient being returned as the second return value is a stopgap in an effort to make
// clusters.Cluster a regular struct with no extra behavior and a much smaller interface.
// https://github.com/gravitational/teleport/issues/13278
func (s *Storage) Add(ctx context.Context, webProxyAddress string) (*Cluster, *client.TeleportClient, error) {
	profiles, err := s.ListProfileNames()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	clusterName := parseName(webProxyAddress)
	for _, pname := range profiles {
		if pname == clusterName {
			cluster, clusterClient, err := s.fromProfile(clusterName, "")
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			return cluster, clusterClient, nil
		}
	}

	cluster, clusterClient, err := s.addCluster(ctx, webProxyAddress)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return cluster, clusterClient, nil
}

// addCluster adds a new cluster. This makes the underlying profile .yaml file to be saved to the
// tsh home dir without logging in the user yet. Adding a cluster makes it show up in the UI as the
// list of clusters depends on the profiles in the home dir of tsh.
func (s *Storage) addCluster(ctx context.Context, webProxyAddress string) (*Cluster, *client.TeleportClient, error) {
	if webProxyAddress == "" {
		return nil, nil, trace.BadParameter("cluster address is missing")
	}

	profileName := parseName(webProxyAddress)
	clusterURI := uri.NewClusterURI(profileName)

	cfg := s.makeClientConfig()
	cfg.WebProxyAddr = webProxyAddress

	clusterClient, err := client.NewClient(cfg)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Ping verifies that the cluster is reachable. It also updates a couple of TeleportClient fields
	// automatically based on the ping response – those fields are then saved to the profile file.
	pingResponse, err := clusterClient.Ping(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	clusterLog := s.Logger.With("cluster", clusterURI)

	pingResponseJSON, err := json.Marshal(pingResponse)
	if err != nil {
		clusterLog.DebugContext(ctx, "Could not marshal ping response to JSON", "error", err)
	} else {
		clusterLog.DebugContext(ctx, "Got ping response", "response", string(pingResponseJSON))
	}

	if err := clusterClient.SaveProfile(false); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return &Cluster{
		URI: clusterURI,
		// The cluster name cannot be deduced from the web proxy address alone. The name of the cluster
		// might be different than the address of the proxy.
		Name:          pingResponse.ClusterName,
		ProfileName:   profileName,
		clusterClient: clusterClient,
		clock:         s.Clock,
		Logger:        clusterLog,
	}, clusterClient, nil
}

// fromProfile creates a new cluster from its profile
func (s *Storage) fromProfile(profileName, leafClusterName string) (*Cluster, *client.TeleportClient, error) {
	if profileName == "" {
		return nil, nil, trace.BadParameter("cluster name is missing")
	}

	clusterNameForKey := profileName
	clusterURI := uri.NewClusterURI(profileName)

	cfg := s.makeClientConfig()
	if err := cfg.LoadProfile(profileName); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if leafClusterName != "" {
		clusterNameForKey = leafClusterName
		clusterURI = clusterURI.AppendLeafCluster(leafClusterName)
		cfg.SiteName = leafClusterName
	}

	clusterClient, err := client.NewClient(cfg)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	status, err := s.loadProfileStatusAndClusterKey(clusterClient, clusterNameForKey)
	cluster := &Cluster{
		URI:           clusterURI,
		Name:          clusterClient.SiteName,
		ProfileName:   profileName,
		clusterClient: clusterClient,
		clock:         s.Clock,
		statusError:   err,
		Logger:        s.Logger.With("cluster", clusterURI),
	}
	if status != nil {
		cluster.status = *status
		cluster.SSOHost = status.SSOHost
	}

	return cluster, clusterClient, trace.Wrap(err)
}

func (s *Storage) loadProfileStatusAndClusterKey(clusterClient *client.TeleportClient, clusterNameForKey string) (*client.ProfileStatus, error) {
	status := &client.ProfileStatus{}

	// load profile status if key exists
	_, err := clusterClient.LocalAgent().GetKeyRing(clusterNameForKey)
	if err != nil {
		if trace.IsNotFound(err) {
			s.Logger.InfoContext(context.Background(), "No keys found for cluster", "cluster", clusterNameForKey)
		} else {
			return nil, trace.Wrap(err)
		}
	}

	if err == nil && clusterClient.Username != "" {
		status, err = clusterClient.ProfileStatus()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := clusterClient.LoadKeyForCluster(context.Background(), status.Cluster); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return status, nil
}

func (s *Storage) makeClientConfig() *client.Config {
	cfg := &client.Config{}
	cfg.InsecureSkipVerify = s.InsecureSkipVerify
	cfg.AddKeysToAgent = s.AddKeysToAgent
	cfg.WebauthnLogin = s.WebauthnLogin
	cfg.ClientStore = s.ClientStore
	cfg.DTAuthnRunCeremony = dtauthn.NewCeremony().Run
	cfg.DTAutoEnroll = dtenroll.AutoEnroll
	return cfg
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
