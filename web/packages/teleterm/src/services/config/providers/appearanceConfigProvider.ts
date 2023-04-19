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

import { ConfigServiceProvider } from '../types';

export type AppearanceConfig = {
  fonts: {
    sansSerif: string;
    mono: string;
  };
};

export const appearanceConfigProvider: ConfigServiceProvider<AppearanceConfig> =
  {
    getDefaults(platform) {
      switch (platform) {
        case 'win32':
          return {
            fonts: {
              sansSerif: "system-ui, 'Segoe WPC', 'Segoe UI', sans-serif",
              mono: "'Consolas', 'Courier New', monospace",
            },
          };
        case 'linux':
          return {
            fonts: {
              sansSerif: "system-ui, 'Ubuntu', 'Droid Sans', sans-serif",
              mono: "'Droid Sans Mono', 'Courier New', monospace, 'Droid Sans Fallback'",
            },
          };
        case 'darwin':
          return {
            fonts: {
              sansSerif: '-apple-system, BlinkMacSystemFont, sans-serif',
              mono: "Menlo, Monaco, 'Courier New', monospace",
            },
          };
      }
    },
  };
