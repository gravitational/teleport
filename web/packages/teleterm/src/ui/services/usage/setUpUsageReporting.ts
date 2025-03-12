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

import Logger from 'teleterm/logger';
import { ConfigService } from 'teleterm/services/config';
import { staticConfig } from 'teleterm/staticConfig';
import { ModalsService } from 'teleterm/ui/services/modals';

export async function setUpUsageReporting(
  configService: ConfigService,
  modalsService: ModalsService
): Promise<void> {
  const logger = new Logger('setUpUsageReporting');
  if (!staticConfig.prehogAddress) {
    logger.info('Prehog address not set, usage reporting disabled.');
    return;
  }

  if (configService.getConfigError()?.source === 'file-loading') {
    // do not show the dialog, response cannot be saved to the file
    return;
  }

  if (configService.get('usageReporting.enabled').metadata.isStored) {
    return;
  }

  return new Promise(resolve => {
    modalsService.openRegularDialog({
      kind: 'usage-data',
      onAllow() {
        configService.set('usageReporting.enabled', true);
        resolve();
      },
      onDecline() {
        configService.set('usageReporting.enabled', false);
        resolve();
      },
      onCancel() {
        resolve();
      },
    });
  });
}
