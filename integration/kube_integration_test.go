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
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	streamspdy "k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport"
	"k8s.io/client-go/transport/spdy"
)

var _ = check.Suite(&KubeSuite{})

type KubeSuite struct {
	*kubernetes.Clientset

	ports utils.PortList
	me    *user.User
	// priv/pub pair to avoid re-generating it
	priv []byte
	pub  []byte

	// kubeconfigPath is a path to valid kubeconfig
	kubeConfigPath string

	// kubeConfig is a kubernetes config struct
	kubeConfig *rest.Config
}

func (s *KubeSuite) SetUpSuite(c *check.C) {
	var err error
	utils.InitLoggerForTests(testing.Verbose())
	SetTestTimeouts(time.Millisecond * time.Duration(100))

	s.priv, s.pub, err = testauthority.New().GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	s.ports, err = utils.GetFreeTCPPorts(AllocatePortsNum, utils.PortStartingNumber+AllocatePortsNum+1)
	if err != nil {
		c.Fatal(err)
	}
	s.me, err = user.Current()
	c.Assert(err, check.IsNil)

	// close & re-open stdin because 'go test' runs with os.stdin connected to /dev/null
	stdin, err := os.Open("/dev/tty")
	if err != nil {
		os.Stdin.Close()
		os.Stdin = stdin
	}

	testEnabled := os.Getenv(teleport.KubeRunTests)
	if ok, _ := strconv.ParseBool(testEnabled); !ok {
		c.Skip("Skipping Kubernetes test suite.")
	}

	s.kubeConfigPath = os.Getenv(teleport.EnvKubeConfig)
	if s.kubeConfigPath == "" {
		c.Fatal("This test requires path to valid kubeconfig")
	}
	s.Clientset, s.kubeConfig, err = kubeutils.GetKubeClient(s.kubeConfigPath)
	c.Assert(err, check.IsNil)

	ns := newNamespace(testNamespace)
	_, err = s.CoreV1().Namespaces().Create(ns)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			c.Fatalf("Failed to create namespace: %v.", err)
		}
	}
}

const kubeSystemNamespace = "kube-system"

var kubeDNSLabels = labels.Set{"k8s-app": "kube-dns"}

func (s *KubeSuite) TearDownSuite(c *check.C) {
	var err error
	// restore os.Stdin to its original condition: connected to /dev/null
	os.Stdin.Close()
	os.Stdin, err = os.Open("/dev/null")
	c.Assert(err, check.IsNil)
}

