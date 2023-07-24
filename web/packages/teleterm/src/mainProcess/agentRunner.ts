/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { spawn, ChildProcess } from 'node:child_process';
import os from 'node:os';

import Logger from 'teleterm/logger';
import { RootClusterUri } from 'teleterm/ui/uri';

import { generateAgentConfigPaths } from './createAgentConfigFile';
import { AgentProcessState, RuntimeSettings } from './types';

const MAX_STDERR_LINES = 10;

export class AgentRunner {
  private logger = new Logger('AgentRunner');
  private agentProcesses = new Map<RootClusterUri, ChildProcess>();

  constructor(
    private settings: RuntimeSettings,
    private sendProcessState: (
      rootClusterUri: RootClusterUri,
      state: AgentProcessState
    ) => void
  ) {}

  /**
   * Starts a new agent process.
   * If an existing process exists for the given root cluster, the old one will be killed with SIGKILL.
   * To kill the old process gracefully before starting the new one, use `kill()`.
   */
  start(rootClusterUri: RootClusterUri): void {
    if (this.agentProcesses.has(rootClusterUri)) {
      this.agentProcesses.get(rootClusterUri).kill('SIGKILL');
      this.logger.warn(`Forcefully killed agent process for ${rootClusterUri}`);
    }

    const { agentBinaryPath } = this.settings;
    const { configFile } = generateAgentConfigPaths(
      this.settings,
      rootClusterUri
    );

    const args = [
      'start',
      `--config=${configFile}`,
      this.settings.appVersion === '1.0.0-dev' && '--skip-version-check',
    ].filter(Boolean);

    this.logger.info(
      `Starting agent from ${agentBinaryPath} with arguments ${args}`
    );

    const agentProcess = spawn(agentBinaryPath, args, {
      windowsHide: true,
      env: process.env,
    });

    this.sendProcessState(rootClusterUri, {
      status: 'running',
    });

    this.addListeners(rootClusterUri, agentProcess);
    this.agentProcesses.set(rootClusterUri, agentProcess);
  }

  kill(rootClusterUri: RootClusterUri): void {
    this.agentProcesses.get(rootClusterUri).kill('SIGTERM');
    this.agentProcesses.delete(rootClusterUri);
  }

  killAll(): void {
    this.agentProcesses.forEach((agent, rootClusterUri) => {
      agent.kill('SIGTERM');
      this.agentProcesses.delete(rootClusterUri);
    });
  }

  private addListeners(
    rootClusterUri: RootClusterUri,
    process: ChildProcess
  ): void {
    // Teleport logs output to stderr.
    let stderrOutput = '';
    process.stderr.setEncoding('utf-8');
    process.stderr.on('data', error => {
      stderrOutput += error;
      stderrOutput = limitProcessOutputLines(stderrOutput);
    });

    const errorHandler = (error: Error) => {
      this.sendProcessState(rootClusterUri, {
        status: 'error',
        message: `${error}`,
      });
    };

    const exitHandler = (
      code: number | null,
      signal: NodeJS.Signals | null
    ) => {
      // Remove error handler when the process exits.
      process.off('error', errorHandler);

      this.sendProcessState(rootClusterUri, {
        status: 'exited',
        code,
        signal,
        stackTrace: code !== 0 ? stderrOutput : undefined,
      });
    };

    process.once('error', errorHandler);
    process.once('exit', exitHandler);
  }
}

function limitProcessOutputLines(output: string): string {
  return output.split(os.EOL).slice(-MAX_STDERR_LINES).join(os.EOL);
}
