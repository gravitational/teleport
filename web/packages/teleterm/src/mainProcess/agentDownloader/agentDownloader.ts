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

import { pipeline } from 'node:stream/promises';
import { createReadStream } from 'node:fs';
import { join } from 'node:path';
import { createUnzip } from 'node:zlib';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';

import { extract } from 'tar-fs';

import { compareSemVers } from 'shared/utils/semVer';

import Logger from 'teleterm/logger';

import { RuntimeSettings } from '../types';

import type { IFileDownloader } from './fileDownloader';

const TELEPORT_CDN_ADDRESS = 'https://cdn.teleport.dev';
const logger = new Logger('agentDownloader');

interface AgentBinaryParams {
  version: string;
  platform: string;
  arch: string;
}

/**
 * Downloads and unpacks the agent binary, if it has not already been downloaded.
 *
 * The agent version to download is taken from settings.appVersion if it is not a dev version (1.0.0-dev).
 * The settings.appVersion is set to a real version only for packaged apps that went through our CI build pipeline.
 * In local builds, both for the development version and for packaged apps, settings.appVersion is set to 1.0.0-dev.
 * In those cases, we fetch the latest available stable version of the agent.
 * CONNECT_CMC_AGENT_VERSION is available as an escape hatch for cases where we want to fetch a different version.
 */
export async function downloadAgent(
  fileDownloader: IFileDownloader,
  settings: RuntimeSettings
): Promise<void> {
  const version = await calculateAgentVersion(settings.appVersion);

  if (await isAgentAlreadyDownloaded(settings.agentBinaryPath, version)) {
    logger.info(`Agent v.${version} is already downloaded. Skipping.`);
    return;
  }

  const binaryName = createAgentBinaryName({
    arch: settings.arch,
    platform: settings.platform,
    version,
  });
  const url = `${TELEPORT_CDN_ADDRESS}/${binaryName}`;
  await fileDownloader.run(url, settings.tempDataDir);

  await unpack(join(settings.tempDataDir, binaryName), settings.sessionDataDir);

  logger.info(`Downloaded agent v.${version}.`);
}

async function calculateAgentVersion(appVersion: string): Promise<string> {
  if (appVersion !== '1.0.0-dev') {
    return appVersion;
  }
  if (process.env.CONNECT_CMC_AGENT_VERSION) {
    return process.env.CONNECT_CMC_AGENT_VERSION;
  }
  return await fetchLatestTeleportRelease();
}

/**
 * Takes the first page of teleport releases (30 items) and looks for the highest version.
 * We don't have a way to simply take the latest tag.
 */
async function fetchLatestTeleportRelease(): Promise<string> {
  const response = await fetch('https://rlz.teleport.sh/teleport?page=0');
  if (!response.ok) {
    throw response;
  }
  const teleportVersions = (
    (await response.json()) as {
      version: string;
    }[]
  ).map(r => r.version);

  // get the last element
  const latest = teleportVersions.sort(compareSemVers)?.at(-1);
  if (latest) {
    return latest;
  }
  throw new Error('Failed to read the latest teleport release.');
}

/**
 * Generates following binary names:
 * teleport-v<version>-linux-arm64-bin.tar.gz
 * teleport-v<version>-linux-amd64-bin.tar.gz
 * teleport-v<version>-darwin-arm64-bin.tar.gz
 * teleport-v<version>-darwin-amd64-bin.tar.gz
 */
function createAgentBinaryName(params: AgentBinaryParams): string {
  const arch = params.arch === 'x64' ? 'amd64' : params.arch;
  return `teleport-v${params.version}-${params.platform}-${arch}-bin.tar.gz`;
}

async function isAgentAlreadyDownloaded(
  agentBinaryPath: string,
  neededVersion: string
): Promise<boolean> {
  const asyncExec = promisify(exec);
  try {
    const agentVersion = await asyncExec(
      `${agentBinaryPath.replace(/ /g, '\\ ')} version --raw`,
      {
        timeout: 10_000, // 10 seconds
      }
    );
    return agentVersion.stdout.trim() === neededVersion;
  } catch (e) {
    logger.error(e);
    return false;
  }
}

function unpack(sourceFile: string, targetDirectory: string): Promise<void> {
  return pipeline(
    createReadStream(sourceFile),
    createUnzip(),
    extract(targetDirectory, {
      ignore: (_, headers) => {
        // Keep only the teleport binary
        return headers.name !== 'teleport/teleport';
      },
    })
  );
}
