/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { SendNotificationRequest } from 'teleterm/services/tshdEvents';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { routing } from 'teleterm/ui/uri';

export class TshdNotificationsService {
  constructor(
    private notificationsService: NotificationsService,
    private clustersService: ClustersService
  ) {}

  sendNotification(request: SendNotificationRequest) {
    if (request.cannotProxyGatewayConnection) {
      const { gatewayUri, targetUri, error } =
        request.cannotProxyGatewayConnection;
      const gateway = this.clustersService.findGateway(gatewayUri);
      const clusterName = routing.parseClusterName(targetUri);
      let shortTargetDesc: string;
      let longTargetDesc: string;

      if (gateway) {
        shortTargetDesc = `${gateway.targetName} as ${gateway.targetUser}`;
        longTargetDesc = shortTargetDesc;
      } else {
        const targetName = routing.parseDbUri(targetUri)?.params['dbId'];

        if (targetName) {
          shortTargetDesc = targetName;
          longTargetDesc = shortTargetDesc;
        } else {
          shortTargetDesc = 'a database server';
          longTargetDesc = `a database server under ${targetUri}`;
        }
      }

      const notificationContent = {
        title: `Cannot connect to ${shortTargetDesc} (${clusterName})`,
        description: `You tried to connect to ${longTargetDesc} but we encountered an unexpected error: ${error}`,
      };

      this.notificationsService.notifyError(notificationContent);
    }
  }
}
