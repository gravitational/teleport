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
