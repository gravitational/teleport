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

package integration

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubetypes "k8s.io/apimachinery/pkg/types"
	streamspdy "k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/transport"
	"k8s.io/client-go/transport/spdy"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/integration/kube"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/events"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
)

type KubeSuite struct {
	*kubernetes.Clientset

	me *user.User
	// priv/pub pair to avoid re-generating it
	priv []byte
	pub  []byte

	// kubeconfigPath is a path to valid kubeconfig
	kubeConfigPath string

	// kubeConfig is a kubernetes config struct
	kubeConfig *rest.Config

	// log defines the test-specific logger
	log utils.Logger
}

func newKubeSuite(t *testing.T) *KubeSuite {
	testEnabled := os.Getenv(teleport.KubeRunTests)
	if ok, _ := strconv.ParseBool(testEnabled); !ok {
		t.Skip("Skipping Kubernetes test suite.")
	}
	suite := &KubeSuite{
		kubeConfigPath: os.Getenv(teleport.EnvKubeConfig),
	}
	require.NotEmpty(t, suite.kubeConfigPath, "This test requires path to valid kubeconfig.")
	var err error
	suite.priv, suite.pub, err = testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	suite.me, err = user.Current()
	require.NoError(t, err)

	// close & re-open stdin because 'go test' runs with os.stdin connected to /dev/null
	stdin, err := os.Open("/dev/tty")
	if err == nil {
		os.Stdin.Close()
		os.Stdin = stdin
	}

	t.Cleanup(func() {
		var err error
		// restore os.Stdin to its original condition: connected to /dev/null
		os.Stdin.Close()
		os.Stdin, err = os.Open("/dev/null")
		require.NoError(t, err)
	})

	suite.Clientset, suite.kubeConfig, err = kubeutils.GetKubeClient(suite.kubeConfigPath)
	require.NoError(t, err)

	// Create test namespace and pod to run k8s commands against.
	ns := newNamespace(testNamespace)
	_, err = suite.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil {
		require.True(t, errors.IsAlreadyExists(err), "Failed to create namespace: %v:", err)
	}
	p := newPod(testNamespace, testPod)
	_, err = suite.CoreV1().Pods(testNamespace).Create(context.Background(), p, metav1.CreateOptions{})
	if err != nil {
		require.True(t, errors.IsAlreadyExists(err), "Failed to create test pod: %v", err)
	}
	// Wait for pod to be running.
	require.Eventually(t, func() bool {
		rsp, err := suite.CoreV1().Pods(testNamespace).Get(context.Background(), testPod, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return rsp.Status.Phase == v1.PodRunning
	}, 60*time.Second, time.Millisecond*500)
	return suite
}

type kubeIntegrationTest func(t *testing.T, suite *KubeSuite)

func (s *KubeSuite) bind(test kubeIntegrationTest) func(t *testing.T) {
	return func(t *testing.T) {
		s.log = utils.NewLoggerForTests()
		os.RemoveAll(profile.FullProfilePath(""))
		t.Cleanup(func() { s.log = nil })
		test(t, s)
	}
}

func TestKube(t *testing.T) {
	suite := newKubeSuite(t)
	t.Run("Exec", suite.bind(testKubeExec))
	t.Run("Deny", suite.bind(testKubeDeny))
	t.Run("PortForward", suite.bind(testKubePortForward))
	t.Run("TransportProtocol", suite.bind(testKubeTransportProtocol))
	t.Run("TrustedClustersClientCert", suite.bind(testKubeTrustedClustersClientCert))
	t.Run("TrustedClustersSNI", suite.bind(testKubeTrustedClustersSNI))
	t.Run("Disconnect", suite.bind(testKubeDisconnect))
	t.Run("Join", suite.bind(testKubeJoin))
	t.Run("IPPinning", suite.bind(testIPPinning))
	// ExecWithNoAuth tests that a user can get the pod and exec into it when
	// moderated session is not enforced.
	// Users under moderated session should only be able to get the pod and shouldn't
	// be able to exec into a pod
	t.Run("ExecWithNoAuth", suite.bind(testExecNoAuth))
	t.Run("EphemeralContainers", suite.bind(testKubeEphemeralContainers))
	t.Run("ExecInWeb", suite.bind(testKubeExecWeb))
}

func testExec(t *testing.T, suite *KubeSuite, pinnedIP string, clientError string) {
	tconf := suite.teleKubeConfig(Host)

	teleport := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: helpers.Site,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	username := suite.me.Username
	kubeGroups := []string{kube.TestImpersonationGroup}
	kubeUsers := []string{"alice@example.com"}
	role, err := types.NewRole("kubemaster", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
			KubeUsers:  kubeUsers,
			KubernetesLabels: types.Labels{
				types.Wildcard: {types.Wildcard},
			},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
		},
		Options: types.RoleOptions{
			PinSourceIP: pinnedIP != "",
		},
	})
	require.NoError(t, err)
	teleport.AddUserWithRole(username, role)

	err = teleport.CreateEx(t, nil, tconf)
	require.NoError(t, err)

	err = teleport.Start()
	require.NoError(t, err)
	defer teleport.StopAll()

	// impersonating client requests will be denied if the headers
	// are referencing users or groups not allowed by the existing roles
	impersonatingProxyClient, impersonatingProxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:             teleport,
		Username:      username,
		PinnedIP:      pinnedIP,
		KubeUsers:     kubeUsers,
		KubeGroups:    kubeGroups,
		Impersonation: &rest.ImpersonationConfig{UserName: "bob", Groups: []string{kube.TestImpersonationGroup}},
	})

	require.NoError(t, err)

	// try get request to fetch a pod
	ctx := context.Background()
	_, err = impersonatingProxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.Error(t, err)

	// scoped client requests will be allowed, as long as the impersonation headers
	// are referencing users and groups allowed by existing roles
	scopedProxyClient, scopedProxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:          teleport,
		Username:   username,
		PinnedIP:   pinnedIP,
		KubeUsers:  kubeUsers,
		KubeGroups: kubeGroups,
		Impersonation: &rest.ImpersonationConfig{
			UserName: role.GetKubeUsers(types.Allow)[0],
			Groups:   role.GetKubeGroups(types.Allow),
		},
	})
	require.NoError(t, err)

	_, err = scopedProxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	if clientError != "" {
		require.ErrorContains(t, err, clientError)
		return
	}

	// set up kube configuration using proxy
	proxyClient, proxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:          teleport,
		Username:   username,
		KubeUsers:  kubeUsers,
		PinnedIP:   pinnedIP,
		KubeGroups: kubeGroups,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	pod, err := proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, execInContainer, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/cat", "/var/run/secrets/kubernetes.io/serviceaccount/namespace"},
		stdout:       out,
	})
	require.NoError(t, err)

	data := out.Bytes()
	require.Equal(t, testNamespace, string(data))

	// interactive command, allocate pty
	term := NewTerminal(250)
	// lets type "echo hi" followed by "enter" and then "exit" + "enter":
	term.Type("\aecho hi\n\r\aexit\n\r\a")

	out = &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, execInContainer, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/sh"},
		stdout:       out,
		tty:          true,
		stdin:        term,
	})
	require.NoError(t, err)

	// verify the session stream output
	sessionStream := out.String()
	require.Contains(t, sessionStream, "echo hi")
	require.Contains(t, sessionStream, "exit")

	// verify traffic capture and upload, wait for the upload to hit
	var sessionID string
	timeoutC := time.After(10 * time.Second)
loop:
	for {
		select {
		case event := <-teleport.UploadEventsC:
			sessionID = event.SessionID
			break loop
		case <-timeoutC:
			t.Fatalf("Timeout waiting for upload of session to complete")
		}
	}

	// read back the entire session and verify that it matches the stated output
	capturedStream, err := teleport.Process.GetAuthServer().GetSessionChunk(apidefaults.Namespace, session.ID(sessionID), 0, events.MaxChunkBytes)
	require.NoError(t, err)

	require.Equal(t, sessionStream, string(capturedStream))

	// impersonating kube exec should be denied
	// interactive command, allocate pty
	term = NewTerminal(250)
	term.Type("\aecho hi\n\r\aexit\n\r\a")
	out = &bytes.Buffer{}
	err = kubeExec(impersonatingProxyClientConfig, execInContainer, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/sh"},
		stdout:       out,
		tty:          true,
		stdin:        term,
	})
	require.Error(t, err)
	require.Regexp(t, ".*impersonation request has been denied.*", err.Error())

	// scoped kube exec is allowed, impersonation headers
	// are allowed by the role
	term = NewTerminal(250)
	term.Type("\aecho hi\n\r\aexit\n\r\a")
	out = &bytes.Buffer{}
	err = kubeExec(scopedProxyClientConfig, execInContainer, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/sh"},
		stdout:       out,
		tty:          true,
		stdin:        term,
	})
	require.NoError(t, err)
}

