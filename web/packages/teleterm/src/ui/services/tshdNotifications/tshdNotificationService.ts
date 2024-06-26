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

import { getTargetNameFromUri } from 'teleterm/services/tshd/gateway';
import { SendNotificationRequest } from 'teleterm/services/tshdEvents';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { routing } from 'teleterm/ui/uri';
import {
  notificationRequestOneOfIsCannotProxyGatewayConnection,
  notificationRequestOneOfIsCannotProxyVnetConnection,
} from 'teleterm/helpers';

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
        const clusterName = routing.parseClusterName(targetUri);
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
        const { publicAddr, targetUri, error } =
          subject.cannotProxyVnetConnection;
        const clusterName = routing.parseClusterName(targetUri);

        return {
          title: `Cannot connect to ${publicAddr}`,
          description: `A connection attempt to the app in the cluster ${clusterName} failed due to an unexpected error: ${error}`,
        };
      }
      default: {
        subject satisfies never;
        return;
      }
    }
  }
}
