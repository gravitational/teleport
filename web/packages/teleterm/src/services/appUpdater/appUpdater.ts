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

import { rm } from 'node:fs/promises';
import process from 'process';

import {
  autoUpdater,
  CancellationToken,
  AppUpdater as ElectronAppUpdater,
  ProgressInfo,
  UpdateInfo,
} from 'electron-updater';
import { ProviderRuntimeOptions } from 'electron-updater/out/providers/Provider';

import { GetAutoUpdateResponse } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';

import Logger from 'teleterm/logger';
import { RendererIpc } from 'teleterm/mainProcess/types';
import {
  ResolvedUpdateSource,
  UnresolvedUpdate,
} from 'teleterm/services/appUpdater/updateSource';
import { FileStorage } from 'teleterm/services/fileStorage';
import { RootClusterUri } from 'teleterm/ui/uri';

import {
  ClientToolsUpdateProvider,
  ClientToolsVersionGetter,
} from './clientToolsUpdateProvider';
import { resolveUpdateSource, UpdateSource } from './updateSource';

export class AppUpdater {
  private readonly logger = new Logger('AppUpdater');
  private readonly unregisterEvents: () => void;
  private downloadedFilePath: string | undefined;
  private updateSource: UpdateSource | undefined;
  private downloadCancellationToken: CancellationToken | undefined;

  constructor(
    eventsSender: Sender,
    private getAutoUpdateFn: () => Promise<GetAutoUpdateResponse>,
    private statePersistenceService: FileStorage
  ) {
    const getClientToolsVersion = () => this.getClientToolsVersion();

    autoUpdater.setFeedURL({
      provider: 'custom',
      // Wraps ClientToolsUpdateProvider to allow passing getClientToolsVersion.
      updateProvider: class extends ClientToolsUpdateProvider {
        constructor(
          options: unknown,
          updater: ElectronAppUpdater,
          runtimeOptions: ProviderRuntimeOptions
        ) {
          super(getClientToolsVersion, updater, runtimeOptions);
        }
      },
    });

    autoUpdater.logger = this.logger;
    autoUpdater.autoDownload = true;
    autoUpdater.allowDowngrade = true;
    autoUpdater.allowPrerelease = true;
    autoUpdater.forceDevUpdateConfig = true;
    autoUpdater.autoInstallOnAppQuit = true;

    this.unregisterEvents = registerEvents(
      eventsSender,
      () => this.updateSource
    );
    autoUpdater.on('update-downloaded', e => {
      this.downloadedFilePath = e.downloadedFile;
    });
  }

  dispose() {
    this.unregisterEvents();
  }

  /**
   * Sets given cluster as managing app version.
   * Cancels download when it's active and removes any potentially downloaded
   * version which no longer should be auto-applied.
   */
  async changeUpdatesSource(
    source:
      | { kind: 'auto' }
      | { kind: 'cluster-override'; clusterUri: RootClusterUri }
  ): Promise<void> {
    this.cancelDownload();
    if (source.kind === 'auto') {
      this.statePersistenceService.put('managedUpdateSource', undefined);
    } else {
      this.statePersistenceService.put(
        'managedUpdateSource',
        source.clusterUri
      );
    }
    if (this.downloadedFilePath) {
      this.logger.info(
        'Changed managed update source, clearing downloaded update at',
        this.downloadedFilePath
      );
      await rm(this.downloadedFilePath);
    }
    await this.checkForUpdates();
  }

  /**
   * Asks the server whether there is an update.
   * If a new version is available, it is automatically downloaded.
   */
  async checkForUpdates(): Promise<void> {
    const result = await autoUpdater.checkForUpdates();
    this.downloadCancellationToken = result.cancellationToken;
  }

  /** Cancels download when in progress. */
  cancelDownload(): void {
    this.downloadCancellationToken?.cancel();
  }

  /**
   * Restarts the app and installs the update after it has been downloaded.
   * It should only be called after update-downloaded has been emitted.
   */
  quitAndInstall(): void {
    autoUpdater.quitAndInstall();
  }

  private getClientToolsVersion: ClientToolsVersionGetter = async () => {
    const updateSource = await resolveUpdateSource({
      getEnvVars: () => ({
        TELEPORT_TOOLS_VERSION: process.env['TELEPORT_TOOLS_VERSION'],
        TELEPORT_CDN_BASE_URL:
          process.env['TELEPORT_CDN_BASE_URL'] || 'cdn.teleport.dev',
      }),
      getAutoUpdate: async () => this.getAutoUpdateFn().then(a => a.versions),
      getManagedVersion: () =>
        this.statePersistenceService.get('managedUpdateSource'),
    });
    // Update whenever we fetch a new version.
    this.updateSource = updateSource;
    if (!updateSource.resolved) {
      return;
    }
    return updateSource;
  };
}

