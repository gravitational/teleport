// Teleport
// Copyright (C) 2023  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package kube

import (
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tlsca"
)

var log = logrus.WithFields(logrus.Fields{
	teleport.ComponentKey: teleport.ComponentKubeClient,
})

// CheckIfCertsAreAllowedToAccessCluster evaluates if the new cert created by the user
// to access kubeCluster has at least one kubernetes_user or kubernetes_group
// defined. If not, it returns an error.
// This is a safety check in order to print a better message to the user even
// before hitting Teleport Kubernetes Proxy.
func CheckIfCertsAreAllowedToAccessCluster(k *client.KeyRing, rootCluster, teleportCluster, kubeCluster string) error {
	// This is a safety check in order to print a better message to the user even
	// before hitting Teleport Kubernetes Proxy.
	// We only enforce this check for root clusters, since we don't have knowledge
	// of the RBAC role mappings for remote clusters.
	if rootCluster != teleportCluster {
		return nil
	}
	for k8sCluster, cert := range k.KubeTLSCerts {
		if k8sCluster != kubeCluster {
			continue
		}
		log.Debugf("Got TLS cert for Kubernetes cluster %q", k8sCluster)
		exist, err := checkIfCertHasKubeGroupsAndUsers(cert)
		if err != nil {
			return trace.Wrap(err)
		} else if exist {
			return nil
		}
	}
	errMsg := "Your user's Teleport role does not allow Kubernetes access." +
		" Please ask cluster administrator to ensure your role has appropriate kubernetes_groups and kubernetes_users set."
	return trace.AccessDenied(errMsg)
}

// checkIfCertHasKubeGroupsAndUsers checks if the certificate has Kubernetes groups or users
// in the Subject Name. If it does, it returns true, otherwise false.
// Having no Kubernetes groups or users in the certificate means that the user
// is not allowed to access the Kubernetes cluster since Kubernetes Access enforces
// the presence of at least one of Kubernetes groups or users in the certificate.
// If the certificate does not have any Kubernetes groups or users, the
func checkIfCertHasKubeGroupsAndUsers(certB []byte) (bool, error) {
	cert, err := tlsca.ParseCertificatePEM(certB)
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, name := range cert.Subject.Names {
		if name.Type.Equal(tlsca.KubeGroupsASN1ExtensionOID) || name.Type.Equal(tlsca.KubeUsersASN1ExtensionOID) {
			return true, nil
		}
	}
	return false, nil
}
