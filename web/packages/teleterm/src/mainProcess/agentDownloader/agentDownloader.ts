/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { execFile } from 'node:child_process';
import { createReadStream } from 'node:fs';
import fs from 'node:fs/promises';
import path from 'node:path';
import { pipeline } from 'node:stream/promises';
import { promisify } from 'node:util';
import { createUnzip } from 'node:zlib';

import { extract } from 'tar-fs';

import { compareSemVers } from 'shared/utils/semVer';

import Logger from 'teleterm/logger';

import { RuntimeSettings } from '../types';
import type { IFileDownloader } from './fileDownloader';

const TELEPORT_CDN_ADDRESS = 'https://cdn.teleport.dev';
const TELEPORT_RELEASES_ADDRESS = 'https://rlz.teleport.sh/teleport?page=0';
const logger = new Logger('agentDownloader');
const asyncExecFile = promisify(execFile);

interface AgentBinary {
  version: string;
  platform: string;
  arch: string;
}

/**
 * Downloads and unpacks the agent binary, if it has not already been downloaded.
 *
 * The agent version to download is taken from settings.appVersion if settings.isLocalBuild is false.
 * If it isn't, we fetch the latest available stable version of the agent.
 * CONNECT_CMC_AGENT_VERSION is available as an escape hatch for cases where we want to fetch a different version.
 */
export async function downloadAgent(
  fileDownloader: IFileDownloader,
  settings: RuntimeSettings,
  env: Record<string, any>
): Promise<void> {
  const version = await calculateAgentVersion(settings, env);

  if (
    await isCorrectAgentVersionAlreadyDownloaded(
      settings.agentBinaryPath,
      version
    )
  ) {
    logger.info(`Agent v${version} is already downloaded. Skipping.`);
    return;
  }

  const tarballName = createAgentTarballName({
    arch: settings.arch,
    platform: settings.platform,
    version,
  });
  const url = `${TELEPORT_CDN_ADDRESS}/${tarballName}`;

  const agentTempDirectory = await fs.mkdtemp(
    path.join(settings.tempDataDir, 'connect-my-computer-')
  );
  await fileDownloader.run(url, agentTempDirectory);
  const tarballPath = path.join(agentTempDirectory, tarballName);
  await unpack(tarballPath, settings.sessionDataDir);
  await fs.rm(agentTempDirectory, { recursive: true });

  logger.info(`Downloaded agent v${version}.`);
}

async function calculateAgentVersion(
  settings: RuntimeSettings,
  env: Record<string, any>
): Promise<string> {
  if (!settings.isLocalBuild) {
    return settings.appVersion;
  }
  if (env.CONNECT_CMC_AGENT_VERSION) {
    return env.CONNECT_CMC_AGENT_VERSION;
  }
  return await fetchLatestTeleportRelease();
}

/**
 * Takes the first page of teleport releases (30 items) and looks for the highest version.
 * We don't have a way to simply take the latest tag.
 */
async function fetchLatestTeleportRelease(): Promise<string> {
  const response = await fetch(TELEPORT_RELEASES_ADDRESS);
  if (!response.ok) {
    throw new Error(
      `Failed to fetch ${TELEPORT_RELEASES_ADDRESS}. Status code: ${response.status}.`
    );
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
function createAgentTarballName(params: AgentBinary): string {
  const arch = params.arch === 'x64' ? 'amd64' : params.arch;
  return `teleport-v${params.version}-${params.platform}-${arch}-bin.tar.gz`;
}

async function isCorrectAgentVersionAlreadyDownloaded(
  agentBinaryPath: string,
  neededVersion: string
): Promise<boolean> {
  try {
    const agentVersion = await asyncExecFile(
      agentBinaryPath,
      ['version', '--raw'],
      { timeout: EXEC_AGENT_BINARY_TIMEOUT }
    );
    return agentVersion.stdout.trim() === neededVersion;
  } catch (e) {
    // When the agent is being downloaded for the first time, the binary does not yet exist.
    if (e.code !== 'ENOENT') {
      throw e;
    }
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

/**
 * verifyAgent checks if the binary can be executed. Used to trigger OS-level checks (like
 * Gatekeeper on macOS) before we actually do any real work with the binary.
 */
export async function verifyAgent(agentBinaryPath: string) {
  try {
    await asyncExecFile(agentBinaryPath, ['version', '--raw'], {
      timeout: EXEC_AGENT_BINARY_TIMEOUT,
    });
  } catch (error) {
    logger.error(
      `Error while verifying the agent: ${error}`,
      // Report the whole error object as the error objects returned by exec have extra metadata
      // such as exit code or signal of the process.
      JSON.stringify(error)
    );
    throw error;
  }
}

const EXEC_AGENT_BINARY_TIMEOUT = 10_000;
