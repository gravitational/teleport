import { ConfigServiceProvider } from '../types';

/**
 * Modifier keys must be defined in the following order:
 * Command-Control-Option-Shift for macOS
 * Ctrl-Alt-Shift for other platforms
 */

export type KeyboardShortcutType =
  | 'tab-1'
  | 'tab-2'
  | 'tab-3'
  | 'tab-4'
  | 'tab-5'
  | 'tab-6'
  | 'tab-7'
  | 'tab-8'
  | 'tab-9'
  | 'tab-close'
  | 'tab-new'
  | 'tab-previous'
  | 'tab-next'
  | 'open-quick-input'
  | 'toggle-connections'
  | 'toggle-clusters'
  | 'toggle-identity';

export type KeyboardShortcutsConfig = Record<KeyboardShortcutType, string>;

export const keyboardShortcutsConfigProvider: ConfigServiceProvider<KeyboardShortcutsConfig> =
  {
    getDefaults(platform) {
      const macShortcuts: KeyboardShortcutsConfig = {
        'tab-1': 'Command-1',
        'tab-2': 'Command-2',
        'tab-3': 'Command-3',
        'tab-4': 'Command-4',
        'tab-5': 'Command-5',
        'tab-6': 'Command-6',
        'tab-7': 'Command-7',
        'tab-8': 'Command-8',
        'tab-9': 'Command-9',
        'tab-close': 'Command-W',
        'tab-new': 'Command-T',
        'tab-previous': 'Control-Shift-Tab',
        'tab-next': 'Control-Tab',
        'open-quick-input': 'Command-K',
        'toggle-connections': 'Command-P',
        'toggle-clusters': 'Command-E',
        'toggle-identity': 'Command-I',
      };

      const windowsAndLinuxShortcuts: KeyboardShortcutsConfig = {
        'tab-1': 'Alt-1',
        'tab-2': 'Alt-2',
        'tab-3': 'Alt-3',
        'tab-4': 'Alt-4',
        'tab-5': 'Alt-5',
        'tab-6': 'Alt-6',
        'tab-7': 'Alt-7',
        'tab-8': 'Alt-8',
        'tab-9': 'Alt-9',
        'tab-close': 'Ctrl-Shift-W',
        'tab-new': 'Ctrl-Shift-T',
        'tab-previous': 'Ctrl-PageUp',
        'tab-next': 'Ctrl-PageDown',
        'open-quick-input': 'Ctrl-K',
        'toggle-connections': 'Ctrl-P',
        'toggle-clusters': 'Ctrl-E',
        'toggle-identity': 'Ctrl-I',
      };

      switch (platform) {
        case 'win32':
        case 'linux':
          return windowsAndLinuxShortcuts;
        case 'darwin':
          return macShortcuts;
      }
    },
  };
