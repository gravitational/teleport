/**
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

import { NotificationItemContent } from 'shared/components/Notification';

import {
  cannotProxyVnetConnectionReasonIsCertReissueError,
  cannotProxyVnetConnectionReasonIsInvalidLocalPort,
  notificationRequestOneOfIsCannotProxyGatewayConnection,
  notificationRequestOneOfIsCannotProxyVnetConnection,
} from 'teleterm/helpers';
import {
  formatPortRange,
  publicAddrWithTargetPort,
} from 'teleterm/services/tshd/app';
import { getTargetNameFromUri } from 'teleterm/services/tshd/gateway';
import { SendNotificationRequest } from 'teleterm/services/tshdEvents';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { ResourceUri, routing } from 'teleterm/ui/uri';

export class TshdNotificationsService {
  constructor(
    private notificationsService: NotificationsService,
    private clustersService: ClustersService
  ) {}

  sendNotification(request: SendNotificationRequest) {
    const notificationContent = this.getNotificationContent(request);
    this.notificationsService.notifyError(notificationContent);
  }

  private getNotificationContent(
    request: SendNotificationRequest
  ): NotificationItemContent {
    const { subject } = request;
    // switch followed by a type guard is awkward, but it helps with ensuring that we get type
    // errors whenever a new request reason is added.
    //
    // Type guards must be called because of how protobuf-ts generates types for oneOf in protos.
    switch (subject.oneofKind) {
      case 'cannotProxyGatewayConnection': {
        if (!notificationRequestOneOfIsCannotProxyGatewayConnection(subject)) {
          return;
        }
        const { gatewayUri, targetUri, error } =
          subject.cannotProxyGatewayConnection;
        const gateway = this.clustersService.findGateway(gatewayUri);
        const clusterName = this.getClusterName(targetUri);
        let targetName: string;
        let targetUser: string;
        let targetDesc: string;

        if (gateway) {
          targetName = gateway.targetName;
          targetUser = gateway.targetUser;
        } else {
          targetName = getTargetNameFromUri(targetUri);
        }

        if (targetUser) {
          targetDesc = `${targetName} as ${targetUser}`;
        } else {
          targetDesc = targetName;
        }

        return {
          title: `Cannot connect to ${targetDesc} (${clusterName})`,
          description: `A connection attempt to ${targetDesc} failed due to an unexpected error: ${error}`,
        };
      }
      case 'cannotProxyVnetConnection': {
        if (!notificationRequestOneOfIsCannotProxyVnetConnection(subject)) {
          return;
        }
        const { routeToApp, targetUri, reason } =
          subject.cannotProxyVnetConnection;
        const clusterName = this.getClusterName(targetUri);

        switch (reason.oneofKind) {
          case 'certReissueError': {
            if (!cannotProxyVnetConnectionReasonIsCertReissueError(reason)) {
              return;
            }
            const { error } = reason.certReissueError;

            return {
              title: `Cannot connect to ${publicAddrWithTargetPort(routeToApp)}`,
              description: `A connection attempt to the app in the cluster ${clusterName} failed due to an unexpected error: ${error}`,
            };
          }
          case 'invalidLocalPort': {
            if (!cannotProxyVnetConnectionReasonIsInvalidLocalPort(reason)) {
              return;
            }

            // Ports are not present if there's more than 10 port ranges defined on an app.
            const ports = reason.invalidLocalPort.tcpPorts
              .map(portRange => formatPortRange(portRange))
              .join(', ');

            let description =
              `A connection attempt on the port ${routeToApp.targetPort} was refused ` +
              `as that port is not included in the target ports of the app ${routeToApp.clusterName} in the cluster ${clusterName}.`;

            if (ports) {
              description += ` Valid ports: ${ports}.`;
            }

            return {
              title: `Invalid port for ${publicAddrWithTargetPort(routeToApp)}`,
              description,
              // 3rd-party clients can potentially make dozens of attempts to connect to an invalid
              // port within a short time. As all notifications from this service go as errors, we
              // don't want to force the user to manually close each notification.
              isAutoRemovable: true,
            };
          }
          default: {
            reason satisfies never;
            return;
          }
        }
      }
      default: {
        subject satisfies never;
        return;
      }
    }
  }

  private getClusterName(uri: ResourceUri): string {
    const clusterUri = routing.ensureClusterUri(uri);
    const cluster = this.clustersService.findCluster(clusterUri);

    return cluster ? cluster.name : routing.parseClusterName(clusterUri);
  }
}
