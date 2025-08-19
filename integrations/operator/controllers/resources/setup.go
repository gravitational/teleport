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
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/lib/modules"
)

type reconcilerFactory struct {
	cr      string
	factory func(kclient.Client, *client.Client) (controllers.Reconciler, error)
}

// SetupAllControllers sets up all controllers
func SetupAllControllers(log logr.Logger, mgr manager.Manager, teleportClient *client.Client, features *proto.Features) error {
	reconcilers := []reconcilerFactory{
		{"TeleportRole", NewRoleReconciler},
		{"TeleportRoleV6", NewRoleV6Reconciler},
		{"TeleportRoleV7", NewRoleV7Reconciler},
		{"TeleportRoleV8", NewRoleV8Reconciler},
		{"TeleportUser", NewUserReconciler},
		{"TeleportGithubConnector", NewGithubConnectorReconciler},
		{"TeleportProvisionToken", NewProvisionTokenReconciler},
		{"TeleportOpenSSHServerV2", NewOpenSSHServerV2Reconciler},
		{"TeleportOpenSSHEICEServerV2", NewOpenSSHEICEServerV2Reconciler},
		{"TeleportTrustedClusterV2", NewTrustedClusterV2Reconciler},
		{"TeleportBotV1", NewBotV1Reconciler},
		{"TeleportWorkloadIdentityV1", NewWorkloadIdentityV1Reconciler},
		{"TeleportAutoupdateConfigV1", NewAutoUpdateConfigV1Reconciler},
		{"TeleportAutoupdateVersionV1", NewAutoUpdateVersionV1Reconciler},
		{"TeleportAppV3", NewAppV3Reconciler},
		{"TeleportDatabaseV3", NewDatabaseV3Reconciler},
	}

	oidc := modules.GetProtoEntitlement(features, entitlements.OIDC)
	saml := modules.GetProtoEntitlement(features, entitlements.SAML)

	if oidc.Enabled {
		reconcilers = append(reconcilers, reconcilerFactory{"TeleportOIDCConnector", NewOIDCConnectorReconciler})
	} else {
		log.Info("OIDC connectors are only available in Teleport Enterprise edition. TeleportOIDCConnector resources won't be reconciled")
	}

	if saml.Enabled {
		reconcilers = append(reconcilers, reconcilerFactory{"TeleportSAMLConnector", NewSAMLConnectorReconciler})
	} else {
		log.Info("SAML connectors are only available in Teleport Enterprise edition. TeleportSAMLConnector resources won't be reconciled")
	}

	// Login Rules are enterprise-only but there is no specific feature flag for them.
	if oidc.Enabled || saml.Enabled {
		reconcilers = append(reconcilers, reconcilerFactory{"TeleportLoginRule", NewLoginRuleReconciler})
	} else {
		log.Info("Login Rules are only available in Teleport Enterprise edition. TeleportLoginRule resources won't be reconciled")
	}

	// AccessLists, OktaImports are enterprise-only but there is no specific feature-flag for them.
	if features.GetAdvancedAccessWorkflows() {
		reconcilers = append(reconcilers, reconcilerFactory{"TeleportAccessList", NewAccessListReconciler})
		reconcilers = append(reconcilers, reconcilerFactory{"TeleportOktaImportRule", NewOktaImportRuleReconciler})
	} else {
		log.Info("The cluster license does not contain advanced workflows. TeleportAccessList, TeleportOktaImportRule resources won't be reconciled")
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
