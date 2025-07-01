/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import {
  autoUpdater,
  CancellationToken,
  UpdateCheckResult,
} from 'electron-updater';

import { GetAutoUpdateResponse } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import { wait } from 'shared/utils/wait';

import { AppUpdater } from './appUpdater';
import { ClientToolsVersionGetter } from './clientToolsUpdateProvider';

// jest.mock('electron-updater', () => {
//   return {
//     Provider: class {},
//     autoUpdater: {
//       setFeedURL: jest.fn(),
//       checkForUpdates: jest.fn(),
//       downloadUpdate: jest.fn(),
//       quitAndInstall: jest.fn(),
//       checkForUpdatesAndNotify: jest.fn(),
//       on: jest.fn(),
//       off: jest.fn(),
//       logger: null,
//     },
//     CancellationToken: jest.fn().mockImplementation(() => ({
//       cancel: jest.fn(),
//     })),
//   };
// });
//
// let mockSender: {
//   send(): void;
// };
// let releaseFetcher: VersionFetcher;
//
// beforeEach(() => {
//   jest.clearAllMocks();
//   mockSender = { send: jest.fn() };
//   releaseFetcher = {
//     getAutoUpdate(): Promise<GetAutoUpdateResponse> {
//       return Promise.resolve({
//         url: '',
//         sha256: '',
//         toolsAutoUpdate: false,
//         toolsVersion: '1.0.0',
//       });
//     },
//   };
// });
//
// test('should configure autoUpdater in constructor', () => {
//   new AppUpdater(mockSender, releaseFetcher);
//   expect(autoUpdater.setFeedURL).toHaveBeenCalledWith(
//     expect.objectContaining({
//       provider: 'custom',
//     })
//   );
//   expect(autoUpdater.logger).toBeDefined();
//   expect(autoUpdater.allowDowngrade).toBe(true);
//   expect(autoUpdater.allowPrerelease).toBe(true);
//   expect(autoUpdater.autoInstallOnAppQuit).toBe(true);
// });
//
// test('checkForUpdates calls autoUpdater.checkForUpdates', async () => {
//   const mockResult: UpdateCheckResult = {
//     isUpdateAvailable: false,
//     versionInfo: {},
//     updateInfo: {
//       version: '1.0.0',
//       files: [],
//       releaseDate: '',
//       sha512: '',
//       path: '',
//     },
//   };
//   jest.spyOn(autoUpdater, 'checkForUpdates').mockResolvedValue(mockResult);
//   const updater = new AppUpdater(mockSender, releaseFetcher);
//
//   const result = await updater.checkForUpdates();
//   expect(result).toBe(mockResult);
//   expect(autoUpdater.checkForUpdates).toHaveBeenCalled();
// });
//
// test('downloadAndInstall triggers download and install once', async () => {
//   const downloadUpdateMock = jest.fn().mockResolvedValue(undefined);
//   const quitAndInstallMock = jest.fn();
//
//   autoUpdater.downloadUpdate = downloadUpdateMock;
//   autoUpdater.quitAndInstall = quitAndInstallMock;
//
//   const updater = new AppUpdater(mockSender, releaseFetcher);
//   await updater.downloadAndInstall();
//
//   expect(downloadUpdateMock).toHaveBeenCalled();
//   expect(quitAndInstallMock).toHaveBeenCalled();
// });
//
// test('downloadAndInstall returns same promise if called twice', async () => {
//   const resolveLater = async () => {
//     await wait(10);
//     return [];
//   };
//   jest.spyOn(autoUpdater, 'downloadUpdate').mockImplementation(resolveLater);
//   const updater = new AppUpdater(mockSender, releaseFetcher);
//
//   const promise1 = updater.downloadAndInstall();
//   const promise2 = updater.downloadAndInstall();
//
//   expect(promise1).toBe(promise2);
// });
//
// test('aborts download if signal is triggered', async () => {
//   const cancelMock = jest.fn();
//   (CancellationToken as jest.Mock).mockImplementation(() => ({
//     cancel: cancelMock,
//   }));
//   // (autoUpdater.downloadUpdate as jest.Mock).mockResolvedValue(undefined);
//   // (autoUpdater.quitAndInstall as jest.Mock).mockResolvedValue(undefined);
//
//   const updater = new AppUpdater(mockSender, releaseFetcher);
//   const abortController = new AbortController();
//
//   const promise = updater.downloadAndInstall(abortController.signal);
//   abortController.abort();
//
//   await promise;
//
//   expect(cancelMock).toHaveBeenCalled();
// });
//
// test('Symbol.dispose unregisters events', () => {
//   const unregisterMock = jest.fn();
//   const registerEventsMock = jest.fn().mockReturnValue(unregisterMock);
//
//   jest.doMock('./app-updater', () => {
//     const original = jest.requireActual('./app-updater');
//     return {
//       ...original,
//       registerEvents: registerEventsMock,
//     };
//   });
//
//   const updater = new AppUpdater(mockSender, releaseFetcher);
//   updater[Symbol.dispose]();
//
//   expect(unregisterMock).toHaveBeenCalled();
// });
