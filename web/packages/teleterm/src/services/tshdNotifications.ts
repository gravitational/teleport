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
