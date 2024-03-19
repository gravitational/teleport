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

import { getTargetNameFromUri } from 'teleterm/services/tshd/gateway';
import { SendNotificationRequest } from 'teleterm/services/tshdEvents';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { routing } from 'teleterm/ui/uri';
import { notificationRequestOneOfIsCannotProxyGatewayConnection } from 'teleterm/helpers';

export class TshdNotificationsService {
  constructor(
    private notificationsService: NotificationsService,
    private clustersService: ClustersService
  ) {}

  sendNotification(request: SendNotificationRequest) {
    if (
      notificationRequestOneOfIsCannotProxyGatewayConnection(request.subject)
    ) {
      const { gatewayUri, targetUri, error } =
        request.subject.cannotProxyGatewayConnection;
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

      const notificationContent = {
        title: `Cannot connect to ${targetDesc} (${clusterName})`,
        description: `You tried to connect to ${targetDesc} but we encountered an unexpected error: ${error}`,
      };

      this.notificationsService.notifyError(notificationContent);
    }
  }
}
