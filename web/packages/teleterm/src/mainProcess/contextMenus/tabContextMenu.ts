/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
