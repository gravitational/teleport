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
