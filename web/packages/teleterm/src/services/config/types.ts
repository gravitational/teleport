import { Platform } from '../../mainProcess/types';

import { KeyboardShortcutsConfig } from './providers/keyboardShortcutsConfigProvider';
import { AppearanceConfig } from './providers/appearanceConfigProvider';

export interface Config {
  keyboardShortcuts: KeyboardShortcutsConfig;
  appearance: AppearanceConfig;
}

export interface ConfigServiceProvider<T extends Record<string, any>> {
  getDefaults(platform: Platform): T;
}

export interface ConfigService {
  get(): Config;
  update(newConfig: Partial<Config>): void;
}
