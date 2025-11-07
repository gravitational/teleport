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

import { Menu, nativeImage, NativeImage, Tray } from 'electron';

import { getAssetPath } from 'teleterm/mainProcess/runtimeSettings';
import { RuntimeSettings } from 'teleterm/mainProcess/types';

export function setTray(
  runtimeSettings: RuntimeSettings,
  window: { show(): void }
): void {
  const tray = new Tray(
    getIcon(runtimeSettings),
    // Random GUIDs that allows the icon to retain its position between relaunches.
    runtimeSettings.dev
      ? 'b3f163ae-bba3-4513-9593-ce186a3c3eb7'
      : 'acf0cb59-0f9e-412a-8973-9ee803bc39f6'
  );

  // On Windows, the app tray menu is displayed on the right mouse click.
  // The left mouse click should open the window.
  if (runtimeSettings.platform === 'win32') {
    tray.on('click', () => window.show());
  }

  const contextMenu = Menu.buildFromTemplate([
    {
      label: 'Open Teleport Connect',
      click: () => window.show(),
    },
    { type: 'separator' },
    { label: 'Quit', role: 'quit' },
  ]);
  tray.setContextMenu(contextMenu);
}

function getIcon(runtimeSettings: RuntimeSettings): string | NativeImage {
  switch (runtimeSettings.platform) {
    case 'darwin':
      const image = nativeImage.createFromPath(
        getAssetPath('icon-macTemplate@2x.png')
      );
      image.setTemplateImage(true);
      return image;
    case 'win32':
      return getAssetPath('icon-win.ico');
    case 'linux':
      return getAssetPath('icon-linux/tray.png');
  }
}
