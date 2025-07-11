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
	"github.com/gravitational/teleport/integrations/operator/controllers"
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
		{"TeleportUser", NewUserReconciler},
		{"TeleportGithubConnector", NewGithubConnectorReconciler},
		{"TeleportProvisionToken", NewProvisionTokenReconciler},
		{"TeleportOpenSSHServerV2", NewOpenSSHServerV2Reconciler},
		{"TeleportOpenSSHEICEServerV2", NewOpenSSHEICEServerV2Reconciler},
		{"TeleportTrustedClusterV2", NewTrustedClusterV2Reconciler},
		{"TeleportBotV1", NewBotV1Reconciler},
		{"TeleportWorkloadIdentityV1", NewWorkloadIdentityV1Reconciler},
	}

	reconcilers = append(reconcilers, reconcilerFactory{"TeleportOIDCConnector", NewOIDCConnectorReconciler})
	reconcilers = append(reconcilers, reconcilerFactory{"TeleportSAMLConnector", NewSAMLConnectorReconciler})
	reconcilers = append(reconcilers, reconcilerFactory{"TeleportLoginRule", NewLoginRuleReconciler})
	reconcilers = append(reconcilers, reconcilerFactory{"TeleportAccessList", NewAccessListReconciler})
	reconcilers = append(reconcilers, reconcilerFactory{"TeleportOktaImportRule", NewOktaImportRuleReconciler})

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
