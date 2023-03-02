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
import { updateJsonSchema } from './updateJsonSchema';

const createAppConfigSchema = (platform: Platform) => {
  const defaultKeymap = getDefaultKeymap(platform);
  const defaultTerminalFont = getDefaultTerminalFont(platform);

  const keyboardShortcutSchema = getKeyboardShortcutSchema(platform);

  // `keymap.` prefix is used in `initUi.ts` in a predicate function.
  return z.object({
    'usageReporting.enabled': z.boolean().default(false),
    'keymap.tab1': keyboardShortcutSchema
      .default(defaultKeymap['tab1'])
      .describe(getKeyboardShortcutDescription('open tab 1')),
    'keymap.tab2': keyboardShortcutSchema
      .default(defaultKeymap['tab2'])
      .describe(getKeyboardShortcutDescription('open tab 2')),
    'keymap.tab3': keyboardShortcutSchema
      .default(defaultKeymap['tab3'])
      .describe(getKeyboardShortcutDescription('open tab 3')),
    'keymap.tab4': keyboardShortcutSchema
      .default(defaultKeymap['tab4'])
      .describe(getKeyboardShortcutDescription('open tab 4')),
    'keymap.tab5': keyboardShortcutSchema
      .default(defaultKeymap['tab5'])
      .describe(getKeyboardShortcutDescription('open tab 5')),
    'keymap.tab6': keyboardShortcutSchema
      .default(defaultKeymap['tab6'])
      .describe(getKeyboardShortcutDescription('open tab 6')),
    'keymap.tab7': keyboardShortcutSchema
      .default(defaultKeymap['tab7'])
      .describe(getKeyboardShortcutDescription('open tab 7')),
    'keymap.tab8': keyboardShortcutSchema
      .default(defaultKeymap['tab8'])
      .describe(getKeyboardShortcutDescription('open tab 8')),
    'keymap.tab9': keyboardShortcutSchema
      .default(defaultKeymap['tab9'])
      .describe(getKeyboardShortcutDescription('open tab 9')),
    'keymap.closeTab': keyboardShortcutSchema
      .default(defaultKeymap['closeTab'])
      .describe(getKeyboardShortcutDescription('close a tab')),
    'keymap.newTab': keyboardShortcutSchema
      .default(defaultKeymap['newTab'])
      .describe(getKeyboardShortcutDescription('open a new tab')),
    'keymap.previousTab': keyboardShortcutSchema
      .default(defaultKeymap['previousTab'])
      .describe(getKeyboardShortcutDescription('go to the previous tab')),
    'keymap.nextTab': keyboardShortcutSchema
      .default(defaultKeymap['nextTab'])
      .describe(getKeyboardShortcutDescription('go to the next tab')),
    'keymap.openConnections': keyboardShortcutSchema
      .default(defaultKeymap['openConnections'])
      .describe(getKeyboardShortcutDescription('open the connection panel')),
    'keymap.openClusters': keyboardShortcutSchema
      .default(defaultKeymap['openClusters'])
      .describe(getKeyboardShortcutDescription('open the clusters panel')),
    'keymap.openProfiles': keyboardShortcutSchema
      .default(defaultKeymap['openProfiles'])
      .describe(getKeyboardShortcutDescription('open the profiles panel')),
    'keymap.openQuickInput': keyboardShortcutSchema
      .default(defaultKeymap['openQuickInput'])
      .describe(getKeyboardShortcutDescription('open the command bar')),
    /**
     * This value can be provided by the user and is unsanitized. This means that it cannot be directly interpolated
     * in a styled component or used in CSS, as it may inject malicious CSS code.
     * Before using it, sanitize it with `CSS.escape` or pass it as a `style` prop.
     * Read more https://frontarm.com/james-k-nelson/how-can-i-use-css-in-js-securely/.
     */
    'terminal.fontFamily': z
      .string()
      .default(defaultTerminalFont)
      .describe('Font family for the terminal.'),
    'terminal.fontSize': z
      .number()
      .int()
      .min(1)
      .max(256)
      .default(15)
      .describe('Font size for the terminal.'),
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
        tab1: 'Command+1',
        tab2: 'Command+2',
        tab3: 'Command+3',
        tab4: 'Command+4',
        tab5: 'Command+5',
        tab6: 'Command+6',
        tab7: 'Command+7',
        tab8: 'Command+8',
        tab9: 'Command+9',
        closeTab: 'Command+W',
        newTab: 'Command+T',
        previousTab: 'Control+Shift+Tab',
        nextTab: 'Control+Tab',
        openQuickInput: 'Command+K',
        openConnections: 'Command+P',
        openClusters: 'Command+E',
        openProfiles: 'Command+I',
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

function getKeyboardShortcutDescription(about: string): string {
  return `Shortcut to ${about}. A valid shortcut contains at least one modifier and a single key code, for example "Ctrl+Shift+A". Function keys do not require a modifier.`;
}

export function createConfigService({
  configFile,
  configJsonSchemaFile,
  platform,
}: {
  configFile: FileStorage;
  configJsonSchemaFile: FileStorage;
  platform: Platform;
}) {
  const schema = createAppConfigSchema(platform);

  updateJsonSchema({
    configSchema: schema,
    configFile,
    configJsonSchemaFile,
  });

  return createConfigStore(schema, configFile);
}

export type ConfigService = ReturnType<typeof createConfigService>;
