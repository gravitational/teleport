/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/secretlookup"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

type trustedClusterV2TestingPrimitives struct {
	// remoteCluster specifies the remote trusted cluster instance.
	remoteCluster *helpers.TeleInstance
	// trustedClusterSpec specifies the trusted cluster specs.
	trustedClusterSpec types.TrustedClusterSpecV2

	setup *testSetup
	reconcilers.ResourceWithoutLabelsAdapter[types.TrustedCluster]
}

func (r *trustedClusterV2TestingPrimitives) Init(setup *testSetup) {
	r.setup = setup
}

func (r *trustedClusterV2TestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (r *trustedClusterV2TestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	trustedCluster, err := types.NewTrustedCluster(name, r.trustedClusterSpec)
	if err != nil {
		return trace.Wrap(err)
	}
	trustedCluster.SetOrigin(types.OriginKubernetes)
	_, err = r.setup.TeleportClient.CreateTrustedCluster(ctx, trustedCluster)
	return trace.Wrap(err)
}

func (r *trustedClusterV2TestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.TrustedCluster, error) {
	return r.setup.TeleportClient.GetTrustedCluster(ctx, name)
}

func (r *trustedClusterV2TestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(r.setup.TeleportClient.DeleteTrustedCluster(ctx, name))
}

func (r *trustedClusterV2TestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	trustedCluster := &resourcesv1.TeleportTrustedClusterV2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportTrustedClusterV2Spec(r.trustedClusterSpec),
	}
	return trace.Wrap(r.setup.K8sClient.Create(ctx, trustedCluster))
}

func (r *trustedClusterV2TestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	trustedCluster := &resourcesv1.TeleportTrustedClusterV2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.setup.Namespace.Name,
		},
	}
	return trace.Wrap(r.setup.K8sClient.Delete(ctx, trustedCluster))
}

func (r *trustedClusterV2TestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportTrustedClusterV2, error) {
	trustedCluster := &resourcesv1.TeleportTrustedClusterV2{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: r.setup.Namespace.Name,
	}
	err := r.setup.K8sClient.Get(ctx, obj, trustedCluster)
	return trustedCluster, trace.Wrap(err)
}

func (r *trustedClusterV2TestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	trustedCluster, err := r.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	trustedCluster.Spec.RoleMap[0] = types.RoleMapping{
		Remote: "remote-admin",
		Local:  []string{"local-dev"},
	}
	return trace.Wrap(r.setup.K8sClient.Update(ctx, trustedCluster))
}

func (r *trustedClusterV2TestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.TrustedCluster, kubeResource *resourcesv1.TeleportTrustedClusterV2) (bool, string) {
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions()...)
	return diff == "", diff
}

// setupTest initializes a remote cluster for testing trusted clusters.
func (r *trustedClusterV2TestingPrimitives) setupTest(t *testing.T, clusterName string) {
	ctx := context.Background()

	remoteCluster := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: clusterName,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      utils.NewSlogLoggerForTests(),
	})
	r.remoteCluster = remoteCluster

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.Version = "v2"

	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(false) })

	require.NoError(t, remoteCluster.CreateEx(t, nil, rcConf))
	require.NoError(t, remoteCluster.Start())
	t.Cleanup(func() { require.NoError(t, remoteCluster.StopAll()) })

	// Create trusted cluster join token
	token := "secret_token"
	tokenResource, err := types.NewProvisionToken(token, []types.SystemRole{types.RoleTrustedCluster}, time.Time{})
	require.NoError(t, err)
	remoteCluster.Process.GetAuthServer().UpsertToken(ctx, tokenResource)

	// Create required role
	localDev := "local-dev"
	require.NoError(t, teleportCreateDummyRole(ctx, localDev, r.setup.TeleportClient))

	r.trustedClusterSpec = types.TrustedClusterSpecV2{
		Enabled:              true,
		Token:                token,
		ProxyAddress:         remoteCluster.Web,
		ReverseTunnelAddress: remoteCluster.ReverseTunnel,
		RoleMap: []types.RoleMapping{
			{
				Remote: "remote-dev",
				Local:  []string{localDev},
			},
		},
	}
}

func TestTrustedClusterV2Creation(t *testing.T) {
	test := &trustedClusterV2TestingPrimitives{}
	setup := testlib.SetupTestEnv(t)
	test.Init(setup)
	ctx := context.Background()

	resourceName := "remote.example.com"
	test.setupTest(t, resourceName)

	require.NoError(t, test.CreateKubernetesResource(ctx, resourceName))

	var resource types.TrustedCluster
	var err error
	testlib.FastEventually(t, func() bool {
		resource, err = test.GetTeleportResource(ctx, resourceName)
		return !trace.IsNotFound(err)
	})
	require.NoError(t, err)
	require.Equal(t, resourceName, test.GetResourceName(resource))
	require.Equal(t, types.OriginKubernetes, test.GetResourceOrigin(resource))

	err = test.DeleteKubernetesResource(ctx, resourceName)
	require.NoError(t, err)

	testlib.FastEventually(t, func() bool {
		_, err = test.GetTeleportResource(ctx, resourceName)
		return trace.IsNotFound(err)
	})
}

