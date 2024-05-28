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
		cluster, _, err := s.fromProfile(name, "")
		if err != nil {
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
	if err := profile.RemoveProfile(s.Dir, profileName); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Add adds a cluster
//
// clusterClient being returned as the second return value is a stopgap in an effort to make
// clusters.Cluster a regular struct with no extra behavior and a much smaller interface.
// https://github.com/gravitational/teleport/issues/13278
func (s *Storage) Add(ctx context.Context, webProxyAddress string) (*Cluster, *client.TeleportClient, error) {
	profiles, err := profile.ListProfileNames(s.Dir)
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

	cluster, clusterClient, err := s.addCluster(ctx, s.Dir, webProxyAddress)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return cluster, clusterClient, nil
}

// addCluster adds a new cluster. This makes the underlying profile .yaml file to be saved to the
// tsh home dir without logging in the user yet. Adding a cluster makes it show up in the UI as the
// list of clusters depends on the profiles in the home dir of tsh.
func (s *Storage) addCluster(ctx context.Context, dir, webProxyAddress string) (*Cluster, *client.TeleportClient, error) {
	if webProxyAddress == "" {
		return nil, nil, trace.BadParameter("cluster address is missing")
	}

	if dir == "" {
		return nil, nil, trace.BadParameter("cluster directory is missing")
	}

	cfg := s.makeDefaultClientConfig()
	cfg.WebProxyAddr = webProxyAddress

	profileName := parseName(webProxyAddress)
	clusterURI := uri.NewClusterURI(profileName)
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

	clusterLog := s.Log.WithField("cluster", clusterURI)

	pingResponseJSON, err := json.Marshal(pingResponse)
	if err != nil {
		clusterLog.WithError(err).Debugln("Could not marshal ping response to JSON")
	} else {
		clusterLog.WithField("response", string(pingResponseJSON)).Debugln("Got ping response")
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
		dir:           s.Dir,
		clock:         s.Clock,
		Log:           clusterLog,
	}, clusterClient, nil
}

// fromProfile creates a new cluster from its profile
func (s *Storage) fromProfile(profileName, leafClusterName string) (*Cluster, *client.TeleportClient, error) {
	if profileName == "" {
		return nil, nil, trace.BadParameter("cluster name is missing")
	}

	clusterNameForKey := profileName
	clusterURI := uri.NewClusterURI(profileName)

	profileStore := client.NewFSProfileStore(s.Dir)

	cfg := s.makeDefaultClientConfig()
	if err := cfg.LoadProfile(profileStore, profileName); err != nil {
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
	if err != nil {
		return nil, nil, trace.Wrap(err)
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
	}, clusterClient, nil
}

func (s *Storage) loadProfileStatusAndClusterKey(clusterClient *client.TeleportClient, clusterNameForKey string) (*client.ProfileStatus, error) {
	status := &client.ProfileStatus{}

	// load profile status if key exists
	_, err := clusterClient.LocalAgent().GetKey(clusterNameForKey)
	if err != nil {
		if trace.IsNotFound(err) {
			s.Log.Infof("No keys found for cluster %v.", clusterNameForKey)
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

func (s *Storage) makeDefaultClientConfig() *client.Config {
	cfg := client.MakeDefaultConfig()

	cfg.HomePath = s.Dir
	cfg.KeysDir = s.Dir
	cfg.InsecureSkipVerify = s.InsecureSkipVerify
	cfg.WebauthnLogin = s.WebauthnLogin
	// Set AllowStdinHijack to true to enable daemon.mfaPrompt to ask for both TOTP and Webauthn at
	// the same time if available.
	//
	// tsh sets AllowStdinHijack to true only during tsh login to avoid input swallowing bugs where
	// calling a command would prompt for MFA and then expect some further data through stdin. tsh
	// login does not ask for any further input after the MFA prompt.
	//
	// Since tsh daemon ran by Connect never expects data over stdin, it can always set this flag to
	// true.
	cfg.AllowStdinHijack = true

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
