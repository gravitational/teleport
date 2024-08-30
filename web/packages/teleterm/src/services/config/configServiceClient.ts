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

import { ipcMain, ipcRenderer } from 'electron';

import { CONFIG_MODIFIABLE_FROM_RENDERER } from 'teleterm/services/config/appConfigSchema';

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
          if (!CONFIG_MODIFIABLE_FROM_RENDERER.includes(item.path)) {
            throw new Error(
              `Could not update "${item.path}". This field is readonly in the renderer process.`
            );
          }
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