func TestTrustedClusterV2DeletionDrift(t *testing.T) {
	test := &trustedClusterV2TestingPrimitives{}
	setup := testlib.SetupTestEnv(t)
	test.Init(setup)
	ctx := context.Background()

	resourceName := "remote.example.com"
	test.setupTest(t, resourceName)

	require.NoError(t, test.CreateKubernetesResource(ctx, resourceName))

	var resource types.TrustedCluster
	var err error
	testlib.FastEventually(t, func() bool {
		resource, err = test.GetTeleportResource(ctx, resourceName)
		return !trace.IsNotFound(err)
	})
	require.NoError(t, err)
	require.Equal(t, resourceName, test.GetResourceName(resource))
	require.Equal(t, types.OriginKubernetes, test.GetResourceOrigin(resource))

	// We cause a drift by altering the Teleport resource.
	// To make sure the operator does not reconcile while we're finished we suspend the operator
	setup.StopKubernetesOperator()

	err = test.DeleteTeleportResource(ctx, resourceName)
	require.NoError(t, err)
	testlib.FastEventually(t, func() bool {
		_, err = test.GetTeleportResource(ctx, resourceName)
		return trace.IsNotFound(err)
	})

	// We flag the resource for deletion in Kubernetes (it won't be fully removed until the operator has processed it and removed the finalizer)
	err = test.DeleteKubernetesResource(ctx, resourceName)
	require.NoError(t, err)

	// Test section: We resume the operator, it should reconcile and recover from the drift
	setup.StartKubernetesOperator(t)

	// The operator should handle the failed Teleport deletion gracefully and unlock the Kubernetes resource deletion
	testlib.FastEventually(t, func() bool {
		_, err = test.GetKubernetesResource(ctx, resourceName)
		return kerrors.IsNotFound(err)
	})
}

func TestTrustedClusterV2Update(t *testing.T) {
	test := &trustedClusterV2TestingPrimitives{}
	setup := testlib.SetupTestEnv(t)
	test.Init(setup)
	ctx := context.Background()

	resourceName := "remote.example.com"
	test.setupTest(t, resourceName)

	// The resource is created in Teleport
	require.NoError(t, test.CreateTeleportResource(ctx, resourceName))

	// The resource is created in Kubernetes, with at least a field altered
	require.NoError(t, test.CreateKubernetesResource(ctx, resourceName))

	// Check the resource was updated in Teleport
	testlib.FastEventuallyWithT(t, func(c *assert.CollectT) {
		tResource, err := test.GetTeleportResource(ctx, resourceName)
		require.NoError(c, err)

		kubeResource, err := test.GetKubernetesResource(ctx, resourceName)
		require.NoError(c, err)

		// Kubernetes and Teleport resources are in-sync
		equal, diff := test.CompareTeleportAndKubernetesResource(tResource, kubeResource)
		if !equal {
			t.Logf("Kubernetes and Teleport resources not sync-ed yet: %s", diff)
		}
		assert.True(c, equal)
	})

	// Updating the resource in Kubernetes
	// The modification can fail because of a conflict with the resource controller. We retry if that happens.
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return test.ModifyKubernetesResource(ctx, resourceName)
	})
	require.NoError(t, err)

	// Check the resource was updated in Teleport
	testlib.FastEventuallyWithT(t, func(c *assert.CollectT) {
		kubeResource, err := test.GetKubernetesResource(ctx, resourceName)
		require.NoError(c, err)

		tResource, err := test.GetTeleportResource(ctx, resourceName)
		require.NoError(c, err)

		// Kubernetes and Teleport resources are in-sync
		equal, diff := test.CompareTeleportAndKubernetesResource(tResource, kubeResource)
		if !equal {
			t.Logf("Kubernetes and Teleport resources not sync-ed yet: %s", diff)
		}
		assert.True(c, equal)
	})

	// Delete the resource to avoid leftover state.
	err = test.DeleteTeleportResource(ctx, resourceName)
	require.NoError(t, err)
}

func TestTrustedClusterV2SecretLookup(t *testing.T) {
	test := &trustedClusterV2TestingPrimitives{}
	setup := testlib.SetupTestEnv(t)
	test.Init(setup)
	ctx := context.Background()

	resourceName := "remote.example.com"
	test.setupTest(t, resourceName)

	secretName := validRandomResourceName("trusted-cluster-secret")
	secretKey := "token"
	secretValue := test.trustedClusterSpec.Token

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: setup.Namespace.Name,
			Annotations: map[string]string{
				secretlookup.AllowLookupAnnotation: resourceName,
			},
		},
		StringData: map[string]string{
			secretKey: secretValue,
		},
		Type: v1.SecretTypeOpaque,
	}
	kubeClient := setup.K8sClient
	require.NoError(t, kubeClient.Create(ctx, secret))

	test.trustedClusterSpec.Token = "secret://" + secretName + "/" + secretKey
	require.NoError(t, test.CreateKubernetesResource(ctx, resourceName))

	testlib.FastEventually(t, func() bool {
		trustedCluster, err := test.GetTeleportResource(ctx, resourceName)
		if err != nil {
			return false
		}
		return trustedCluster.GetToken() == secretValue
	})
}
