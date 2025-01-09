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

import { PromptHardwareKeyPINChangeResponse } from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb';

import Logger from 'teleterm/logger';
import { TshdEventContextBridgeService } from 'teleterm/types';
import { IAppContext } from 'teleterm/ui/types';

export function createTshdEventsContextBridgeService(
  ctx: IAppContext
): TshdEventContextBridgeService {
  const logger = new Logger('tshd events UI');

  return {
    relogin: async ({ request, onRequestCancelled }) => {
      await ctx.reloginService.relogin(request, onRequestCancelled);
      return {};
    },

    sendNotification: async ({ request }) => {
      ctx.tshdNotificationsService.sendNotification(request);
      return {};
    },

    sendPendingHeadlessAuthentication: async ({
      request,
      onRequestCancelled,
    }) => {
      await ctx.headlessAuthenticationService.sendPendingHeadlessAuthentication(
        request,
        onRequestCancelled
      );
      return {};
    },

    promptMFA: async ({ request, onRequestCancelled }) => {
      const { totpCode, hasCanceledModal } = await new Promise<{
        totpCode: string;
        hasCanceledModal: boolean;
      }>(resolve => {
        const { closeDialog } = ctx.modalsService.openImportantDialog({
          kind: 'reauthenticate',
          promptMfaRequest: request,
          onSuccess: totpCode => resolve({ hasCanceledModal: false, totpCode }),
          onSsoContinue: (redirectUrl: string) => {
            window.open(redirectUrl);
          },
          onCancel: () =>
            resolve({
              hasCanceledModal: true,
              totpCode: undefined,
            }),
        });

        // If Webauthn is available, tshd starts two goroutines – one that sends this request and
        // one that starts listening for a hardware key tap. When a tap is detected, tshd cancels
        // this request.
        onRequestCancelled(closeDialog);
      });

      if (hasCanceledModal) {
        // Throwing an object here instead of an error to future-proof this code in case we need to
        // return more than just the error name and message.
        // Refer to processEvent in the preload part of tshd events service for more details.
        throw {
          isCrossContextError: true,
          name: 'AbortError',
          message: 'MFA modal closed by user',
        };
      }

      return {
        totpCode,
      };
    },

    getUsageReportingSettings: async () => {
      return {
        usageReportingSettings: {
          enabled: ctx.configService.get('usageReporting.enabled').value,
        },
      };
    },

    reportUnexpectedVnetShutdown: async ({ request }) => {
      if (!ctx.unexpectedVnetShutdownListener) {
        logger.warn(
          `Dropping unexpected VNet shutdown event, no listener present; error: ${request.error}`
        );
      } else {
        ctx.unexpectedVnetShutdownListener(request);
      }
      return {};
    },

    promptHardwareKeyPIN: async ({ request, onRequestCancelled }) => {
      ctx.mainProcessClient.forceFocusWindow();
      const { pin, hasCanceledModal } = await new Promise<{
        pin: string;
        hasCanceledModal: boolean;
      }>(resolve => {
        const { closeDialog } = ctx.modalsService.openImportantDialog({
          kind: 'hardware-key-pin',
          req: request,
          onSuccess: pin => resolve({ hasCanceledModal: false, pin }),
          onCancel: () =>
            resolve({
              hasCanceledModal: true,
              pin: undefined,
            }),
        });

        // tshd can cancel the request
        onRequestCancelled(closeDialog);
      });

      if (hasCanceledModal) {
        throw {
          isCrossContextError: true,
          name: 'AbortError',
          message: 'hardware key PIN modal closed by user',
        };
      }

      return { pin: pin };
    },

    promptHardwareKeyTouch: async ({ request, onRequestCancelled }) => {
      ctx.mainProcessClient.forceFocusWindow();
      const { hasCanceledModal } = await new Promise<{
        hasCanceledModal: boolean;
      }>(resolve => {
        const { closeDialog } = ctx.modalsService.openImportantDialog({
          kind: 'hardware-key-touch',
          req: request,
          onCancel: () =>
            resolve({
              hasCanceledModal: true,
            }),
        });

        // When a tap is detected, tshd cancels this request.
        onRequestCancelled(closeDialog);
      });

      if (hasCanceledModal) {
        throw {
          isCrossContextError: true,
          name: 'AbortError',
          message: 'hardware key touch modal closed by user',
        };
      }

      return {};
    },

    promptHardwareKeyPINChange: async ({ request, onRequestCancelled }) => {
      const { hasCanceledModal, res } = await new Promise<{
        hasCanceledModal: boolean;
        res: PromptHardwareKeyPINChangeResponse;
      }>(resolve => {
        const { closeDialog } = ctx.modalsService.openImportantDialog({
          kind: 'hardware-key-pin-change',
          req: request,
          onSuccess: res => {
            resolve({ hasCanceledModal: false, res });
          },
          onCancel: () =>
            resolve({
              hasCanceledModal: true,
              res: undefined,
            }),
        });

        // tshd can cancel the request
        onRequestCancelled(closeDialog);
      });

      if (hasCanceledModal) {
        throw {
          isCrossContextError: true,
          name: 'AbortError',
          message: 'hardware key change PIN modal closed by user',
        };
      }

      return res;
    },

    confirmHardwareKeySlotOverwrite: async ({
      request,
      onRequestCancelled,
    }) => {
      const { hasCanceledModal, confirmed } = await new Promise<{
        hasCanceledModal: boolean;
        confirmed: boolean;
      }>(resolve => {
        const { closeDialog } = ctx.modalsService.openImportantDialog({
          kind: 'hardware-key-slot-overwrite',
          req: request,
          onConfirm: () => {
            resolve({ hasCanceledModal: false, confirmed: true });
          },
          onCancel: () =>
            resolve({
              hasCanceledModal: true,
              confirmed: false,
            }),
        });

        // tshd can cancel the request
        onRequestCancelled(closeDialog);
      });

      if (hasCanceledModal) {
        throw {
          isCrossContextError: true,
          name: 'AbortError',
          message: 'hardware key slot overwrite modal closed by user',
        };
      }

      return { confirmed };
    },
  };
}
