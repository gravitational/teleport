/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package local

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestUpdateCertAuthorityCondActs(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// setup closure creates our initial state and returns its components
	setup := func(active bool) (types.TrustedCluster, types.CertAuthority, *CA) {
		bk, err := memory.New(memory.Config{})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, bk.Close()) })
		service := NewCAService(bk)

		tc, err := types.NewTrustedCluster("tc", types.TrustedClusterSpecV2{
			Enabled:              active,
			Roles:                []string{"rrr"},
			Token:                "xxx",
			ProxyAddress:         "xxx",
			ReverseTunnelAddress: "xxx",
		})
		require.NoError(t, err)

		ca := newCertAuthority(t, types.HostCA, "tc")
		revision, err := service.CreateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
		require.NoError(t, err)
		tc.SetRevision(revision)
		ca.SetRevision(revision)
		return tc, ca, service
	}

	// putCA is a helper for injecting a CA into the backend, bypassing atomic condition protections
	putCA := func(ctx context.Context, service *CA, ca types.CertAuthority, active bool) {
		key := activeCAKey(ca.GetID())
		if !active {
			key = inactiveCAKey(ca.GetID())
		}
		item, err := caToItem(key, ca)
		require.NoError(t, err)
		_, err = service.Put(ctx, item)
		require.NoError(t, err)
	}

	// delCA is a helper for deleting a CA from the backend, bypassing atomic condition protections
	delCA := func(ctx context.Context, service *CA, ca types.CertAuthority, active bool) {
		key := activeCAKey(ca.GetID())
		if !active {
			key = inactiveCAKey(ca.GetID())
		}
		require.NoError(t, service.Delete(ctx, key))
	}

	// -- update active in place ---
	tc, ca, service := setup(true /* active */)

	// verify basic update works
	tc.SetRoles([]string{"rrr", "zzz"})
	revision, err := service.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.NoError(t, err)
	tc.SetRevision(revision)
	ca.SetRevision(revision)

	gotTC, err := service.GetTrustedCluster(ctx, tc.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(tc, gotTC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	_, err = service.GetCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)
	_, err = service.GetInactiveCertAuthority(ctx, ca.GetID(), true)
	require.True(t, trace.IsNotFound(err), "err=%v", err)

	// verify that an inactive CA doesn't prevent update
	putCA(ctx, service, ca, false /* inactive */)
	tc.SetRoles([]string{"rrr", "zzz", "aaa"})
	revision, err = service.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.NoError(t, err)
	tc.SetRevision(revision)
	ca.SetRevision(revision)

	gotTC, err = service.GetTrustedCluster(ctx, tc.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(tc, gotTC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	_, err = service.GetCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)
	_, err = service.GetInactiveCertAuthority(ctx, ca.GetID(), true)
	require.True(t, trace.IsNotFound(err), "err=%v", err)

	// verify that concurrent update of the active CA causes update to fail
	putCA(ctx, service, ca, true /* active */)
	tc.SetRoles([]string{"rrr", "zzz", "aaa", "bbb"})
	_, err = service.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.True(t, trace.IsCompareFailed(err), "err=%v", err)

	// --- update inactive in place ---
	tc, ca, service = setup(false /* inactive */)

	// verify basic update works
	tc.SetRoles([]string{"rrr", "zzz"})
	revision, err = service.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.NoError(t, err)
	tc.SetRevision(revision)
	ca.SetRevision(revision)

	gotTC, err = service.GetTrustedCluster(ctx, tc.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(tc, gotTC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	_, err = service.GetCertAuthority(ctx, ca.GetID(), true)
	require.True(t, trace.IsNotFound(err), "err=%v", err)
	_, err = service.GetInactiveCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)

	// verify that an active CA prevents update
	putCA(ctx, service, ca, true /* active */)
	tc.SetRoles([]string{"rrr", "zzz", "aaa"})
	_, err = service.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.True(t, trace.IsCompareFailed(err), "err=%v", err)
	delCA(ctx, service, ca, true /* active */)

	// verify that concurrent update of the inactive CA causes update to fail
	putCA(ctx, service, ca, false /* inactive */)
	tc.SetRoles([]string{"rrr", "zzz", "aaa", "bbb"})
	_, err = service.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.True(t, trace.IsCompareFailed(err), "err=%v", err)

	// --- activate/deactivate ---
	tc, ca, service = setup(false /* inactive */)

	// verify that activating works
	tc.SetEnabled(true)
	revision, err = service.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.NoError(t, err)
	tc.SetRevision(revision)
	ca.SetRevision(revision)

	gotTC, err = service.GetTrustedCluster(ctx, tc.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(tc, gotTC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	_, err = service.GetCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)
	_, err = service.GetInactiveCertAuthority(ctx, ca.GetID(), true)
	require.True(t, trace.IsNotFound(err), "err=%v", err)

	// verify that deactivating works
	tc.SetEnabled(false)
	revision, err = service.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.NoError(t, err)
	tc.SetRevision(revision)
	ca.SetRevision(revision)

	gotTC, err = service.GetTrustedCluster(ctx, tc.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(tc, gotTC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	_, err = service.GetCertAuthority(ctx, ca.GetID(), true)
	require.True(t, trace.IsNotFound(err), "err=%v", err)
	_, err = service.GetInactiveCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)

	// verify that an active CA conflicts with activation
	putCA(ctx, service, ca, true /* active */)
	tc.SetEnabled(true)
	_, err = service.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.True(t, trace.IsCompareFailed(err), "err=%v", err)
	delCA(ctx, service, ca, true /* active */)

	// activation should work after deleting conlicting CA
	revision, err = service.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.NoError(t, err)
	tc.SetRevision(revision)
	ca.SetRevision(revision)

	gotTC, err = service.GetTrustedCluster(ctx, tc.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(tc, gotTC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	_, err = service.GetCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)
	_, err = service.GetInactiveCertAuthority(ctx, ca.GetID(), true)
	require.True(t, trace.IsNotFound(err), "err=%v", err)

	// verify that deactivation works even if there is an inaactive CA present
	putCA(ctx, service, ca, false /* inactive */)
	tc.SetEnabled(false)
	revision, err = service.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.NoError(t, err)
	tc.SetRevision(revision)
	ca.SetRevision(revision)

	gotTC, err = service.GetTrustedCluster(ctx, tc.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(tc, gotTC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	_, err = service.GetCertAuthority(ctx, ca.GetID(), true)
	require.True(t, trace.IsNotFound(err), "err=%v", err)
	_, err = service.GetInactiveCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)
}

func TestRemoteClusterCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	trustService := NewCAService(bk)
	clock := clockwork.NewFakeClockAt(time.Now())

	originalLabels := map[string]string{
		"a": "b",
		"c": "d",
	}

	rc, err := types.NewRemoteCluster("foo")
	require.NoError(t, err)
	rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	rc.SetLastHeartbeat(clock.Now())
	rc.SetMetadata(types.Metadata{
		Name:   "foo",
		Labels: originalLabels,
	})

	src, err := types.NewRemoteCluster("bar")
	require.NoError(t, err)
	src.SetConnectionStatus(teleport.RemoteClusterStatusOnline)
	src.SetLastHeartbeat(clock.Now().Add(-time.Hour))

	// set up fake CAs for the remote clusters
	ca := newCertAuthority(t, types.HostCA, "foo")
	sca := newCertAuthority(t, types.HostCA, "bar")

	// create remote cluster
	revision, err := trustService.CreateRemoteClusterInternal(ctx, rc, []types.CertAuthority{ca})
	require.NoError(t, err)
	rc.SetRevision(revision)
	ca.SetRevision(revision)

	_, err = trustService.CreateRemoteClusterInternal(ctx, rc, []types.CertAuthority{ca})
	require.True(t, trace.IsAlreadyExists(err), "err=%v", err)

	revision, err = trustService.CreateRemoteClusterInternal(ctx, src, []types.CertAuthority{sca})
	require.NoError(t, err)
	src.SetRevision(revision)
	sca.SetRevision(revision)

	// get remote cluster make sure it's correct
	gotRC, err := trustService.GetRemoteCluster(ctx, "foo")
	require.NoError(t, err)
	require.Equal(t, "foo", gotRC.GetName())
	require.Equal(t, teleport.RemoteClusterStatusOffline, gotRC.GetConnectionStatus())
	require.Equal(t, clock.Now().Nanosecond(), gotRC.GetLastHeartbeat().Nanosecond())
	require.Equal(t, originalLabels, gotRC.GetMetadata().Labels)

	// get remote cluster CA make sure it's correct
	gotCA, err := trustService.GetCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(ca, gotCA, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	rc = gotRC
	updatedLabels := map[string]string{
		"e": "f",
		"g": "h",
	}

	// update remote clusters
	rc.SetConnectionStatus(teleport.RemoteClusterStatusOnline)
	rc.SetLastHeartbeat(clock.Now().Add(time.Hour))
	meta := rc.GetMetadata()
	meta.Labels = updatedLabels
	rc.SetMetadata(meta)
	gotRC, err = trustService.UpdateRemoteCluster(ctx, rc)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rc, gotRC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	src.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	src.SetLastHeartbeat(clock.Now())
	gotSRC, err := trustService.UpdateRemoteCluster(ctx, src)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(src, gotSRC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// get remote cluster make sure it's correct
	gotRC, err = trustService.GetRemoteCluster(ctx, "foo")
	require.NoError(t, err)
	require.Equal(t, "foo", gotRC.GetName())
	require.Equal(t, teleport.RemoteClusterStatusOnline, gotRC.GetConnectionStatus())
	require.Equal(t, clock.Now().Add(time.Hour).Nanosecond(), gotRC.GetLastHeartbeat().Nanosecond())
	require.Equal(t, updatedLabels, gotRC.GetMetadata().Labels)

	gotRC, err = trustService.GetRemoteCluster(ctx, "bar")
	require.NoError(t, err)
	require.Equal(t, "bar", gotRC.GetName())
	require.Equal(t, teleport.RemoteClusterStatusOffline, gotRC.GetConnectionStatus())
	require.Equal(t, clock.Now().Nanosecond(), gotRC.GetLastHeartbeat().Nanosecond())

	// get all clusters
	allRC, err := trustService.GetRemoteClusters(ctx)
	require.NoError(t, err)
	require.Len(t, allRC, 2)

	// delete cluster
	err = trustService.DeleteRemoteClusterInternal(ctx, "foo", []types.CertAuthID{ca.GetID()})
	require.NoError(t, err)

	// make sure it's really gone
	_, err = trustService.GetRemoteCluster(ctx, "foo")
	require.True(t, trace.IsNotFound(err))
	_, err = trustService.GetCertAuthority(ctx, ca.GetID(), true)
	require.True(t, trace.IsNotFound(err))

	// make sure we can't create trusted clusters with the same name as an extant remote cluster
	tc, err := types.NewTrustedCluster("bar", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{"bar", "baz"},
		Token:                "qux",
		ProxyAddress:         "quux",
		ReverseTunnelAddress: "quuz",
	})
	require.NoError(t, err)
	_, err = trustService.CreateTrustedCluster(ctx, tc, nil)
	require.True(t, trace.IsBadParameter(err), "err=%v", err)
}

func TestPresenceService_PatchRemoteCluster(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	trustService := NewCAService(bk)

	rc, err := types.NewRemoteCluster("bar")
	require.NoError(t, err)
	rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	_, err = trustService.CreateRemoteCluster(ctx, rc)
	require.NoError(t, err)

	updatedRC, err := trustService.PatchRemoteCluster(
		ctx,
		rc.GetName(),
		func(rc types.RemoteCluster) (types.RemoteCluster, error) {
			require.Equal(t, teleport.RemoteClusterStatusOffline, rc.GetConnectionStatus())
			rc.SetConnectionStatus(teleport.RemoteClusterStatusOnline)
			return rc, nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, teleport.RemoteClusterStatusOnline, updatedRC.GetConnectionStatus())

	// Ensure this was persisted.
	fetchedRC, err := trustService.GetRemoteCluster(ctx, rc.GetName())
	require.NoError(t, err)
	require.Equal(t, teleport.RemoteClusterStatusOnline, fetchedRC.GetConnectionStatus())
	// Ensure other fields unchanged
	require.Empty(t,
		cmp.Diff(
			rc,
			fetchedRC,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
			cmpopts.IgnoreFields(types.RemoteClusterStatusV3{}, "Connection"),
		),
	)

	// Ensure that name cannot be updated
	_, err = trustService.PatchRemoteCluster(
		ctx,
		rc.GetName(),
		func(rc types.RemoteCluster) (types.RemoteCluster, error) {
			rc.SetName("baz")
			return rc, nil
		},
	)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "metadata.name: cannot be patched")

	// Ensure that revision cannot be updated
	_, err = trustService.PatchRemoteCluster(
		ctx,
		rc.GetName(),
		func(rc types.RemoteCluster) (types.RemoteCluster, error) {
			rc.SetRevision("baz")
			return rc, nil
		},
	)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "metadata.revision: cannot be patched")
}

func TestPresenceService_ListRemoteClusters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	trustService := NewCAService(bk)

	// With no resources, we should not get an error but we should get an empty
	// token and an empty slice.
	rcs, pageToken, err := trustService.ListRemoteClusters(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, pageToken)
	require.Empty(t, rcs)

	// Create a few remote clusters
	for i := 0; i < 10; i++ {
		rc, err := types.NewRemoteCluster(fmt.Sprintf("rc-%d", i))
		require.NoError(t, err)
		_, err = trustService.CreateRemoteCluster(ctx, rc)
		require.NoError(t, err)
	}

	// Check limit behaves
	rcs, pageToken, err = trustService.ListRemoteClusters(ctx, 1, "")
	require.NoError(t, err)
	require.NotEmpty(t, pageToken)
	require.Len(t, rcs, 1)

	// Iterate through all pages with a low limit to ensure that pageToken
	// behaves correctly.
	rcs = []types.RemoteCluster{}
	pageToken = ""
	for i := 0; i < 10; i++ {
		var got []types.RemoteCluster
		got, pageToken, err = trustService.ListRemoteClusters(ctx, 1, pageToken)
		require.NoError(t, err)
		if i == 9 {
			// For the final page, we should not get a page token
			require.Empty(t, pageToken)
		} else {
			require.NotEmpty(t, pageToken)
		}
		require.Len(t, got, 1)
		rcs = append(rcs, got...)
	}
	require.Len(t, rcs, 10)

	// Check that with a higher limit, we get all resources
	rcs, pageToken, err = trustService.ListRemoteClusters(ctx, 20, "")
	require.NoError(t, err)
	require.Empty(t, pageToken)
	require.Len(t, rcs, 10)
}

func TestTrustedClusterCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := lite.New(ctx, backend.Params{"path": t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	trustService := NewCAService(bk)

	tc, err := types.NewTrustedCluster("foo", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{"bar", "baz"},
		Token:                "qux",
		ProxyAddress:         "quux",
		ReverseTunnelAddress: "quuz",
	})
	require.NoError(t, err)

	// we just insert this one for get all
	stc, err := types.NewTrustedCluster("bar", types.TrustedClusterSpecV2{
		Enabled:              false,
		Roles:                []string{"baz", "aux"},
		Token:                "quux",
		ProxyAddress:         "quuz",
		ReverseTunnelAddress: "corge",
	})
	require.NoError(t, err)

	ca := newCertAuthority(t, types.HostCA, "foo")
	sca := newCertAuthority(t, types.HostCA, "bar")

	// create trusted clusters
	_, err = trustService.CreateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.NoError(t, err)
	_, err = trustService.CreateTrustedCluster(ctx, stc, []types.CertAuthority{sca})
	require.NoError(t, err)

	// get trusted cluster make sure it's correct
	gotTC, err := trustService.GetTrustedCluster(ctx, "foo")
	require.NoError(t, err)
	require.Equal(t, "foo", gotTC.GetName())
	require.True(t, gotTC.GetEnabled())
	require.EqualValues(t, []string{"bar", "baz"}, gotTC.GetRoles())
	require.Equal(t, "qux", gotTC.GetToken())
	require.Equal(t, "quux", gotTC.GetProxyAddress())
	require.Equal(t, "quuz", gotTC.GetReverseTunnelAddress())

	// get trusted cluster CA make sure it's correct
	gotCA, err := trustService.GetCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(ca, gotCA, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// get all clusters
	allTC, err := trustService.GetTrustedClusters(ctx)
	require.NoError(t, err)
	require.Len(t, allTC, 2)

	// verify that enabling/disabling correctly shows/hides CAs
	tc.SetEnabled(false)
	tc.SetRevision(gotTC.GetRevision())
	ca.SetRevision(gotCA.GetRevision())
	revision, err := trustService.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.NoError(t, err)
	_, err = trustService.GetCertAuthority(ctx, ca.GetID(), true)
	require.True(t, trace.IsNotFound(err), "err=%v", err)

	_, err = trustService.GetInactiveCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)

	tc.SetEnabled(true)
	tc.SetRevision(revision)
	ca.SetRevision(revision)
	_, err = trustService.UpdateTrustedCluster(ctx, tc, []types.CertAuthority{ca})
	require.NoError(t, err)

	_, err = trustService.GetCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)
	_, err = trustService.GetInactiveCertAuthority(ctx, ca.GetID(), true)
	require.True(t, trace.IsNotFound(err), "err=%v", err)

	// delete cluster
	err = trustService.DeleteTrustedClusterInternal(ctx, "foo", []types.CertAuthID{ca.GetID()})
	require.NoError(t, err)

	// make sure it's really gone
	_, err = trustService.GetTrustedCluster(ctx, "foo")
	require.True(t, trace.IsNotFound(err), "err=%v", err)
	_, err = trustService.GetCertAuthority(ctx, ca.GetID(), true)
	require.True(t, trace.IsNotFound(err), "err=%v", err)

	// make sure we can't create remote clusters with the same name as an extant trusted cluster
	rc, err := types.NewRemoteCluster("bar")
	require.NoError(t, err)
	_, err = trustService.CreateRemoteCluster(ctx, rc)
	require.True(t, trace.IsBadParameter(err), "err=%v", err)
}

func newCertAuthority(t *testing.T, caType types.CertAuthType, domain string) types.CertAuthority {
	t.Helper()

	ta := testauthority.New()
	priv, pub, err := ta.GenerateKeyPair()
	require.NoError(t, err)

	key, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: domain}, nil, time.Hour)
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caType,
		ClusterName: domain,
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{
				PrivateKey:     priv,
				PrivateKeyType: types.PrivateKeyType_RAW,
				PublicKey:      pub,
			}},
			TLS: []*types.TLSKeyPair{{
				Cert: cert,
				Key:  key,
			}},
			JWT: []*types.JWTKeyPair{{
				PublicKey:      pub,
				PrivateKey:     priv,
				PrivateKeyType: types.PrivateKeyType_RAW,
			}},
		},
	})
	require.NoError(t, err)

	return ca
}
