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
import { rm } from 'node:fs/promises';
import path from 'node:path';

import { RuntimeSettings } from 'teleterm/mainProcess/types';

import type * as tsh from 'teleterm/services/tshd/types';

export interface AgentConfigFileClusterProperties {
  profileName: string;
  proxy: string;
  token: string;
  suggestedLabels: tsh.Label[];
}

export async function createAgentConfigFile(
  runtimeSettings: RuntimeSettings,
  clusterProperties: AgentConfigFileClusterProperties
): Promise<void> {
  const asyncExecFile = promisify(execFile);
  const agentDirectory = getAgentDirectoryOrThrow(
    runtimeSettings.userDataDir,
    clusterProperties.profileName
  );

  const configFile = path.resolve(agentDirectory, 'config.yaml');
  const dataDirectory = path.resolve(agentDirectory, 'data');

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
      `--labels=${clusterProperties.suggestedLabels
        .map(toNameAndValue)
        .join(',')}`,
    ],
    {
      timeout: 10_000, // 10 seconds
    }
  );
}

function getAgentDirectoryOrThrow(
  userDataDir: string,
  profileName: string
): string {
  const agentsDirectory = path.resolve(userDataDir, 'agents');
  const resolved = path.resolve(agentsDirectory, profileName);

  // check if the path doesn't contain any unexpected segments
  const isValidPath =
    path.dirname(resolved) === agentsDirectory &&
    path.basename(resolved) === profileName;
  if (!isValidPath) {
    throw new Error(`The agent config file path is incorrect: ${resolved}`);
  }
  return resolved;
}

function toNameAndValue(label: tsh.Label): string {
  return `${label.name}=${label.value}`;
}
