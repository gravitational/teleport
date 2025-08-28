/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Menu, MenuItemConstructorOptions, nativeImage, Tray } from 'electron';

import Logger from 'teleterm/logger';
import { getAssetPath } from 'teleterm/mainProcess/runtimeSettings';
import { RuntimeSettings } from 'teleterm/mainProcess/types';
import { TrackedConnection } from 'teleterm/ui/services/connectionTracker';
import { StatePersistenceService } from 'teleterm/ui/services/statePersistence';

export function setTray(
  runtimeSettings: RuntimeSettings,
  statePersis: StatePersistenceService,
  args: { showWindow(): void }
): void {
  const logger = new Logger('setTray');
  const tray = new Tray(
    getIcon(runtimeSettings),
    'acf0cb59-0f9e-412a-8973-9ee803bc39f6'
  );

  logger.info(getAssetPath('iconTemplate@2x.png'));

  const contextMenu = Menu.buildFromTemplate([
    {
      label: 'Open Teleport Connect',
      type: 'normal',
      click: () => args.showWindow(),
    },
    { type: 'separator' },
    { label: 'Quit', type: 'normal', role: 'quit' },
  ]);
  tray.setContextMenu(contextMenu);
}

function getIcon(runtimeSettings: RuntimeSettings) {
  switch (runtimeSettings.platform) {
    case 'darwin':
      const image = nativeImage.createFromPath(
        getAssetPath('iconTemplate@2x.png')
      );
      image.setTemplateImage(true);
      return image;
    case 'win32':
      return getAssetPath('icon-win.ico');
    case 'linux':
      return getAssetPath('tray-icon-linux.png');
  }
}
