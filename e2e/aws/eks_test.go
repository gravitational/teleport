/*
Copyright 2023 Gravitational, Inc.

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
package integration

import (
	"context"
	"os"
	"os/user"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/integration/kube"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	// Username used for the test.
	username string
	// kubernetes groups and users used for the test.
	// discovery-ci-eks
	// The kubernetes service IAM role can only impersonate the user and group listed below.
	// This is a security measure to prevent the kubernetes service from impersonating any user/group
	// including system:masters.
	// If you need to impersonate a different user/group, you need to update the RBAC
	// permissions for the kubernetes service IAM role.
	kubeGroups = []string{kube.TestImpersonationGroup}
	kubeUsers  = []string{"alice@example.com"}
)

func init() {
	me, err := user.Current()
	if err != nil {
		panic(err)
	}
	username = me.Username
}

func TestKube(t *testing.T) {
	testEnabled := os.Getenv(teleport.KubeRunTests)
	if ok, _ := strconv.ParseBool(testEnabled); !ok {
		t.Skip("Skipping Kubernetes test suite.")
	}

	t.Run("AWS EKS Discovery - Matched cluster", awsEKSDiscoveryMatchedCluster)
	t.Run("AWS EKS Discovery - Unmatched cluster", awsEKSDiscoveryUnmatchedCluster)
}

// awsEKSDiscoveryMatchedCluster tests that the discovery service can discover an EKS
// cluster and create a KubernetesCluster resource.
func awsEKSDiscoveryMatchedCluster(t *testing.T) {
	t.Parallel()
	teleport := createTeleportClusterWithDiscovery(
		t,
		types.Labels{
			types.Wildcard: {types.Wildcard},
		},
	)
	// Get the auth server.
	authC := teleport.Process.GetAuthServer()
	// Wait for the discovery service to discover the cluster and create a
	// KubernetesCluster resource.
	// Discovery service will scan the AWS account each minutes.
	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		clusters, err := authC.GetKubernetesClusters(ctx)
		return err == nil && len(clusters) == 1 && clusters[0].GetName() == os.Getenv(discoveredClusterNameEnv)
	}, 3*time.Minute, 10*time.Second, "wait for the discovery service to create a cluster")

	// Wait for the kubernetes service to create a KubernetesServer resource.
	// This will happen after the discovery service creates the KubernetesCluster
	// resource and the kubernetes service receives the event.
	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		kubeServers, err := authC.GetKubernetesServers(ctx)
		return err == nil && len(kubeServers) == 1
	}, 2*time.Minute, time.Second, "wait for the kubernetes service to create a KubernetesServer")

	// kubeClient is a Kubernetes client for the user created above
	// that will be used to verify that the user can access the cluster and
	// the permissions are correct.
	kubeClient, _, err := kube.ProxyClient(kube.ProxyConfig{
		T:          teleport,
		Username:   username,
		KubeUsers:  kubeUsers,
		KubeGroups: kubeGroups,
	})
	require.NoError(t, err)

	// Retrieve the list of services in the default namespace to verify that
	// the user can access the cluster and the kubernetes service can
	// impersonate the user's kubernetes_groups and kubernetes_users.
	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		list, err := kubeClient.CoreV1().Services(metav1.NamespaceDefault).List(ctx, metav1.ListOptions{})
		return err == nil && len(list.Items) > 0
	}, 30*time.Second, time.Second)
}

// awsEKSDiscoveryUnmatchedCluster tests a scenario where the discovery service
// discovers an EKS cluster but the cluster does not match the discovery
// selectors and therefore no KubernetesCluster resource is created.
func awsEKSDiscoveryUnmatchedCluster(t *testing.T) {
	t.Parallel()
	teleport := createTeleportClusterWithDiscovery(
		t,
		types.Labels{
			// This label will not match the EKS cluster.
			"env": {"tag_not_found"},
		},
	)
	// Get the auth server.
	authC := teleport.Process.GetAuthServer()
	// Wait for the discovery service to not create a KubernetesCluster resource
	// because the cluster does not match the selectors.
	require.Never(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		clusters, err := authC.GetKubernetesClusters(ctx)
		return err == nil && len(clusters) != 0
	}, 2*time.Minute, 10*time.Second, "discovery service incorrectly created a kube_cluster")
}

const (
	// awsRegionEnv is the environment variable that specifies the AWS region
	// where the EKS cluster is running.
	awsRegionEnv = "AWS_REGION"
	// kubernetesServiceAssumeRoleEnv is the environment variable that specifies
	// the IAM role that Teleport Kubernetes Service will assume to access the EKS cluster.
	// This role needs to have the following permissions:
	// - eks:DescribeCluster
	// But it also requires the role to be mapped to a Kubernetes group with the following RBAC permissions:

	//	apiVersion: rbac.authorization.k8s.io/v1
	//	kind: ClusterRole
	//	metadata:
	//		name: teleport-role
	//	rules:
	//	- apiGroups:
	//		- ""
	//		resources:
	//		- users
	//		- groups
	//		- serviceaccounts
	//		verbs:
	//		- impersonate
	//	- apiGroups:
	//		- ""
	//		resources:
	//		- pods
	//		verbs:
	//		- get
	//	- apiGroups:
	//		- "authorization.k8s.io"
	//		resources:
	//		- selfsubjectaccessreviews
	//		- selfsubjectrulesreviews
	//		verbs:
	//		- create

	// check modules/eks-discovery-ci/ from cloud-terraform repo for more details.
	kubernetesServiceAssumeRoleEnv = "KUBERNETES_SERVICE_ASSUME_ROLE"
	// discoveryServiceAssumeRoleEnv is the environment variable that specifies
	// the IAM role that Teleport Discovery Service will assume to list the EKS clusters.
	// This role needs to have the following permissions:
	// - eks:DescribeCluster
	// - eks:ListClusters
	// check modules/eks-discovery-ci/ from cloud-terraform repo for more details.
	discoveryServiceAssumeRoleEnv = "DISCOVERY_SERVICE_ASSUME_ROLE"
	// discoveredClusterNameEnv is the environment variable that specifies the name of the EKS cluster
	// that will be created by Teleport Discovery Service.
	discoveredClusterNameEnv = "DISCOVERED_CLUSTER_NAME"
)

// checkRequiredEnvVars ensures that the required environment variables are set.
func checkRequiredEnvVars(t *testing.T) {
	require.NotEmpty(t, os.Getenv(awsRegionEnv), "AWS_REGION environment variable must be set")
	require.NotEmpty(t, os.Getenv(kubernetesServiceAssumeRoleEnv), "KUBERNETES_SERVICE_ASSUME_ROLE environment variable must be set")
	require.NotEmpty(t, os.Getenv(discoveryServiceAssumeRoleEnv), "DISCOVERY_SERVICE_ASSUME_ROLE environment variable must be set")
	require.NotEmpty(t, os.Getenv(discoveredClusterNameEnv), "DISCOVERED_CLUSTER_NAME environment variable must be set")
}

// createTeleportClusterWithDiscovery creates a Teleport cluster with Discovery Service enabled for
// the given EKS cluster tags.
func createTeleportClusterWithDiscovery(t *testing.T, tags types.Labels) *helpers.TeleInstance {
	// ensures that the required environment variables are set.
	checkRequiredEnvVars(t)

	// Create the CA authority that will be used in Auth.
	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	const (
		host   = helpers.Host
		site   = helpers.Site
		hostID = helpers.HostID
	)
	log := utils.NewLoggerForTests()

	teleport := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: site,
		HostID:      host,
		NodeName:    host,
		Priv:        priv,
		Pub:         pub,
		Log:         log,
	})

	// Create a new role with full access to all resources.
	role, err := types.NewRole(
		"kubemaster",
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubeGroups: kubeGroups,
				KubeUsers:  kubeUsers,
				KubernetesLabels: types.Labels{
					types.Wildcard: {types.Wildcard},
				},
				KubernetesResources: []types.KubernetesResource{
					{
						Kind: types.Wildcard, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
					},
				},
			},
		},
	)
	require.NoError(t, err)

	// Create a new user with the role created above.
	teleport.AddUserWithRole(username, role)
	// Create a new teleport instance with the auth server.
	err = teleport.CreateEx(t, nil, newTeleportConfig(t, log, tags))
	require.NoError(t, err)
	// Start the teleport instance and wait for it to be ready.
	err = teleport.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, teleport.StopAll())
	})
	return teleport
}

func newTeleportConfig(t *testing.T, log utils.Logger, tags types.Labels) *servicecfg.Config {
	tconf := servicecfg.MakeDefaultConfig()
	// Replace the default auth and proxy listeners with the ones so we can
	// run multiple tests in parallel.
	tconf.Auth.ListenAddr = *utils.MustParseAddr(helpers.NewListener(t, service.ListenerAuth, &(tconf.FileDescriptors)))
	tconf.Proxy.WebAddr = *utils.MustParseAddr(helpers.NewListener(t, service.ListenerProxyWeb, &(tconf.FileDescriptors)))
	tconf.Proxy.Kube.ListenAddr = *utils.MustParseAddr(helpers.NewListener(t, service.ListenerProxyKube, &(tconf.FileDescriptors)))
	tconf.DataDir = t.TempDir()
	tconf.Console = nil
	tconf.Log = log
	tconf.SSH.Enabled = true
	tconf.Proxy.DisableWebInterface = true
	tconf.PollingPeriod = 500 * time.Millisecond
	tconf.ClientTimeout = time.Second
	tconf.ShutdownTimeout = 2 * tconf.ClientTimeout
	tconf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	// Enable kubernetes proxy
	tconf.Proxy.Kube.Enabled = true

	enableKubeService(t, tconf)
	enableDiscoveryService(t, tconf, tags)
	return tconf
}

// enableKubeService sets up the kubernetes service to watch for kubernetes
// clusters created by the discovery service.
func enableKubeService(t *testing.T, cfg *servicecfg.Config) {
	// set kubernetes specific parameters
	cfg.Kube.Enabled = true
	cfg.Kube.ListenAddr = utils.MustParseAddr(helpers.NewListener(t, service.ListenerKube, &(cfg.FileDescriptors)))
	cfg.Kube.ResourceMatchers = []services.ResourceMatcher{
		{
			Labels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			AWS: services.ResourceMatcherAWS{
				AssumeRoleARN: os.Getenv(kubernetesServiceAssumeRoleEnv),
			},
		},
	}
}

// enableDiscoveryService sets up the discovery service to watch for EKS clusters
// in the AWS account.
func enableDiscoveryService(t *testing.T, cfg *servicecfg.Config, tags types.Labels) {
	cfg.Discovery.Enabled = true
	cfg.Discovery.DiscoveryGroup = "e2e-test"
	// Reduce the polling interval to speed up the test execution
	// in the case of a failure of the first attempt.
	// The default polling interval is 5 minutes.
	cfg.Discovery.PollInterval = 1 * time.Minute
	cfg.Discovery.AWSMatchers = []types.AWSMatcher{
		{
			Types:   []string{services.AWSMatcherEKS},
			Tags:    tags,
			Regions: []string{os.Getenv(awsRegionEnv)},
			AssumeRole: &types.AssumeRole{
				RoleARN: os.Getenv(discoveryServiceAssumeRoleEnv),
			},
		},
	}
}
