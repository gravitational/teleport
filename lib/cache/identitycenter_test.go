// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
)

func newIdentityCenterAccount(id string) *identitycenterv1.Account {
	return &identitycenterv1.Account{
		Kind:    types.KindIdentityCenterAccount,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Spec: &identitycenterv1.AccountSpec{
			Id:  id,
			Arn: "arn:aws:sso:::permissionSet/ssoins-722326ecc902a06a/" + id,
		},
		Status: &identitycenterv1.AccountStatus{},
	}
}

// TestIdentityCenterAccount asserts that an Identity Center Account can be cached
func TestIdentityCenterAccount(t *testing.T) {
	t.Parallel()

	fixturePack := newTestPack(t, ForAuth)
	t.Cleanup(fixturePack.Close)

	testResources153(t, fixturePack, testFuncs[*identitycenterv1.Account]{
		newResource: func(s string) (*identitycenterv1.Account, error) {
			return newIdentityCenterAccount(s), nil
		},
		create: func(ctx context.Context, item *identitycenterv1.Account) error {
			_, err := fixturePack.identityCenter.CreateIdentityCenterAccount2(ctx, item)
			return trace.Wrap(err)
		},
		update: func(ctx context.Context, item *identitycenterv1.Account) error {
			_, err := fixturePack.identityCenter.UpdateIdentityCenterAccount2(ctx, item)
			return trace.Wrap(err)
		},
		list: fixturePack.identityCenter.ListIdentityCenterAccounts2,
		delete: func(ctx context.Context, id string) error {
			return trace.Wrap(fixturePack.identityCenter.DeleteIdentityCenterAccount(
				ctx, services.IdentityCenterAccountID(id)))
		},
		deleteAll: fixturePack.identityCenter.DeleteAllIdentityCenterAccounts,
		cacheList: fixturePack.cache.ListIdentityCenterAccounts,
		cacheGet:  fixturePack.cache.GetIdentityCenterAccount,
	}, withSkipPaginationTest())
}

func newIdentityCenterPrincipalAssignment(id string) *identitycenterv1.PrincipalAssignment {
	return &identitycenterv1.PrincipalAssignment{
		Kind:    types.KindIdentityCenterPrincipalAssignment,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Spec: &identitycenterv1.PrincipalAssignmentSpec{
			PrincipalType: identitycenterv1.PrincipalType_PRINCIPAL_TYPE_USER,
			PrincipalId:   id,
			ExternalId:    "ext_" + id,
		},
		Status: &identitycenterv1.PrincipalAssignmentStatus{
			ProvisioningState: identitycenterv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED,
		},
	}
}

// TestIdentityCenterPrincpialAssignment asserts that an Identity Center PrincipalAssignment can be cached
func TestIdentityCenterPrincipalAssignment(t *testing.T) {
	t.Parallel()
	fixturePack := newTestPack(t, ForAuth)
	t.Cleanup(fixturePack.Close)

	testResources153(t, fixturePack, testFuncs[*identitycenterv1.PrincipalAssignment]{
		newResource: func(s string) (*identitycenterv1.PrincipalAssignment, error) {
			return newIdentityCenterPrincipalAssignment(s), nil
		},
		create: func(ctx context.Context, item *identitycenterv1.PrincipalAssignment) error {
			_, err := fixturePack.identityCenter.CreatePrincipalAssignment(ctx, item)
			return trace.Wrap(err)
		},
		update: func(ctx context.Context, item *identitycenterv1.PrincipalAssignment) error {
			_, err := fixturePack.identityCenter.UpdatePrincipalAssignment(ctx, item)
			return trace.Wrap(err)
		},
		list: fixturePack.identityCenter.ListPrincipalAssignments2,
		delete: func(ctx context.Context, id string) error {
			return trace.Wrap(fixturePack.identityCenter.DeletePrincipalAssignment(ctx, services.PrincipalAssignmentID(id)))
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(fixturePack.identityCenter.DeleteAllPrincipalAssignments(ctx))
		},
		cacheList: fixturePack.cache.ListPrincipalAssignments,
		cacheGet: func(ctx context.Context, id string) (*identitycenterv1.PrincipalAssignment, error) {
			r, err := fixturePack.cache.identityCenterCache.GetPrincipalAssignment(
				ctx, services.PrincipalAssignmentID(id))
			return r, trace.Wrap(err)
		},
	}, withSkipPaginationTest())
}

func newIdentityCenterAccountAssignment(id string) *identitycenterv1.AccountAssignment {
	return &identitycenterv1.AccountAssignment{
		Kind:    types.KindIdentityCenterAccountAssignment,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Spec: &identitycenterv1.AccountAssignmentSpec{
			Display:       "account " + id,
			PermissionSet: &identitycenterv1.PermissionSetInfo{},
			AccountName:   id,
			AccountId:     id,
		},
	}
}

// TestIdentityCenterAccountAssignment asserts that an Identity Center
// AccountAssignment can be cached
func TestIdentityCenterAccountAssignment(t *testing.T) {
	t.Parallel()
	fixturePack := newTestPack(t, ForAuth)
	t.Cleanup(fixturePack.Close)

	testResources153(t, fixturePack, testFuncs[*identitycenterv1.AccountAssignment]{
		newResource: func(s string) (*identitycenterv1.AccountAssignment, error) {
			return newIdentityCenterAccountAssignment(s), nil
		},
		create: func(ctx context.Context, item *identitycenterv1.AccountAssignment) error {
			_, err := fixturePack.identityCenter.CreateIdentityCenterAccountAssignment(ctx, item)
			return trace.Wrap(err)
		},
		update: func(ctx context.Context, item *identitycenterv1.AccountAssignment) error {
			_, err := fixturePack.identityCenter.UpdateIdentityCenterAccountAssignment(ctx, item)
			return trace.Wrap(err)
		},
		list: fixturePack.identityCenter.ListIdentityCenterAccountAssignments,
		delete: func(ctx context.Context, id string) error {
			return trace.Wrap(fixturePack.identityCenter.DeleteAccountAssignment(ctx, services.IdentityCenterAccountAssignmentID(id)))
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(fixturePack.identityCenter.DeleteAllAccountAssignments(ctx))
		},
		cacheList: fixturePack.cache.ListIdentityCenterAccountAssignments,
		cacheGet: func(ctx context.Context, id string) (*identitycenterv1.AccountAssignment, error) {
			r, err := fixturePack.cache.GetAccountAssignment(ctx, services.IdentityCenterAccountAssignmentID(id))
			return r.AccountAssignment, trace.Wrap(err)
		},
	}, withSkipPaginationTest())
}