function registerEvents(
  sender: Sender,
  getManagedUpdate: () => UpdateSource
): () => void {
  let updateInfo: UpdateInfo | undefined;
  const sendAppUpdateEvent = (event: AppUpdateEvent) =>
    sender.send(RendererIpc.AppUpdateEvent, event);

  const checkingForUpdate = () => {
    updateInfo = undefined;
    sendAppUpdateEvent({
      kind: 'checking-for-update',
      updateSource: getManagedUpdate(),
    });
  };
  const updateAvailable = (update: UpdateInfo) => {
    updateInfo = update;
    sendAppUpdateEvent({
      kind: 'update-available',
      update,
      updateSource: getManagedUpdate() as ResolvedUpdateSource,
    });
  };
  const updateCancelled = (update: UpdateInfo) => {
    sendAppUpdateEvent({
      kind: 'update-cancelled',
      update,
      updateSource: getManagedUpdate() as ResolvedUpdateSource,
    });
  };
  const updateNotAvailable = () =>
    sendAppUpdateEvent({
      kind: 'update-not-available',
      updateSource: getManagedUpdate(),
    });
  const error = (error: Error) => {
    // Cloning functions throws an error.
    delete error.toString;

    sendAppUpdateEvent({
      kind: 'error',
      error: error,
      update: updateInfo,
      updateSource: getManagedUpdate(),
    });
  };
  const downloadProgress = (progress: ProgressInfo) =>
    sendAppUpdateEvent({
      kind: 'download-progress',
      progress,
      update: updateInfo,
      updateSource: getManagedUpdate() as ResolvedUpdateSource,
    });
  const updateDownloaded = () =>
    sendAppUpdateEvent({
      kind: 'update-downloaded',
      update: updateInfo,
      updateSource: getManagedUpdate() as ResolvedUpdateSource,
    });

  autoUpdater.on('checking-for-update', checkingForUpdate);
  autoUpdater.on('update-available', updateAvailable);
  autoUpdater.on('update-not-available', updateNotAvailable);
  autoUpdater.on('update-cancelled', updateCancelled);
  autoUpdater.on('error', error);
  autoUpdater.on('download-progress', downloadProgress);
  autoUpdater.on('update-downloaded', updateDownloaded);

  return () => {
    autoUpdater.off('checking-for-update', checkingForUpdate);
    autoUpdater.off('update-available', updateAvailable);
    autoUpdater.off('update-not-available', updateNotAvailable);
    autoUpdater.off('update-cancelled', updateCancelled);
    autoUpdater.off('error', error);
    autoUpdater.off('download-progress', downloadProgress);
    autoUpdater.off('update-downloaded', updateDownloaded);
  };
}

export type CombinedEvent =
  | (ResolvedUpdateSource & { updateEvent: AppUpdateEvent })
  | UnresolvedUpdate;

/** Represents the different kinds of events that can occur during an app's update process. */
export type AppUpdateEvent =
  /** Event emitted when checking for an available update has started. */
  | {
      kind: 'checking-for-update';
      updateSource?: UpdateSource;
    }
  /** Emitted when there is an available update. The update is downloaded automatically. */
  | {
      kind: 'update-available';
      update: UpdateInfo;
      updateSource: ResolvedUpdateSource;
    }
  /** Event emitted when no update is available. */
  | {
      kind: 'update-not-available';
      updateSource: UpdateSource;
    }
  /** Event emitted when there is an error while updating. */
  | {
      kind: 'error';
      error: Error;
      update?: UpdateInfo;
      updateSource?: UpdateSource;
    }
  /** Event emitted when the update was canceled. */
  | {
      kind: 'update-cancelled';
      update: UpdateInfo;
      updateSource: ResolvedUpdateSource;
    }
  /** Event emitted to indicate download progress of the update. */
  | {
      kind: 'download-progress';
      update: UpdateInfo;
      progress: ProgressInfo;
      updateSource: ResolvedUpdateSource;
    }
  /** Event emitted when the update has been successfully downloaded. */
  | {
      kind: 'update-downloaded';
      update: UpdateInfo;
      updateSource: ResolvedUpdateSource;
    };

interface Sender {
  send(channel: string, ...args: any[]): void;
}