// TestKubeExec tests kubernetes Exec command set
func (s *KubeSuite) TestKubeExec(c *check.C) {
	tconf := s.teleKubeConfig(Host)

	t := NewInstance(InstanceConfig{
		ClusterName: Site,
		HostID:      HostID,
		NodeName:    Host,
		Ports:       s.ports.PopIntSlice(5),
		Priv:        s.priv,
		Pub:         s.pub,
	})

	username := s.me.Username
	kubeGroups := []string{teleport.KubeSystemMasters}
	kubeUsers := []string{"alice@example.com"}
	role, err := services.NewRole("kubemaster", services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
			KubeUsers:  kubeUsers,
		},
	})
	c.Assert(err, check.IsNil)
	t.AddUserWithRole(username, role)

	err = t.CreateEx(nil, tconf)
	c.Assert(err, check.IsNil)

	err = t.Start()
	c.Assert(err, check.IsNil)
	defer t.StopAll()

	// impersonating client requests will be denied if the headers
	// are referencing users or groups not allowed by the existing roles
	impersonatingProxyClient, impersonatingProxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:             t,
		username:      username,
		kubeUsers:     kubeUsers,
		kubeGroups:    kubeGroups,
		impersonation: &rest.ImpersonationConfig{UserName: "bob", Groups: []string{"system: masters"}},
	})
	c.Assert(err, check.IsNil)

	// try get request to fetch available pods
	_, err = impersonatingProxyClient.CoreV1().Pods(kubeSystemNamespace).List(metav1.ListOptions{
		LabelSelector: kubeDNSLabels.AsSelector().String(),
	})
	c.Assert(err, check.NotNil)

	// scoped client requests will be allowed, as long as the impersonation headers
	// are referencing users and groups allowed by existing roles
	scopedProxyClient, scopedProxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:          t,
		username:   username,
		kubeUsers:  kubeUsers,
		kubeGroups: kubeGroups,
		impersonation: &rest.ImpersonationConfig{
			UserName: role.GetKubeUsers(services.Allow)[0],
			Groups:   role.GetKubeGroups(services.Allow),
		},
	})
	c.Assert(err, check.IsNil)

	_, err = scopedProxyClient.CoreV1().Pods(kubeSystemNamespace).List(metav1.ListOptions{
		LabelSelector: kubeDNSLabels.AsSelector().String(),
	})
	c.Assert(err, check.IsNil)

	// set up kube configuration using proxy
	proxyClient, proxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:          t,
		username:   username,
		kubeUsers:  kubeUsers,
		kubeGroups: kubeGroups,
	})
	c.Assert(err, check.IsNil)

	// try get request to fetch available pods
	pods, err := proxyClient.CoreV1().Pods(kubeSystemNamespace).List(metav1.ListOptions{
		LabelSelector: kubeDNSLabels.AsSelector().String(),
	})
	c.Assert(len(pods.Items), check.Not(check.Equals), int(0))
	c.Assert(err, check.IsNil)

	// Exec through proxy and collect output
	pod := pods.Items[0]

	out := &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/cat", "/var/run/secrets/kubernetes.io/serviceaccount/namespace"},
		stdout:       out,
	})
	c.Assert(err, check.IsNil)

	data := out.Bytes()
	c.Assert(string(data), check.Equals, pod.Namespace)

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
	c.Assert(err, check.IsNil)

	// verify the session stream output
	sessionStream := out.String()
	comment := check.Commentf("%q", sessionStream)
	c.Assert(strings.Contains(sessionStream, "echo hi"), check.Equals, true, comment)
	c.Assert(strings.Contains(sessionStream, "exit"), check.Equals, true, comment)

	// verify traffic capture and upload, wait for the upload to hit
	var sessionID string
	timeoutC := time.After(10 * time.Second)
loop:
	for {
		select {
		case event := <-t.UploadEventsC:
			sessionID = event.SessionID
			break loop
		case <-timeoutC:
			c.Fatalf("Timeout waiting for upload of session to complete")
		}
	}

	// read back the entire session and verify that it matches the stated output
	capturedStream, err := t.Process.GetAuthServer().GetSessionChunk(defaults.Namespace, session.ID(sessionID), 0, events.MaxChunkBytes)
	c.Assert(err, check.IsNil)

	c.Assert(string(capturedStream), check.Equals, sessionStream)

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
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Matches, ".*impersonation request has been denied.*")

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
	c.Assert(err, check.IsNil)
}

// TestKubeDeny makes sure that deny rule conflicting with allow
// rule takes precedence
func (s *KubeSuite) TestKubeDeny(c *check.C) {
	tconf := s.teleKubeConfig(Host)

	t := NewInstance(InstanceConfig{
		ClusterName: Site,
		HostID:      HostID,
		NodeName:    Host,
		Ports:       s.ports.PopIntSlice(5),
		Priv:        s.priv,
		Pub:         s.pub,
	})

	username := s.me.Username
	kubeGroups := []string{teleport.KubeSystemMasters}
	kubeUsers := []string{"alice@example.com"}
	role, err := services.NewRole("kubemaster", services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
			KubeUsers:  kubeUsers,
		},
		Deny: services.RoleConditions{
			KubeGroups: kubeGroups,
			KubeUsers:  kubeUsers,
		},
	})
	c.Assert(err, check.IsNil)
	t.AddUserWithRole(username, role)

	err = t.CreateEx(nil, tconf)
	c.Assert(err, check.IsNil)

	err = t.Start()
	c.Assert(err, check.IsNil)
	defer t.StopAll()

	// set up kube configuration using proxy
	proxyClient, _, err := kubeProxyClient(kubeProxyConfig{
		t:          t,
		username:   username,
		kubeUsers:  kubeUsers,
		kubeGroups: kubeGroups,
	})
	c.Assert(err, check.IsNil)

	// try get request to fetch available pods
	_, err = proxyClient.CoreV1().Pods(kubeSystemNamespace).List(metav1.ListOptions{
		LabelSelector: kubeDNSLabels.AsSelector().String(),
	})
	c.Assert(err, check.NotNil)
}