// TestKubeExec tests kubernetes Exec command set
func testKubeExec(t *testing.T, suite *KubeSuite) {
	testExec(t, suite, "", "")
}

func testIPPinning(t *testing.T, suite *KubeSuite) {
	testCases := []struct {
		desc      string
		pinnedIP  string
		wantError string
	}{
		{
			desc:     "pinned correct IP",
			pinnedIP: "127.0.0.1",
		},
		{
			desc:      "pinned incorrect IP",
			pinnedIP:  "127.0.0.2",
			wantError: "pinned IP doesn't match observed client IP",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			testExec(t, suite, tt.pinnedIP, tt.wantError)
		})
	}
}

// TestKubeDeny makes sure that deny rule conflicting with allow
// rule takes precedence
func testKubeDeny(t *testing.T, suite *KubeSuite) {
	tconf := suite.teleKubeConfig(Host)

	teleport := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: helpers.Site,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	username := suite.me.Username
	kubeGroups := []string{kube.TestImpersonationGroup}
	kubeUsers := []string{"alice@example.com"}
	role, err := types.NewRole("kubemaster", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
			KubeUsers:  kubeUsers,
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
		},
		Deny: types.RoleConditions{
			KubeGroups: kubeGroups,
			KubeUsers:  kubeUsers,
		},
	})
	require.NoError(t, err)
	teleport.AddUserWithRole(username, role)

	err = teleport.CreateEx(t, nil, tconf)
	require.NoError(t, err)

	err = teleport.Start()
	require.NoError(t, err)
	defer teleport.StopAll()

	// set up kube configuration using proxy
	proxyClient, _, err := kube.ProxyClient(kube.ProxyConfig{
		T:          teleport,
		Username:   username,
		KubeUsers:  kubeUsers,
		KubeGroups: kubeGroups,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	ctx := context.Background()
	_, err = proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.Error(t, err)
}

// TestKubePortForward tests kubernetes port forwarding
func testKubePortForward(t *testing.T, suite *KubeSuite) {
	tconf := suite.teleKubeConfig(Host)

	teleport := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: helpers.Site,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	username := suite.me.Username
	kubeGroups := []string{kube.TestImpersonationGroup}
	role, err := types.NewRole("kubemaster", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
			KubernetesLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	teleport.AddUserWithRole(username, role)

	err = teleport.CreateEx(t, nil, tconf)
	require.NoError(t, err)

	err = teleport.Start()
	require.NoError(t, err)
	defer teleport.StopAll()

	// set up kube configuration using proxy
	_, proxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:          teleport,
		Username:   username,
		KubeGroups: kubeGroups,
	})
	require.NoError(t, err)

	// forward local port to target port 80 of the nginx container
	localPort := newPortValue()

	forwarder, err := newPortForwarder(proxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:80", localPort)},
		podName:      testPod,
		podNamespace: testNamespace,
	})
	require.NoError(t, err)

	forwarderCh := make(chan error)
	go func() { forwarderCh <- forwarder.ForwardPorts() }()
	defer func() {
		assert.NoError(t, <-forwarderCh, "Forward ports exited with error")
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout waiting for port forwarding.")
	case <-forwarder.readyC:
	}
	defer close(forwarder.stopC)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%v", localPort))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// impersonating client requests will bse denied
	_, impersonatingProxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:             teleport,
		Username:      username,
		KubeGroups:    kubeGroups,
		Impersonation: &rest.ImpersonationConfig{UserName: "bob", Groups: []string{kube.TestImpersonationGroup}},
	})
	require.NoError(t, err)

	localPort = newPortValue()
	impersonatingForwarder, err := newPortForwarder(impersonatingProxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:80", localPort)},
		podName:      testPod,
		podNamespace: testNamespace,
	})
	require.NoError(t, err)

	// This request should be denied
	err = impersonatingForwarder.ForwardPorts()
	require.Error(t, err)
	require.Regexp(t, ".*impersonation request has been denied.*", err.Error())
}

// TestKubeTrustedClustersClientCert tests scenario with trusted clusters
// using metadata encoded in the certificate
func testKubeTrustedClustersClientCert(t *testing.T, suite *KubeSuite) {
	ctx := context.Background()
	clusterMain := "cluster-main"
	mainConf := suite.teleKubeConfig(Host)
	// Main cluster doesn't need a kubeconfig to forward requests to auxiliary
	// cluster.
	mainConf.Proxy.Kube.KubeconfigPath = ""
	main := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: clusterMain,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	// main cluster has a role and user called main-kube
	username := suite.me.Username
	mainKubeGroups := []string{kube.TestImpersonationGroup}
	mainRole, err := types.NewRole("main-kube", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: mainKubeGroups,
			KubernetesLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	main.AddUserWithRole(username, mainRole)

	clusterAux := "cluster-aux"
	auxConf := suite.teleKubeConfig(Host)
	aux := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: clusterAux,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	mainConf.Proxy.Kube.Enabled = true
	err = main.CreateEx(t, nil, mainConf)
	require.NoError(t, err)

	err = aux.CreateEx(t, nil, auxConf)
	require.NoError(t, err)

	// auxiliary cluster has a role aux-kube
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "aux-kube" to local role "main-kube"
	auxKubeGroups := []string{kube.TestImpersonationGroup}
	auxRole, err := types.NewRole("aux-kube", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{username},
			// Note that main cluster can pass it's kubernetes groups
			// to the remote cluster, and remote cluster
			// can choose to use them by using special variable
			KubeGroups: auxKubeGroups,
			KubernetesLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	auxRole, err = aux.Process.GetAuthServer().UpsertRole(ctx, auxRole)
	require.NoError(t, err)
	trustedClusterToken := "trusted-clsuter-token"
	err = main.Process.GetAuthServer().UpsertToken(ctx,
		services.MustCreateProvisionToken(trustedClusterToken, []types.SystemRole{types.RoleTrustedCluster}, time.Time{}))
	require.NoError(t, err)
	trustedCluster := main.AsTrustedCluster(trustedClusterToken, types.RoleMap{
		{Remote: mainRole.GetName(), Local: []string{auxRole.GetName()}},
	})
	require.NoError(t, err)

	// start both clusters
	err = main.Start()
	require.NoError(t, err)
	defer main.StopAll()

	err = aux.Start()
	require.NoError(t, err)
	defer aux.StopAll()

	// try and upsert a trusted cluster
	var upsertSuccess bool
	for i := 0; i < 10; i++ {
		log.Debugf("Will create trusted cluster %v, attempt %v", trustedCluster, i)
		_, err = aux.Process.GetAuthServer().UpsertTrustedCluster(ctx, trustedCluster)
		if err != nil {
			if trace.IsConnectionProblem(err) {
				log.Debugf("retrying on connection problem: %v", err)
				continue
			}
			t.Fatalf("got non connection problem %v", err)
		}
		upsertSuccess = true
		break
	}
	// make sure we upsert a trusted cluster
	require.True(t, upsertSuccess)

	// Wait for both cluster to see each other via reverse tunnels.
	require.Eventually(t, helpers.WaitForClusters(main.Tunnel, 1), 10*time.Second, 1*time.Second,
		"Two clusters do not see each other: tunnels are not working.")

	require.Eventually(t, func() bool {
		tc, err := main.Process.GetAuthServer().GetRemoteCluster(ctx, aux.Secrets.SiteName)
		if err != nil {
			return false
		}
		return tc.GetConnectionStatus() == teleport.RemoteClusterStatusOnline
	}, 60*time.Second, 1*time.Second, "Main cluster does not see aux cluster as connected")

	// impersonating client requests will be denied
	impersonatingProxyClient, impersonatingProxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:              main,
		Username:       username,
		KubeGroups:     mainKubeGroups,
		KubeCluster:    clusterAux,
		Impersonation:  &rest.ImpersonationConfig{UserName: "bob", Groups: []string{kube.TestImpersonationGroup}},
		RouteToCluster: clusterAux,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	_, err = impersonatingProxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.Error(t, err)

	// set up kube configuration using main proxy
	proxyClient, proxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:              main,
		Username:       username,
		KubeGroups:     mainKubeGroups,
		KubeCluster:    clusterAux,
		RouteToCluster: clusterAux,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	pod, err := proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, execInContainer, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/cat", "/var/run/secrets/kubernetes.io/serviceaccount/namespace"},
		stdout:       out,
	})
	require.NoError(t, err)

	data := out.Bytes()
	require.Equal(t, pod.Namespace, string(data))

	// interactive command, allocate pty
	term := NewTerminal(250)
	// lets type "echo hi" followed by "enter" and then "exit" + "enter":
	term.Type("\aecho hi\n\r\aexit\n\r\a")

	out = &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, execInContainer, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/sh"},
		stdout:       out,
		tty:          true,
		stdin:        term,
	})
	require.NoError(t, err)

	// verify the session stream output
	sessionStream := out.String()
	require.Contains(t, sessionStream, "echo hi")
	require.Contains(t, sessionStream, "exit")

	// verify traffic capture and upload, wait for the upload to hit
	var sessionID string
	timeoutC := time.After(10 * time.Second)
