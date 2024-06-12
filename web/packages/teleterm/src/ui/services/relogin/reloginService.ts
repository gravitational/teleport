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

import { MainProcessClient } from 'teleterm/types';
import { ReloginRequest } from 'teleterm/services/tshdEvents';
import {
  ModalsService,
  ClusterConnectReason,
} from 'teleterm/ui/services/modals';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { reloginReasonOneOfIsGatewayCertExpired } from 'teleterm/helpers';

export class ReloginService {
  constructor(
    private mainProcessClient: MainProcessClient,
    private modalsService: ModalsService,
    private clustersService: ClustersService
  ) {}

  relogin(
    request: ReloginRequest,
    onRequestCancelled: (callback: () => void) => void
  ): Promise<void> {
    this.mainProcessClient.forceFocusWindow();
    let reason: ClusterConnectReason;

    if (reloginReasonOneOfIsGatewayCertExpired(request.reason)) {
      const gateway = this.clustersService.findGateway(
        request.reason.gatewayCertExpired.gatewayUri
      );
      reason = {
        kind: 'reason.gateway-cert-expired',
        targetUri: request.reason.gatewayCertExpired.targetUri,
        gateway: gateway,
      };
    }

    return new Promise((resolve, reject) => {
      // GatewayCertReissuer in tshd makes sure that we only ever have one concurrent request to the
      // relogin event. So at the moment, ReloginService won't ever call openImportantDialog twice.
      const { closeDialog } = this.modalsService.openImportantDialog({
        kind: 'cluster-connect',
        clusterUri: request.rootClusterUri,
        reason,
        prefill: undefined,
        onSuccess: () => resolve(),
        onCancel: () =>
          reject(new Error('Login process was canceled by the user')),
      });

      onRequestCancelled(closeDialog);
    });
  }
}