// TestKubePortForward tests kubernetes port forwarding
func (s *KubeSuite) TestKubePortForward(c *check.C) {
	tconf := s.teleKubeConfig(Host)

	t := NewInstance(InstanceConfig{
		ClusterName: Site,
		HostID:      HostID,
		NodeName:    Host,
		Ports:       s.ports.PopIntSlice(5),
		Priv:        s.priv,
		Pub:         s.pub,
	})

	username := s.me.Username
	kubeGroups := []string{teleport.KubeSystemMasters}
	role, err := services.NewRole("kubemaster", services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
		},
	})
	c.Assert(err, check.IsNil)
	t.AddUserWithRole(username, role)

	err = t.CreateEx(nil, tconf)
	c.Assert(err, check.IsNil)

	err = t.Start()
	c.Assert(err, check.IsNil)
	defer t.StopAll()

	// set up kube configuration using proxy
	_, proxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:          t,
		username:   username,
		kubeGroups: kubeGroups,
	})
	c.Assert(err, check.IsNil)

	// pick the first kube-dns pod and run port forwarding on it
	pods, err := s.CoreV1().Pods(kubeSystemNamespace).List(metav1.ListOptions{
		LabelSelector: kubeDNSLabels.AsSelector().String(),
	})
	c.Assert(err, check.IsNil)
	c.Assert(len(pods.Items), check.Not(check.Equals), int(0))

	pod := pods.Items[0]

	// forward local port to target port 53 of the dnsmasq container
	localPort := s.ports.Pop()

	forwarder, err := newPortForwarder(proxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:53", localPort)},
		podName:      pod.Name,
		podNamespace: pod.Namespace,
	})
	c.Assert(err, check.IsNil)
	go func() {
		err := forwarder.ForwardPorts()
		if err != nil {
			c.Fatalf("Forward ports exited with error: %v.", err)
		}
	}()

	select {
	case <-time.After(5 * time.Second):
		c.Fatalf("Timeout waiting for port forwarding.")
	case <-forwarder.readyC:
	}
	defer close(forwarder.stopC)

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return net.Dial("tcp", fmt.Sprintf("localhost:%v", localPort))
		},
	}
	addr, err := resolver.LookupHost(context.TODO(), "kubernetes.default.svc.cluster.local")
	c.Assert(err, check.IsNil)
	c.Assert(len(addr), check.Not(check.Equals), 0)

	// impersonating client requests will be denied
	_, impersonatingProxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:             t,
		username:      username,
		kubeGroups:    kubeGroups,
		impersonation: &rest.ImpersonationConfig{UserName: "bob", Groups: []string{"system: masters"}},
	})
	c.Assert(err, check.IsNil)

	localPort = s.ports.Pop()
	impersonatingForwarder, err := newPortForwarder(impersonatingProxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:53", localPort)},
		podName:      pod.Name,
		podNamespace: pod.Namespace,
	})
	c.Assert(err, check.IsNil)

	// This request should be denied
	err = impersonatingForwarder.ForwardPorts()
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Matches, ".*impersonation request has been denied.*")
}