loop:
	for {
		select {
		case event := <-main.UploadEventsC:
			sessionID = event.SessionID
			break loop
		case <-timeoutC:
			t.Fatalf("Timeout waiting for upload of session to complete")
		}
	}

	// read back the entire session and verify that it matches the stated output
	capturedStream, err := main.Process.GetAuthServer().GetSessionChunk(apidefaults.Namespace, session.ID(sessionID), 0, events.MaxChunkBytes)
	require.NoError(t, err)

	require.Equal(t, sessionStream, string(capturedStream))

	// impersonating kube exec should be denied
	// interactive command, allocate pty
	term = NewTerminal(250)
	term.Type("\aecho hi\n\r\aexit\n\r\a")
	out = &bytes.Buffer{}
	err = kubeExec(impersonatingProxyClientConfig, execInContainer, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/sh"},
		stdout:       out,
		tty:          true,
		stdin:        term,
	})
	require.Error(t, err)
	require.Regexp(t, ".*impersonation request has been denied.*", err.Error())

	// forward local port to target port 80 of the nginx container
	localPort := newPortValue()

	forwarder, err := newPortForwarder(proxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:80", localPort)},
		podName:      pod.Name,
		podNamespace: pod.Namespace,
	})
	require.NoError(t, err)

	forwarderCh := make(chan error)
	go func() { forwarderCh <- forwarder.ForwardPorts() }()
	defer func() {
		require.NoError(t, <-forwarderCh, "Forward ports exited with error")
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout waiting for port forwarding.")
	case <-forwarder.readyC:
	}
	defer close(forwarder.stopC)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%v", localPort))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// impersonating client requests will be denied
	localPort = newPortValue()
	impersonatingForwarder, err := newPortForwarder(impersonatingProxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:80", localPort)},
		podName:      pod.Name,
		podNamespace: pod.Namespace,
	})
	require.NoError(t, err)

	// This request should be denied
	err = impersonatingForwarder.ForwardPorts()
	require.Error(t, err)
}

// TestKubeTrustedClustersSNI tests scenario with trusted clusters
// using SNI-forwarding
// DELETE IN(4.3.0)
func testKubeTrustedClustersSNI(t *testing.T, suite *KubeSuite) {
	ctx := context.Background()

	clusterMain := "cluster-main"
	mainConf := suite.teleKubeConfig(Host)
	main := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: clusterMain,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	// main cluster has a role and user called main-kube
	username := suite.me.Username
	mainKubeGroups := []string{kube.TestImpersonationGroup}
	mainRole, err := types.NewRole("main-kube", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: mainKubeGroups,
			KubernetesLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	main.AddUserWithRole(username, mainRole)

	clusterAux := "cluster-aux"
	auxConf := suite.teleKubeConfig(Host)
	aux := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: clusterAux,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	// route all the traffic to the aux cluster
	mainConf.Proxy.Kube.Enabled = true
	// ClusterOverride forces connection to be routed
	// to cluster aux
	mainConf.Proxy.Kube.ClusterOverride = clusterAux
	err = main.CreateEx(t, nil, mainConf)
	require.NoError(t, err)

	err = aux.CreateEx(t, nil, auxConf)
	require.NoError(t, err)

	// auxiliary cluster has a role aux-kube
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "aux-kube" to local role "main-kube"
	auxKubeGroups := []string{kube.TestImpersonationGroup}
	auxRole, err := types.NewRole("aux-kube", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{username},
			KubernetesLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			// Note that main cluster can pass it's kubernetes groups
			// to the remote cluster, and remote cluster
			// can choose to use them by using special variable
			KubeGroups: auxKubeGroups,
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	auxRole, err = aux.Process.GetAuthServer().UpsertRole(ctx, auxRole)
	require.NoError(t, err)
	trustedClusterToken := "trusted-cluster-token"
	err = main.Process.GetAuthServer().UpsertToken(ctx,
		services.MustCreateProvisionToken(trustedClusterToken, []types.SystemRole{types.RoleTrustedCluster}, time.Time{}))
	require.NoError(t, err)
	trustedCluster := main.AsTrustedCluster(trustedClusterToken, types.RoleMap{
		{Remote: mainRole.GetName(), Local: []string{auxRole.GetName()}},
	})
	require.NoError(t, err)

	// start both clusters
	err = main.Start()
	require.NoError(t, err)
	defer main.StopAll()

	err = aux.Start()
	require.NoError(t, err)
	defer aux.StopAll()

	// try and upsert a trusted cluster
	var upsertSuccess bool
	for i := 0; i < 10; i++ {
		log.Debugf("Will create trusted cluster %v, attempt %v", trustedCluster, i)
		_, err = aux.Process.GetAuthServer().UpsertTrustedCluster(ctx, trustedCluster)
		if err != nil {
			if trace.IsConnectionProblem(err) {
				log.Debugf("retrying on connection problem: %v", err)
				continue
			}
			t.Fatalf("got non connection problem %v", err)
		}
		upsertSuccess = true
		break
	}
	// make sure we upsert a trusted cluster
	require.True(t, upsertSuccess)

	// Wait for both cluster to see each other via reverse tunnels.
	require.Eventually(t, helpers.WaitForClusters(main.Tunnel, 1), 10*time.Second, 1*time.Second,
		"Two clusters do not see each other: tunnels are not working.")

	require.Eventually(t, func() bool {
		tc, err := main.Process.GetAuthServer().GetRemoteCluster(ctx, aux.Secrets.SiteName)
		if err != nil {
			return false
		}
		return tc.GetConnectionStatus() == teleport.RemoteClusterStatusOnline
	}, 60*time.Second, 1*time.Second, "Main cluster does not see aux cluster as connected")

	// impersonating client requests will be denied
	impersonatingProxyClient, impersonatingProxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:             main,
		Username:      username,
		KubeGroups:    mainKubeGroups,
		Impersonation: &rest.ImpersonationConfig{UserName: "bob", Groups: []string{kube.TestImpersonationGroup}},
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	_, err = impersonatingProxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.Error(t, err)

	// set up kube configuration using main proxy
	proxyClient, proxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:          main,
		Username:   username,
		KubeGroups: mainKubeGroups,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	pod, err := proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, execInContainer, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/cat", "/var/run/secrets/kubernetes.io/serviceaccount/namespace"},
		stdout:       out,
	})
	require.NoError(t, err)

	data := out.Bytes()
	require.Equal(t, pod.Namespace, string(data))

	// interactive command, allocate pty
	term := NewTerminal(250)
	// lets type "echo hi" followed by "enter" and then "exit" + "enter":
	term.Type("\aecho hi\n\r\aexit\n\r\a")

	out = &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, execInContainer, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/sh"},
		stdout:       out,
		tty:          true,
		stdin:        term,
	})
	require.NoError(t, err)

	// verify the session stream output
	sessionStream := out.String()
	require.Contains(t, sessionStream, "echo hi")
	require.Contains(t, sessionStream, "exit")

	// verify traffic capture and upload, wait for the upload to hit
	var sessionID string
	timeoutC := time.After(10 * time.Second)
