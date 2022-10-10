/*
Copyright 2022 Gravitational, Inc.

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

package conntest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// KubeConnectionTesterConfig defines the config fields for KubeConnectionTester.
type KubeConnectionTesterConfig struct {
	// UserClient is an auth client that has a User's identity.
	UserClient auth.ClientI

	// ProxyHostPort is the proxy to use in the `--proxy` format (host:webPort,sshPort)
	ProxyHostPort string

	// KubernetesPublicProxyAddr is the kubernetes proxy address.
	KubernetesPublicProxyAddr string

	// TLSRoutingEnabled indicates that proxy supports ALPN SNI server where
	// all proxy services are exposed on a single TLS listener (Proxy Web Listener).
	TLSRoutingEnabled bool
}

// KubeConnectionTester implements the ConnectionTester interface for Testing Kubernetes access.
type KubeConnectionTester struct {
	cfg          KubeConnectionTesterConfig
	webProxyAddr string
}

// NewKubeConnectionTester returns a new KubeConnectionTester
func NewKubeConnectionTester(cfg KubeConnectionTesterConfig) (*KubeConnectionTester, error) {
	parsedProxyHostAddr, err := client.ParseProxyHost(cfg.ProxyHostPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &KubeConnectionTester{
		cfg:          cfg,
		webProxyAddr: parsedProxyHostAddr.WebProxyAddr,
	}, nil
}

// TestConnection tests an Kubernetes Access to the target Kubernetes Cluster using
//   - the provided client
//   - resource name
//
// A new ConnectionDiagnostic is created and used to store the traces as it goes through the checkpoints
// To connect to the KubeCluster, we will create a cert-key pair and setup a Kubernetes client back to Teleport Proxy.
// We report the following cases:
//   - trace of whether the Kubernetes cluster is reachable
//   - trace of whether the User Role defines Kubernetes principals for the cluster: `kubernetes_groups` & `kubernetes_users`
//   - trace of whether the User role has access to the desired kubernetes cluster: `kubernetes_labels` allow access.
//   - trace of weather the cluster is accessible and we can list pods on the desired namespace.
func (s *KubeConnectionTester) TestConnection(ctx context.Context, req TestConnectionRequest) (types.ConnectionDiagnostic, error) {
	if req.ResourceKind != types.KindKubernetesCluster {
		return nil, trace.BadParameter("invalid value for ResourceKind, expected %q got %q", types.KindKubernetesCluster, req.ResourceKind)
	}

	connectionDiagnosticID := uuid.NewString()
	connectionDiagnostic, err := types.NewConnectionDiagnosticV1(connectionDiagnosticID, map[string]string{},
		types.ConnectionDiagnosticSpecV1{
			// We start with a failed state so that we don't need to set it to each return statement once an error is returned.
			// if the test reaches the end, we force the test to be a success by calling
			// 	connectionDiagnostic.SetMessage(types.DiagnosticMessageSuccess)
			//	connectionDiagnostic.SetSuccess(true)
			Message: types.DiagnosticMessageFailed,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.cfg.UserClient.CreateConnectionDiagnostic(ctx, connectionDiagnostic); err != nil {
		return nil, trace.Wrap(err)
	}

	currentUser, err := s.cfg.UserClient.GetCurrentUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsCfg, err := s.genKubeRestTLSClientConfig(ctx, connectionDiagnosticID, req.ResourceName, currentUser.GetName())
	diag, diagErr := s.handleUserGenCertsErr(ctx, req.ResourceName, connectionDiagnosticID, err)
	if err != nil || diagErr != nil {
		return diag, diagErr
	}

	client, err := s.getKubeClient(tlsCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctxWithTimeout, cancelFunc := context.WithTimeout(ctx, req.DialTimeout)
	defer cancelFunc()
	_, err = client.CoreV1().Pods(req.KubernetesNamespace).List(ctxWithTimeout, v1.ListOptions{})
	diag, diagErr = s.handleErrFromKube(ctx, req.ResourceName, connectionDiagnosticID, err, req.KubernetesNamespace)
	if err != nil || diagErr != nil {
		return diag, diagErr
	}

	traceType := types.ConnectionDiagnosticTrace_KUBE_PRINCIPAL
	message := "Access to the Kubernetes Cluster granted."
	connDiag, err := s.appendDiagnosticTrace(ctxWithTimeout, connectionDiagnosticID, traceType, message, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connDiag.SetMessage(types.DiagnosticMessageSuccess)
	connDiag.SetSuccess(true)

	if err := s.cfg.UserClient.UpdateConnectionDiagnostic(ctxWithTimeout, connDiag); err != nil {
		return nil, trace.Wrap(err)
	}

	return connDiag, nil
}

// genKubeRestTLSClientConfig creates the Teleport user credentials to access
// the given Kubernetes cluster name.
func (s KubeConnectionTester) genKubeRestTLSClientConfig(ctx context.Context, connectionDiagnosticID string, clusterName, userName string) (rest.TLSClientConfig, error) {
	key, err := client.GenerateRSAKey()
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	certs, err := s.cfg.UserClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey:              key.MarshalSSHPublicKey(),
		Username:               userName,
		Expires:                time.Now().Add(time.Minute).UTC(),
		ConnectionDiagnosticID: connectionDiagnosticID,
		KubernetesCluster:      clusterName,
	})
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	key.TLSCert = certs.TLS

	ca, err := s.cfg.UserClient.GetClusterCACert(ctx)
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	return rest.TLSClientConfig{
		CAData:   ca.TLSCA,
		CertData: key.TLSCert,
		KeyData:  key.PrivateKeyPEM(),
	}, nil
}

// getKubeClient creates a Kubernetes client with the authentication given by tlsCfg
// to teleport Proxy or Kube proxy depending on whether tls routing is enabled.
func (s KubeConnectionTester) getKubeClient(tlsCfg rest.TLSClientConfig) (kubernetes.Interface, error) {
	restConfig := &rest.Config{
		Host:            "https://" + s.cfg.KubernetesPublicProxyAddr,
		TLSClientConfig: tlsCfg,
	}

	if s.cfg.TLSRoutingEnabled {
		// passing an empty string to GetKubeTLSServerName results in
		// a server name = kube.teleport.cluster.local.
		restConfig.TLSClientConfig.ServerName = client.GetKubeTLSServerName("")
		restConfig.Host = "https://" + s.webProxyAddr
	}

	client, err := kubernetes.NewForConfig(restConfig)
	return client, trace.Wrap(err)
}

// handleErrFromKube parses the errors received from the Teleport when generating
// user credentials to access the cluster.
func (s KubeConnectionTester) handleUserGenCertsErr(ctx context.Context, clusterName string, connectionDiagnosticID string, actionErr error) (types.ConnectionDiagnostic, error) {
	if trace.IsBadParameter(actionErr) {
		message := "Failed to connect to Kubernetes cluster. Ensure the cluster is registered and online."
		traceType := types.ConnectionDiagnosticTrace_CONNECTIVITY
		err := fmt.Errorf("kubernetes cluster %q is not registered or is offline", clusterName)
		return s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, err)
	} else if actionErr != nil {
		return nil, trace.Wrap(actionErr)
	}
	// success message is delayed until we reach kube proxy since the agent can be
	// registered but unreachable
	return nil, nil
}

// handleErrFromKube parses the errors received from the Teleport and marks the
// steps according to the given error.
func (s KubeConnectionTester) handleErrFromKube(ctx context.Context, clusterName string, connectionDiagnosticID string, actionErr error, namespace string) (types.ConnectionDiagnostic, error) {
	var kubeErr *kubeerrors.StatusError
	if actionErr != nil && !errors.As(actionErr, &kubeErr) {
		traceType := types.ConnectionDiagnosticTrace_UNKNOWN_ERROR
		message := fmt.Sprintf("Unknown error. %v", actionErr)
		connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
		return connDiag, trace.Wrap(err)
	}

	// check the the cluster is registered but offline
	// WARNING: Check compatibility between this error message in the current version of
	// Teleport and the previous version so that old agents connected to the
	// Teleport cluster continue to be supported.
	if kubeErr != nil && strings.Contains(kubeErr.ErrStatus.Message, "This usually means that the agent is offline or has disconnected") {
		message := "Failed to connect to Kubernetes cluster. Ensure the cluster is registered and online."
		traceType := types.ConnectionDiagnosticTrace_CONNECTIVITY
		err := fmt.Errorf("kubernetes cluster %q is not registered or is offline", clusterName)
		return s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, err)
	}

	message := "Kubernetes Cluster is registered in Teleport."
	traceType := types.ConnectionDiagnosticTrace_CONNECTIVITY
	s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, nil)

	if kubeErr != nil {
		// WARNING: Check compatibility between this error message in the current version of
		// Teleport and the previous version so that old agents connected to the
		// Teleport cluster continue to be supported.
		noAssignedGroups := strings.Contains(kubeErr.ErrStatus.Message, "has no assigned groups or users")
		if noAssignedGroups {
			message := `User-associated roles do not configure "kubernetes_groups" or "kubernetes_users". Make sure that at least one is configured for the user.`
			traceType := types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL

			connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
			return connDiag, trace.Wrap(err)
		}
	}
	message = "User-associated roles define valid Kubernetes principals."
	traceType = types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL

	_, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if kubeErr != nil {
		// WARNING: Check compatibility between this error messages in the current version of
		// Teleport and the previous version so that old agents connected to the
		// Teleport cluster continue to be supported.
		notFound := strings.Contains(kubeErr.ErrStatus.Message, "not found")
		accessDenied := strings.Contains(kubeErr.ErrStatus.Message, "[00] access denied")
		if notFound || accessDenied {
			message := "You are not authorized to access this Kubernetes Cluster. Ensure your role grants access by adding it to the 'kubernetes_labels' property."
			traceType := types.ConnectionDiagnosticTrace_RBAC_KUBE
			connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
			return connDiag, trace.Wrap(err)
		}

		//  this is a kubernetes RBAC error
		cannotListPods := strings.Contains(kubeErr.ErrStatus.Message, "cannot list resource \"pods\"")
		if cannotListPods {
			message := fmt.Sprintf("You are not allowed to list pods in the %q namespace. "+
				"Make sure your \"kubernetes_groups\" or \"kubernetes_users\" exist in the cluster and grant you access to list pods.", namespace)
			traceType := types.ConnectionDiagnosticTrace_KUBE_PRINCIPAL
			connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
			return connDiag, trace.Wrap(err)
		}

		// return unknown error if an error is still present.
		traceType := types.ConnectionDiagnosticTrace_UNKNOWN_ERROR
		message := fmt.Sprintf("Unknown error. %v", actionErr)
		connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
		return connDiag, trace.Wrap(err)
	}

	message = "You are authorized to access this Kubernetes Cluster."
	traceType = types.ConnectionDiagnosticTrace_RBAC_KUBE

	connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connDiag, nil
}

func (s KubeConnectionTester) appendDiagnosticTrace(ctx context.Context, connectionDiagnosticID string, traceType types.ConnectionDiagnosticTrace_TraceType, message string, err error) (types.ConnectionDiagnostic, error) {
	connDiag, err := s.cfg.UserClient.AppendDiagnosticTrace(
		ctx,
		connectionDiagnosticID,
		types.NewTraceDiagnosticConnection(
			traceType,
			message,
			err,
		))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return connDiag, nil
}
