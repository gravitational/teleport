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
import fs from 'node:fs/promises';
import path from 'node:path';
import { promisify } from 'node:util';

import * as connectMyComputer from 'shared/connectMyComputer';

import { RuntimeSettings } from 'teleterm/mainProcess/types';
import { RootClusterUri, routing } from 'teleterm/ui/uri';

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

  const { stdout } = await asyncExecFile(
    runtimeSettings.agentBinaryPath,
    [
      'node',
      'configure',
      '--output=stdout',
      `--data-dir=${dataDirectory}`,
      `--proxy=${args.proxy}`,
      `--token=${args.token}`,
      `--labels=${labels}`,
    ],
    {
      timeout: 10_000, // 10 seconds
    }
  );

  try {
    await fs.mkdir(path.dirname(configFile), {
      // Create the agents dir too if it doesn't already exist.
      recursive: true,
    });
  } catch (error) {
    // Ignore error if directory already exists.
    if (error['code'] !== 'EEXIST') {
      throw error;
    }
  }

  await fs.writeFile(configFile, stdout + disableDebugServiceStanza);
}

// The debug service is enabled by default. It starts when the teleport agent is launched and it
// creates a debug.sock file in the data directory. Unfortunately, there's a length limit on the
// socket path – 107 characters on Linux and 104 characters on macOS [1]. If exceeded, creating a
// new listener fails with "bind: invalid argument".
//
// The default path for debug.sock for Connect My Computer on macOS is
// /Users/<user>/Library/Application Support/Teleport Connect/agents/<proxy hostname>/data/debug.sock
// The constant part is 76 characters which leaves just 28 characters for the hostname and user.
//
// As a workaround, we disable the debug service. This is going to work until someone adds another
// socket which is crucial to run a Teleport agent.
//
// See the GitHub issue for more details: https://github.com/gravitational/teleport/issues/43250
//
// [1] https://unix.stackexchange.com/questions/367008/why-is-socket-path-length-limited-to-a-hundred-chars
export const disableDebugServiceStanza = `
debug_service:
  enabled: false
`;

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
