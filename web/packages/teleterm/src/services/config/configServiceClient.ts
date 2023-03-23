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

import { ipcMain, ipcRenderer } from 'electron';

import {
  ConfigServiceEventChannel,
  ConfigServiceEventType,
} from '../../mainProcess/types';
import { ConfigService } from '../../services/config';

export function subscribeToConfigServiceEvents(
  configService: ConfigService
): void {
  ipcMain.on(
    ConfigServiceEventChannel,
    (event, eventType: ConfigServiceEventType, item) => {
      switch (eventType) {
        case ConfigServiceEventType.Get:
          return (event.returnValue = configService.get(item.path));
        case ConfigServiceEventType.Set:
          return configService.set(item.path, item.value);
        case ConfigServiceEventType.GetConfigError:
          return (event.returnValue = configService.getConfigError());
      }
    }
  );
}

export function createConfigServiceClient(): ConfigService {
  return {
    get: path =>
      ipcRenderer.sendSync(
        ConfigServiceEventChannel,
        ConfigServiceEventType.Get,
        { path }
      ),
    set: (path, value) => {
      ipcRenderer.send(ConfigServiceEventChannel, ConfigServiceEventType.Set, {
        path,
        value,
      });
    },
    getConfigError: () => {
      return ipcRenderer.sendSync(
        ConfigServiceEventChannel,
        ConfigServiceEventType.GetConfigError
      );
    },
  };
}
