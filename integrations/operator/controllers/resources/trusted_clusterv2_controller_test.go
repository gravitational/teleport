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
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/secretlookup"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
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
	// Create trusted cluster join token
	token := "secret_token"
	tokenResource, err := types.NewProvisionToken(token, []types.SystemRole{types.RoleTrustedCluster}, time.Time{})
	if err != nil {
		return trace.Wrap(err)
	}
	r.remoteCluster.Process.GetAuthServer().UpsertToken(ctx, tokenResource)

	// Create required role
	localDev := "local-dev"
	if err := teleportCreateDummyRole(ctx, localDev, r.setup.TeleportClient); err != nil {
		return trace.Wrap(err, "creating dummy role")
	}

	r.trustedClusterSpec = types.TrustedClusterSpecV2{
		Enabled:              true,
		Token:                token,
		ProxyAddress:         r.remoteCluster.Web,
		ReverseTunnelAddress: r.remoteCluster.ReverseTunnel,
		RoleMap: []types.RoleMapping{
			{
				Remote: "remote-dev",
				Local:  []string{localDev},
			},
		},
	}
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
// This runs before we start any test.
// Init() runs after the standardized test suite setup is done.
func (r *trustedClusterV2TestingPrimitives) setupTest(t *testing.T, clusterName string) {
	remoteCluster := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: clusterName,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      logtest.NewLogger(),
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
}

func TestTrustedClusterV2Creation(t *testing.T) {
	test := &trustedClusterV2TestingPrimitives{}
	const remoteClusterName = "remote.example.com"
	test.setupTest(t, remoteClusterName)
	testlib.ResourceCreationSynchronousTest(
		t,
		resources.NewTrustedClusterV2Reconciler,
		test,
		testlib.WithResourceName(remoteClusterName),
	)
}

func TestTrustedClusterV2Deletion(t *testing.T) {
	test := &trustedClusterV2TestingPrimitives{}
	const remoteClusterName = "remote.example.com"
	test.setupTest(t, remoteClusterName)
	testlib.ResourceDeletionSynchronousTest(
		t,
		resources.NewTrustedClusterV2Reconciler,
		test,
		testlib.WithResourceName(remoteClusterName),
	)
}

func TestTrustedClusterV2DeletionDrift(t *testing.T) {
	test := &trustedClusterV2TestingPrimitives{}
	const remoteClusterName = "remote.example.com"
	test.setupTest(t, remoteClusterName)
	testlib.ResourceDeletionDriftSynchronousTest(
		t,
		resources.NewTrustedClusterV2Reconciler,
		test,
		testlib.WithResourceName(remoteClusterName),
	)
}

func TestTrustedClusterUpdate(t *testing.T) {
	test := &trustedClusterV2TestingPrimitives{}
	const remoteClusterName = "remote.example.com"
	test.setupTest(t, remoteClusterName)
	testlib.ResourceUpdateTestSynchronous(
		t,
		resources.NewTrustedClusterV2Reconciler,
		test,
		testlib.WithResourceName(remoteClusterName),
	)
}

func TestTrustedClusterV2SecretLookup(t *testing.T) {
	test := &trustedClusterV2TestingPrimitives{}
	const remoteClusterName = "remote.example.com"
	test.setupTest(t, remoteClusterName)
	setup := testlib.SetupFakeKubeTestEnv(t)
	test.Init(setup)
	ctx := t.Context()
	require.NoError(t, test.SetupTeleportFixtures(ctx))

	secretName := validRandomResourceName("trusted-cluster-secret")
	secretKey := "token"
	secretValue := test.trustedClusterSpec.Token

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: setup.Namespace.Name,
			Annotations: map[string]string{
				secretlookup.AllowLookupAnnotation: remoteClusterName,
			},
		},
		// Real kube servers convert stringData into data.
		// The fake client does not, so we must use Data instead.
		Data: map[string][]byte{
			secretKey: []byte(secretValue),
		},
		Type: v1.SecretTypeOpaque,
	}
	kubeClient := setup.K8sClient
	require.NoError(t, kubeClient.Create(ctx, secret))

	test.trustedClusterSpec.Token = "secret://" + secretName + "/" + secretKey
	require.NoError(t, test.CreateKubernetesResource(ctx, remoteClusterName))

	reconciler, err := resources.NewTrustedClusterV2Reconciler(kubeClient, setup.TeleportClient)
	require.NoError(t, err)

	// Test execution: Kick off the reconciliation.
	req := reconcile.Request{
		NamespacedName: apimachinerytypes.NamespacedName{
			Namespace: setup.Namespace.Name,
			Name:      remoteClusterName,
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

	testlib.FastEventually(t, func() bool {
		trustedCluster, err := test.GetTeleportResource(ctx, remoteClusterName)
		if err != nil {
			return false
		}
		return trustedCluster.GetToken() == secretValue
	})

	// Test cleanup: Delete the resource to avoid leftover state if we were running on a real instance.
	require.NoError(t, test.DeleteKubernetesResource(ctx, remoteClusterName))
	require.NoError(t, kubeClient.Delete(ctx, secret))
	require.NoError(t, test.DeleteTeleportResource(ctx, remoteClusterName))
	// Kicking of a reconciliation to remove the finalizer and let Kube remove the resource.
	_, err = reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
}
