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
  AppUpdateEvent,
  AutoUpdatesEnabled,
  AutoUpdatesStatus,
  UpdateInfo,
} from 'teleterm/services/appUpdater';
import { shouldAutoDownload } from 'teleterm/services/appUpdater/autoUpdatesStatus';

export function makeUpdateInfo(
  nonTeleportCdn: boolean,
  version: string,
  updateKind: 'upgrade' | 'downgrade'
): UpdateInfo {
  return {
    files: [
      {
        url: nonTeleportCdn
          ? 'https://custom-hosting.local/Teleport%20Connect-mac.zip'
          : 'https://cdn.teleport.dev/Teleport%20Connect-mac.zip',
        sha512: '',
        size: 123214312,
      },
    ],
    releaseDate: '',
    updateKind,
    version,
    path: '',
    sha512: '',
  };
}

export function makeUpdateNotAvailableEvent(
  status: AutoUpdatesStatus
): AppUpdateEvent {
  return {
    kind: 'update-not-available',
    autoUpdatesStatus: status,
  };
}

export function makeCheckingForUpdateEvent(
  status?: AutoUpdatesStatus
): AppUpdateEvent {
  return {
    kind: 'checking-for-update',
    autoUpdatesStatus: status,
  };
}

export function makeUpdateAvailableEvent(
  updateInfo: UpdateInfo,
  status: AutoUpdatesEnabled
): AppUpdateEvent {
  return {
    kind: 'update-available',
    update: updateInfo,
    autoDownload: shouldAutoDownload(status),
    autoUpdatesStatus: status,
  };
}

export function makeDownloadProgressEvent(
  updateInfo: UpdateInfo,
  status: AutoUpdatesEnabled
): AppUpdateEvent {
  return {
    kind: 'download-progress',
    progress: {
      total: 123214312,
      transferred: 4322432,
      percent: 12,
      delta: 1,
      bytesPerSecond: 12333,
    },
    update: updateInfo,
    autoUpdatesStatus: status,
  };
}

export function makeErrorEvent(
  updateInfo: UpdateInfo,
  status: AutoUpdatesEnabled
): AppUpdateEvent {
  return {
    kind: 'error',
    error: new Error('No permissions'),
    update: updateInfo,
    autoUpdatesStatus: status,
  };
}

export function makeUpdateDownloadedEvent(
  updateInfo: UpdateInfo,
  status: AutoUpdatesEnabled
): AppUpdateEvent {
  return {
    kind: 'update-downloaded',
    update: updateInfo,
    autoUpdatesStatus: status,
  };
}
