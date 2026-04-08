/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package resources_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var lockSpec = types.LockSpecV2{
	Target: types.LockTarget{
		User: "john",
	},
}

type lockTestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithoutLabelsAdapter[types.Lock]
}

func (g *lockTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *lockTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *lockTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	lock := &types.LockV2{
		Kind:    types.KindLock,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: name,
		},
		Spec: lockSpec,
	}
	lock.SetOrigin(types.OriginKubernetes)
	return trace.Wrap(g.setup.TeleportClient.UpsertLock(ctx, lock))
}

func (g *lockTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.Lock, error) {
	return g.setup.TeleportClient.GetLock(ctx, name)
}

func (g *lockTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteLock(ctx, name))
}

func (g *lockTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	lock := &resourcesv1.TeleportLockV2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportLockV2Spec(lockSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, lock))
}

func (g *lockTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	lock := &resourcesv1.TeleportLockV2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return g.setup.K8sClient.Delete(ctx, lock)
}

func (g *lockTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportLockV2, error) {
	lock := &resourcesv1.TeleportLockV2{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, lock)
	return lock, trace.Wrap(err)
}

func (g *lockTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	lock, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	lock.Spec.Target = types.LockTarget{User: "john"}
	return trace.Wrap(g.setup.K8sClient.Update(ctx, lock))
}

func (g *lockTestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.Lock, kubeResource *resourcesv1.TeleportLockV2) (bool, string) {
	ignoreServerSideDefaults := []cmp.Option{
		cmpopts.IgnoreFields(types.LockSpecV2{}, "CreatedAt"),
		cmpopts.IgnoreFields(types.LockSpecV2{}, "CreatedBy"),
	}
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions(ignoreServerSideDefaults...)...)
	return diff == "", diff
}

func TestLockCreation(t *testing.T) {
	test := &lockTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(t, resources.NewLockV2Reconciler, test)
}

func TestLockDeletion(t *testing.T) {
	test := &lockTestingPrimitives{}
	testlib.ResourceDeletionSynchronousTest(t, resources.NewLockV2Reconciler, test)
}

func TestLockDeletionDrift(t *testing.T) {
	test := &lockTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(t, resources.NewLockV2Reconciler, test)
}

func TestLockUpdate(t *testing.T) {
	test := &lockTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(t, resources.NewLockV2Reconciler, test)
}

// TestLockMutateExisting tests CreatedAt and CreatedBy fields are persisted
// across multiple reconciliations.
func TestLockMutateExisting(t *testing.T) {
	ctx := t.Context()
	setup := testlib.SetupFakeKubeTestEnv(t)

	name := validRandomResourceName("lock-")
	createdBy := validRandomResourceName("operator-test")
	now := time.Now()

	// The lock is created in Teleport.
	tLock, err := types.NewLock(name, lockSpec)
	tLock.SetOrigin(types.OriginKubernetes)
	tLock.SetCreatedAt(now)
	tLock.SetCreatedBy(createdBy)

	require.NoError(t, setup.TeleportClient.UpsertLock(ctx, tLock))

	// Wait for the lock to enter the cache.
	fastEventually(t, func() bool {
		tLock, err = setup.TeleportClient.GetLock(ctx, tLock.GetName())
		// Fail if we see an unknown error
		if err != nil {
			require.True(t, trace.IsNotFound(err))
		}
		return !trace.IsNotFound(err)
	})
	require.NoError(t, err)

	// The lock is created in K8S.
	k8sLock := &resourcesv1.TeleportLockV2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportLockV2Spec(lockSpec),
	}
	require.NoError(t, setup.K8sClient.Create(ctx, k8sLock))

	reconciler, err := resources.NewLockV2Reconciler(setup.K8sClient, setup.TeleportClient)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: apimachinerytypes.NamespacedName{
			Namespace: setup.Namespace.Name,
			Name:      name,
		},
	}
	// First reconciliation should set the finalizer and exit.
	_, err = reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	// Second reconciliation should create the Teleport resource.
	// In a real cluster we should receive the event of our own finalizer change
	// and this wakes us for a second round.
	_, err = reconciler.Reconcile(ctx, req)
	require.NoError(t, err)

	// Check if CreatedAt and CreatedBy are preserved in Teleport.
	fastEventually(t, func() bool {
		lock, err := setup.TeleportClient.GetLock(ctx, name)
		if err != nil {
			return false
		}
		return lock.CreatedAt().Equal(now) && lock.CreatedBy() == createdBy
	})
}
