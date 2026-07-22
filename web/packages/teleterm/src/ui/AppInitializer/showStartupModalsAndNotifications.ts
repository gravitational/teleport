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

import { ConfigService } from 'teleterm/services/config';
import { KeyboardShortcutsService } from 'teleterm/ui/services/keyboardShortcuts';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import {
  askAboutUserJobRoleIfNeeded,
  setUpUsageReporting,
} from 'teleterm/ui/services/usage';
import { IAppContext } from 'teleterm/ui/types';

/**
 * Runs after the UI becomes visible.
 */
export async function showStartupModalsAndNotifications(
  ctx: IAppContext
): Promise<void> {
  const { configService } = ctx.mainProcessClient;

  await askAboutUserJobRoleIfNeeded(
    ctx.statePersistenceService,
    configService,
    ctx.modalsService,
    ctx.usageService
  );
  // Setting up usage reporting after asking for a job role prevents a situation
  // where these dialogs are shown one after another.
  // Instead, on the first launch only "usage reporting" dialog shows up.
  // "User job role" dialog is shown on the second launch (only if user agreed to reporting earlier).
  await setUpUsageReporting(configService, ctx.modalsService);

  notifyAboutConfigErrors(configService, ctx.notificationsService);
  notifyAboutDuplicatedShortcutsCombinations(
    ctx.keyboardShortcutsService,
    ctx.notificationsService
  );
}

function notifyAboutConfigErrors(
  configService: ConfigService,
  notificationsService: NotificationsService
): void {
  const configError = configService.getConfigError();
  if (configError) {
    switch (configError.source) {
      case 'file-loading': {
        notificationsService.notifyError({
          title: 'Failed to load config file',
          description: `Using the default config instead.\n${configError.error}`,
        });
        break;
      }
      case 'validation': {
        const isKeymapError = configError.errors.some(e =>
          e.path[0].toString().startsWith('keymap.')
        );
        notificationsService.notifyError({
          title: 'Encountered errors in config file',
          list: configError.errors.map(
            e => `${e.path[0].toString()}: ${e.message}`
          ),
          description:
            isKeymapError &&
            'A valid shortcut contains at least one modifier and a single key code, for example "Shift+Tab".\nFunction keys do not require a modifier.',
          action: {
            href: 'https://goteleport.com/docs/connect-your-client/teleport-connect/#configuration',
            content: 'See the config file documentation',
          },
        });
      }
    }
  }
}

function notifyAboutDuplicatedShortcutsCombinations(
  keyboardShortcutsService: KeyboardShortcutsService,
  notificationsService: NotificationsService
): void {
  const duplicates = keyboardShortcutsService.getDuplicateAccelerators();
  if (Object.keys(duplicates).length) {
    notificationsService.notifyError({
      title: 'Shortcuts conflicts',
      list: Object.entries(duplicates).map(
        ([accelerator, actions]) =>
          `${accelerator} is used for actions: ${actions.join(
            ', '
          )}. Only one of them will work.`
      ),
    });
  }
}
