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

	// KubernetesPublicProxyHostPort is the kubernetes proxy.
	KubernetesPublicProxyHostPort string

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
			// We start with a failed state so that we don't need an extra update when returning non-happy paths.
			// For the happy path, we do update the Message to Success.
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

	key, err := client.GenerateRSAKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := s.cfg.UserClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey:              key.MarshalSSHPublicKey(),
		Username:               currentUser.GetName(),
		Expires:                time.Now().Add(time.Minute).UTC(),
		ConnectionDiagnosticID: connectionDiagnosticID,
		KubernetesCluster:      req.ResourceName,
	})

	diag, diagErr := s.handleUserGenCertsErr(ctx, connectionDiagnosticID, err)
	if err != nil || diagErr != nil {
		return diag, diagErr
	}

	key.TLSCert = certs.TLS

	certAuths, err := s.cfg.UserClient.GetCertAuthorities(ctx, types.HostCA, false /* loadKeys */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.TrustedCA = auth.AuthoritiesToTrustedCerts(certAuths)
	ca, err := s.cfg.UserClient.GetClusterCACert(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctxWithTimeout, cancelFunc := context.WithTimeout(ctx, req.DialTimeout)
	defer cancelFunc()

	restConfig := &rest.Config{
		Host: "https://" + s.cfg.KubernetesPublicProxyHostPort,
		TLSClientConfig: rest.TLSClientConfig{
			CAData:   ca.TLSCA,
			CertData: key.TLSCert,
			KeyData:  key.PrivateKeyPEM(),
		},
	}

	if s.cfg.TLSRoutingEnabled {
		k8host := strings.Split(s.webProxyAddr, ":")[0]
		// replace localhost with "127.0.0.1" so GetKubeTLSServerName can generate a domain kube.cluster.local.
		if strings.EqualFold(k8host, "localhost") {
			k8host = "127.0.0.1"
		}
		restConfig.TLSClientConfig.ServerName = client.GetKubeTLSServerName(k8host)
		restConfig.Host = "https://" + s.webProxyAddr
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = client.CoreV1().Pods(req.KubernetesNamespace).List(ctxWithTimeout, v1.ListOptions{})
	diag, diagErr = s.handleErrFromKube(ctxWithTimeout, connectionDiagnosticID, err, req.KubernetesNamespace)
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

func (s KubeConnectionTester) handleErrFromKube(ctx context.Context, connectionDiagnosticID string, actionErr error, namespace string) (types.ConnectionDiagnostic, error) {
	var kubeErr *kubeerrors.StatusError
	if actionErr != nil && !errors.As(actionErr, &kubeErr) {
		traceType := types.ConnectionDiagnosticTrace_UNKNOWN_ERROR
		message := fmt.Sprintf("Unknown error. %v", actionErr)
		connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
		return connDiag, trace.Wrap(err)
	}

	if kubeErr != nil && strings.Contains(kubeErr.ErrStatus.Message, "has no assigned groups or users") {
		message := `User-associated roles do not configure "kubernetes_groups" or "kubernetes_users". Make sure that at least one is configured for the user.`
		traceType := types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL

		connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
		return connDiag, trace.Wrap(err)
	}

	message := "User-associated roles define valid Kubernetes principals."
	traceType := types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL

	_, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if kubeErr != nil && (strings.Contains(kubeErr.ErrStatus.Message, "not found") || strings.Contains(kubeErr.ErrStatus.Message, "[00] access denied")) {
		message := "You are not authorized to access this Kubernetes Cluster. Ensure your role grants access by adding it to the 'kubernetes_labels' property."
		traceType := types.ConnectionDiagnosticTrace_RBAC_KUBE
		connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
		return connDiag, trace.Wrap(err)
	}

	if kubeErr != nil && strings.Contains(kubeErr.ErrStatus.Message, "cannot list resource \"pods\"") {
		message := fmt.Sprintf("You are not allowed to list pods in the %q namespace. "+
			"Make sure your \"kubernetes_groups\" or \"kubernetes_users\" exist in the cluster and grant you access to list pods.", namespace)
		traceType := types.ConnectionDiagnosticTrace_KUBE_PRINCIPAL
		connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
		return connDiag, trace.Wrap(err)
	}

	if kubeErr != nil {
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

func (s KubeConnectionTester) handleUserGenCertsErr(ctx context.Context, connectionDiagnosticID string, actionErr error) (types.ConnectionDiagnostic, error) {
	if trace.IsBadParameter(actionErr) {
		message := "Failed to connect to kubernetes cluster. Ensure the cluster is registered."
		traceType := types.ConnectionDiagnosticTrace_CONNECTIVITY
		return s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
	} else if actionErr != nil {
		return nil, trace.Wrap(actionErr)
	}
	message := "Kubernetes Cluster is registered in Teleport."
	traceType := types.ConnectionDiagnosticTrace_CONNECTIVITY
	return s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, nil)
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