loop:
	for {
		select {
		case event := <-main.UploadEventsC:
			sessionID = event.SessionID
			break loop
		case <-timeoutC:
			t.Fatalf("Timeout waiting for upload of session to complete")
		}
	}

	// read back the entire session and verify that it matches the stated output
	capturedStream, err := main.Process.GetAuthServer().GetSessionChunk(apidefaults.Namespace, session.ID(sessionID), 0, events.MaxChunkBytes)
	require.NoError(t, err)

	require.Equal(t, sessionStream, string(capturedStream))

	// impersonating kube exec should be denied
	// interactive command, allocate pty
	term = NewTerminal(250)
	term.Type("\aecho hi\n\r\aexit\n\r\a")
	out = &bytes.Buffer{}
	err = kubeExec(impersonatingProxyClientConfig, execInContainer, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/sh"},
		stdout:       out,
		tty:          true,
		stdin:        term,
	})
	require.Error(t, err)
	require.Regexp(t, ".*impersonation request has been denied.*", err.Error())

	// forward local port to target port 80 of the nginx container
	localPort := newPortValue()

	forwarder, err := newPortForwarder(proxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:80", localPort)},
		podName:      pod.Name,
		podNamespace: pod.Namespace,
	})
	require.NoError(t, err)
	forwarderCh := make(chan error)

	go func() { forwarderCh <- forwarder.ForwardPorts() }()
	defer func() {
		require.NoError(t, <-forwarderCh, "Forward ports exited with error")
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout waiting for port forwarding.")
	case <-forwarder.readyC:
	}
	defer close(forwarder.stopC)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%v", localPort))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// impersonating client requests will be denied
	localPort = newPortValue()
	impersonatingForwarder, err := newPortForwarder(impersonatingProxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:80", localPort)},
		podName:      pod.Name,
		podNamespace: pod.Namespace,
	})
	require.NoError(t, err)

	// This request should be denied
	err = impersonatingForwarder.ForwardPorts()
	require.Error(t, err)
	require.Regexp(t, ".*impersonation request has been denied.*", err.Error())
}

// TestKubeDisconnect tests kubernetes session disconnects
func testKubeDisconnect(t *testing.T, suite *KubeSuite) {
	testCases := []disconnectTestCase{
		{
			name: "idle timeout",
			options: types.RoleOptions{
				ClientIdleTimeout: types.NewDuration(500 * time.Millisecond),
			},
			disconnectTimeout: 2 * time.Second,
		},
		{
			name: "expired cert",
			options: types.RoleOptions{
				DisconnectExpiredCert: types.NewBool(true),
				MaxSessionTTL:         types.NewDuration(3 * time.Second),
			},
			disconnectTimeout: 6 * time.Second,
		},
	}

	for i := 0; i < utils.GetIterations(); i++ {
		t.Run(fmt.Sprintf("Iteration=%d", i), func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					runKubeDisconnectTest(t, suite, tc)
				})
			}
		})
	}
}

// TestKubeDisconnect tests kubernetes session disconnects
func runKubeDisconnectTest(t *testing.T, suite *KubeSuite, tc disconnectTestCase) {
	tconf := suite.teleKubeConfig(Host)

	teleport := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: helpers.Site,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	username := suite.me.Username
	kubeGroups := []string{kube.TestImpersonationGroup}
	role, err := types.NewRole("kubemaster", types.RoleSpecV6{
		Options: tc.options,
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
			KubernetesLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	teleport.AddUserWithRole(username, role)

	err = teleport.CreateEx(t, nil, tconf)
	require.NoError(t, err)

	err = teleport.Start()
	require.NoError(t, err)
	defer teleport.StopAll()

	// set up kube configuration using proxy
	proxyClient, proxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:          teleport,
		Username:   username,
		KubeGroups: kubeGroups,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	ctx := context.Background()
	pod, err := proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, execInContainer, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/cat", "/var/run/secrets/kubernetes.io/serviceaccount/namespace"},
		stdout:       out,
	})
	require.NoError(t, err)

	data := out.Bytes()
	require.Equal(t, pod.Namespace, string(data))

	// interactive command, allocate pty
	term := NewTerminal(250)
	sessionCtx, sessionCancel := context.WithCancel(ctx)
	go func() {
		defer sessionCancel()
		err := kubeExec(proxyClientConfig, execInContainer, kubeExecArgs{
			podName:      pod.Name,
			podNamespace: pod.Namespace,
			container:    pod.Spec.Containers[0].Name,
			command:      []string{"/bin/sh"},
			stdout:       term,
			tty:          true,
			stdin:        term,
		})
		require.NoError(t, err)
	}()

	// lets type something followed by "enter" and then hang the session
	require.NoError(t, enterInput(sessionCtx, term, "echo boring platypus\r\n", ".*boring platypus.*"))
	time.Sleep(tc.disconnectTimeout)
	select {
	case <-time.After(tc.disconnectTimeout):
		t.Fatalf("timeout waiting for session to exit")
	case <-sessionCtx.Done():
		// session closed
	}
}

// testKubeTransportProtocol tests the proxy transport protocol capabilities
func testKubeTransportProtocol(t *testing.T, suite *KubeSuite) {
	tconf := suite.teleKubeConfig(Host)

	teleport := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: helpers.Site,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	username := suite.me.Username
	kubeGroups := []string{kube.TestImpersonationGroup}
	role, err := types.NewRole("kubemaster", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
			KubernetesLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	teleport.AddUserWithRole(username, role)

	err = teleport.CreateEx(t, nil, tconf)
	require.NoError(t, err)

	err = teleport.Start()
	require.NoError(t, err)
	defer teleport.StopAll()

	// set up kube configuration using proxy
	proxyClient, proxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:          teleport,
		Username:   username,
		KubeGroups: kubeGroups,
	})
	require.NoError(t, err)

	ctx := context.Background()
	pod, err := proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.NoError(t, err)

	u, err := url.Parse(proxyClientConfig.Host)
	require.NoError(t, err)

	u.Scheme = "https"
	u.Path = fmt.Sprintf("/api/v1/namespaces/%v/pods/%v", pod.Namespace, pod.Name)

	tlsConfig, err := tlsClientConfig(proxyClientConfig)
	require.NoError(t, err)

	trans := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// call proxy with an HTTP1 client
	client := &http.Client{Transport: trans}
	resp1, err := client.Get(u.String())
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, 200, resp1.StatusCode)
	require.Equal(t, "HTTP/1.1", resp1.Proto)

	// call proxy with an HTTP2 client
	err = http2.ConfigureTransport(trans)
	require.NoError(t, err)

	resp2, err := client.Get(u.String())
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, 200, resp2.StatusCode)
	require.Equal(t, "HTTP/2.0", resp2.Proto)

	// stream succeeds with an h1 transport
	command := kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"ls"},
	}

	err = kubeExec(proxyClientConfig, execInContainer, command)
	require.NoError(t, err)

	// stream fails with an h2 transport
	proxyClientConfig.TLSClientConfig.NextProtos = []string{"h2"}
	err = kubeExec(proxyClientConfig, execInContainer, command)
	require.Error(t, err)
}