// TestKubeTrustedClustersClientCert tests scenario with trusted clusters
// using metadata encoded in the certificate
func (s *KubeSuite) TestKubeTrustedClustersClientCert(c *check.C) {
	ctx := context.Background()
	clusterMain := "cluster-main"
	mainConf := s.teleKubeConfig(Host)
	// Main cluster doesn't need a kubeconfig to forward requests to auxiliary
	// cluster.
	mainConf.Proxy.Kube.KubeconfigPath = ""
	main := NewInstance(InstanceConfig{
		ClusterName: clusterMain,
		HostID:      HostID,
		NodeName:    Host,
		Ports:       s.ports.PopIntSlice(5),
		Priv:        s.priv,
		Pub:         s.pub,
	})

	// main cluster has a role and user called main-kube
	username := s.me.Username
	mainKubeGroups := []string{teleport.KubeSystemMasters}
	mainRole, err := services.NewRole("main-kube", services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins:     []string{username},
			KubeGroups: mainKubeGroups,
		},
	})
	c.Assert(err, check.IsNil)
	main.AddUserWithRole(username, mainRole)

	clusterAux := "cluster-aux"
	auxConf := s.teleKubeConfig(Host)
	aux := NewInstance(InstanceConfig{
		ClusterName: clusterAux,
		HostID:      HostID,
		NodeName:    Host,
		Ports:       s.ports.PopIntSlice(5),
		Priv:        s.priv,
		Pub:         s.pub,
	})

	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	mainConf.Proxy.Kube.Enabled = true
	err = main.CreateEx(nil, mainConf)
	c.Assert(err, check.IsNil)

	err = aux.CreateEx(nil, auxConf)
	c.Assert(err, check.IsNil)

	// auxiliary cluster has a role aux-kube
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "aux-kube" to local role "main-kube"
	auxKubeGroups := []string{teleport.TraitInternalKubeGroupsVariable}
	auxRole, err := services.NewRole("aux-kube", services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins: []string{username},
			// Note that main cluster can pass it's kubernetes groups
			// to the remote cluster, and remote cluster
			// can choose to use them by using special variable
			KubeGroups: auxKubeGroups,
		},
	})
	c.Assert(err, check.IsNil)
	err = aux.Process.GetAuthServer().UpsertRole(ctx, auxRole)
	c.Assert(err, check.IsNil)
	trustedClusterToken := "trusted-clsuter-token"
	err = main.Process.GetAuthServer().UpsertToken(
		services.MustCreateProvisionToken(trustedClusterToken, []teleport.Role{teleport.RoleTrustedCluster}, time.Time{}))
	c.Assert(err, check.IsNil)
	trustedCluster := main.Secrets.AsTrustedCluster(trustedClusterToken, services.RoleMap{
		{Remote: mainRole.GetName(), Local: []string{auxRole.GetName()}},
	})
	c.Assert(err, check.IsNil)

	// start both clusters
	err = main.Start()
	c.Assert(err, check.IsNil)
	defer main.StopAll()

	err = aux.Start()
	c.Assert(err, check.IsNil)
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
			c.Fatalf("got non connection problem %v", err)
		}
		upsertSuccess = true
		break
	}
	// make sure we upsert a trusted cluster
	c.Assert(upsertSuccess, check.Equals, true)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(main.Tunnel.GetSites()) < 2 && len(aux.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two clusters do not see each other: tunnels are not working")
		}
	}

	// impersonating client requests will be denied
	impersonatingProxyClient, impersonatingProxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:              main,
		username:       username,
		kubeGroups:     mainKubeGroups,
		impersonation:  &rest.ImpersonationConfig{UserName: "bob", Groups: []string{"system: masters"}},
		routeToCluster: clusterAux,
	})
	c.Assert(err, check.IsNil)

	// try get request to fetch available pods
	_, err = impersonatingProxyClient.CoreV1().Pods(kubeSystemNamespace).List(metav1.ListOptions{
		LabelSelector: kubeDNSLabels.AsSelector().String(),
	})
	c.Assert(err, check.NotNil)

	// set up kube configuration using main proxy
	proxyClient, proxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:              main,
		username:       username,
		kubeGroups:     mainKubeGroups,
		routeToCluster: clusterAux,
	})
	c.Assert(err, check.IsNil)

	// try get request to fetch available pods
	pods, err := proxyClient.CoreV1().Pods(kubeSystemNamespace).List(metav1.ListOptions{
		LabelSelector: kubeDNSLabels.AsSelector().String(),
	})
	c.Assert(err, check.IsNil)
	c.Assert(len(pods.Items), check.Not(check.Equals), int(0))

	// Exec through proxy and collect output
	pod := pods.Items[0]

	out := &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/cat", "/var/run/secrets/kubernetes.io/serviceaccount/namespace"},
		stdout:       out,
	})
	c.Assert(err, check.IsNil)

	data := out.Bytes()
	c.Assert(string(data), check.Equals, pod.Namespace)

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
	c.Assert(err, check.IsNil)

	// verify the session stream output
	sessionStream := out.String()
	comment := check.Commentf("%q", sessionStream)
	c.Assert(strings.Contains(sessionStream, "echo hi"), check.Equals, true, comment)
	c.Assert(strings.Contains(sessionStream, "exit"), check.Equals, true, comment)

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
			c.Fatalf("Timeout waiting for upload of session to complete")
		}
	}

	// read back the entire session and verify that it matches the stated output
	capturedStream, err := main.Process.GetAuthServer().GetSessionChunk(defaults.Namespace, session.ID(sessionID), 0, events.MaxChunkBytes)
	c.Assert(err, check.IsNil)

	c.Assert(string(capturedStream), check.Equals, sessionStream)

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
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Matches, ".*impersonation request has been denied.*")

	// forward local port to target port 53 of the dnsmasq container
	localPort := s.ports.Pop()

	forwarder, err := newPortForwarder(proxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:53", localPort)},
		podName:      pod.Name,
		podNamespace: pod.Namespace,
	})
	c.Assert(err, check.IsNil)
	go func() {
		err := forwarder.ForwardPorts()
		if err != nil {
			c.Fatalf("Forward ports exited with error: %v.", err)
		}
	}()

	select {
	case <-time.After(5 * time.Second):
		c.Fatalf("Timeout waiting for port forwarding.")
	case <-forwarder.readyC:
	}
	defer close(forwarder.stopC)

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return net.Dial("tcp", fmt.Sprintf("localhost:%v", localPort))
		},
	}
	addr, err := resolver.LookupHost(ctx, "kubernetes.default.svc.cluster.local")
	c.Assert(err, check.IsNil)
	c.Assert(len(addr), check.Not(check.Equals), 0)

	// impersonating client requests will be denied
	localPort = s.ports.Pop()
	impersonatingForwarder, err := newPortForwarder(impersonatingProxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:53", localPort)},
		podName:      pod.Name,
		podNamespace: pod.Namespace,
	})
	c.Assert(err, check.IsNil)

	// This request should be denied
	err = impersonatingForwarder.ForwardPorts()
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Matches, ".*impersonation request has been denied.*")

}

