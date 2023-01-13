import { z } from 'zod';

import { FileStorage } from 'teleterm/services/fileStorage';
import { Platform } from 'teleterm/mainProcess/types';

import { createConfigStore } from './configStore';

const createAppConfigSchema = (platform: Platform) => {
  const defaultKeymap = getDefaultKeymap(platform);
  const defaultFonts = getDefaultFonts(platform);

  // Important: all keys except 'usageReporting.enabled' are currently not
  // configurable by the user. Before we let the user configure them,
  // we need to set up some actual validation, so that for example
  // arbitrary CSS cannot be injected into the app through font settings.
  //
  // However, we want them to be in the config schema, so we included
  // them here, but we do not read their value from the stored config.
  return z.object({
    'usageReporting.enabled': z.boolean().default(false),
    'keymap.tab1': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-1'])
    ),
    'keymap.tab2': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-2'])
    ),
    'keymap.tab3': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-3'])
    ),
    'keymap.tab4': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-4'])
    ),
    'keymap.tab5': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-5'])
    ),
    'keymap.tab6': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-6'])
    ),
    'keymap.tab7': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-7'])
    ),
    'keymap.tab8': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-8'])
    ),
    'keymap.tab9': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-9'])
    ),
    'keymap.tabClose': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-close'])
    ),
    'keymap.tabNew': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-new'])
    ),
    'keymap.tabPrevious': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-previous'])
    ),
    'keymap.tabNext': omitStoredConfigValue(
      z.string().default(defaultKeymap['tab-next'])
    ),
    'keymap.toggleConnections': omitStoredConfigValue(
      z.string().default(defaultKeymap['toggle-connections'])
    ),
    'keymap.toggleClusters': omitStoredConfigValue(
      z.string().default(defaultKeymap['toggle-clusters'])
    ),
    'keymap.toggleIdentity': omitStoredConfigValue(
      z.string().default(defaultKeymap['toggle-identity'])
    ),
    'keymap.openQuickInput': omitStoredConfigValue(
      z.string().default(defaultKeymap['open-quick-input'])
    ),
    'fonts.sansSerifFamily': omitStoredConfigValue(
      z.string().default(defaultFonts['sansSerif'])
    ),
    'fonts.monoFamily': omitStoredConfigValue(
      z.string().default(defaultFonts['mono'])
    ),
  });
};

const omitStoredConfigValue = <T>(schema: z.ZodType<T>) =>
  z.preprocess(() => undefined, schema);

export type AppConfig = z.infer<ReturnType<typeof createAppConfigSchema>>;

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
const getDefaultKeymap = (platform: Platform) => {
  switch (platform) {
    case 'win32':
      return {
        'tab-1': 'Ctrl-1',
        'tab-2': 'Ctrl-2',
        'tab-3': 'Ctrl-3',
        'tab-4': 'Ctrl-4',
        'tab-5': 'Ctrl-5',
        'tab-6': 'Ctrl-6',
        'tab-7': 'Ctrl-7',
        'tab-8': 'Ctrl-8',
        'tab-9': 'Ctrl-9',
        'tab-close': 'Ctrl-W',
        'tab-new': 'Ctrl-T',
        'tab-previous': 'Ctrl-Shift-Tab',
        'tab-next': 'Ctrl-Tab',
        'open-quick-input': 'Ctrl-K',
        'toggle-connections': 'Ctrl-P',
        'toggle-clusters': 'Ctrl-E',
        'toggle-identity': 'Ctrl-I',
      };
    case 'linux':
      return {
        'tab-1': 'Alt-1',
        'tab-2': 'Alt-2',
        'tab-3': 'Alt-3',
        'tab-4': 'Alt-4',
        'tab-5': 'Alt-5',
        'tab-6': 'Alt-6',
        'tab-7': 'Alt-7',
        'tab-8': 'Alt-8',
        'tab-9': 'Alt-9',
        'tab-close': 'Ctrl-W',
        'tab-new': 'Ctrl-T',
        'tab-previous': 'Ctrl-Shift-Tab',
        'tab-next': 'Ctrl-Tab',
        'open-quick-input': 'Ctrl-K',
        'toggle-connections': 'Ctrl-P',
        'toggle-clusters': 'Ctrl-E',
        'toggle-identity': 'Ctrl-I',
      };
    case 'darwin':
      return {
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
  }
};

function getDefaultFonts(platform: Platform) {
  switch (platform) {
    case 'win32':
      return {
        sansSerif: "system-ui, 'Segoe WPC', 'Segoe UI', sans-serif",
        mono: "'Consolas', 'Courier New', monospace",
      };
    case 'linux':
      return {
        sansSerif: "system-ui, 'Ubuntu', 'Droid Sans', sans-serif",
        mono: "'Droid Sans Mono', 'Courier New', monospace, 'Droid Sans Fallback'",
      };
    case 'darwin':
      return {
        sansSerif: '-apple-system, BlinkMacSystemFont, sans-serif',
        mono: "Menlo, Monaco, 'Courier New', monospace",
      };
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