// TODO: test against tsh kubectl
func testKubeEphemeralContainers(t *testing.T, suite *KubeSuite) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Kubernetes: true,
		},
	})

	tconf := suite.teleKubeConfig(Host)
	teleport := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: helpers.Site,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	username := suite.me.Username
	kubeUsers := []string{username}
	kubeGroups := []string{kube.TestImpersonationGroup}
	kubeAccessRole, err := types.NewRole("kubemaster", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeUsers:  kubeUsers,
			KubeGroups: kubeGroups,
			KubernetesLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Name:      types.Wildcard,
					Namespace: types.Wildcard,
					Verbs:     []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	peerRole, err := types.NewRole("peer", types.RoleSpecV6{
		Allow: types.RoleConditions{
			RequireSessionJoin: []*types.SessionRequirePolicy{
				{
					Name:   "Requires oversight",
					Filter: `equals("true", "true")`,
					Kinds: []string{
						string(types.KubernetesSessionKind),
					},
					Count: 1,
					Modes: []string{
						string(types.SessionModeratorMode),
					},
					OnLeave: string(types.OnSessionLeaveTerminate),
				},
			},
		},
	})
	require.NoError(t, err)
	teleport.AddUserWithRole(username, kubeAccessRole, peerRole)

	moderatorUser := username + "-moderator"
	moderatorRole, err := types.NewRole("moderator", types.RoleSpecV6{
		Allow: types.RoleConditions{
			JoinSessions: []*types.SessionJoinPolicy{{
				Name:  "Session moderator",
				Roles: []string{"kubemaster"},
				Kinds: []string{string(types.KubernetesSessionKind)},
				Modes: []string{string(types.SessionModeratorMode), string(types.SessionObserverMode)},
			}},
		},
	})
	require.NoError(t, err)
	teleport.AddUserWithRole(moderatorUser, kubeAccessRole, moderatorRole)

	err = teleport.CreateEx(t, nil, tconf)
	require.NoError(t, err)

	err = teleport.Start()
	require.NoError(t, err)
	defer teleport.StopAll()

	// set up kube configuration using proxy
	proxyClient, kubeConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:          teleport,
		Username:   username,
		KubeUsers:  kubeUsers,
		KubeGroups: kubeGroups,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	ctx := context.Background()
	podsClient := proxyClient.CoreV1().Pods(testNamespace)
	pod, err := podsClient.Get(ctx, testPod, metav1.GetOptions{})
	require.NoError(t, err)

	podJS, err := json.Marshal(pod)
	require.NoError(t, err)

	// create an ephemeral container and attach to it just like kubectl would
	contName := "ephemeral-container"
	sessCreatorTerm := NewTerminal(250)
	group := &errgroup.Group{}
	group.Go(func() error {
		cmd := []string{"/bin/sh", "echo", "hello from an ephemeral container"}
		debugPod, _, err := generateDebugContainer(contName, cmd, pod)
		if err != nil {
			return trace.Wrap(err)
		}

		debugJS, err := json.Marshal(debugPod)
		if err != nil {
			return trace.Wrap(err)
		}
		patch, err := strategicpatch.CreateTwoWayMergePatch(podJS, debugJS, pod)
		if err != nil {
			return trace.Wrap(err)
		}

		_, err = podsClient.Patch(ctx, pod.Name, kubetypes.StrategicMergePatchType, patch, metav1.PatchOptions{}, "ephemeralcontainers")
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = waitForContainer(ctx, podsClient, pod.Name, contName)
		if err != nil {
			return trace.Wrap(err)
		}

		err = kubeExec(kubeConfig, attachToContainer, kubeExecArgs{
			podName:      pod.Name,
			podNamespace: testNamespace,
			container:    contName,
			command:      cmd,
			stdout:       sessCreatorTerm,
			stderr:       sessCreatorTerm,
			stdin:        sessCreatorTerm,
			tty:          true,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})

	// We need to wait for the exec request to be handled here for the session to be
	// created. Sadly though the k8s API doesn't give us much indication of when that is.
	var session types.SessionTracker
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// We need to wait for the session to be created here. We can't use the
		// session manager's WaitUntilExists method because it doesn't work for
		// kubernetes sessions.
		sessions, err := teleport.Process.GetAuthServer().GetActiveSessionTrackers(context.Background())
		if !assert.NoError(t, err) || !assert.NotEmpty(t, sessions) {
			return
		}
		session = sessions[0]
	}, 10*time.Second, 100*time.Millisecond)

	// join the created session as a moderator
	group.Go(func() error {
		// verify that the ephemeral container hasn't actually been created yet
		proxyClient, _, err := kube.ProxyClient(kube.ProxyConfig{
			T:          teleport,
			Username:   moderatorUser,
			KubeUsers:  kubeUsers,
			KubeGroups: kubeGroups,
		})
		require.NoError(t, err)

		podsClient := proxyClient.CoreV1().Pods(testNamespace)
		pod, err := podsClient.Get(ctx, testPod, metav1.GetOptions{})
		require.NoError(t, err)
		for _, status := range pod.Status.EphemeralContainerStatuses {
			if !assert.NotEqual(t, status.Name, contName) {
				return trace.AlreadyExists("ephemeral container already started")
			}
		}

		tc, err := teleport.NewClient(helpers.ClientConfig{
			TeleportUser: moderatorUser,
			Cluster:      helpers.Site,
			Host:         Host,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		stream, err := kubeJoin(kube.ProxyConfig{
			T:          teleport,
			Username:   moderatorUser,
			KubeUsers:  kubeUsers,
			KubeGroups: kubeGroups,
		}, tc, session, types.SessionModeratorMode)
		if err != nil {
			return trace.Wrap(err)
		}

		stream.Wait()
		return trace.Wrap(stream.Detach())
	})

	require.NoError(t, group.Wait())
}

func generateDebugContainer(name string, cmd []string, pod *v1.Pod) (*v1.Pod, *v1.EphemeralContainer, error) {
	ec := &v1.EphemeralContainer{
		EphemeralContainerCommon: v1.EphemeralContainerCommon{
			Name:                     name,
			Image:                    "alpine:latest",
			Command:                  cmd,
			ImagePullPolicy:          v1.PullIfNotPresent,
			Stdin:                    true,
			TerminationMessagePolicy: v1.TerminationMessageReadFile,
			TTY:                      true,
		},
		TargetContainerName: pod.Spec.Containers[0].Name,
	}

	copied := pod.DeepCopy()
	copied.Spec.EphemeralContainers = append(copied.Spec.EphemeralContainers, *ec)
	ec = &copied.Spec.EphemeralContainers[len(copied.Spec.EphemeralContainers)-1]

	return copied, ec, nil
}

func waitForContainer(ctx context.Context, podClient corev1client.PodInterface, podName, containerName string) (*v1.Pod, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", podName).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return podClient.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return podClient.Watch(ctx, options)
		},
	}

	ev, err := watchtools.UntilWithSync(ctx, lw, &v1.Pod{}, nil, func(ev watch.Event) (bool, error) {
		switch ev.Type {
		case watch.Deleted:
			return false, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
		}

		p, ok := ev.Object.(*v1.Pod)
		if !ok {
			return false, fmt.Errorf("watch did not return a pod: %v", ev.Object)
		}

		s := getContainerStatusByName(p, containerName)
		fmt.Println("test", s)
		if s == nil {
			return false, nil
		}
		if s.State.Running != nil || s.State.Terminated != nil {
			return true, nil
		}

		return false, nil
	})
	if ev != nil {
		return ev.Object.(*v1.Pod), nil
	}
	return nil, err
}

func getContainerStatusByName(pod *v1.Pod, containerName string) *v1.ContainerStatus {
	allContainerStatus := [][]v1.ContainerStatus{pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses, pod.Status.EphemeralContainerStatuses}
	for _, statusSlice := range allContainerStatus {
		for i := range statusSlice {
			if statusSlice[i].Name == containerName {
				return &statusSlice[i]
			}
		}
	}
	return nil
}

