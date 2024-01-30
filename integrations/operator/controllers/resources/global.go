/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package resources

import (
	"github.com/go-logr/logr"
	"github.com/gravitational/trace"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	resourcesv2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	resourcesv3 "github.com/gravitational/teleport/integrations/operator/apis/resources/v3"
	resourcesv5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
)

// Scheme is a singleton scheme for all controllers
var Scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(resourcesv5.AddToScheme(Scheme))
	utilruntime.Must(resourcesv3.AddToScheme(Scheme))
	utilruntime.Must(resourcesv2.AddToScheme(Scheme))
	utilruntime.Must(resourcesv1.AddToScheme(Scheme))

	// Not needed to reconcile the teleport CRs, but needed for the controller manager.
	// We are not doing something very kubernetes friendly, but it's easier to have a single
	// scheme rather than having to build and propagate schemes in multiple places, which
	// is error-prone and can lead to inconsistencies.
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(apiextv1.AddToScheme(Scheme))
}

type reconcilerFactory struct {
	cr      string
	factory func(kclient.Client, *client.Client) (Reconciler, error)
}

// Reconciler extends the reconcile.Reconciler interface by adding a
// SetupWithManager function that creates a controller in the given manager.
type Reconciler interface {
	reconcile.Reconciler
	SetupWithManager(mgr manager.Manager) error
}

func SetupAllControllers(log logr.Logger, mgr manager.Manager, teleportClient *client.Client, features *proto.Features) error {
	reconcilers := []reconcilerFactory{
		{"TeleportRole", NewRoleReconciler},
		{"TeleportRoleV6", NewRoleV6Reconciler},
		{"TeleportRoleV7", NewRoleV7Reconciler},
		{"TeleportUser", NewUserReconciler},
		{"TeleportGithubConnector", NewGithubConnectorReconciler},
		{"TeleportProvisionToken", NewProvisionTokenReconciler},
		{"TeleportOktaImportRule", NewOktaImportRuleReconciler},
	}

	if features.GetOIDC() {
		reconcilers = append(reconcilers, reconcilerFactory{"TeleportOIDCConnector", NewOIDCConnectorReconciler})
	} else {
		log.Info("OIDC connectors are only available in Teleport Enterprise edition. TeleportOIDCConnector resources won't be reconciled")
	}

	if features.GetSAML() {
		reconcilers = append(reconcilers, reconcilerFactory{"TeleportSAMLConnector", NewSAMLConnectorReconciler})
	} else {
		log.Info("SAML connectors are only available in Teleport Enterprise edition. TeleportSAMLConnector resources won't be reconciled")
	}

	// Login Rules are enterprise-only but there is no specific feature flag for them.
	if features.GetOIDC() || features.GetSAML() {
		reconcilers = append(reconcilers, reconcilerFactory{"TeleportLoginRule", NewLoginRuleReconciler})
	} else {
		log.Info("Login Rules are only available in Teleport Enterprise edition. TeleportLoginRule resources won't be reconciled")
	}

	// AccessLists are enterprise-only but there is no specific feature-flag for them.
	if features.GetAdvancedAccessWorkflows() {
		reconcilers = append(reconcilers, reconcilerFactory{"TeleportAccessList", NewAccessListReconciler})
	} else {
		log.Info("The cluster license does not contain advanced workflows. TeleportAccessList resources won't be reconciled")
	}

	kubeClient := mgr.GetClient()
	for _, reconciler := range reconcilers {
		r, err := reconciler.factory(kubeClient, teleportClient)
		if err != nil {
			return trace.Wrap(err, "failed to create controller for %s", reconciler.cr)
		}
		err = r.SetupWithManager(mgr)
		if err != nil {
			return trace.Wrap(err, "failed to setup controller for: %s", reconciler.cr)
		}
	}

	return nil
}