// TestKubeTrustedClustersSNI tests scenario with trusted clsuters
// using SNI-forwarding
// DELETE IN(4.3.0)
func (s *KubeSuite) TestKubeTrustedClustersSNI(c *check.C) {
	ctx := context.Background()

	clusterMain := "cluster-main"
	mainConf := s.teleKubeConfig(Host)
	main := NewInstance(InstanceConfig{
		ClusterName: clusterMain,
		HostID:      HostID,
		NodeName:    Host,
		Ports:       s.ports.PopIntSlice(5),
		Priv:        s.priv,
		Pub:         s.pub,
	})

	// main cluster has a role and user called main-kube
	username := s.me.Username
	mainKubeGroups := []string{teleport.KubeSystemMasters}
	mainRole, err := services.NewRole("main-kube", services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins:     []string{username},
			KubeGroups: mainKubeGroups,
		},
	})
	c.Assert(err, check.IsNil)
	main.AddUserWithRole(username, mainRole)

	clusterAux := "cluster-aux"
	auxConf := s.teleKubeConfig(Host)
	aux := NewInstance(InstanceConfig{
		ClusterName: clusterAux,
		HostID:      HostID,
		NodeName:    Host,
		Ports:       s.ports.PopIntSlice(5),
		Priv:        s.priv,
		Pub:         s.pub,
	})

	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	// route all the traffic to the aux cluster
	mainConf.Proxy.Kube.Enabled = true
	// ClusterOverride forces connection to be routed
	// to cluster aux
	mainConf.Proxy.Kube.ClusterOverride = clusterAux
	err = main.CreateEx(nil, mainConf)
	c.Assert(err, check.IsNil)

	err = aux.CreateEx(nil, auxConf)
	c.Assert(err, check.IsNil)

	// auxiliary cluster has a role aux-kube
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "aux-kube" to local role "main-kube"
	auxKubeGroups := []string{teleport.TraitInternalKubeGroupsVariable}
	auxRole, err := services.NewRole("aux-kube", services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins: []string{username},
			// Note that main cluster can pass it's kubernetes groups
			// to the remote cluster, and remote cluster
			// can choose to use them by using special variable
			KubeGroups: auxKubeGroups,
		},
	})
	c.Assert(err, check.IsNil)
	err = aux.Process.GetAuthServer().UpsertRole(ctx, auxRole)
	c.Assert(err, check.IsNil)
	trustedClusterToken := "trusted-clsuter-token"
	err = main.Process.GetAuthServer().UpsertToken(
		services.MustCreateProvisionToken(trustedClusterToken, []teleport.Role{teleport.RoleTrustedCluster}, time.Time{}))
	c.Assert(err, check.IsNil)
	trustedCluster := main.Secrets.AsTrustedCluster(trustedClusterToken, services.RoleMap{
		{Remote: mainRole.GetName(), Local: []string{auxRole.GetName()}},
	})
	c.Assert(err, check.IsNil)

	// start both clusters
	err = main.Start()
	c.Assert(err, check.IsNil)
	defer main.StopAll()

	err = aux.Start()
	c.Assert(err, check.IsNil)
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
			c.Fatalf("got non connection problem %v", err)
		}
		upsertSuccess = true
		break
	}
	// make sure we upsert a trusted cluster
	c.Assert(upsertSuccess, check.Equals, true)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(main.Tunnel.GetSites()) < 2 && len(aux.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two clusters do not see each other: tunnels are not working")
		}
	}

	// impersonating client requests will be denied
	impersonatingProxyClient, impersonatingProxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:             main,
		username:      username,
		kubeGroups:    mainKubeGroups,
		impersonation: &rest.ImpersonationConfig{UserName: "bob", Groups: []string{"system: masters"}},
	})
	c.Assert(err, check.IsNil)

	// try get request to fetch available pods
	_, err = impersonatingProxyClient.CoreV1().Pods(kubeSystemNamespace).List(metav1.ListOptions{
		LabelSelector: kubeDNSLabels.AsSelector().String(),
	})
	c.Assert(err, check.NotNil)

	// set up kube configuration using main proxy
	proxyClient, proxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:          main,
		username:   username,
		kubeGroups: mainKubeGroups,
	})
	c.Assert(err, check.IsNil)

	// try get request to fetch available pods
	pods, err := proxyClient.CoreV1().Pods(kubeSystemNamespace).List(metav1.ListOptions{
		LabelSelector: kubeDNSLabels.AsSelector().String(),
	})
	c.Assert(err, check.IsNil)
	c.Assert(len(pods.Items), check.Not(check.Equals), int(0))

	// Exec through proxy and collect output
	pod := pods.Items[0]

	out := &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/cat", "/var/run/secrets/kubernetes.io/serviceaccount/namespace"},
		stdout:       out,
	})
	c.Assert(err, check.IsNil)

	data := out.Bytes()
	c.Assert(string(data), check.Equals, pod.Namespace)

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
	c.Assert(err, check.IsNil)

	// verify the session stream output
	sessionStream := out.String()
	comment := check.Commentf("%q", sessionStream)
	c.Assert(strings.Contains(sessionStream, "echo hi"), check.Equals, true, comment)
	c.Assert(strings.Contains(sessionStream, "exit"), check.Equals, true, comment)

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
			c.Fatalf("Timeout waiting for upload of session to complete")
		}
	}

	// read back the entire session and verify that it matches the stated output
	capturedStream, err := main.Process.GetAuthServer().GetSessionChunk(defaults.Namespace, session.ID(sessionID), 0, events.MaxChunkBytes)
	c.Assert(err, check.IsNil)

	c.Assert(string(capturedStream), check.Equals, sessionStream)

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
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Matches, ".*impersonation request has been denied.*")

	// forward local port to target port 53 of the dnsmasq container
	localPort := s.ports.Pop()

	forwarder, err := newPortForwarder(proxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:53", localPort)},
		podName:      pod.Name,
		podNamespace: pod.Namespace,
	})
	c.Assert(err, check.IsNil)
	go func() {
		err := forwarder.ForwardPorts()
		if err != nil {
			c.Fatalf("Forward ports exited with error: %v.", err)
		}
	}()

	select {
	case <-time.After(5 * time.Second):
		c.Fatalf("Timeout waiting for port forwarding.")
	case <-forwarder.readyC:
	}
	defer close(forwarder.stopC)

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return net.Dial("tcp", fmt.Sprintf("localhost:%v", localPort))
		},
	}
	addr, err := resolver.LookupHost(context.TODO(), "kubernetes.default.svc.cluster.local")
	c.Assert(err, check.IsNil)
	c.Assert(len(addr), check.Not(check.Equals), 0)

	// impersonating client requests will be denied
	localPort = s.ports.Pop()
	impersonatingForwarder, err := newPortForwarder(impersonatingProxyClientConfig, kubePortForwardArgs{
		ports:        []string{fmt.Sprintf("%v:53", localPort)},
		podName:      pod.Name,
		podNamespace: pod.Namespace,
	})
	c.Assert(err, check.IsNil)

	// This request should be denied
	err = impersonatingForwarder.ForwardPorts()
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Matches, ".*impersonation request has been denied.*")

}

