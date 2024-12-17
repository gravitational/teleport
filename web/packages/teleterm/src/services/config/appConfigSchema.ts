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

import { z } from 'zod';

import { Platform, RuntimeSettings } from 'teleterm/mainProcess/types';

import { createKeyboardShortcutSchema } from './keyboardShortcutSchema';

// When adding a new config property, add it to the docs too
// (teleport-connect.mdx#configuration).

export type AppConfigSchema = ReturnType<typeof createAppConfigSchema>;
export type AppConfig = z.infer<AppConfigSchema>;

/** ID of the custom shell. When it is set, the shell path should be read from `terminal.customShell`. */
export const CUSTOM_SHELL_ID = 'custom' as const;

/**
 * List of properties that can be modified from the renderer process.
 * The motivation for adding this was to make it impossible to change
 * `terminal.customShell` from the renderer.
 */
export const CONFIG_MODIFIABLE_FROM_RENDERER: (keyof AppConfig)[] = [
  'usageReporting.enabled',
];

export const createAppConfigSchema = (settings: RuntimeSettings) => {
  const defaultKeymap = getDefaultKeymap(settings.platform);
  const defaultTerminalFont = getDefaultTerminalFont(settings.platform);
  const availableShellIdsWithCustom = [
    ...settings.availableShells.map(({ id }) => id),
    CUSTOM_SHELL_ID,
  ];

  const shortcutSchema = createKeyboardShortcutSchema(settings.platform);

  // `keymap.` prefix is used in `initUi.ts` in a predicate function.
  return z.object({
    theme: z
      .enum(['light', 'dark', 'system'])
      .default('system')
      .describe('Color theme for the app.'),
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
    'terminal.windowsBackend': z
      .enum(['auto', 'winpty'])
      .default('auto')
      .describe(
        '`auto` uses modern ConPTY system if available, which requires Windows 10 (19H1) or above. Set to `winpty` to use winpty even if ConPTY is available.'
      ),
    'terminal.shell': z
      .string()
      .default(settings.defaultOsShellId)
      .describe(
        'A default terminal shell. Can be set to `custom` to take the shell path from `terminal.customShell`. It is best to configure it through UI (right click on a terminal tab > Default Shell).'
      )
      .refine(
        configuredShell =>
          availableShellIdsWithCustom.some(
            shellId => shellId === configuredShell
          ),
        configuredShell => ({
          message: `Cannot find the shell "${configuredShell}". Available options are: ${availableShellIdsWithCustom.join(', ')}. Using platform default.`,
        })
      ),
    'terminal.customShell': z
      .string()
      .default('')
      .describe(
        'Path to the custom shell that is used when `terminal.shell` is set to `custom`. It is best to configure it through UI (right click on a terminal tab > Custom Shell…).'
      ),
    'terminal.rightClick': z
      .enum(['paste', 'copyPaste', 'menu'])
      .default(settings.platform === 'win32' ? 'copyPaste' : 'menu')
      .describe(
        '`paste` pastes clipboard content, `copyPaste` copies if text is selected, otherwise pastes, `menu` shows context menu.'
      ),
    'terminal.copyOnSelect': z
      .boolean()
      .default(false)
      .describe('Automatically copies selected text to the clipboard.'),
    'usageReporting.enabled': z
      .boolean()
      .default(false)
      .describe('Enables collecting of anonymous usage data.'),
    'keymap.tab1': shortcutSchema
      .default(defaultKeymap['tab1'])
      .describe(getShortcutDesc('open tab 1')),
    'keymap.tab2': shortcutSchema
      .default(defaultKeymap['tab2'])
      .describe(getShortcutDesc('open tab 2')),
    'keymap.tab3': shortcutSchema
      .default(defaultKeymap['tab3'])
      .describe(getShortcutDesc('open tab 3')),
    'keymap.tab4': shortcutSchema
      .default(defaultKeymap['tab4'])
      .describe(getShortcutDesc('open tab 4')),
    'keymap.tab5': shortcutSchema
      .default(defaultKeymap['tab5'])
      .describe(getShortcutDesc('open tab 5')),
    'keymap.tab6': shortcutSchema
      .default(defaultKeymap['tab6'])
      .describe(getShortcutDesc('open tab 6')),
    'keymap.tab7': shortcutSchema
      .default(defaultKeymap['tab7'])
      .describe(getShortcutDesc('open tab 7')),
    'keymap.tab8': shortcutSchema
      .default(defaultKeymap['tab8'])
      .describe(getShortcutDesc('open tab 8')),
    'keymap.tab9': shortcutSchema
      .default(defaultKeymap['tab9'])
      .describe(getShortcutDesc('open tab 9')),
    'keymap.closeTab': shortcutSchema
      .default(defaultKeymap['closeTab'])
      .describe(getShortcutDesc('close a tab')),
    'keymap.newTab': shortcutSchema
      .default(defaultKeymap['newTab'])
      .describe(getShortcutDesc('open a new tab')),
    'keymap.newTerminalTab': shortcutSchema
      .default(defaultKeymap['newTerminalTab'])
      .describe(getShortcutDesc('open a new terminal tab')),
    // Even if this is set to a non-default copy shortcut for a given platform,
    // the default shortcut will still work (for example, Command+C on Macs).
    'keymap.terminalCopy': shortcutSchema
      .default(defaultKeymap['terminalCopy'])
      .describe(getShortcutDesc('copy text in the terminal')),
    // Even if this is set to a non-default paste shortcut for a given platform,
    // the default shortcut will still work (for example, Command+V on Macs).
    'keymap.terminalPaste': shortcutSchema
      .default(defaultKeymap['terminalPaste'])
      .describe(getShortcutDesc('paste text in the terminal')),
    'keymap.terminalSearch': shortcutSchema
      .default(defaultKeymap['terminalSearch'])
      .describe(getShortcutDesc('search for text in the terminal')),
    'keymap.previousTab': shortcutSchema
      .default(defaultKeymap['previousTab'])
      .describe(getShortcutDesc('go to the previous tab')),
    'keymap.nextTab': shortcutSchema
      .default(defaultKeymap['nextTab'])
      .describe(getShortcutDesc('go to the next tab')),
    'keymap.openConnections': shortcutSchema
      .default(defaultKeymap['openConnections'])
      .describe(getShortcutDesc('open the connection list')),
    'keymap.openClusters': shortcutSchema
      .default(defaultKeymap['openClusters'])
      .describe(getShortcutDesc('open the cluster selector')),
    'keymap.openProfiles': shortcutSchema
      .default(defaultKeymap['openProfiles'])
      .describe(getShortcutDesc('open the profile selector')),
    'keymap.openSearchBar': shortcutSchema
      .default(defaultKeymap['openSearchBar'])
      .describe(getShortcutDesc('open the search bar')),
    'headless.skipConfirm': z
      .boolean()
      .default(false)
      .describe(
        'Skips the confirmation prompt for headless login approval and instead prompts for WebAuthn immediately.'
      ),
    'ssh.noResume': z
      .boolean()
      .default(false)
      .describe('Disables SSH connection resumption.'),
    'ssh.forwardAgent': z
      .boolean()
      .default(false)
      .describe(
        "Enables agent forwarding when connecting to SSH nodes. It's the equivalent of the forward-agent flag in tsh ssh."
      ),
    'sshAgent.addKeysToAgent': z
      .enum(['auto', 'no', 'yes', 'only'])
      .default('auto')
      .describe(
        'Controls how keys are added to a local SSH agent. ' +
          "'auto' adds the keys only if the agent supports SSH certificates, " +
          "'no' never attempts to add them, 'yes' always attempts to add them, " +
          "'only' always attempts to add the keys to the agent but it does not save them on disk."
      ),
  });
};

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
  | 'newTerminalTab'
  | 'previousTab'
  | 'nextTab'
  | 'openSearchBar'
  | 'openConnections'
  | 'openClusters'
  | 'openProfiles'
  | 'terminalCopy'
  | 'terminalPaste'
  | 'terminalSearch';

