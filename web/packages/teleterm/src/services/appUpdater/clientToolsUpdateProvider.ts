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

import { app } from 'electron';
import {
  AppUpdater,
  DebUpdater,
  MacUpdater,
  NsisUpdater,
  Provider,
  ResolvedUpdateFileInfo,
  RpmUpdater,
  UpdateInfo,
} from 'electron-updater';
import { ProviderRuntimeOptions } from 'electron-updater/out/providers/Provider';

/**
 * Implements electron-updater's `Provider` using client tools version.
 */
export class ClientToolsUpdateProvider extends Provider<UpdateInfo> {
  constructor(
    private getClientToolsVersion: ClientToolsVersionGetter,
    private nativeUpdater: AppUpdater,
    runtimeOptions: ProviderRuntimeOptions
  ) {
    super(runtimeOptions);
  }

  async getLatestVersion(): Promise<UpdateInfo> {
    const autoUpdate = await this.getClientToolsVersion();

    // If no cluster manages the updates, return the current version.
    if (!autoUpdate) {
      return {
        version: app.getVersion(),
        releaseDate: '',
        path: '',
        sha512: '',
        files: [],
      };
    }

    const { cdnBaseUrl, version } = autoUpdate;

    const fileUrl = `https://${cdnBaseUrl}/${makeDownloadFilename(this.nativeUpdater, version)}`;
    const checksumUrl = `${fileUrl}.sha256`;
    const checksum = await fetch(checksumUrl, {
      signal: AbortSignal.timeout(5_000),
    });
    const checksumText = await checksum.text();
    const sha256 = checksumText.split(' ').at(0);

    return {
      version: version,
      releaseDate: '',
      path: '',
      sha512: '',
      files: [
        {
          url: fileUrl,
          // @ts-expect-error sha2 field doesn't exist in the types but is supported.
          sha2: sha256,
        },
      ],
    };
  }

  resolveFiles(updateInfo: UpdateInfo): ResolvedUpdateFileInfo[] {
    return updateInfo.files.map(fileInfo => ({
      url: new URL(fileInfo.url),
      info: fileInfo,
    }));
  }
}

/** Returns undefined when client version is not managed. */
export type ClientToolsVersionGetter = () => Promise<
  { cdnBaseUrl: string; version: string } | undefined
>;

function makeDownloadFilename(
  nativeUpdater: AppUpdater,
  version: string
): string {
  if (nativeUpdater instanceof MacUpdater) {
    return `Teleport Connect-${version}-mac.zip`;
  }
  if (nativeUpdater instanceof NsisUpdater) {
    return `Teleport Connect Setup-${version}.exe`;
  }
  if (nativeUpdater instanceof RpmUpdater) {
    return `teleport-connect-${version}.x86_64.rpm`;
  }
  if (nativeUpdater instanceof DebUpdater) {
    return `teleport-connect_${version}_amd64.deb`;
  }

  throw new Error(
    `Unsupported app updater: ${nativeUpdater?.constructor?.name}`
  );
}