// TestKubeDisconnect tests kubernetes session disconnects
func (s *KubeSuite) TestKubeDisconnect(c *check.C) {
	testCases := []disconnectTestCase{
		{
			options: services.RoleOptions{
				ClientIdleTimeout: services.NewDuration(500 * time.Millisecond),
			},
			disconnectTimeout: 2 * time.Second,
		},
		{
			options: services.RoleOptions{
				DisconnectExpiredCert: services.NewBool(true),
				MaxSessionTTL:         services.NewDuration(3 * time.Second),
			},
			disconnectTimeout: 6 * time.Second,
		},
	}
	for i := 0; i < utils.GetIterations(); i++ {
		for _, tc := range testCases {
			s.runKubeDisconnectTest(c, tc)
		}
	}
}

// TestKubeDisconnect tests kubernetes session disconnects
func (s *KubeSuite) runKubeDisconnectTest(c *check.C, tc disconnectTestCase) {
	tconf := s.teleKubeConfig(Host)

	t := NewInstance(InstanceConfig{
		ClusterName: Site,
		HostID:      HostID,
		NodeName:    Host,
		Ports:       s.ports.PopIntSlice(5),
		Priv:        s.priv,
		Pub:         s.pub,
	})

	username := s.me.Username
	kubeGroups := []string{teleport.KubeSystemMasters}
	role, err := services.NewRole("kubemaster", services.RoleSpecV3{
		Options: tc.options,
		Allow: services.RoleConditions{
			Logins:     []string{username},
			KubeGroups: kubeGroups,
		},
	})
	c.Assert(err, check.IsNil)
	t.AddUserWithRole(username, role)

	err = t.CreateEx(nil, tconf)
	c.Assert(err, check.IsNil)

	err = t.Start()
	c.Assert(err, check.IsNil)
	defer t.StopAll()

	// set up kube configuration using proxy
	proxyClient, proxyClientConfig, err := kubeProxyClient(kubeProxyConfig{
		t:          t,
		username:   username,
		kubeGroups: kubeGroups,
	})
	c.Assert(err, check.IsNil)

	// try get request to fetch available pods
	pods, err := proxyClient.CoreV1().Pods(kubeSystemNamespace).List(metav1.ListOptions{
		LabelSelector: kubeDNSLabels.AsSelector().String(),
	})
	c.Assert(err, check.IsNil)
	c.Assert(len(pods.Items), check.Not(check.Equals), int(0))

	// Exec through proxy and collect output
	pod := pods.Items[0]

	out := &bytes.Buffer{}
	err = kubeExec(proxyClientConfig, kubeExecArgs{
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		container:    pod.Spec.Containers[0].Name,
		command:      []string{"/bin/cat", "/var/run/secrets/kubernetes.io/serviceaccount/namespace"},
		stdout:       out,
	})
	c.Assert(err, check.IsNil)

	data := out.Bytes()
	c.Assert(string(data), check.Equals, pod.Namespace)

	// interactive command, allocate pty
	term := NewTerminal(250)
	sessionCtx, sessionCancel := context.WithCancel(context.TODO())
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
		c.Assert(err, check.IsNil)
	}()

	// lets type something followed by "enter" and then hang the session
	enterInput(c, term, "echo boring platapus\r\n", ".*boring platapus.*")
	time.Sleep(tc.disconnectTimeout)
	select {
	case <-time.After(tc.disconnectTimeout):
		c.Fatalf("timeout waiting for session to exit")
	case <-sessionCtx.Done():
		// session closed
	}
}

