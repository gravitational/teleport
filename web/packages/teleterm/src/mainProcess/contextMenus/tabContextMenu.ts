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

import {
  ipcMain,
  ipcRenderer,
  Menu,
  MenuItemConstructorOptions,
} from 'electron';

import {
  TabContextMenuEventChannel,
  TabContextMenuEventType,
  TabContextMenuOptions,
} from '../types';

type MainTabContextMenuOptions = Pick<TabContextMenuOptions, 'documentKind'>;

export function subscribeToTabContextMenuEvent(): void {
  ipcMain.handle(
    TabContextMenuEventChannel,
    (event, options: MainTabContextMenuOptions) => {
      return new Promise(resolve => {
        function getCommonTemplate(): MenuItemConstructorOptions[] {
          return [
            {
              label: 'Close',
              click: () => resolve(TabContextMenuEventType.Close),
            },
            {
              label: 'Close Others',
              click: () => resolve(TabContextMenuEventType.CloseOthers),
            },
            {
              label: 'Close to the Right',
              click: () => resolve(TabContextMenuEventType.CloseToRight),
            },
          ];
        }

        function getPtyTemplate(): MenuItemConstructorOptions[] {
          if (
            options.documentKind === 'doc.terminal_shell' ||
            options.documentKind === 'doc.terminal_tsh_node'
          ) {
            return [
              {
                type: 'separator',
              },
              {
                label: 'Duplicate Tab',
                click: () => resolve(TabContextMenuEventType.DuplicatePty),
              },
            ];
          }
        }

        Menu.buildFromTemplate(
          [getCommonTemplate(), getPtyTemplate()]
            .filter(Boolean)
            .flatMap(template => template)
        ).popup({
          callback: () => resolve(undefined),
        });
      });
    }
  );
}

export async function openTabContextMenu(
  options: TabContextMenuOptions
): Promise<void> {
  const mainOptions: MainTabContextMenuOptions = {
    documentKind: options.documentKind,
  };
  const eventType = await ipcRenderer.invoke(
    TabContextMenuEventChannel,
    mainOptions
  );
  switch (eventType) {
    case TabContextMenuEventType.Close:
      return options.onClose();
    case TabContextMenuEventType.CloseOthers:
      return options.onCloseOthers();
    case TabContextMenuEventType.CloseToRight:
      return options.onCloseToRight();
    case TabContextMenuEventType.DuplicatePty:
      return options.onDuplicatePty();
  }
}
