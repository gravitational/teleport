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
import { nativeImage, Tray, Menu } from 'electron';

import { getAssetPath } from 'teleterm/mainProcess/runtimeSettings';
import { TshdClient } from 'teleterm/services/tshd';

export function addTray(tshdClient: TshdClient) {
  const image = nativeImage.createFromPath(getAssetPath('iconTemplate.png'));
  const resizedImage = image.resize({ width: 16 });
  resizedImage.setTemplateImage(true);
  const tray = new Tray(resizedImage);
  tray.on('mouse-enter', () => {
    // TODO: Guarantee that there is only one promise running that updates the menu.
    const contextMenu = Menu.buildFromTemplate([
      {
        label: 'Open Teleport Connect',
        icon: nativeImage
          .createFromNamedImage('NSImageNameApplicationIcon')
          .resize({ width: 16 }),
        type: 'normal',
      },
      {
        label: 'bob@platform.teleport.sh',
        icon: nativeImage
          .createFromNamedImage('NSImageNameUser')
          .resize({ width: 16 }),
        type: 'submenu',
        submenu: [
          {
            label: 'alice@teleport-ent-15.asteroid.earth',
            type: 'radio',
          },
          {
            label: 'bob@platform.teleport.sh',
            type: 'radio',
            checked: true,
          },
          { label: 'sam@example.com', type: 'radio' },
        ],
      },
      { type: 'separator' },
      {
        label: 'Local proxies',
        type: 'normal',
        enabled: false,
      },
      {
        label: 'dba@postgres (platform.teleport.sh)',
        icon: nativeImage
          .createFromNamedImage('NSImageNameStatusAvailable')
          .resize({ width: 16 }),
        type: 'submenu',
        submenu: [
          { label: 'localhost:48219', type: 'normal', enabled: false },
          { type: 'separator' },
          { label: 'Copy address', type: 'normal' },
          { label: 'Turn off' },
        ],
      },
      {
        label: 'grafana (teleport-ent-15.asteroid.earth)',
        icon: nativeImage
          .createFromNamedImage('NSImageNameStatusNone')
          .resize({ width: 16 }),
        type: 'submenu',
        submenu: [
          { label: 'localhost:51284', type: 'normal', enabled: false },
          { type: 'separator' },
          { label: 'Copy address', type: 'normal' },
          { label: 'Turn off' },
        ],
      },
      {
        label: 'minikube (example.com)',

        icon: nativeImage
          .createFromNamedImage('NSImageNameStatusNone')
          .resize({ width: 16 }),
        type: 'submenu',
        submenu: [
          { label: 'localhost:11726', type: 'normal', enabled: false },
          { type: 'separator' },
          { label: 'Copy address', type: 'normal' },
          { label: 'Turn off' },
        ],
      },
      { type: 'separator' },
      { label: 'Quit', type: 'normal' },
    ]);
    tray.setContextMenu(contextMenu);
  });
}