// teleKubeConfig sets up teleport with kubernetes turned on
func (s *KubeSuite) teleKubeConfig(hostname string) *service.Config {
	tconf := service.MakeDefaultConfig()
	tconf.SSH.Enabled = true
	tconf.Proxy.DisableWebInterface = true
	tconf.PollingPeriod = 500 * time.Millisecond
	tconf.ClientTimeout = time.Second
	tconf.ShutdownTimeout = 2 * tconf.ClientTimeout

	// set kubernetes specific parameters
	tconf.Proxy.Kube.Enabled = true
	tconf.Proxy.Kube.ListenAddr.Addr = net.JoinHostPort(hostname, s.ports.Pop())
	tconf.Proxy.Kube.KubeconfigPath = s.kubeConfigPath

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
	t              *TeleInstance
	username       string
	kubeUsers      []string
	kubeGroups     []string
	impersonation  *rest.ImpersonationConfig
	routeToCluster string
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

	ca, err := authServer.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	tlsCA, err := ca.TLSCA()
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
		CAData:   ca.GetTLSKeyPairs()[0].Cert,
		CertData: cert,
		KeyData:  privPEM,
	}
	config := &rest.Config{
		Host:            "https://" + cfg.t.Config.Proxy.Kube.ListenAddr.Addr,
		TLSClientConfig: tlsClientConfig,
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
)

func newNamespace(name string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
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

	upgradeRoundTripper := streamspdy.NewSpdyRoundTripper(tlsConfig, true, false)
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
