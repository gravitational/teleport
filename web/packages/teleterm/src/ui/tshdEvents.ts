/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
