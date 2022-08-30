import { merge } from 'lodash';

import { keyboardShortcutsConfigProvider } from './providers/keyboardShortcutsConfigProvider';
import { appearanceConfigProvider } from './providers/appearanceConfigProvider';
import { Config, ConfigService, ConfigServiceProvider } from './types';

type ConfigServiceProviders = {
  readonly [Property in keyof Config]: ConfigServiceProvider<Config[Property]>;
};

export class ConfigServiceImpl implements ConfigService {
  private config: Config;
  private configProviders: ConfigServiceProviders = {
    keyboardShortcuts: keyboardShortcutsConfigProvider,
    appearance: appearanceConfigProvider,
  };

  constructor() {
    this.createDefaultConfig();
  }

  get(): Config {
    return this.config;
  }

  update(newConfig: Partial<Config>): void {
    this.config = merge(this.config, newConfig);
  }

  private createDefaultConfig(): void {
    this.config = Object.entries(this.configProviders).reduce<Partial<Config>>(
      (partialConfig, [name, provider]) => {
        partialConfig[name] = merge({}, provider.getDefaults(process.platform));
        return partialConfig;
      },
      {}
    ) as Config;
  }
}
