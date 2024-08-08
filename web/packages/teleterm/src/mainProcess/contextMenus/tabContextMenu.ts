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

import { ConfigService } from 'teleterm/services/config';
import { Shell } from 'teleterm/mainProcess/shell';

import {
  TabContextMenuEventChannel,
  TabContextMenuEventType,
  TabContextMenuOptions,
} from '../types';

type MainTabContextMenuOptions = Pick<TabContextMenuOptions, 'document'>;

export function subscribeToTabContextMenuEvent(
  shells: Shell[],
  configService: ConfigService
): void {
  ipcMain.handle(
    TabContextMenuEventChannel,
    (event, options: MainTabContextMenuOptions) => {
      return new Promise(resolve => {
        function getCommonTemplate(): MenuItemConstructorOptions[] {
          return [
            {
              label: 'Close',
              click: () => resolve({ event: TabContextMenuEventType.Close }),
            },
            {
              label: 'Close Others',
              click: () =>
                resolve({ event: TabContextMenuEventType.CloseOthers }),
            },
            {
              label: 'Close to the Right',
              click: () =>
                resolve({ event: TabContextMenuEventType.CloseToRight }),
            },
          ];
        }

        function getPtyTemplate(): MenuItemConstructorOptions[] {
          if (
            options.document.kind === 'doc.terminal_shell' ||
            options.document.kind === 'doc.terminal_tsh_node'
          ) {
            return [
              {
                label: 'Duplicate Tab',
                click: () =>
                  resolve({ event: TabContextMenuEventType.DuplicatePty }),
              },
            ];
          }
        }

        function getShellTemplate(): MenuItemConstructorOptions[] {
          const doc = options.document;
          if (
            doc.kind === 'doc.terminal_shell' ||
            doc.kind === 'doc.gateway_kube'
          ) {
            const activeShell = doc.shellId;
            const defaultShell = configService.get('terminal.shell').value;
            return [
              {
                type: 'separator',
              },
              {
                label: `Active Shell (${activeShell})`,
                type: 'submenu',
                submenu: shells.map(shell => ({
                  label: shell.friendlyName,
                  id: shell.id,
                  type: 'radio',
                  checked: shell.id === activeShell,
                  click: () => {
                    resolve({
                      event: TabContextMenuEventType.ReopenPtyInShell,
                      item: shell,
                    });
                  },
                })),
              },
              {
                label: `Default Shell (${defaultShell})`,
                type: 'submenu',
                submenu: shells.map(shell => ({
                  label: shell.friendlyName,
                  id: shell.id,
                  type: 'radio',
                  checked: shell.id === defaultShell,
                  click: () => {
                    configService.set('terminal.shell', shell.id);
                    resolve(undefined);
                  },
                })),
              },
            ];
          }
        }

        Menu.buildFromTemplate(
          [getCommonTemplate(), getPtyTemplate(), getShellTemplate()]
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
    document: options.document,
  };
  const response = await ipcRenderer.invoke(
    TabContextMenuEventChannel,
    mainOptions
  );
  if (!response) {
    return;
  }

  switch (response.event) {
    case TabContextMenuEventType.Close:
      return options.onClose();
    case TabContextMenuEventType.CloseOthers:
      return options.onCloseOthers();
    case TabContextMenuEventType.CloseToRight:
      return options.onCloseToRight();
    case TabContextMenuEventType.DuplicatePty:
      return options.onDuplicatePty();
    case TabContextMenuEventType.ReopenPtyInShell:
      return options.onReopenPtyInShell(response.item);
  }
}
