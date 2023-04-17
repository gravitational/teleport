/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { ConfigService } from 'teleterm/services/config';
import { ModalsService } from 'teleterm/ui/services/modals';
import { staticConfig } from 'teleterm/staticConfig';
import Logger from 'teleterm/logger';

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
