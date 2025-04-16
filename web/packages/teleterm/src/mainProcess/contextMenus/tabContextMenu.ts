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
  dialog,
  ipcMain,
  ipcRenderer,
  Menu,
  MenuItemConstructorOptions,
} from 'electron';

import { makeCustomShellFromPath, Shell } from 'teleterm/mainProcess/shell';
import { ConfigService } from 'teleterm/services/config';
import {
  canDocChangeShell,
  Document,
} from 'teleterm/ui/services/workspacesService';

import {
  TabContextMenuEventChannel,
  TabContextMenuEventType,
  TabContextMenuOptions,
} from '../types';

type MainTabContextMenuOptions = {
  document: Document;
};

type TabContextMenuEvent =
  | {
      event: TabContextMenuEventType.ReopenPtyInShell;
      item: Shell;
    }
  | {
      event:
        | TabContextMenuEventType.Close
        | TabContextMenuEventType.CloseOthers
        | TabContextMenuEventType.CloseToRight
        | TabContextMenuEventType.DuplicatePty;
    };

export function subscribeToTabContextMenuEvent(
  shells: Shell[],
  configService: ConfigService
): void {
  ipcMain.handle(
    TabContextMenuEventChannel,
    (event, options: MainTabContextMenuOptions) => {
      return new Promise<TabContextMenuEvent>(resolve => {
        let preventAutoPromiseResolveOnMenuClose = false;

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
          if (!canDocChangeShell(doc)) {
            return;
          }
          const activeShellId = doc.shellId;
          const defaultShellId = configService.get('terminal.shell').value;
          const customShellPath = configService.get(
            'terminal.customShell'
          ).value;
          const customShell =
            customShellPath && makeCustomShellFromPath(customShellPath);
          const shellsWithCustom = [...shells, customShell].filter(Boolean);
          const isMoreThanOneShell = shellsWithCustom.length > 1;
          return [
            {
              type: 'separator',
            },
            ...shellsWithCustom.map(shell => ({
              label: shell.friendlyName,
              id: shell.id,
              type: 'radio' as const,
              visible: isMoreThanOneShell,
              checked: shell.id === activeShellId,
              click: () => {
                // Do nothing when the shell doesn't change.
                if (shell.id === activeShellId) {
                  return;
                }
                resolve({
                  event: TabContextMenuEventType.ReopenPtyInShell,
                  item: shell,
                });
              },
            })),
            {
              label: customShell
                ? `Change Custom Shell (${customShell.friendlyName})…`
                : 'Select Custom Shell…',
              click: async () => {
                // By default, when the popup menu is closed, the promise is
                // resolved (popup.callback).
                // Here we need to prevent this behavior to wait for the file
                // to be selected.
                // A more universal way of handling this problem:
                // https://github.com/gravitational/teleport/pull/45152#discussion_r1723314524
                preventAutoPromiseResolveOnMenuClose = true;
                const { filePaths, canceled } = await dialog.showOpenDialog({
                  properties: ['openFile'],
                  defaultPath: customShell.binPath,
                });
                if (canceled) {
                  resolve(undefined);
                  return;
                }
                const file = filePaths[0];
                configService.set('terminal.customShell', file);
                resolve({
                  event: TabContextMenuEventType.ReopenPtyInShell,
                  item: makeCustomShellFromPath(file),
                });
              },
            },
            {
              label: 'Default Shell',
              visible: isMoreThanOneShell,
              type: 'submenu',
              sublabel:
                shellsWithCustom.find(s => defaultShellId === s.id)
                  ?.friendlyName || defaultShellId,
              submenu: [
                ...shellsWithCustom.map(shell => ({
                  label: shell.friendlyName,
                  id: shell.id,
                  checked: shell.id === defaultShellId,
                  type: 'radio' as const,
                  click: () => {
                    configService.set('terminal.shell', shell.id);
                    resolve(undefined);
                  },
                })),
              ],
            },
          ];
        }

        Menu.buildFromTemplate(
          [getCommonTemplate(), getPtyTemplate(), getShellTemplate()]
            .filter(Boolean)
            .flatMap(template => template)
        ).popup({
          callback: () =>
            !preventAutoPromiseResolveOnMenuClose && resolve(undefined),
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
  const response = (await ipcRenderer.invoke(
    TabContextMenuEventChannel,
    mainOptions
  )) as TabContextMenuEvent | undefined;
  // Undefined when the menu gets closed without clicking on any action.
  if (!response) {
    return;
  }
  const { event } = response;

  switch (event) {
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
    default:
      event satisfies never;
  }
}