func testKubeExecWeb(t *testing.T, suite *KubeSuite) {
	clusterName := "cluster"
	kubeClusterName := "cluster"
	clusterConf := suite.teleKubeConfig(Host)
	clusterConf.Auth.Preference.SetSecondFactor("off") // So we can do web login.

	cluster := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: clusterName,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	// Setup user and role.
	testUser := suite.me.Username
	kubeGroups := []string{kube.TestImpersonationGroup}
	kubeUsers := []string{testUser}
	role, err := types.NewRole("kubemaster", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{testUser},
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
	})
	require.NoError(t, err)
	cluster.AddUserWithRole(testUser, role)

	err = cluster.CreateEx(t, nil, clusterConf)
	require.NoError(t, err)

	// Start the cluster.
	err = cluster.Start()
	require.NoError(t, err)
	defer cluster.StopAll()

	proxyAddr, err := cluster.Process.ProxyWebAddr()
	require.NoError(t, err)

	auth := cluster.Process.GetAuthServer()

	userPassword := uuid.NewString()
	require.NoError(t, auth.UpsertPassword(testUser, []byte(userPassword)))

	// Login and run the tests.
	webPack := helpers.LoginWebClient(t, proxyAddr.String(), testUser, userPassword)
	endpoint, err := url.JoinPath("sites", "$site", "kube", kubeClusterName, "connect/ws") // :site/kube/:clusterName/connect/ws
	require.NoError(t, err)

	openWebsocketAndReadSession := func(t *testing.T, endpoint string, req web.PodExecRequest) *websocket.Conn {
		ws, resp, err := webPack.OpenWebsocket(t, endpoint, req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())

		_, data, err := ws.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, `{"type":"create_session_response","status":"ok"}`+"\n", string(data))

		execSocket := executionWebsocketReader{ws}

		// First message: session metadata
		envelope, err := execSocket.Read()
		require.NoError(t, err)
		var sessionMetadata sessionMetadataResponse
		require.NoError(t, json.Unmarshal([]byte(envelope.Payload), &sessionMetadata))

		return ws
	}

	findTextInReader := func(t *testing.T, reader ReaderWithDeadline, text string, timeout time.Duration) {
		// Make sure we don't wait forever on a read.
		err := reader.SetReadDeadline(time.Now().Add(timeout))
		require.NoError(t, err)

		readData := make([]byte, 255)
		accum := make([]byte, 0, 255)
		for {
			n, err := reader.Read(readData)
			require.NoError(t, err)

			accum = append(accum, readData[:n]...)

			if strings.Contains(string(accum), text) {
				break
			}
		}
	}

	t.Run("Non-interactive", func(t *testing.T) {
		req := web.PodExecRequest{
			KubeCluster: kubeClusterName,
			Namespace:   testNamespace,
			Pod:         testPod,
			Command:     "/bin/cat /var/run/secrets/kubernetes.io/serviceaccount/namespace",
			Term:        session.TerminalParams{W: 80, H: 24},
		}

		ws := openWebsocketAndReadSession(t, endpoint, req)

		wsStream := web.NewWStream(context.Background(), ws, suite.log, nil)

		// Check for the expected string in the output.
		findTextInReader(t, wsStream, testNamespace, time.Second*2)

		err = ws.Close()
		require.NoError(t, err)
	})

	t.Run("Interactive", func(t *testing.T) {
		req := web.PodExecRequest{
			KubeCluster:   kubeClusterName,
			Namespace:     testNamespace,
			Pod:           testPod,
			Command:       "/bin/sh",
			IsInteractive: true,
			Term:          session.TerminalParams{W: 80, H: 24},
		}

		ws := openWebsocketAndReadSession(t, endpoint, req)

		wsStream := web.NewWStream(context.Background(), ws, suite.log, nil)

		// Read first prompt from the server.
		readData := make([]byte, 255)
		_, err = wsStream.Read(readData)
		require.NoError(t, err)

		// Send our command.
		_, err = wsStream.Write([]byte("/bin/cat /var/run/secrets/kubernetes.io/serviceaccount/namespace\n"))
		require.NoError(t, err)

		// Check for the expected string in the output.
		findTextInReader(t, wsStream, testNamespace, time.Second*2)

		err = ws.Close()
		require.NoError(t, err)
	})

}

type ReaderWithDeadline interface {
	io.Reader
	SetReadDeadline(time.Time) error
}

// Small helper that wraps a websocket and unmarshalls messages as Teleport
// websocket ones.
type executionWebsocketReader struct {
	*websocket.Conn
}

func (r executionWebsocketReader) Read() (web.Envelope, error) {
	_, data, err := r.ReadMessage()
	if err != nil {
		return web.Envelope{}, trace.Wrap(err)
	}
	var envelope web.Envelope
	return envelope, trace.Wrap(proto.Unmarshal(data, &envelope))
}

// This is used for unmarshalling
type sessionMetadataResponse struct {
	Session session.Session `json:"session"`
}

// teleKubeConfig sets up teleport with kubernetes turned on
func (s *KubeSuite) teleKubeConfig(hostname string) *servicecfg.Config {
	tconf := servicecfg.MakeDefaultConfig()
	tconf.Console = nil
	tconf.Log = s.log
	tconf.SSH.Enabled = true
	tconf.Proxy.DisableWebInterface = true
	tconf.PollingPeriod = 500 * time.Millisecond
	tconf.Testing.ClientTimeout = time.Second
	tconf.Testing.ShutdownTimeout = 2 * tconf.Testing.ClientTimeout

	// set kubernetes specific parameters
	tconf.Proxy.Kube.Enabled = true
	tconf.Proxy.Kube.ListenAddr.Addr = net.JoinHostPort(hostname, newPortStr())
	tconf.Proxy.Kube.KubeconfigPath = s.kubeConfigPath
	tconf.Proxy.Kube.LegacyKubeProxy = true
	tconf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	return tconf
}

// teleKubeConfig sets up teleport with kubernetes turned on
func (s *KubeSuite) teleAuthConfig(hostname string) *servicecfg.Config {
	tconf := servicecfg.MakeDefaultConfig()
	tconf.Console = nil
	tconf.Log = s.log
	tconf.PollingPeriod = 500 * time.Millisecond
	tconf.Testing.ClientTimeout = time.Second
	tconf.Testing.ShutdownTimeout = 2 * tconf.Testing.ClientTimeout
	tconf.Proxy.Enabled = false
	tconf.SSH.Enabled = false
	tconf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	return tconf
}

// tlsClientConfig returns TLS configuration for client
func tlsClientConfig(cfg *rest.Config) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(cfg.TLSClientConfig.CertData, cfg.TLSClientConfig.KeyData)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM(cfg.TLSClientConfig.CAData)
	if !ok {
		return nil, trace.BadParameter("failed to append certs from PEM")
	}

	return &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}, nil
}

func kubeProxyTLSConfig(cfg kube.ProxyConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{}
	_, kubeConfig, err := kube.ProxyClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCert, err := tlsca.ParseCertificatePEM(kubeConfig.TLSClientConfig.CAData)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := tls.X509KeyPair(kubeConfig.TLSClientConfig.CertData, kubeConfig.TLSClientConfig.KeyData)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig.RootCAs = x509.NewCertPool()
	tlsConfig.RootCAs.AddCert(caCert)
	tlsConfig.Certificates = []tls.Certificate{cert}
	tlsConfig.ServerName = kubeConfig.TLSClientConfig.ServerName
	return tlsConfig, nil
}

const (
	testNamespace = "teletest"
	testPod       = "test-pod"
)

func newNamespace(name string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func newPod(ns, name string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:  "nginx",
				Image: "nginx:alpine",
			}},
		},
	}
}

type kubeExecArgs struct {
	podName      string
	podNamespace string
	container    string
	command      []string
	stdout       io.Writer
	stderr       io.Writer
	stdin        io.Reader
	tty          bool
}

type kubePortForwardArgs struct {
	ports        []string
	podName      string
	podNamespace string
}

type kubePortForwarder struct {
	*portforward.PortForwarder
	stopC  chan struct{}
	readyC chan struct{}
}

