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

import { rmSync } from 'node:fs';
import { rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import process from 'process';

import { app, autoUpdater as nar } from 'electron';
import {
  autoUpdater,
  AppUpdater as ElectronAppUpdater,
  MacUpdater,
  ProgressInfo,
  UpdateCheckResult,
  UpdateInfo,
} from 'electron-updater';
import { ProviderRuntimeOptions } from 'electron-updater/out/providers/Provider';

import type { GetClusterVersionsResponse } from 'gen-proto-ts/teleport/lib/teleterm/auto_update/v1/auto_update_service_pb';
import { AbortError } from 'shared/utils/error';

import Logger from 'teleterm/logger';
import { RootClusterUri } from 'teleterm/ui/uri';

import {
  AutoUpdatesEnabled,
  AutoUpdatesStatus,
  resolveAutoUpdatesStatus,
  shouldAutoDownload,
} from './autoUpdatesStatus';
import {
  ClientToolsUpdateProvider,
  ClientToolsVersionGetter,
} from './clientToolsUpdateProvider';

const TELEPORT_TOOLS_VERSION_ENV_VAR = 'TELEPORT_TOOLS_VERSION';

export class AppUpdater {
  private readonly logger = new Logger('AppUpdater');
  private readonly unregisterEventHandlers: () => void;
  private autoUpdatesStatus: AutoUpdatesStatus | undefined;
  private updateCheckResult: UpdateCheckResult | undefined;
  private checkForUpdatesPromise: Promise<void> | undefined;
  private downloadPromise: Promise<string[]> | undefined;
  private downloadedUpdatePath = '';
  private forceNoAutoDownload = false;

  constructor(
    private readonly storage: AppUpdaterStorage,
    private readonly getClusterVersions: () => Promise<GetClusterVersionsResponse>,
    readonly getDownloadBaseUrl: () => Promise<string>,
    private readonly emit: (event: AppUpdateEvent) => void
  ) {
    const getClientToolsVersion: ClientToolsVersionGetter = async () => {
      await this.refreshAutoUpdatesStatus();

      if (this.autoUpdatesStatus.enabled) {
        return {
          version: this.autoUpdatesStatus.version,
          baseUrl: await getDownloadBaseUrl(),
        };
      }
    };

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
    autoUpdater.allowDowngrade = true;
    autoUpdater.autoDownload = false;
    autoUpdater.autoInstallOnAppQuit = true;
    // Enables checking for updates and downloading them in dev mode.
    // It makes testing this feature easier.
    // Only installing updates requires the packaged app.
    // Downloads are saved to the path specified in dev-app-update.yml.
    autoUpdater.forceDevUpdateConfig = true;

    this.unregisterEventHandlers = registerEventHandlers(
      this.emit,
      () => this.autoUpdatesStatus,
      () => this.shouldAutoDownload()
    );

    app.on('will-quit', () => {
      if (!(autoUpdater instanceof MacUpdater)) {
        return;
      }
      if (!this.downloadedUpdatePath) {
        try {
          rmSync(getShipItCachePath(), {
            recursive: true,
            force: true,
            maxRetries: 2,
          });
        } catch {}
      }
    });
  }

  dispose(): void {
    this.unregisterEventHandlers();
  }

  /**
   * Checks for app updates and handles update state transitions.
   *
   * Behavior:
   * - Compares the new update with the previously stored update.
   * - Cancels and clears the current download if the update is unavailable or has changed.
   * - Starts downloading the update automatically if auto-download is enabled and allowed.
   * - Re-checks for updates if a download was canceled to refresh metadata.
   *
   * @param options - Optional configuration.
   * @param options.noAutoDownload - If `true`, disables automatic download after update check.
   * @returns A promise that resolves when the check and any follow-up actions are complete.
   */
  async checkForUpdates(
    options: { noAutoDownload?: boolean } = {}
  ): Promise<void> {
    if (this.checkForUpdatesPromise) {
      this.logger.info('Check for updates already in progress.');
      return this.checkForUpdatesPromise;
    }

    this.checkForUpdatesPromise = this.doCheckForUpdates(options);
    try {
      await this.checkForUpdatesPromise;
    } catch (error) {
      this.logger.error('Check for updates failed.', error);
    } finally {
      this.checkForUpdatesPromise = undefined;
    }
  }

  /** Not safe for concurrent use. */
  private async doCheckForUpdates(
    options: { noAutoDownload?: boolean } = {}
  ): Promise<void> {
    this.forceNoAutoDownload = options.noAutoDownload;
    const result = await autoUpdater.checkForUpdates();

    const newSha = result.updateInfo?.files[0]?.sha512;
    const oldSha = this.updateCheckResult?.updateInfo.files[0]?.sha512;
    const isSameUpdate = newSha && oldSha && newSha === oldSha;

    this.updateCheckResult = result;

    const updateUnavailable = !result.isUpdateAvailable;
    const updateChanged = !isSameUpdate;

    let canceledDownload = false;

    if (updateUnavailable || updateChanged) {
      canceledDownload = await this.cancelDownload();
      await this.clearDownloadedUpdateIfAny();
    }

    // This may happen if the user manually downloaded an update
    // and then checked for updates. Since we downloaded the update,
    // the updater should go to the updated state.
    if (this.shouldAutoDownload() || this.downloadedUpdatePath) {
      void this.download();
      return;
    }

    // If not auto downloading and the previous download was canceled,
    // refresh the updates to send new events to frontend.
    if (canceledDownload) {
      this.updateCheckResult = await autoUpdater.checkForUpdates();
    }
  }

  /** Starts download. */
  async download(): Promise<void> {
    console.warn('DOWNLOADINSTALL');
    if (this.downloadPromise) {
      this.logger.info('Download already in progress.');
      return this.downloadPromise;
    }

    this.downloadPromise = autoUpdater.downloadUpdate();
    try {
      // The second element in this array can be packagePath.
      const [updatePath] = await this.downloadPromise;
      this.downloadedUpdatePath = updatePath;
    } finally {
      console.warn('DOWNLOADINSTALLFINISHED');
      this.downloadPromise = undefined;
    }
  }

  /** Cancels download. Returns true if cancelled. */
  async cancelDownload(): Promise<boolean> {
    if (!this.downloadPromise) {
      return;
    }
    // In theory, we should pass a cancellation token to autoUpdater.downloadUpdate()
    // and cancel it.
    // Unfortunately, there is a bug in electron-updater: after starting and
    // canceling the download two times, the updater stops reacting to download
    // attempts.
    // There is no way to recover it from that broken state, besides killing the app.
    // Instead, we can abort all network connections performed by the updater
    // (there is only one - the download process).
    await autoUpdater.netSession.closeAllConnections();
    try {
      await this.downloadPromise;
    } catch {
      return true;
    }
    return false;
  }

  /**
   * Removes the previously downloaded update if it exists.
   * This prevents no longer valid updates from being applied on quit.
   */
  private async clearDownloadedUpdateIfAny(): Promise<void> {
    if (!this.downloadedUpdatePath) {
      return;
    }
    this.logger.info('Clearing update at', this.downloadedUpdatePath);
    try {
      await rm(this.downloadedUpdatePath);
      this.downloadedUpdatePath = '';
    } catch (error) {
      this.logger.error('Failed to clear update', error);
    }
  }

  /**
   * Sets given cluster as managing app version.
   * When `undefined` is passed, the managing cluster is cleared.
   *
   * Immediately cancels download when it's active and removes any potentially downloaded
   * version which no longer should be auto-applied.
   */
  async changeManagingCluster(
    clusterUri: RootClusterUri | undefined
  ): Promise<void> {
    this.storage.put({ managingClusterUri: clusterUri });
    await this.cancelDownload();
    await this.clearDownloadedUpdateIfAny();
    await this.checkForUpdates();
  }

  /**
   * Removes managing cluster if matches the passed cluster URI.
   *
   * If this cluster was considered during update status resolution,
   * any ongoing download is cancelled and any previously downloaded update is removed.
   */
  async maybeRemoveManagingCluster(clusterUri: RootClusterUri): Promise<void> {
    const { managingClusterUri } = this.storage.get();
    if (managingClusterUri === clusterUri) {
      this.storage.put({ managingClusterUri: undefined });
    }

    await this.checkForUpdates({ noAutoDownload: true });
  }

  /**
   * Restarts the app and installs the update after it has been downloaded.
   * It should only be called after update-downloaded has been emitted.
   */
  quitAndInstall(): void {
    autoUpdater.quitAndInstall();
  }

  private shouldAutoDownload(): boolean {
    return (
      !this.forceNoAutoDownload &&
      this.autoUpdatesStatus?.enabled &&
      shouldAutoDownload(this.autoUpdatesStatus)
    );
  }

  private async refreshAutoUpdatesStatus(): Promise<void> {
    const versionEnvVar = process.env[TELEPORT_TOOLS_VERSION_ENV_VAR];
    const { managingClusterUri } = this.storage.get();

    this.autoUpdatesStatus = await resolveAutoUpdatesStatus({
      versionEnvVar,
      managingClusterUri,
      getClusterVersions: this.getClusterVersions,
    });
    this.logger.info('Resolved auto updates status', this.autoUpdatesStatus);
  }
}

function getShipItCachePath(): string {
  return path.join(
    os.homedir(),
    'Library',
    'Caches',
    'gravitational.teleport.connect.ShipIt'
  );
}

export interface AppUpdaterStorage<
  T = {
    /** User-selected cluster managing updates. */
    managingClusterUri?: string;
  },
> {
  get(): T;
  put(value: Partial<T>): void;
}

function registerEventHandlers(
  emit: (event: AppUpdateEvent) => void,
  getAutoUpdatesStatus: () => AutoUpdatesStatus,
  getAutoDownload: () => boolean
): () => void {
  // updateInfo becomes defined when an update is available (see onUpdateAvailable).
  // It is later attached to other events, like 'download-progress' or 'error'.
  let updateInfo: UpdateInfo | undefined;

  const onCheckingForUpdate = () => {
    emit({
      kind: 'checking-for-update',
      autoUpdatesStatus: getAutoUpdatesStatus(),
    });
  };
  const onUpdateAvailable = (update: UpdateInfo) => {
    updateInfo = update;
    emit({
      kind: 'update-available',
      update,
      autoDownload: getAutoDownload(),
      autoUpdatesStatus: getAutoUpdatesStatus() as AutoUpdatesEnabled,
    });
  };
  const onUpdateNotAvailable = () => {
    updateInfo = undefined;
    emit({
      kind: 'update-not-available',
      autoUpdatesStatus: getAutoUpdatesStatus(),
    });
  };
  const onError = (error: Error) => {
    if (error.message.includes('net::ERR_ABORTED')) {
      error = new AbortError('Update download was canceled');
    }
    const serializedError = {
      name: error.name,
      message: error.message,
      cause: error.cause,
      stack: error.stack,
    };

    emit({
      kind: 'error',
      error: serializedError,
      update: updateInfo,
      autoUpdatesStatus: getAutoUpdatesStatus() as AutoUpdatesEnabled,
    });
  };
  const onDownloadProgress = (progress: ProgressInfo) =>
    emit({
      kind: 'download-progress',
      progress,
      update: updateInfo,
      autoUpdatesStatus: getAutoUpdatesStatus() as AutoUpdatesEnabled,
    });
  const onUpdateDownloaded = () =>
    emit({
      kind: 'update-downloaded',
      update: updateInfo,
      autoUpdatesStatus: getAutoUpdatesStatus() as AutoUpdatesEnabled,
    });

  autoUpdater.on('checking-for-update', onCheckingForUpdate);
  autoUpdater.on('update-available', onUpdateAvailable);
  autoUpdater.on('update-not-available', onUpdateNotAvailable);
  autoUpdater.on('error', onError);
  autoUpdater.on('download-progress', onDownloadProgress);
  autoUpdater.on('update-downloaded', onUpdateDownloaded);

  return () => {
    autoUpdater.off('checking-for-update', onCheckingForUpdate);
    autoUpdater.off('update-available', onUpdateAvailable);
    autoUpdater.off('update-not-available', onUpdateNotAvailable);
    autoUpdater.off('error', onError);
    autoUpdater.off('download-progress', onDownloadProgress);
    autoUpdater.off('update-downloaded', onUpdateDownloaded);
  };
}

/** Represents the various events during the app update process. */
export type AppUpdateEvent =
  | {
      /** Checking for an available update has started. */
      kind: 'checking-for-update';
      /**
       * Status of auto updates.
       * Empty if checking updates for the first time.
       */
      autoUpdatesStatus?: AutoUpdatesStatus;
    }
  | {
      /** An update is available. The update is downloaded automatically. */
      kind: 'update-available';
      /** Information about the available update. */
      update: UpdateInfo;
      /** Whether updates are downloaded automatically. */
      autoDownload: boolean;
      /** Status of enabled auto updates. */
      autoUpdatesStatus: AutoUpdatesEnabled;
    }
  | {
      /**  No update is available. */
      kind: 'update-not-available';
      /** Auto updates status, can be enabled or disabled. */
      autoUpdatesStatus: AutoUpdatesStatus;
    }
  | {
      /**  Error while checking for updates, downloading or installing. */
      kind: 'error';
      /** The error encountered during the update process. */
      error: Error;
      /**
       * Information about the update.
       * May be empty if an error happened when checking for updates.
       */
      update?: UpdateInfo;
      /**
       * Status of enabled auto updates.
       * May be empty if an error happened when checking for updates.
       */
      autoUpdatesStatus?: AutoUpdatesEnabled;
    }
  | {
      /** Indicates download progress of the update. */
      kind: 'download-progress';
      /** Information about the update being downloaded. */
      update: UpdateInfo;
      /** Download progress. */
      progress: ProgressInfo;
      /** Status of enabled auto updates. */
      autoUpdatesStatus: AutoUpdatesEnabled;
    }
  | {
      /** The update has been successfully downloaded. */
      kind: 'update-downloaded';
      /** Information about the downloaded update. */
      update: UpdateInfo;
      /** Status of enabled auto updates. */
      autoUpdatesStatus: AutoUpdatesEnabled;
    };
