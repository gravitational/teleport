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

import { MainProcessClient } from 'teleterm/types';
import { ReloginRequest } from 'teleterm/services/tshdEvents';
import {
  ModalsService,
  ClusterConnectReason,
} from 'teleterm/ui/services/modals';
import { ClustersService } from 'teleterm/ui/services/clusters';

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

    if (request.gatewayCertExpired) {
      const gateway = this.clustersService.findGateway(
        request.gatewayCertExpired.gatewayUri
      );
      reason = {
        kind: 'reason.gateway-cert-expired',
        targetUri: request.gatewayCertExpired.targetUri,
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
        onSuccess: () => resolve(),
        onCancel: () =>
          reject(new Error('Login process was canceled by the user')),
      });

      onRequestCancelled(closeDialog);
    });
  }
}
