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

interface UpdateCheckResult {
  isUpdateAvailable: boolean;
  updateInfo: UpdateInfo;
  downloadPromise?: Promise<Array<string>> | null;
  cancellationToken?: CancellationToken;
}

export class AppUpdater {
  private readonly logger = new Logger('AppUpdater');
  private readonly unregisterEventHandlers: () => void;
  private autoUpdatesStatus: AutoUpdatesStatus | undefined;
  private updateCheckResult: UpdateCheckResult | undefined;
  private downloadPromise: Promise<string[]> | undefined;
  private downloadedUpdatePath = '';

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
    autoUpdater.autoInstallOnAppQuit = true;
    // Enables checking for updates and downloading them in dev mode.
    // It makes testing this feature easier.
    // Only installing updates requires the packaged app.
    // Downloads are saved to the path specified in dev-app-update.yml.
    autoUpdater.forceDevUpdateConfig = true;

    this.unregisterEventHandlers = registerEventHandlers(
      this.emit,
      () => this.autoUpdatesStatus
    );
  }

  dispose(): void {
    this.unregisterEventHandlers();
  }

  /**
   * Sets given cluster as managing app version.
   * When `undefined` is passed, the managing cluster is cleared.
   *
   * Cancels download when it's active and removes any potentially downloaded
   * version which no longer should be auto-applied.
   */
  async changeManagingCluster(
    clusterUri: RootClusterUri | undefined
  ): Promise<void> {
    this.storage.put({ managingClusterUri: clusterUri });
    await this.cancelDownload();
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

    if (this.autoUpdatesStatus.enabled === false) {
      return;
    }

    const tookPartInHighestCompatibleResolution =
      this.autoUpdatesStatus.source === 'highest-compatible' &&
      this.autoUpdatesStatus.options.clusters.some(
        c => c.clusterUri === clusterUri
      );
    const wasManaging =
      this.autoUpdatesStatus.source === 'managing-cluster' &&
      this.autoUpdatesStatus.options.managingClusterUri === clusterUri;

    if (tookPartInHighestCompatibleResolution || wasManaging) {
      await this.cancelDownload();
      await this.clearDownloadedUpdateIfAny();
    }
  }

  /**
   * Asks the server whether there is an update.
   * Cancels
   * If a newer version is found and `autoUpdatesStatus` permits it,
   * the update is downloaded automatically.
   */
  async checkForUpdates(): Promise<void> {
    // Do nothing if download is in progress.
    // In the ideal world, if a new update info was returned but the previous
    // version would be downloaded, it would be cancelled and the user
    // would either see a new version
    if (this.downloadPromise) {
      return;
    }
    const result = await autoUpdater.checkForUpdates();
    const newSha = result.updateInfo?.files.at(0)?.sha512;
    const oldSha = this.updateCheckResult?.updateInfo.files.at(0)?.sha512;
    const isSameUpdate = oldSha === newSha;

    if (result.downloadPromise) {
      void this.storeDownloadPromise(result.downloadPromise);
    }

    this.updateCheckResult = result;
    const shouldClearDownload =
      !result.isUpdateAvailable || (!autoUpdater.autoDownload && !isSameUpdate);
    if (shouldClearDownload) {
      await this.clearDownloadedUpdateIfAny();
    }
  }

  /** Starts download manually. */
  async download(): Promise<void> {
    await this.storeDownloadPromise(autoUpdater.downloadUpdate());
  }

  /** Cancels download. */
  async cancelDownload(): Promise<void> {
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
    await this.downloadPromise.catch(() => {});
  }

  private async storeDownloadPromise(
    downloadPromise: Promise<string[]>
  ): Promise<void> {
    this.downloadPromise = downloadPromise;
    try {
      const paths = await downloadPromise;
      this.downloadedUpdatePath = paths.at(0);
    } catch {
      // Swallow error.
    } finally {
      this.downloadPromise = undefined;
    }
  }

  /**
   * Removes the previously downloaded update if it exists.
   * This prevents no longer valid updates from being applied on quit.
   */
  private async clearDownloadedUpdateIfAny(): Promise<void> {
    if (!this.downloadedUpdatePath) {
      return;
    }
    this.logger.info(
      'Changed managing cluster, clearing downloaded update at',
      this.downloadedUpdatePath
    );
    try {
      await rm(this.downloadedUpdatePath);
      this.downloadedUpdatePath = '';
    } catch (error) {
      this.logger.error('Failed to clear downloaded update', error);
    }
  }

  /**
   * Restarts the app and installs the update after it has been downloaded.
   * It should only be called after update-downloaded has been emitted.
   */
  quitAndInstall(): void {
    autoUpdater.quitAndInstall();
  }

  private async refreshAutoUpdatesStatus(): Promise<void> {
    const versionEnvVar = process.env[TELEPORT_TOOLS_VERSION_ENV_VAR];
    const { managingClusterUri } = this.storage.get();

    this.autoUpdatesStatus = await resolveAutoUpdatesStatus({
      versionEnvVar,
      managingClusterUri,
      getClusterVersions: this.getClusterVersions,
    });
    if (this.autoUpdatesStatus.enabled) {
      autoUpdater.autoDownload = shouldAutoDownload(this.autoUpdatesStatus);
    }
    this.logger.info('Resolved auto updates status', this.autoUpdatesStatus);
  }
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
  getAutoUpdatesStatus: () => AutoUpdatesStatus
): () => void {
  // updateInfo becomes defined when an update is available (see onUpdateAvailable).
  // It is later attached to other events, like 'download-progress' or 'error'.
  let updateInfo: UpdateInfo | undefined;

  const onCheckingForUpdate = () => {
    updateInfo = undefined;
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
      autoDownload: autoUpdater.autoDownload,
      autoUpdatesStatus: getAutoUpdatesStatus() as AutoUpdatesEnabled,
    });
  };
  const onUpdateNotAvailable = () =>
    emit({
      kind: 'update-not-available',
      autoUpdatesStatus: getAutoUpdatesStatus(),
    });
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
