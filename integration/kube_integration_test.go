/*
Copyright 2016-2020 Gravitational, Inc.

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
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport"
	"k8s.io/client-go/transport/spdy"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	streamspdy "k8s.io/apimachinery/pkg/util/httpstream/spdy"
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
	SetTestTimeouts(time.Millisecond * time.Duration(100))

	suite.priv, suite.pub, err = testauthority.New().GenerateKeyPair("")
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

	return suite
}

// For this test suite to work, the target Kubernetes cluster must have the
// following RBAC objects configured:
// https://github.com/gravitational/teleport/blob/master/fixtures/ci-teleport-rbac/ci-teleport.yaml
const testImpersonationGroup = "teleport-ci-test-group"

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
}

// TestKubeExec tests kubernetes Exec command set
func testKubeExec(t *testing.T, suite *KubeSuite) {
	tconf := suite.teleKubeConfig(Host)

	teleport := NewInstance(InstanceConfig{
		ClusterName: Site,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		log:         suite.log,
	})

	username := suite.me.Username
	kubeGroups := []string{testImpersonationGroup}
	kubeUsers := []string{"alice@example.com"}
	role, err := types.NewRole("kubemaster", types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins:     []string{username},
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

	// impersonating client requests will be denied if the headers
	// are referencing users or groups not allowed by the existing roles
	impersonatingProxyClient, impersonatingProxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:             teleport,
		username:      username,
		kubeUsers:     kubeUsers,
		kubeGroups:    kubeGroups,
		impersonation: &rest.ImpersonationConfig{UserName: "bob", Groups: []string{testImpersonationGroup}},
	})
	require.NoError(t, err)

	// try get request to fetch a pod
	ctx := context.Background()
	_, err = impersonatingProxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.Error(t, err)

	// scoped client requests will be allowed, as long as the impersonation headers
	// are referencing users and groups allowed by existing roles
	scopedProxyClient, scopedProxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:          teleport,
		username:   username,
		kubeUsers:  kubeUsers,
		kubeGroups: kubeGroups,
		impersonation: &rest.ImpersonationConfig{
			UserName: role.GetKubeUsers(types.Allow)[0],
			Groups:   role.GetKubeGroups(types.Allow),
		},
	})
	require.NoError(t, err)

	_, err = scopedProxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.NoError(t, err)

	// set up kube configuration using proxy
	proxyClient, proxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:          teleport,
		username:   username,
		kubeUsers:  kubeUsers,
		kubeGroups: kubeGroups,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	pod, err := proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, kubeExecArgs{
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
	err = kubeExec(proxyClientConfig, kubeExecArgs{
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
	err = kubeExec(impersonatingProxyClientConfig, kubeExecArgs{
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
	err = kubeExec(scopedProxyClientConfig, kubeExecArgs{
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

// TestKubeDeny makes sure that deny rule conflicting with allow
// rule takes precedence
func testKubeDeny(t *testing.T, suite *KubeSuite) {
	tconf := suite.teleKubeConfig(Host)

	teleport := NewInstance(InstanceConfig{
		ClusterName: Site,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		log:         suite.log,
	})

	username := suite.me.Username
	kubeGroups := []string{testImpersonationGroup}
	kubeUsers := []string{"alice@example.com"}
	role, err := types.NewRole("kubemaster", types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
			KubeUsers:  kubeUsers,
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
	proxyClient, _, err := kubeProxyClient(kubeProxyConfig{
		t:          teleport,
		username:   username,
		kubeUsers:  kubeUsers,
		kubeGroups: kubeGroups,
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

	teleport := NewInstance(InstanceConfig{
		ClusterName: Site,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		log:         suite.log,
	})

	username := suite.me.Username
	kubeGroups := []string{testImpersonationGroup}
	role, err := types.NewRole("kubemaster", types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
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
	_, proxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:          teleport,
		username:   username,
		kubeGroups: kubeGroups,
	})
	require.NoError(t, err)

	// forward local port to target port 80 of the nginx container
	localPort := ports.Pop()

	forwarder, err := newPortForwarder(proxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:80", localPort)},
		podName:      testPod,
		podNamespace: testNamespace,
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
	_, impersonatingProxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:             teleport,
		username:      username,
		kubeGroups:    kubeGroups,
		impersonation: &rest.ImpersonationConfig{UserName: "bob", Groups: []string{testImpersonationGroup}},
	})
	require.NoError(t, err)

	localPort = ports.Pop()
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
	main := NewInstance(InstanceConfig{
		ClusterName: clusterMain,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		log:         suite.log,
	})

	// main cluster has a role and user called main-kube
	username := suite.me.Username
	mainKubeGroups := []string{testImpersonationGroup}
	mainRole, err := types.NewRole("main-kube", types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: mainKubeGroups,
		},
	})
	require.NoError(t, err)
	main.AddUserWithRole(username, mainRole)

	clusterAux := "cluster-aux"
	auxConf := suite.teleKubeConfig(Host)
	aux := NewInstance(InstanceConfig{
		ClusterName: clusterAux,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		log:         suite.log,
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
	auxKubeGroups := []string{teleport.TraitInternalKubeGroupsVariable}
	auxRole, err := types.NewRole("aux-kube", types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{username},
			// Note that main cluster can pass it's kubernetes groups
			// to the remote cluster, and remote cluster
			// can choose to use them by using special variable
			KubeGroups: auxKubeGroups,
		},
	})
	require.NoError(t, err)
	err = aux.Process.GetAuthServer().UpsertRole(ctx, auxRole)
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
	waitForClusters(t, main.Tunnel, 1, 10*time.Second)
	waitForClusters(t, aux.Tunnel, 1, 10*time.Second)

	// impersonating client requests will be denied
	impersonatingProxyClient, impersonatingProxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:              main,
		username:       username,
		kubeGroups:     mainKubeGroups,
		impersonation:  &rest.ImpersonationConfig{UserName: "bob", Groups: []string{testImpersonationGroup}},
		routeToCluster: clusterAux,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	_, err = impersonatingProxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.Error(t, err)

	// set up kube configuration using main proxy
	proxyClient, proxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:              main,
		username:       username,
		kubeGroups:     mainKubeGroups,
		routeToCluster: clusterAux,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	pod, err := proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, kubeExecArgs{
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
	err = kubeExec(proxyClientConfig, kubeExecArgs{
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
	err = kubeExec(impersonatingProxyClientConfig, kubeExecArgs{
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
	localPort := ports.Pop()

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
	localPort = ports.Pop()
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

// TestKubeTrustedClustersSNI tests scenario with trusted clusters
// using SNI-forwarding
// DELETE IN(4.3.0)
func testKubeTrustedClustersSNI(t *testing.T, suite *KubeSuite) {
	ctx := context.Background()

	clusterMain := "cluster-main"
	mainConf := suite.teleKubeConfig(Host)
	main := NewInstance(InstanceConfig{
		ClusterName: clusterMain,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		log:         suite.log,
	})

	// main cluster has a role and user called main-kube
	username := suite.me.Username
	mainKubeGroups := []string{testImpersonationGroup}
	mainRole, err := types.NewRole("main-kube", types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: mainKubeGroups,
		},
	})
	require.NoError(t, err)
	main.AddUserWithRole(username, mainRole)

	clusterAux := "cluster-aux"
	auxConf := suite.teleKubeConfig(Host)
	aux := NewInstance(InstanceConfig{
		ClusterName: clusterAux,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		log:         suite.log,
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
	auxKubeGroups := []string{teleport.TraitInternalKubeGroupsVariable}
	auxRole, err := types.NewRole("aux-kube", types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{username},
			// Note that main cluster can pass it's kubernetes groups
			// to the remote cluster, and remote cluster
			// can choose to use them by using special variable
			KubeGroups: auxKubeGroups,
		},
	})
	require.NoError(t, err)
	err = aux.Process.GetAuthServer().UpsertRole(ctx, auxRole)
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
	waitForClusters(t, main.Tunnel, 1, 10*time.Second)
	waitForClusters(t, aux.Tunnel, 1, 10*time.Second)

	// impersonating client requests will be denied
	impersonatingProxyClient, impersonatingProxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:             main,
		username:      username,
		kubeGroups:    mainKubeGroups,
		impersonation: &rest.ImpersonationConfig{UserName: "bob", Groups: []string{testImpersonationGroup}},
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	_, err = impersonatingProxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.Error(t, err)

	// set up kube configuration using main proxy
	proxyClient, proxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:          main,
		username:   username,
		kubeGroups: mainKubeGroups,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	pod, err := proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, kubeExecArgs{
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
	err = kubeExec(proxyClientConfig, kubeExecArgs{
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
	err = kubeExec(impersonatingProxyClientConfig, kubeExecArgs{
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
	localPort := ports.Pop()

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
	localPort = ports.Pop()
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
			options: types.RoleOptions{
				ClientIdleTimeout: types.NewDuration(500 * time.Millisecond),
			},
			disconnectTimeout: 2 * time.Second,
		},
		{
			options: types.RoleOptions{
				DisconnectExpiredCert: types.NewBool(true),
				MaxSessionTTL:         types.NewDuration(3 * time.Second),
			},
			disconnectTimeout: 6 * time.Second,
		},
	}
	for i := 0; i < utils.GetIterations(); i++ {
		for j, tc := range testCases {
			t.Run(fmt.Sprintf("#%02d_iter_%d", j, i), func(t *testing.T) {
				runKubeDisconnectTest(t, suite, tc)
			})
		}
	}
}

// TestKubeDisconnect tests kubernetes session disconnects
func runKubeDisconnectTest(t *testing.T, suite *KubeSuite, tc disconnectTestCase) {
	tconf := suite.teleKubeConfig(Host)

	teleport := NewInstance(InstanceConfig{
		ClusterName: Site,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		log:         suite.log,
	})

	username := suite.me.Username
	kubeGroups := []string{testImpersonationGroup}
	role, err := types.NewRole("kubemaster", types.RoleSpecV4{
		Options: tc.options,
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
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
	proxyClient, proxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:          teleport,
		username:   username,
		kubeGroups: kubeGroups,
	})
	require.NoError(t, err)

	// try get request to fetch available pods
	ctx := context.Background()
	pod, err := proxyClient.CoreV1().Pods(testNamespace).Get(ctx, testPod, metav1.GetOptions{})
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, kubeExecArgs{
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
		err := kubeExec(proxyClientConfig, kubeExecArgs{
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
	require.NoError(t, enterInput(sessionCtx, term, "echo boring platapus\r\n", ".*boring platapus.*"))
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

	teleport := NewInstance(InstanceConfig{
		ClusterName: Site,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		log:         suite.log,
	})

	username := suite.me.Username
	kubeGroups := []string{testImpersonationGroup}
	role, err := types.NewRole("kubemaster", types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
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
	_, proxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:          teleport,
		username:   username,
		kubeGroups: kubeGroups,
	})
	require.NoError(t, err)

	u, err := url.Parse(proxyClientConfig.Host)
	require.NoError(t, err)

	u.Scheme = "https"
	u.Path = fmt.Sprintf("/api/v1/namespaces/%v/pods/%v", testNamespace, testPod)

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
	require.Equal(t, resp1.Proto, "HTTP/1.1")

	// call proxy with an HTTP2 client
	err = http2.ConfigureTransport(trans)
	require.NoError(t, err)

	resp2, err := client.Get(u.String())
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, resp2.Proto, "HTTP/2.0")
}

// teleKubeConfig sets up teleport with kubernetes turned on
func (s *KubeSuite) teleKubeConfig(hostname string) *service.Config {
	tconf := service.MakeDefaultConfig()
	tconf.Console = nil
	tconf.Log = s.log
	tconf.SSH.Enabled = true
	tconf.Proxy.DisableWebInterface = true
	tconf.PollingPeriod = 500 * time.Millisecond
	tconf.ClientTimeout = time.Second
	tconf.ShutdownTimeout = 2 * tconf.ClientTimeout

	// set kubernetes specific parameters
	tconf.Proxy.Kube.Enabled = true
	tconf.Proxy.Kube.ListenAddr.Addr = net.JoinHostPort(hostname, ports.Pop())
	tconf.Proxy.Kube.KubeconfigPath = s.kubeConfigPath
	tconf.Proxy.Kube.LegacyKubeProxy = true

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

	tlsConfig := &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	tlsConfig.BuildNameToCertificate()
	return tlsConfig, nil
}

type kubeProxyConfig struct {
	t                   *TeleInstance
	username            string
	kubeUsers           []string
	kubeGroups          []string
	impersonation       *rest.ImpersonationConfig
	routeToCluster      string
	customTLSServerName string
	targetAddress       utils.NetAddr
}

// kubeProxyClient returns kubernetes client using local teleport proxy
func kubeProxyClient(cfg kubeProxyConfig) (*kubernetes.Clientset, *rest.Config, error) {
	authServer := cfg.t.Process.GetAuthServer()
	clusterName, err := authServer.GetClusterName()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Fetch user info to get roles and max session TTL.
	user, err := authServer.GetUser(cfg.username, false)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	roles, err := services.FetchRoles(user.GetRoles(), authServer, user.GetTraits())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	ttl := roles.AdjustSessionTTL(10 * time.Minute)

	ca, err := authServer.GetCertAuthority(types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	caCert, signer, err := authServer.GetKeyStore().GetTLSCertAndSigner(ca)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	tlsCA, err := tlsca.FromCertAndSigner(caCert, signer)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	privPEM, _, err := authServer.GenerateKeyPair("")
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	priv, err := tlsca.ParsePrivateKeyPEM(privPEM)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	id := tlsca.Identity{
		Username:         cfg.username,
		Groups:           user.GetRoles(),
		KubernetesUsers:  cfg.kubeUsers,
		KubernetesGroups: cfg.kubeGroups,
		RouteToCluster:   cfg.routeToCluster,
	}
	subj, err := id.Subject()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	cert, err := tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     authServer.GetClock(),
		PublicKey: priv.Public(),
		Subject:   subj,
		NotAfter:  authServer.GetClock().Now().Add(ttl),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	tlsClientConfig := rest.TLSClientConfig{
		CAData:     ca.GetActiveKeys().TLS[0].Cert,
		CertData:   cert,
		KeyData:    privPEM,
		ServerName: cfg.customTLSServerName,
	}
	config := &rest.Config{
		Host:            "https://" + cfg.t.Config.Proxy.Kube.ListenAddr.Addr,
		TLSClientConfig: tlsClientConfig,
	}
	if !cfg.targetAddress.IsEmpty() {
		config.Host = "https://" + cfg.targetAddress.Addr
	}
	if cfg.impersonation != nil {
		config.Impersonate = *cfg.impersonation
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return client, config, nil
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

	upgradeRoundTripper := streamspdy.NewRoundTripper(tlsConfig, true, false)
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

// kubeExec executes command against kubernetes API server
func kubeExec(kubeConfig *rest.Config, args kubeExecArgs) error {
	query := make(url.Values)
	for _, arg := range args.command {
		query.Add("command", arg)
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
		args.stderr = ioutil.Discard
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
	u.Path = fmt.Sprintf("/api/v1/namespaces/%v/pods/%v/exec", args.podNamespace, args.podName)
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
	return executor.Stream(opts)
}