func TestIdentityCenterCacheCompleteness(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	p := newTestPackWithoutCache(t)
	t.Cleanup(p.Close)

	accounts := make([]string, 0, 2)
	accountAssignments := make([]string, 0, 2)
	principalAssignments := make([]string, 0, 2)
	var err error
	for i := range 2 {
		aName := "account" + strconv.Itoa(i)
		_, err = p.identityCenter.CreateIdentityCenterAccount(ctx, newIdentityCenterAccount(aName))
		require.NoError(t, err)
		accounts = append(accounts, aName)

		aaName := "account_assignment" + strconv.Itoa(i)
		_, err = p.identityCenter.CreateIdentityCenterAccountAssignment(ctx, newIdentityCenterAccountAssignment(aaName))
		require.NoError(t, err)
		accountAssignments = append(accountAssignments, aaName)

		paName := "principal_assignment" + strconv.Itoa(i)
		_, err = p.identityCenter.CreatePrincipalAssignment(ctx, newIdentityCenterPrincipalAssignment(paName))
		require.NoError(t, err)
		principalAssignments = append(principalAssignments, paName)
	}

	p.cacheBackend, err = memory.New(
		memory.Config{
			Context: ctx,
			Mirror:  true,
		})
	require.NoError(t, err)
	p.cache, err = New(ForAuth(Config{
		Context:                 ctx,
		Backend:                 p.cacheBackend,
		Events:                  p.eventsS,
		ClusterConfig:           p.clusterConfigS,
		Provisioner:             p.provisionerS,
		Trust:                   p.trustS,
		Users:                   p.usersS,
		Access:                  p.accessS,
		DynamicAccess:           p.dynamicAccessS,
		Presence:                p.presenceS,
		AppSession:              p.appSessionS,
		WebSession:              p.webSessionS,
		SnowflakeSession:        p.snowflakeSessionS,
		SAMLIdPSession:          p.samlIdPSessionsS,
		WebToken:                p.webTokenS,
		Restrictions:            p.restrictions,
		Apps:                    p.apps,
		Kubernetes:              p.kubernetes,
		DatabaseServices:        p.databaseServices,
		Databases:               p.databases,
		WindowsDesktops:         p.windowsDesktops,
		DynamicWindowsDesktops:  p.dynamicWindowsDesktops,
		SAMLIdPServiceProviders: p.samlIDPServiceProviders,
		UserGroups:              p.userGroups,
		Okta:                    p.okta,
		Integrations:            p.integrations,
		UserTasks:               p.userTasks,
		DiscoveryConfigs:        p.discoveryConfigs,
		UserLoginStates:         p.userLoginStates,
		SecReports:              p.secReports,
		AccessLists:             p.accessLists,
		KubeWaitingContainers:   p.kubeWaitingContainers,
		Notifications:           p.notifications,
		AccessMonitoringRules:   p.accessMonitoringRules,
		CrownJewels:             p.crownJewels,
		DatabaseObjects:         p.databaseObjects,
		SPIFFEFederations:       p.spiffeFederations,
		StaticHostUsers:         p.staticHostUsers,
		AutoUpdateService:       p.autoUpdateService,
		ProvisioningStates:      p.provisioningStates,
		WorkloadIdentity:        p.workloadIdentity,
		MaxRetryPeriod:          200 * time.Millisecond,
		IdentityCenter:          p.identityCenter,
		PluginStaticCredentials: p.pluginStaticCredentials,
		EventsC:                 p.eventsC,
		GitServers:              p.gitServers,
		BotInstanceService:      p.botInstanceService,
		Plugin:                  p.plugin,
	}))
	require.NoError(t, err)

	accountsOut, _, err := p.cache.ListIdentityCenterAccounts(ctx, 0, "")
	require.NoError(t, err)
	require.ElementsMatch(t, accounts, aNames(accountsOut))

	assignmentsOut, _, err := p.cache.ListIdentityCenterAccountAssignments(ctx, 0, "")
	require.NoError(t, err)
	require.ElementsMatch(t, accountAssignments, aaNames(assignmentsOut))

	pAssignmentsOut, _, err := p.cache.ListPrincipalAssignments(ctx, 0, "")
	require.NoError(t, err)
	require.ElementsMatch(t, principalAssignments, paNames(pAssignmentsOut))

	require.NoError(t, p.cache.Close())
	require.NoError(t, p.cacheBackend.Close())
}

func aNames(in []*identitycenterv1.Account) (out []string) {
	for _, i := range in {
		out = append(out, i.GetMetadata().GetName())
	}
	return
}

func aaNames(in []*identitycenterv1.AccountAssignment) (out []string) {
	for _, i := range in {
		out = append(out, i.GetMetadata().GetName())
	}
	return
}

func paNames(in []*identitycenterv1.PrincipalAssignment) (out []string) {
	for _, i := range in {
		out = append(out, i.GetMetadata().GetName())
	}
	return
}
