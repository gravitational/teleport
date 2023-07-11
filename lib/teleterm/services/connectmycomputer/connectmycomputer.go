// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connectmycomputer

import (
	"context"
	"fmt"
	"os/user"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

type RoleSetup struct {
	cfg *RoleSetupConfig
}

func NewRoleSetup(cfg *RoleSetupConfig) (*RoleSetup, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, err
	}

	return &RoleSetup{cfg: cfg}, nil
}

type RoleSetupResult struct {
	CertsReloaded bool
}

// Run ensures that at the end of its execution the user has their own individual Connect My
// Computer role and that the role includes the current system username in allowed logins.
//
// If the role list of the user got updated, the return value has CertsReloaded set to true.
func (s *RoleSetup) Run(ctx context.Context, accessAndIdentity AccessAndIdentity, certManager CertManager, cluster *clusters.Cluster) (RoleSetupResult, error) {
	noCertsReloaded := RoleSetupResult{}
	if !cluster.URI.IsRoot() {
		return noCertsReloaded, trace.BadParameter("Connect My Computer works only with root clusters")
	}

	// Do not use GetCurrentUser – it returns the current view of the user given the certs, not merely
	// the resource from the backend. This means that GetCurrentUser includes any roles granted
	// through access requests. We don't want that since we're later going to update the user.
	clusterUser, err := accessAndIdentity.GetUser(cluster.GetLoggedInUser().Name, false /* withSecrets */)
	if err != nil {
		return noCertsReloaded, trace.Wrap(err)
	}

	isLocalUser := clusterUser.GetCreatedBy().Connector == nil
	if !isLocalUser {
		return noCertsReloaded,
			trace.BadParameter("Connect My Computer works only with local users, the user %v was created by %v connector",
				clusterUser.GetName(), clusterUser.GetCreatedBy().Connector.Type)
	}

	roleName := fmt.Sprintf("%v%v", teleport.ConnectMyComputerRoleNamePrefix, clusterUser.GetName())

	doesRoleExist := true
	existingRole, err := accessAndIdentity.GetRole(ctx, roleName)
	if err != nil {
		if trace.IsNotFound(err) {
			doesRoleExist = false
		} else {
			return noCertsReloaded, trace.Wrap(err)
		}
	}

	systemUser, err := user.Current()
	if err != nil {
		return noCertsReloaded, trace.Wrap(err)
	}

	if !doesRoleExist {
		s.cfg.Log.Infof("Creating the role %v.", roleName)

		role, err := types.NewRole(roleName, types.RoleSpecV6{
			Allow: types.RoleConditions{
				NodeLabels: types.Labels{
					types.ConnectMyComputerNodeOwnerLabel: []string{clusterUser.GetName()},
				},
				Logins: []string{systemUser.Username},
			},
		})
		if err != nil {
			return noCertsReloaded, trace.Wrap(err)
		}
		if err = accessAndIdentity.UpsertRole(ctx, role); err != nil {
			return noCertsReloaded, trace.Wrap(err, "creating role %v", role.GetName())
		}
	} else {
		allowedLogins := existingRole.GetLogins(types.Allow)

		if slices.Contains(allowedLogins, systemUser.Username) {
			s.cfg.Log.Infof("The role %v already exists and includes current system username.", roleName)
		} else {
			s.cfg.Log.Infof("Adding %v to the logins of the role %v.", systemUser.Username, roleName)

			existingRole.SetLogins(types.Allow, append(allowedLogins, systemUser.Username))

			timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			err = s.syncResourceUpdate(timeoutCtx, accessAndIdentity, existingRole, func(ctx context.Context) error {
				return trace.Wrap(accessAndIdentity.UpsertRole(ctx, existingRole),
					"updating role %v", existingRole.GetName())
			})
			if err != nil {
				return noCertsReloaded, trace.Wrap(err)
			}
		}
	}

	hasCMCRole := slices.Contains(clusterUser.GetRoles(), roleName)

	if hasCMCRole {
		s.cfg.Log.Infof("The user %v already has the role %v.", clusterUser.GetName(), roleName)
		return noCertsReloaded, nil
	}

	s.cfg.Log.Infof("Adding the role %v to the user %v.", roleName, clusterUser.GetName())
	clusterUser.AddRole(roleName)
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	err = s.syncResourceUpdate(timeoutCtx, accessAndIdentity, clusterUser, func(ctx context.Context) error {
		return trace.Wrap(accessAndIdentity.UpdateUser(ctx, clusterUser),
			"updating user %v", clusterUser.GetName())
	})
	if err != nil {
		return noCertsReloaded, trace.Wrap(err)
	}

	s.cfg.Log.Info("Reissuing certs.")
	// ReissueUserCerts called with CertCacheDrop and a bogus access request ID in DropAccessRequests
	// allows us to refresh the role list in the certs without forcing the user to relogin.
	//
	// Sending bogus request IDs is not documented but it is covered by tests. Refreshing roles based
	// on the server state is necessary for tsh request drop to work.
	//
	// If passing bogus request IDs ever needs to be removed, then there are two options here:
	// * Pass a wildcard instead. This will break setups where people use access requests to make
	//   Connect My Computer work. Most users will probably not use access requests for that though.
	// * Invalidate the stored certs somehow to force the user to relogin. If Connect makes a request
	//   after role setup and [client.IsErrorResolvableWithRelogin] returns true for the error from
	//   the response, Connect will ask the user to relogin.
	//
	// TODO(ravicious): Expand [auth.ServerWithRoles.GenerateUserCerts] to support refreshing role
	// list without having to send a bogus request ID.
	err = certManager.ReissueUserCerts(ctx, client.CertCacheDrop, client.ReissueParams{
		RouteToCluster: cluster.Name,
		// AccessRequests:     cluster.GetLoggedInUser().ActiveRequests,
		DropAccessRequests: []string{fmt.Sprintf("bogus-request-id-%v", uuid.NewString())},
	})
	return RoleSetupResult{CertsReloaded: true}, trace.Wrap(err)
}