func newPortForwarder(kubeConfig *rest.Config, args kubePortForwardArgs) (*kubePortForwarder, error) {
	u, err := url.Parse(kubeConfig.Host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u.Scheme = "https"
	u.Path = fmt.Sprintf("/api/v1/namespaces/%v/pods/%v/portforward", args.podNamespace, args.podName)

	// set up port forwarding request
	tlsConfig, err := tlsClientConfig(kubeConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upgradeRoundTripper, err := streamspdy.NewRoundTripper(tlsConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := &http.Client{
		Transport: upgradeRoundTripper,
	}
	dialer := spdy.NewDialer(upgradeRoundTripper, client, "POST", u)
	if kubeConfig.Impersonate.UserName != "" {
		client.Transport = transport.NewImpersonatingRoundTripper(
			transport.ImpersonationConfig{
				UserName: kubeConfig.Impersonate.UserName,
				Groups:   kubeConfig.Impersonate.Groups,
			},
			upgradeRoundTripper)
	}

	stopC, readyC := make(chan struct{}), make(chan struct{})
	fwd, err := portforward.New(dialer, args.ports, stopC, readyC, nil, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &kubePortForwarder{PortForwarder: fwd, stopC: stopC, readyC: readyC}, nil
}

// execMode is the type of Kubernetes
type execMode int

const (
	execInContainer execMode = iota
	attachToContainer
)

// kubeExec executes command against kubernetes API server
func kubeExec(kubeConfig *rest.Config, mode execMode, args kubeExecArgs) error {
	query := make(url.Values)
	if mode == execInContainer {
		for _, arg := range args.command {
			query.Add("command", arg)
		}
	}
	if args.stdout != nil {
		query.Set("stdout", "true")
	}
	if args.stdin != nil {
		query.Set("stdin", "true")
	}
	// stderr channel is only set if there is no tty allocated
	// otherwise k8s server gets confused
	if !args.tty && args.stderr == nil {
		args.stderr = io.Discard
	}
	if args.stderr != nil && !args.tty {
		query.Set("stderr", "true")
	}
	if args.tty {
		query.Set("tty", "true")
	}
	query.Set("container", args.container)
	u, err := url.Parse(kubeConfig.Host)
	if err != nil {
		return trace.Wrap(err)
	}
	u.Scheme = "https"
	resource := "exec"
	if mode == attachToContainer {
		resource = "attach"
	}
	u.Path = fmt.Sprintf("/api/v1/namespaces/%v/pods/%v/%v", args.podNamespace, args.podName, resource)
	u.RawQuery = query.Encode()
	executor, err := remotecommand.NewSPDYExecutor(kubeConfig, "POST", u)
	if err != nil {
		return trace.Wrap(err)
	}
	opts := remotecommand.StreamOptions{
		Stdin:  args.stdin,
		Stdout: args.stdout,
		Stderr: args.stderr,
		Tty:    args.tty,
	}
	return executor.StreamWithContext(context.Background(), opts)
}

func kubeJoin(kubeConfig kube.ProxyConfig, tc *client.TeleportClient, meta types.SessionTracker, mode types.SessionParticipantMode) (*client.KubeSession, error) {
	tlsConfig, err := kubeProxyTLSConfig(kubeConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sess, err := client.NewKubeSession(context.TODO(), tc, meta, tc.KubeProxyAddr, "", mode, tlsConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

// testKubeJoin tests that that joining an interactive exec session works.
func testKubeJoin(t *testing.T, suite *KubeSuite) {
	tconf := suite.teleKubeConfig(Host)

	teleport := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: helpers.Site,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	// fooey
	hostUsername := suite.me.Username
	participantUsername := suite.me.Username + "-participant"
	kubeGroups := []string{kube.TestImpersonationGroup}
	kubeUsers := []string{"alice@example.com"}
	role, err := types.NewRole("kubemaster", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{hostUsername},
			KubeGroups: kubeGroups,
			KubeUsers:  kubeUsers,
			KubernetesLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	joinRole, err := types.NewRole("participant", types.RoleSpecV6{
		Allow: types.RoleConditions{
			JoinSessions: []*types.SessionJoinPolicy{{
				Name:  "foo",
				Roles: []string{"kubemaster"},
				Kinds: []string{string(types.KubernetesSessionKind)},
				Modes: []string{string(types.SessionPeerMode), string(types.SessionObserverMode)},
			}},
		},
	})
	require.NoError(t, err)
	teleport.AddUserWithRole(hostUsername, role)
	teleport.AddUserWithRole(participantUsername, joinRole)

	err = teleport.CreateEx(t, nil, tconf)
	require.NoError(t, err)

	err = teleport.Start()
	require.NoError(t, err)
	defer teleport.StopAll()

	ctx := context.Background()

	// set up kube configuration using proxy
	proxyClient, proxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:          teleport,
		Username:   hostUsername,
		KubeUsers:  kubeUsers,
		KubeGroups: kubeGroups,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	pod, err := proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.NoError(t, err)

	// interactive command, allocate pty
	term := NewTerminal(250)

	out := &bytes.Buffer{}

	group := &errgroup.Group{}

	// Start the main session.
	group.Go(func() error {
		err := kubeExec(proxyClientConfig, execInContainer, kubeExecArgs{
			podName:      pod.Name,
			podNamespace: pod.Namespace,
			container:    pod.Spec.Containers[0].Name,
			command:      []string{"/bin/sh"},
			stdout:       out,
			tty:          true,
			stdin:        term,
		})
		return trace.Wrap(err)
	})

	// We need to wait for the exec request to be handled here for the session to be
	// created. Sadly though the k8s API doesn't give us much indication of when that is.
	var session types.SessionTracker
	require.Eventually(t, func() bool {
		// We need to wait for the session to be created here. We can't use the
		// session manager's WaitUntilExists method because it doesn't work for
		// kubernetes sessions.
		sessions, err := teleport.Process.GetAuthServer().GetActiveSessionTrackers(context.Background())
		if err != nil || len(sessions) == 0 {
			return false
		}

		session = sessions[0]
		return true
	}, 10*time.Second, time.Second)

	participantStdinR, participantStdinW, err := os.Pipe()
	require.NoError(t, err)
	participantStdoutR, participantStdoutW, err := os.Pipe()
	require.NoError(t, err)
	streamsMu := &sync.Mutex{}
	streams := make([]*client.KubeSession, 0, 3)
	observerCaptures := make([]*bytes.Buffer, 0, 2)
	albProxy := helpers.MustStartMockALBProxy(t, teleport.Config.Proxy.WebAddr.Addr)

	// join peer by KubeProxyAddr
	group.Go(func() error {
		tc, err := teleport.NewClient(helpers.ClientConfig{
			Login:   hostUsername,
			Cluster: helpers.Site,
			Host:    Host,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		tc.Stdin = participantStdinR
		tc.Stdout = participantStdoutW

		stream, err := kubeJoin(kube.ProxyConfig{
			T:          teleport,
			Username:   participantUsername,
			KubeUsers:  kubeUsers,
			KubeGroups: kubeGroups,
		}, tc, session, types.SessionPeerMode)
		if err != nil {
			return trace.Wrap(err)
		}
		streamsMu.Lock()
		streams = append(streams, stream)
		streamsMu.Unlock()
		stream.Wait()
		// close participant stdout so that we can read it after till EOF
		participantStdoutW.Close()
		return nil
	})

	// join observer by WebProxyAddr
	group.Go(func() error {
		stream, capture := kubeJoinByWebAddr(t, teleport, participantUsername, kubeUsers, kubeGroups)
		streamsMu.Lock()
		streams = append(streams, stream)
		observerCaptures = append(observerCaptures, capture)
		streamsMu.Unlock()
		stream.Wait()
		return nil
	})

	// join observer with ALPN conn upgrade
	group.Go(func() error {
		stream, capture := kubeJoinByALBAddr(t, teleport, participantUsername, kubeUsers, kubeGroups, albProxy.Addr().String())
		streamsMu.Lock()
		streams = append(streams, stream)
		observerCaptures = append(observerCaptures, capture)
		streamsMu.Unlock()
		stream.Wait()
		return nil
	})

	// We wait again for the second user to finish joining the session.
	// We allow a bit of time to pass here to give the session manager time to recognize the
	// new IO streams of the second client.
	time.Sleep(time.Second * 5)

	// sent a test message from the participant
	participantStdinW.Write([]byte("\ahi from peer\n\r"))

	// lets type "echo hi" followed by "enter" and then "exit" + "enter":
	term.Type("\ahi from term\n\r")

	// Terminate the session after a moment to allow for the IO to reach the second client.
	time.AfterFunc(5*time.Second, func() {
		// send exit command to close the session
		term.Type("exit 0\n\r\a")
	})

	// wait for all clients to finish
	require.NoError(t, group.Wait())

	// Verify peer.
	participantOutput, err := io.ReadAll(participantStdoutR)
	require.NoError(t, err)
	require.Contains(t, string(participantOutput), "hi from term")

	// Verify original session.
	require.Contains(t, out.String(), "hi from peer")

	// Verify observers.
	for _, capture := range observerCaptures {
		require.Contains(t, capture.String(), "hi from peer")
		require.Contains(t, capture.String(), "hi from term")
	}
}

func kubeJoinByWebAddr(t *testing.T, teleport *helpers.TeleInstance, username string, kubeUsers, kubeGroups []string) (*client.KubeSession, *bytes.Buffer) {
	t.Helper()

	tc, err := teleport.NewClient(helpers.ClientConfig{
		Login:   username,
		Cluster: helpers.Site,
		Host:    Host,
		Proxy: &helpers.ProxyConfig{
			WebAddr:  teleport.Config.Proxy.WebAddr.Addr,
			KubeAddr: teleport.Config.Proxy.WebAddr.Addr,
		},
	})
	require.NoError(t, err)

	buffer := new(bytes.Buffer)
	tc.Stdout = buffer
	return kubeJoinObserverWithSNISet(t, tc, teleport, kubeUsers, kubeGroups), buffer
}

func kubeJoinByALBAddr(t *testing.T, teleport *helpers.TeleInstance, username string, kubeUsers, kubeGroups []string, albAddr string) (*client.KubeSession, *bytes.Buffer) {
	t.Helper()

	tc, err := teleport.NewClient(helpers.ClientConfig{
		Login:   username,
		Cluster: helpers.Site,
		Host:    Host,
		ALBAddr: albAddr,
	})
	require.NoError(t, err)

	buffer := new(bytes.Buffer)
	tc.Stdout = buffer
	return kubeJoinObserverWithSNISet(t, tc, teleport, kubeUsers, kubeGroups), buffer
}

func kubeJoinObserverWithSNISet(t *testing.T, tc *client.TeleportClient, teleport *helpers.TeleInstance, kubeUsers, kubeGroups []string) *client.KubeSession {
	t.Helper()

	sessions, err := teleport.Process.GetAuthServer().GetActiveSessionTrackers(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, sessions)

	stream, err := kubeJoin(kube.ProxyConfig{
		T:                   teleport,
		Username:            tc.Username,
		KubeUsers:           kubeUsers,
		KubeGroups:          kubeGroups,
		CustomTLSServerName: constants.KubeTeleportProxyALPNPrefix + Host,
	}, tc, sessions[0], types.SessionObserverMode)
	require.NoError(t, err)
	return stream
}

// testExecNoAuth tests that a user can get the pod and exec into a pod
// if they do not require any moderated session, if the auth server is not available.
// If moderated session is required, they are only allowed to get the pod but
// not exec into it.
func testExecNoAuth(t *testing.T, suite *KubeSuite) {
	teleport := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: helpers.Site,
		HostID:      helpers.HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		Log:         suite.log,
	})

	adminUsername := "admin"
	kubeGroups := []string{kube.TestImpersonationGroup}
	kubeUsers := []string{"alice@example.com"}
	adminRole, err := types.NewRole("admin", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{adminUsername},
			KubeGroups: kubeGroups,
			KubeUsers:  kubeUsers,
			KubernetesLabels: types.Labels{
				types.Wildcard: {types.Wildcard},
			},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	teleport.AddUserWithRole(adminUsername, adminRole)

	userUsername := "user"
	userRole, err := types.NewRole("userRole", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{userUsername},
			KubeGroups: kubeGroups,
			KubeUsers:  kubeUsers,
			KubernetesLabels: types.Labels{
				types.Wildcard: {types.Wildcard},
			},
			KubernetesResources: []types.KubernetesResource{
				{
					Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard, Verbs: []string{types.Wildcard},
				},
			},
			RequireSessionJoin: []*types.SessionRequirePolicy{
				{
					Name:   "Auditor oversight",
					Filter: fmt.Sprintf("contains(user.spec.roles, %q)", adminRole.GetName()),
					Kinds:  []string{"k8s"},
					Modes:  []string{string(types.SessionModeratorMode)},
					Count:  1,
				},
			},
		},
	})
	require.NoError(t, err)
	teleport.AddUserWithRole(userUsername, userRole)
	authTconf := suite.teleAuthConfig(Host)
	err = teleport.CreateEx(t, nil, authTconf)
	require.NoError(t, err)
	err = teleport.Start()
	require.NoError(t, err)

	// Create a Teleport instance with a Proxy.
	proxyConfig := helpers.ProxyConfig{
		Name:                   "cluster-main-proxy",
		DisableWebService:      true,
		DisableALPNSNIListener: true,
	}
	proxyConfig.SSHAddr = helpers.NewListenerOn(t, teleport.Hostname, service.ListenerNodeSSH, &proxyConfig.FileDescriptors)
	proxyConfig.WebAddr = helpers.NewListenerOn(t, teleport.Hostname, service.ListenerProxyWeb, &proxyConfig.FileDescriptors)
	proxyConfig.KubeAddr = helpers.NewListenerOn(t, teleport.Hostname, service.ListenerProxyKube, &proxyConfig.FileDescriptors)
	proxyConfig.ReverseTunnelAddr = helpers.NewListenerOn(t, teleport.Hostname, service.ListenerProxyTunnel, &proxyConfig.FileDescriptors)

	_, _, err = teleport.StartProxy(proxyConfig, helpers.WithLegacyKubeProxy(suite.kubeConfigPath))
	require.NoError(t, err)

	t.Cleanup(func() {
		teleport.StopAll()
	})
	kubeAddr, err := utils.ParseAddr(proxyConfig.KubeAddr)
	require.NoError(t, err)
	// wait until the proxy and kube are ready
	require.Eventually(t, func() bool {
		// set up kube configuration using proxy
		proxyClient, _, err := kube.ProxyClient(kube.ProxyConfig{
			T:             teleport,
			Username:      adminUsername,
			KubeUsers:     kubeUsers,
			KubeGroups:    kubeGroups,
			TargetAddress: *kubeAddr,
		})
		if err != nil {
			return false
		}
		ctx := context.Background()
		// try get request to fetch available pods
		_, err = proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
		return err == nil
	}, 20*time.Second, 500*time.Millisecond)

	adminProxyClient, adminProxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:             teleport,
		Username:      adminUsername,
		KubeUsers:     kubeUsers,
		KubeGroups:    kubeGroups,
		TargetAddress: *kubeAddr,
	})
	require.NoError(t, err)

	userProxyClient, userProxyClientConfig, err := kube.ProxyClient(kube.ProxyConfig{
		T:             teleport,
		Username:      userUsername,
		KubeUsers:     kubeUsers,
		KubeGroups:    kubeGroups,
		TargetAddress: *kubeAddr,
	})
	require.NoError(t, err)

	// stop auth server to test that user with moderation is denied when no Auth exists.
	// Both admin and user already have valid certificates.
	require.NoError(t, teleport.StopAuth(true))
	tests := []struct {
		name           string
		user           string
		proxyClient    kubernetes.Interface
		clientConfig   *rest.Config
		assetErr       require.ErrorAssertionFunc
		outputContains string
	}{
		{
			name:           "admin user", // admin user does not require any additional moderation.
			proxyClient:    adminProxyClient,
			clientConfig:   adminProxyClientConfig,
			user:           adminUsername,
			assetErr:       require.NoError,
			outputContains: "echo hi",
		},
		{
			name:         "user with moderation", // user requires moderation and his session must be denied when no Auth exists.
			user:         userUsername,
			assetErr:     require.Error,
			proxyClient:  userProxyClient,
			clientConfig: userProxyClientConfig,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			// try get request to fetch available pods
			pod, err := tt.proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
			require.NoError(t, err)

			out := &bytes.Buffer{}
			// interactive command, allocate pty
			term := NewTerminal(250)
			// lets type "echo hi" followed by "enter" and then "exit" + "enter":
			term.Type("\aecho hi\n\r\aexit\n\r\a")
			err = kubeExec(tt.clientConfig, execInContainer, kubeExecArgs{
				podName:      pod.Name,
				podNamespace: pod.Namespace,
				container:    pod.Spec.Containers[0].Name,
				command:      []string{"/bin/sh"},
				stdout:       out,
				stdin:        term,
				tty:          true,
			})
			tt.assetErr(t, err)

			data := out.Bytes()
			require.Contains(t, string(data), tt.outputContains)
		})
	}
}
