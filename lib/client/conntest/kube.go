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

package conntest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// KubeConnectionTesterConfig defines the config fields for KubeConnectionTester.
type KubeConnectionTesterConfig struct {
	// UserClient is an auth client that has a User's identity.
	UserClient authclient.ClientI

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

	mfaResponse, err := req.MFAResponse.GetOptionalMFAResponseProtoReq()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsCfg, err := s.genKubeRestTLSClientConfig(ctx, mfaResponse, connectionDiagnosticID, req.ResourceName, currentUser.GetName())
	diag, diagErr := s.handleUserGenCertsErr(ctx, req.ResourceName, connectionDiagnosticID, err)
	if err != nil || diagErr != nil {
		return diag, diagErr
	}

	client, err := s.getKubeClient(tlsCfg, req.KubernetesImpersonation)
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
func (s KubeConnectionTester) genKubeRestTLSClientConfig(ctx context.Context, mfaResponse *proto.MFAAuthenticateResponse, connectionDiagnosticID, clusterName, userName string) (rest.TLSClientConfig, error) {
	key, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(s.cfg.UserClient),
		cryptosuites.UserTLS)
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	privateKeyPEM, err := keys.MarshalPrivateKey(key)
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}
	publicKeyPEM, err := keys.MarshalPublicKey(key.Public())
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	certs, err := s.cfg.UserClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		TLSPublicKey:           publicKeyPEM,
		Username:               userName,
		Expires:                time.Now().Add(time.Minute).UTC(),
		ConnectionDiagnosticID: connectionDiagnosticID,
		KubernetesCluster:      clusterName,
		MFAResponse:            mfaResponse,
	})
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	ca, err := s.cfg.UserClient.GetClusterCACert(ctx)
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	return rest.TLSClientConfig{
		CAData:   ca.TLSCA,
		CertData: certs.TLS,
		KeyData:  privateKeyPEM,
	}, nil
}

// getKubeClient creates a Kubernetes client with the authentication given by tlsCfg
// to teleport Proxy or Kube proxy depending on whether tls routing is enabled.
// If custom impersonation values are provided, it also configures them to be used
// in the request.
func (s KubeConnectionTester) getKubeClient(tlsCfg rest.TLSClientConfig, impersonation KubernetesImpersonation) (kubernetes.Interface, error) {
	restConfig := &rest.Config{
		Host:            "https://" + s.cfg.KubernetesPublicProxyAddr,
		TLSClientConfig: tlsCfg,
		Impersonate: rest.ImpersonationConfig{
			UserName: impersonation.KubernetesUser,
			Groups:   impersonation.KubernetesGroups,
		},
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

	// WARNING: This message originates from the Teleport `kubernetes_service`.
	// Whenever there is a change to the message returned by `kubernetes_service`
	// you must also ensure backwards compatibility with the previous version of
	// Teleport, otherwise the connection test will fail if one of the kubernetes
	// agents is still running an older version.
	// For this reason, messages are not shared between this connection test and
	// `kubernetes_service` to force detection of incompatible messages.
	// Checks if the cluster is registered but offline.
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
		// WARNING: This message originates from the Teleport `kubernetes_service`.
		// Whenever there is a change to the message returned by `kubernetes_service`
		// you must also ensure backwards compatibility with the previous version of
		// Teleport, otherwise the connection test will fail if one of the kubernetes
		// agents is still running an older version.
		// For this reason, messages are not shared between this connection test and
		// `kubernetes_service` to force detection of incompatible messages.
		//
		noConfiguredGroups := strings.Contains(kubeErr.ErrStatus.Message, "Please ask cluster administrator to ensure your role has appropriate kubernetes_groups and kubernetes_users set.")
		if noConfiguredGroups {
			message := `User-associated roles do not configure "kubernetes_groups" or "kubernetes_users". Make sure that at least one is configured for the user.`
			traceType := types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL

			connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
			return connDiag, trace.Wrap(err)
		}
		// WARNING: This message originates from the Teleport `kubernetes_service`.
		// Whenever there is a change to the message returned by `kubernetes_service`
		// you must also ensure backwards compatibility with the previous version of
		// Teleport, otherwise the connection test will fail if one of the kubernetes
		// agents is still running an older version.
		// For this reason, messages are not shared between this connection test and
		// `kubernetes_service` to force detection of incompatible messages.
		multipleAssignedUsers := strings.Contains(kubeErr.ErrStatus.Message, "please select a user to impersonate, refusing to select a user due to several kubernetes_users set up for this user")
		if multipleAssignedUsers {
			message := `User-associated roles define multiple "kubernetes_users". Make sure that only one value is defined or that you select the target user.`
			traceType := types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL

			connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
			return connDiag, trace.Wrap(err)
		}

		// WARNING: This message originates from the Teleport `kubernetes_service`.
		// Whenever there is a change to the message returned by `kubernetes_service`
		// you must also ensure backwards compatibility with the previous version of
		// Teleport, otherwise the connection test will fail if one of the kubernetes
		// agents is still running an older version.
		// For this reason, messages are not shared between this connection test and
		// `kubernetes_service` to force detection of incompatible messages.
		unauthorizedUserImpersonation := strings.Contains(kubeErr.ErrStatus.Message, "impersonation request has been denied, user header")
		if unauthorizedUserImpersonation {
			message := `User-associated roles do now allow the desired "kubernetes_user" impersonation. Please define a "kubernetes_user" that your roles allow to impersonate.`
			traceType := types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL

			connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
			return connDiag, trace.Wrap(err)
		}

		// WARNING: This message originates from the Teleport `kubernetes_service`.
		// Whenever there is a change to the message returned by `kubernetes_service`
		// you must also ensure backwards compatibility with the previous version of
		// Teleport, otherwise the connection test will fail if one of the kubernetes
		// agents is still running an older version.
		// For this reason, messages are not shared between this connection test and
		// `kubernetes_service` to force detection of incompatible messages.
		unauthorizedGroupImpersonation := strings.Contains(kubeErr.ErrStatus.Message, "impersonation request has been denied, group header")
		if unauthorizedGroupImpersonation {
			message := `User-associated roles do now allow the desired "kubernetes_group" impersonation. Please define a "kubernetes_group" that your roles allow to impersonate.`
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
		// WARNING: This message originates from the Teleport `kubernetes_service`.
		// Whenever there is a change to the message returned by `kubernetes_service`
		// you must also ensure backwards compatibility with the previous version of
		// Teleport, otherwise the connection test will fail if one of the kubernetes
		// agents is still running an older version.
		// For this reason, messages are not shared between this connection test and
		// `kubernetes_service` to force detection of incompatible messages.
		notFound := strings.Contains(kubeErr.ErrStatus.Message, "not found")
		accessDenied := strings.Contains(kubeErr.ErrStatus.Message, "[00] access denied")
		if notFound || accessDenied {
			message := "You are not authorized to access this Kubernetes Cluster. Ensure your role grants access by adding it to the 'kubernetes_labels' property."
			traceType := types.ConnectionDiagnosticTrace_RBAC_KUBE
			connDiag, err := s.appendDiagnosticTrace(ctx, connectionDiagnosticID, traceType, message, actionErr)
			return connDiag, trace.Wrap(err)
		}

		// This is a Kubernetes RBAC error triggered when the kube user/groups do not
		// have permissions to list pods in the specified namespace.
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
