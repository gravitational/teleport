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
  FileStorageEventChannel,
  FileStorageEventType,
} from '../../mainProcess/types';

import { FileStorage } from './fileStorage';

export function subscribeToFileStorageEvents(configService: FileStorage): void {
  ipcMain.on(
    FileStorageEventChannel,
    (event, eventType: FileStorageEventType, item) => {
      switch (eventType) {
        case FileStorageEventType.Get:
          return (event.returnValue = configService.get(item.key));
        case FileStorageEventType.Put:
          return configService.put(item.key, item.json);
        case FileStorageEventType.Write:
          return configService.write();
        case FileStorageEventType.Replace:
          return configService.replace(item.json);
        case FileStorageEventType.GetFilePath:
          return configService.getFilePath();
        case FileStorageEventType.GetFileLoadingError:
          return configService.getFileLoadingError();
      }
    }
  );
}

export function createFileStorageClient(): FileStorage {
  return {
    get: key =>
      ipcRenderer.sendSync(FileStorageEventChannel, FileStorageEventType.Get, {
        key,
      }),
    put: (key, json) =>
      ipcRenderer.send(FileStorageEventChannel, FileStorageEventType.Put, {
        key,
        json,
      }),
    write: () =>
      ipcRenderer.invoke(
        FileStorageEventChannel,
        FileStorageEventType.Write,
        {}
      ),
    replace: json =>
      ipcRenderer.send(FileStorageEventChannel, FileStorageEventType.Replace, {
        json,
      }),
    getFilePath: () =>
      ipcRenderer.sendSync(
        FileStorageEventChannel,
        FileStorageEventType.GetFilePath,
        {}
      ),
    getFileLoadingError: () =>
      ipcRenderer.sendSync(
        FileStorageEventChannel,
        FileStorageEventType.GetFileLoadingError,
        {}
      ),
  };
}
