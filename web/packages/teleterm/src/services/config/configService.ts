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

import { z } from 'zod';

import { FileStorage } from 'teleterm/services/fileStorage';
import { Platform } from 'teleterm/mainProcess/types';

import { createConfigStore } from './configStore';
import { getKeyboardShortcutSchema } from './getKeyboardShortcutSchema';

const createAppConfigSchema = (platform: Platform) => {
  const defaultKeymap = getDefaultKeymap(platform);
  const defaultTerminalFont = getDefaultTerminalFont(platform);

  // `keymap.` prefix is used in `initUi.ts` in a predicate function.
  return z.object({
    'usageReporting.enabled': z.boolean().default(false),
    'keymap.tab1': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['tab1']
    ),
    'keymap.tab2': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['tab2']
    ),
    'keymap.tab3': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['tab3']
    ),
    'keymap.tab4': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['tab4']
    ),
    'keymap.tab5': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['tab5']
    ),
    'keymap.tab6': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['tab6']
    ),
    'keymap.tab7': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['tab7']
    ),
    'keymap.tab8': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['tab8']
    ),
    'keymap.tab9': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['tab9']
    ),
    'keymap.closeTab': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['closeTab']
    ),
    'keymap.newTab': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['newTab']
    ),
    'keymap.previousTab': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['previousTab']
    ),
    'keymap.nextTab': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['nextTab']
    ),
    'keymap.openConnections': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['openConnections']
    ),
    'keymap.openClusters': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['openClusters']
    ),
    'keymap.openProfiles': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['openProfiles']
    ),
    'keymap.openQuickInput': getKeyboardShortcutSchema(platform).default(
      defaultKeymap['openQuickInput']
    ),
    /**
     * This value can be provided by the user and is unsanitized. This means that it cannot be directly interpolated
     * in a styled component or used in CSS, as it may inject malicious CSS code.
     * Before using it, sanitize it with `CSS.escape` or pass it as a `style` prop.
     * Read more https://frontarm.com/james-k-nelson/how-can-i-use-css-in-js-securely/.
     */
    'terminal.fontFamily': z.string().default(defaultTerminalFont),
    'terminal.fontSize': z.number().int().min(1).max(256).default(15),
  });
};

export type AppConfig = z.infer<ReturnType<typeof createAppConfigSchema>>;

export type KeyboardShortcutAction =
  | 'tab1'
  | 'tab2'
  | 'tab3'
  | 'tab4'
  | 'tab5'
  | 'tab6'
  | 'tab7'
  | 'tab8'
  | 'tab9'
  | 'closeTab'
  | 'newTab'
  | 'previousTab'
  | 'nextTab'
  | 'openQuickInput'
  | 'openConnections'
  | 'openClusters'
  | 'openProfiles';

const getDefaultKeymap = (platform: Platform) => {
  switch (platform) {
    case 'win32':
      return {
        tab1: 'Ctrl+1',
        tab2: 'Ctrl+2',
        tab3: 'Ctrl+3',
        tab4: 'Ctrl+4',
        tab5: 'Ctrl+5',
        tab6: 'Ctrl+6',
        tab7: 'Ctrl+7',
        tab8: 'Ctrl+8',
        tab9: 'Ctrl+9',
        closeTab: 'Ctrl+W',
        newTab: 'Ctrl+T',
        previousTab: 'Ctrl+Shift+Tab',
        nextTab: 'Ctrl+Tab',
        openQuickInput: 'Ctrl+K',
        openConnections: 'Ctrl+P',
        openClusters: 'Ctrl+E',
        openProfiles: 'Ctrl+I',
      };
    case 'linux':
      return {
        tab1: 'Alt+1',
        tab2: 'Alt+2',
        tab3: 'Alt+3',
        tab4: 'Alt+4',
        tab5: 'Alt+5',
        tab6: 'Alt+6',
        tab7: 'Alt+7',
        tab8: 'Alt+8',
        tab9: 'Alt+9',
        closeTab: 'Ctrl+W',
        newTab: 'Ctrl+T',
        previousTab: 'Ctrl+Shift+Tab',
        nextTab: 'Ctrl+Tab',
        openQuickInput: 'Ctrl+K',
        openConnections: 'Ctrl+P',
        openClusters: 'Ctrl+E',
        openProfiles: 'Ctrl+I',
      };
    case 'darwin':
      return {
        tab1: 'Cmd+1',
        tab2: 'Cmd+2',
        tab3: 'Cmd+3',
        tab4: 'Cmd+4',
        tab5: 'Cmd+5',
        tab6: 'Cmd+6',
        tab7: 'Cmd+7',
        tab8: 'Cmd+8',
        tab9: 'Cmd+9',
        closeTab: 'Cmd+W',
        newTab: 'Cmd+T',
        previousTab: 'Ctrl+Shift+Tab',
        nextTab: 'Ctrl+Tab',
        openQuickInput: 'Cmd+K',
        openConnections: 'Cmd+P',
        openClusters: 'Cmd+E',
        openProfiles: 'Cmd+I',
      };
  }
};

function getDefaultTerminalFont(platform: Platform) {
  switch (platform) {
    case 'win32':
      return "'Consolas', 'Courier New', monospace";
    case 'linux':
      return "'Droid Sans Mono', 'Courier New', monospace, 'Droid Sans Fallback'";
    case 'darwin':
      return "Menlo, Monaco, 'Courier New', monospace";
  }
}

export function createConfigService(
  appConfigFileStorage: FileStorage,
  platform: Platform
) {
  return createConfigStore(
    createAppConfigSchema(platform),
    appConfigFileStorage
  );
}

export type ConfigService = ReturnType<typeof createConfigService>;
