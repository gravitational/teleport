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
import os from 'node:os';
import path from 'node:path';

import { app } from 'electron';
import {
  autoUpdater,
  DebUpdater,
  MacUpdater,
  AppUpdater as NativeUpdater,
  NsisUpdater,
  ProgressInfo,
  RpmUpdater,
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

export const TELEPORT_TOOLS_VERSION_ENV_VAR = 'TELEPORT_TOOLS_VERSION';

export class AppUpdater {
  private readonly logger = new Logger('AppUpdater');
  private readonly unregisterEventHandlers: () => void;
  private autoUpdatesStatus: AutoUpdatesStatus | undefined;
  private updateCheckResult: UpdateCheckResult | undefined;
  private checkForUpdatesPromise: Promise<void> | undefined;
  private downloadPromise: Promise<string[]> | undefined;
  private isUpdateDownloaded = false;
  private forceNoAutoDownload = false;

  constructor(
    private readonly storage: AppUpdaterStorage,
    private readonly getClusterVersions: () => Promise<GetClusterVersionsResponse>,
    readonly getDownloadBaseUrl: () => Promise<string>,
    private readonly emit: (event: AppUpdateEvent) => void,
    private versionEnvVar: string,
    /** Allows overring autoUpdater in tests. */
    private nativeUpdater: NativeUpdater = autoUpdater
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

    this.nativeUpdater.setFeedURL({
      provider: 'custom',
      // Wraps ClientToolsUpdateProvider to allow passing getClientToolsVersion.
      updateProvider: class extends ClientToolsUpdateProvider {
        constructor(
          options: unknown,
          updater: NativeUpdater,
          runtimeOptions: ProviderRuntimeOptions
        ) {
          super(getClientToolsVersion, updater, runtimeOptions);
        }
      },
    });

    this.nativeUpdater.logger = this.logger;
    this.nativeUpdater.allowDowngrade = true;
    this.nativeUpdater.autoDownload = false;
    // Must be set to true before any download starts.
    // electron-updater registers a listener to install the update when
    // the app quits, after the download has completed.
    // It can be then set to false, it the update shouldn't be installed
    // (except macOS).
    this.nativeUpdater.autoInstallOnAppQuit = true;
    // Enables checking for updates and downloading them in dev mode.
    // It makes testing this feature easier.
    // Only installing updates requires the packaged app.
    // Downloads are saved to the path specified in dev-app-update.yml.
    this.nativeUpdater.forceDevUpdateConfig = true;

    this.unregisterEventHandlers = registerEventHandlers(
      this.nativeUpdater,
      this.emit,
      () => this.autoUpdatesStatus,
      () => this.shouldAutoDownload()
    );
  }

  /** Must be called before `quit` event is emitted. */
  async dispose(): Promise<void> {
    this.unregisterEventHandlers();
    await this.preventInstallingOutdatedUpdates();
  }

  /**
   * Determines whether updates are supported for the current distribution.
   * Note: Updating `.tar.gz` archives is not supported, but `electron-updater`
   * incorrectly treats them as AppImage packages.
   */
  supportsUpdates(): boolean {
    return (
      this.nativeUpdater instanceof MacUpdater ||
      this.nativeUpdater instanceof NsisUpdater ||
      this.nativeUpdater instanceof DebUpdater ||
      this.nativeUpdater instanceof RpmUpdater
    );
  }

  /**
   * Checks for app updates.
   *
   * This method enhances the standard autoUpdater.checkForUpdates() by adding
   * the following behaviors:
   * - It allows update checks during an ongoing download process.
   * If a new update is found (or no update is available), the current download
   * is canceled.
   * - If downloading the update requires user confirmation, but the update has
   * already been downloaded, checking for updates will transition the updater
   * to the `update-downloaded` state (instead of staying in `update-available`
   * state).
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
      // The error from autoUpdater.checkForUpdates is surfaced to the UI through error event.
      this.logger.error('Failed to check for updates.', error);
    } finally {
      this.checkForUpdatesPromise = undefined;
    }
  }

  /** Not safe for concurrent use. */
  private async doCheckForUpdates(
    opts: {
      noAutoDownload?: boolean;
      /**
       * Whether this is a retry attempt.
       * Used as a guard to prevent infinite loops.
       */
      hasRetried?: boolean;
    } = {}
  ): Promise<void> {
    if (!this.supportsUpdates()) {
      return;
    }

    this.forceNoAutoDownload = opts.noAutoDownload;

    const result = await this.nativeUpdater.checkForUpdates();

    const newSha = result.updateInfo?.files[0]?.sha512;
    const oldSha = this.updateCheckResult?.updateInfo.files[0]?.sha512;
    const isSameUpdate = newSha && oldSha && newSha === oldSha;

    this.updateCheckResult = result;

    const updateUnavailable = !result.isUpdateAvailable;
    const updateChanged = !isSameUpdate;

    let downloadCanceled = false;
    if (updateUnavailable || updateChanged) {
      downloadCanceled = await this.cancelDownload();
    }

    if (
      result.isUpdateAvailable &&
      (this.shouldAutoDownload() ||
        // This can occur if the user manually downloads an update
        // and then triggers another check for updates.
        // Since the update is already downloaded, the updater should transition
        // to the `update-downloaded` state automatically.
        // The update file will be read from the local cache.
        this.isUpdateDownloaded)
    ) {
      void this.download();
      return;
    }

    // Retry to refresh the state so that the UI won't be showing
    // a cancellation error.
    if (downloadCanceled && !opts.hasRetried) {
      await this.doCheckForUpdates({ ...opts, hasRetried: true });
    }
  }

  /** Starts download. */
  async download(): Promise<string[]> {
    if (this.downloadPromise) {
      this.logger.info('Download already in progress.');
      return this.downloadPromise;
    }

    this.downloadPromise = this.nativeUpdater.downloadUpdate();
    try {
      await this.downloadPromise;
      this.isUpdateDownloaded = true;
    } catch (error) {
      // The error from autoUpdater.download is surfaced to the UI through error event.
      this.logger.error('Failed to download update.', error);
    } finally {
      this.downloadPromise = undefined;
    }
  }

  /** Cancels download. Returns true if aborted the network request. */
  async cancelDownload(): Promise<boolean> {
    if (!this.downloadPromise) {
      this.isUpdateDownloaded = false;
      return false;
    }

    // Due to a bug in electron-updater, we can't cancel downloads using cancellation
    // token passed to autoUpdater.download().
    // Repeatedly starting and canceling downloads causes the updater to go
    // into a broken state where it becomes unresponsive.
    // To avoid this, we instead close the network connections to abort
    // the current download.
    await this.nativeUpdater.netSession.closeAllConnections();
    try {
      await this.downloadPromise;
      return false;
    } catch {
      return true;
    } finally {
      this.isUpdateDownloaded = false;
    }
  }

  /**
   * Sets given cluster as managing app version.
   * When `undefined` is passed, the managing cluster is cleared.
   *
   * Immediately cancels an in-progress download and then checks for updates.
   */
  async changeManagingCluster(
    clusterUri: RootClusterUri | undefined
  ): Promise<void> {
    this.storage.put({ managingClusterUri: clusterUri });
    await this.cancelDownload();
    await this.checkForUpdates();
  }

  /**
   * Removes the managing cluster if it matches the given cluster URI.
   * Cancels any in-progress update that was triggered by this cluster.
   */
  async maybeRemoveManagingCluster(clusterUri: RootClusterUri): Promise<void> {
    const { managingClusterUri } = this.storage.get();
    if (managingClusterUri === clusterUri) {
      this.storage.put({ managingClusterUri: undefined });
    }

    // checkForUpdates will discard any update triggered by the removed managing
    // cluster. If a different update is found, do not download it automatically.
    // Currently, updates aren't checked in the background, and on Windows and Linux,
    // users may be surprised by an admin prompt when there is an update to install
    // after quitting the app.
    // We may revisit this behavior in the future. For example, if we introduce
    // a UI notification indicating that there's an update to be installed.
    await this.checkForUpdates({ noAutoDownload: true });
  }

  /**
   * Restarts the app and installs the update after it has been downloaded.
   * It should only be called after update-downloaded has been emitted.
   */
  quitAndInstall(): void {
    try {
      this.nativeUpdater.quitAndInstall();
    } catch (error) {
      this.logger.error('Failed to quit and install update', error);
    }
  }

  private shouldAutoDownload(): boolean {
    return (
      !this.forceNoAutoDownload &&
      this.autoUpdatesStatus?.enabled &&
      shouldAutoDownload(this.autoUpdatesStatus)
    );
  }

  private async refreshAutoUpdatesStatus(): Promise<void> {
    const { managingClusterUri } = this.storage.get();

    this.autoUpdatesStatus = await resolveAutoUpdatesStatus({
      versionEnvVar: this.versionEnvVar,
      managingClusterUri,
      getClusterVersions: this.getClusterVersions,
    });
    this.logger.info('Resolved auto updates status', this.autoUpdatesStatus);
  }

  /**
   * Workaround to prevent installing outdated updates.
   * electron-updater lacks support for this: once an update is downloaded,
   * it will be installed on quit—even if subsequent update checks report
   * no new updates.
   */
  private async preventInstallingOutdatedUpdates(): Promise<void> {
    if (this.isUpdateDownloaded) {
      return;
    }

    // Workaround for Windows and Linux.
    this.nativeUpdater.autoInstallOnAppQuit = false;

    // macOS-specific workaround:
    // On macOS, electron-updater downloads the update file and, if
    // `autoUpdater.autoInstallOnAppQuit` is true, passes it to the native Electron
    // autoUpdater via a local server. The update is then handed off to the Squirrel
    // framework for installation (either on demand or after quitting the app).
    // Unfortunately, once Squirrel gets the update, it is always installed
    // on quit, regardless of the `autoInstallOnAppQuit` value.
    // The only workaround I've found is to manually delete the ShipItState.plist
    // file so Squirrel cannot apply the update.
    // The downloaded update will be overwritten with the next update.
    if (this.nativeUpdater instanceof MacUpdater && app.isPackaged) {
      const squirrelPlistFilePath = path.join(
        os.homedir(),
        'Library',
        'Caches',
        'gravitational.teleport.connect.ShipIt',
        'ShipItState.plist'
      );
      try {
        await rm(squirrelPlistFilePath, {
          force: true,
        });
      } catch (error) {
        this.logger.error(error);
      }
    }
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
  nativeUpdater: NativeUpdater,
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
    emit({
      kind: 'error',
      error,
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

  nativeUpdater.on('checking-for-update', onCheckingForUpdate);
  nativeUpdater.on('update-available', onUpdateAvailable);
  nativeUpdater.on('update-not-available', onUpdateNotAvailable);
  nativeUpdater.on('error', onError);
  nativeUpdater.on('download-progress', onDownloadProgress);
  nativeUpdater.on('update-downloaded', onUpdateDownloaded);

  return () => {
    nativeUpdater.off('checking-for-update', onCheckingForUpdate);
    nativeUpdater.off('update-available', onUpdateAvailable);
    nativeUpdater.off('update-not-available', onUpdateNotAvailable);
    nativeUpdater.off('error', onError);
    nativeUpdater.off('download-progress', onDownloadProgress);
    nativeUpdater.off('update-downloaded', onUpdateDownloaded);
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
