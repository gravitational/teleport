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

import { promisify } from 'node:util';
import { execFile } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';

import * as connectMyComputer from 'shared/connectMyComputer';

import { RootClusterUri, routing } from 'teleterm/ui/uri';
import { RuntimeSettings } from 'teleterm/mainProcess/types';

export interface CreateAgentConfigFileArgs {
  rootClusterUri: RootClusterUri;
  proxy: string;
  token: string;
  username: string;
}

export async function createAgentConfigFile(
  runtimeSettings: RuntimeSettings,
  args: CreateAgentConfigFileArgs
): Promise<void> {
  const asyncExecFile = promisify(execFile);
  const { configFile, dataDirectory } = generateAgentConfigPaths(
    runtimeSettings,
    args.rootClusterUri
  );

  // remove the config file if exists
  await fs.rm(configFile, { force: true });

  const labels = Object.entries({
    [connectMyComputer.NodeOwnerLabel]: args.username,
  })
    .map(keyAndValue => keyAndValue.join('='))
    .join(',');

  await asyncExecFile(
    runtimeSettings.agentBinaryPath,
    [
      'node',
      'configure',
      `--output=${configFile}`,
      `--data-dir=${dataDirectory}`,
      `--proxy=${args.proxy}`,
      `--token=${args.token}`,
      `--labels=${labels}`,
    ],
    {
      timeout: 10_000, // 10 seconds
    }
  );
}

export async function removeAgentDirectory(
  runtimeSettings: RuntimeSettings,
  rootClusterUri: RootClusterUri
): Promise<void> {
  const { agentDirectory } = generateAgentConfigPaths(
    runtimeSettings,
    rootClusterUri
  );
  // `force` ignores exceptions if path does not exist
  await fs.rm(agentDirectory, { recursive: true, force: true });
}

export async function isAgentConfigFileCreated(
  runtimeSettings: RuntimeSettings,
  rootClusterUri: RootClusterUri
): Promise<boolean> {
  const { configFile } = generateAgentConfigPaths(
    runtimeSettings,
    rootClusterUri
  );
  try {
    await fs.access(configFile);
    return true;
  } catch (e) {
    if (e.code === 'ENOENT') {
      return false;
    }
    throw e;
  }
}

/**
 * Returns agent config paths.
 * @param runtimeSettings must not come from the renderer process.
 * Otherwise, the generated paths may point outside the user's data directory.
 * @param rootClusterUri may be passed from the renderer process.
 */
export function generateAgentConfigPaths(
  runtimeSettings: RuntimeSettings,
  rootClusterUri: RootClusterUri
): {
  agentDirectory: string;
  configFile: string;
  logsDirectory: string;
  dataDirectory: string;
} {
  const parsed = routing.parseClusterUri(rootClusterUri);
  if (!parsed?.params?.rootClusterId) {
    throw new Error(`Incorrect root cluster URI: ${rootClusterUri}`);
  }

  const agentDirectory = getAgentDirectoryOrThrow(
    runtimeSettings.userDataDir,
    parsed.params.rootClusterId
  );
  const configFile = path.resolve(agentDirectory, 'config.yaml');
  const dataDirectory = path.resolve(agentDirectory, 'data');
  const logsDirectory = path.resolve(agentDirectory, 'logs');

  return {
    agentDirectory,
    configFile,
    dataDirectory,
    logsDirectory,
  };
}

export function getAgentsDir(userDataDir: string): string {
  // Why not put agentsDir into runtimeSettings? That's because we don't want the renderer to have
  // access to this value as it could lead to bad security practices.
  //
  // If agentsDir was sent from the renderer to tshd and the main process, those recipients could
  // not trust that agentsDir has not been tampered with. Instead, the renderer should merely send
  // the root cluster URI and the recipients should build the path to the specific agent dir from
  // that, with agentsDir being supplied out of band.
  return path.resolve(userDataDir, 'agents');
}

function getAgentDirectoryOrThrow(
  userDataDir: string,
  profileName: string
): string {
  const agentsDir = getAgentsDir(userDataDir);
  const resolved = path.resolve(agentsDir, profileName);

  // check if the path doesn't contain any unexpected segments
  const isValidPath =
    path.dirname(resolved) === agentsDir &&
    path.basename(resolved) === profileName;
  if (!isValidPath) {
    throw new Error(`The agent config path is incorrect: ${resolved}`);
  }
  return resolved;
}
