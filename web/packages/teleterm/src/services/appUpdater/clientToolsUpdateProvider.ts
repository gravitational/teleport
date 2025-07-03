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

const CHECKSUM_FETCH_TIMEOUT = 5_000;

/** Implements electron-updater's `Provider` with client tools updates. */
export class ClientToolsUpdateProvider extends Provider<UpdateInfo> {
  constructor(
    private getClientToolsVersion: ClientToolsVersionGetter,
    private nativeUpdater: AppUpdater,
    runtimeOptions: ProviderRuntimeOptions
  ) {
    super(runtimeOptions);
  }

  override async getLatestVersion(): Promise<UpdateInfo> {
    const clientTools = await this.getClientToolsVersion();

    // If no client tools version is specified, return the current version
    // to simulate an up-to-date state.
    if (!clientTools) {
      return {
        version: app.getVersion(),
        releaseDate: '',
        path: '',
        sha512: '',
        files: [],
      };
    }

    const { baseUrl, version } = clientTools;
    const fileUrl = `https://${baseUrl}/${makeDownloadFilename(this.nativeUpdater, version)}`;
    const sha256 = await fetchChecksum(fileUrl);

    return {
      version: version,
      releaseDate: '',
      path: '',
      sha512: '',
      files: [
        {
          // Effective only on Windows.
          isAdminRightsRequired: true,
          url: fileUrl,
          // @ts-expect-error sha2 field doesn't exist in the types but is supported.
          sha2: sha256,
        },
      ],
    };
  }

  override resolveFiles(updateInfo: UpdateInfo): ResolvedUpdateFileInfo[] {
    return updateInfo.files.map(fileInfo => ({
      url: new URL(fileInfo.url),
      info: fileInfo,
    }));
  }
}

/** Should return undefined when client tools version is not available. */
export type ClientToolsVersionGetter = () => Promise<
  | {
      /** Base URL for downloading Teleport packages. e.g. cdn.teleport.dev. */
      baseUrl: string;
      /** Version to download. */
      version: string;
    }
  | undefined
>;

function makeDownloadFilename(updater: AppUpdater, version: string): string {
  if (updater instanceof MacUpdater) {
    return `Teleport Connect-${version}-mac.zip`;
  }
  if (updater instanceof NsisUpdater) {
    return `Teleport Connect Setup-${version}.exe`;
  }
  if (updater instanceof RpmUpdater) {
    return `teleport-connect-${version}.x86_64.rpm`;
  }
  if (updater instanceof DebUpdater) {
    return `teleport-connect_${version}_amd64.deb`;
  }

  throw new Error(`Unsupported app updater: ${updater?.constructor?.name}`);
}

async function fetchChecksum(fileUrl: string): Promise<string> {
  const checksumUrl = `${fileUrl}.sha256`;
  const checksum = await fetch(checksumUrl, {
    signal: AbortSignal.timeout(CHECKSUM_FETCH_TIMEOUT),
  });
  const checksumText = await checksum.text();
  // Extracts a checksum from text in the following format:
  // 60ce9fcaa4104c5746e31b576814b38f376136c08ea73dd5fe943f09725a00cf  Teleport Connect-17.5.4-mac.zip
  return checksumText.split(' ').at(0);
}
