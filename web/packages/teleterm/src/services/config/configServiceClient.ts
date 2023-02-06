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
          return (event.returnValue = configService.get());
        case ConfigServiceEventType.Update:
          return configService.update(item);
      }
    }
  );
}

export function createConfigServiceClient(): ConfigService {
  return {
    get: () =>
      ipcRenderer.sendSync(
        ConfigServiceEventChannel,
        ConfigServiceEventType.Get
      ),
    update: newConfig =>
      ipcRenderer.send(
        ConfigServiceEventChannel,
        ConfigServiceEventType.Update,
        newConfig
      ),
  };
}