const getDefaultKeymap = (
  platform: Platform
): Record<KeyboardShortcutAction, string> => {
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
        closeTab: 'Ctrl+Shift+W',
        newTab: 'Ctrl+Shift+T',
        newTerminalTab: 'Ctrl+Shift+`',
        previousTab: 'Ctrl+Shift+Tab',
        nextTab: 'Ctrl+Tab',
        openSearchBar: 'Ctrl+Shift+K',
        openConnections: 'Ctrl+Shift+P',
        openClusters: 'Ctrl+Shift+E',
        openProfiles: 'Ctrl+Shift+I',
        terminalCopy: 'Ctrl+Shift+C',
        terminalPaste: 'Ctrl+Shift+V',
        terminalSearch: 'Ctrl+Shift+F',
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
        closeTab: 'Ctrl+Shift+W',
        newTab: 'Ctrl+Shift+T',
        newTerminalTab: 'Ctrl+Shift+`',
        previousTab: 'Ctrl+Shift+Tab',
        nextTab: 'Ctrl+Tab',
        openSearchBar: 'Ctrl+Shift+K',
        openConnections: 'Ctrl+Shift+P',
        openClusters: 'Ctrl+Shift+E',
        openProfiles: 'Ctrl+Shift+I',
        terminalCopy: 'Ctrl+Shift+C',
        terminalPaste: 'Ctrl+Shift+V',
        terminalSearch: 'Ctrl+Shift+F',
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
        newTerminalTab: 'Control+Shift+`',
        previousTab: 'Control+Shift+Tab',
        nextTab: 'Control+Tab',
        openSearchBar: 'Command+K',
        openConnections: 'Command+P',
        openClusters: 'Command+E',
        openProfiles: 'Command+I',
        terminalCopy: 'Command+C',
        terminalPaste: 'Command+V',
        terminalSearch: 'Command+F',
      };
  }
};

function getDefaultTerminalFont(platform: Platform) {
  switch (platform) {
    case 'win32':
      return 'Consolas, monospace';
    case 'linux':
      return "'Droid Sans Mono', monospace";
    case 'darwin':
      return 'Menlo, Monaco, monospace';
  }
}

function getShortcutDesc(actionDesc: string): string {
  return `Shortcut to ${actionDesc}. A valid shortcut contains at least one modifier and a single key code, for example "Shift+Tab". Function keys do not require a modifier.`;
}
