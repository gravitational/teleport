import { TshdEventContextBridgeService } from 'teleterm/types';
import { IAppContext } from 'teleterm/ui/types';
import * as tshdEvents from 'teleterm/services/tshdEvents';

export function createTshdEventsContextBridgeService(
  ctx: IAppContext
): TshdEventContextBridgeService {
  return {
    relogin: async ({ request, onRequestCancelled }) => {
      await ctx.reloginService.relogin(
        request as tshdEvents.ReloginRequest,
        onRequestCancelled
      );
      return {};
    },

    sendNotification: async ({ request }) => {
      ctx.tshdNotificationsService.sendNotification(
        request as tshdEvents.SendNotificationRequest
      );

      return {};
    },

    sendPendingHeadlessAuthentication: async ({
      request,
      onRequestCancelled,
    }) => {
      await ctx.headlessAuthenticationService.sendPendingHeadlessAuthentication(
        request as tshdEvents.SendPendingHeadlessAuthenticationRequest,
        onRequestCancelled
      );
      return {};
    },

    promptMFA: async () => {
      throw new Error('Not implemented yet');
    },
  };
}
