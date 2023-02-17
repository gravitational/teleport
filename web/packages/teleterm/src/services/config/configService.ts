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
