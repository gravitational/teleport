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
        case ConfigServiceEventType.GetStoredConfigErrors:
          return (event.returnValue = configService.getStoredConfigErrors());
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
    getStoredConfigErrors: () => {
      return ipcRenderer.sendSync(
        ConfigServiceEventChannel,
        ConfigServiceEventType.GetStoredConfigErrors
      );
    },
  };
}
