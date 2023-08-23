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

import { promisify } from 'node:util';
import { execFile } from 'node:child_process';
import { access, rm } from 'node:fs/promises';
import path from 'node:path';

import { RootClusterUri, routing } from 'teleterm/ui/uri';
import { RuntimeSettings } from 'teleterm/mainProcess/types';

import type * as tsh from 'teleterm/services/tshd/types';

export interface AgentConfigFileClusterProperties {
  rootClusterUri: RootClusterUri;
  proxy: string;
  token: string;
  labels: tsh.Label[];
}

export async function createAgentConfigFile(
  runtimeSettings: RuntimeSettings,
  clusterProperties: AgentConfigFileClusterProperties
): Promise<void> {
  const asyncExecFile = promisify(execFile);
  const { configFile, dataDirectory } = generateAgentConfigPaths(
    runtimeSettings,
    clusterProperties.rootClusterUri
  );

  // remove the config file if exists
  try {
    await rm(configFile);
  } catch (e) {
    if (e.code !== 'ENOENT') {
      throw e;
    }
  }

  await asyncExecFile(
    runtimeSettings.agentBinaryPath,
    [
      'node',
      'configure',
      `--output=${configFile}`,
      `--data-dir=${dataDirectory}`,
      `--proxy=${clusterProperties.proxy}`,
      `--token=${clusterProperties.token}`,
      `--labels=${clusterProperties.labels.map(toNameAndValue).join(',')}`,
    ],
    {
      timeout: 10_000, // 10 seconds
    }
  );
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
    await access(configFile);
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
  configFile: string;
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

  return {
    configFile,
    dataDirectory,
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

function toNameAndValue(label: tsh.Label): string {
  return `${label.name}=${label.value}`;
}