// syncResourceUpdate calls a function which updates the given resource and then waits until the
// cache propagates the change.
func (s *RoleSetup) syncResourceUpdate(ctx context.Context, accessAndIdentity AccessAndIdentity, resource types.Resource, updateFunc func(context.Context) error) error {
	watcher, err := accessAndIdentity.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{Kind: resource.GetKind()},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	// Wait for OpInit.
	select {
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "initializing watcher")
	case <-watcher.Done():
		return trace.Wrap(watcher.Error(), "initializing watcher")
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.Errorf("unexpected event type %q received from resource watcher", event.Type)
		}
	}

	err = updateFunc(ctx)
	if err != nil {
		return trace.Wrap(err, "calling update function")
	}

	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err(), "waiting for OpPut event for %v", resource.GetName())
		case <-watcher.Done():
			return trace.Wrap(watcher.Error(), "waiting for OpPut event for %v", resource.GetName())
		case event := <-watcher.Events():
			if event.Type != types.OpPut {
				continue
			}

			if event.Resource.GetKind() == resource.GetKind() && event.Resource.GetMetadata().Name == resource.GetName() {
				return nil
			}
		}
	}
}

// AccessAndIdentity represents [services.Access], [services.Identity] and [auth.Cache] methods used
// by [RoleSetup]. During a normal operation, [auth.ClientI] is passed as this interface.
type AccessAndIdentity interface {
	// See [services.Access.GetRole].
	GetRole(ctx context.Context, name string) (types.Role, error)
	// See [services.Access.UpsertRole].
	UpsertRole(context.Context, types.Role) error
	// See [auth.Cache.NewWatcher].
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)

	// See [services.Identity.GetUser].
	GetUser(name string, withSecrets bool) (types.User, error)
	// See [services.Identity.UpdateUser].
	UpdateUser(context.Context, types.User) error
}

// CertManager enables the usage of only select methods from [client.ProxyClient] so that there
// is no need to mock the whole ProxyClient interface in tests.
type CertManager interface {
	// See [client.ProxyClient.ReissueUserCerts].
	ReissueUserCerts(context.Context, client.CertCachePolicy, client.ReissueParams) error
}

type RoleSetupConfig struct {
	Log *logrus.Entry
}

func (c *RoleSetupConfig) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.NewEntry(logrus.StandardLogger()).WithField(trace.Component, "CMC role")
	}

	return nil
}
