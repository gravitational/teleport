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

import { ChildProcess, fork, spawn } from 'node:child_process';
import fs from 'node:fs/promises';
import os from 'node:os';

import stripAnsi from 'strip-ansi';

import Logger from 'teleterm/logger';
import { createFileLoggerService, LoggerColor } from 'teleterm/services/logger';
import { RootClusterUri } from 'teleterm/ui/uri';

import { generateAgentConfigPaths } from '../createAgentConfigFile';
import { terminateWithTimeout } from '../terminateWithTimeout';
import { AgentProcessState, RuntimeSettings } from '../types';

const MAX_STDERR_LINES = 10;

export class AgentRunner {
  private logger = new Logger('AgentRunner');
  private agentProcesses = new Map<
    RootClusterUri,
    {
      process: ChildProcess;
      state: AgentProcessState;
      /**
       * logs contains last 10 lines of logs from stderr of the agent.
       */
      logs: string;
    }
  >();

  constructor(
    private settings: RuntimeSettings,
    private agentCleanupDaemonPath: string,
    private sendProcessState: (
      rootClusterUri: RootClusterUri,
      state: AgentProcessState
    ) => void
  ) {}

  /**
   * Starts a new agent process.
   * If an existing process exists for the given root cluster, the old one will be killed.
   */
  async start(rootClusterUri: RootClusterUri): Promise<ChildProcess> {
    if (this.agentProcesses.has(rootClusterUri)) {
      await this.kill(rootClusterUri);
    }

    const { agentBinaryPath } = this.settings;
    const { configFile } = generateAgentConfigPaths(
      this.settings,
      rootClusterUri
    );

    const args = [
      'start',
      `--config=${configFile}`,
      this.settings.isLocalBuild && '--skip-version-check',
      this.settings.insecure && '--insecure',
    ].filter(Boolean);

    this.logger.info(
      `Starting agent for ${rootClusterUri} from ${agentBinaryPath} with arguments ${args.join(
        ' '
      )}`
    );

    const agentProcess = spawn(agentBinaryPath, args, {
      windowsHide: true,
    });

    this.agentProcesses.set(rootClusterUri, {
      process: agentProcess,
      state: { status: 'not-started' },
      logs: '',
    });
    this.addAgentListeners(rootClusterUri, agentProcess);
    this.setupCleanupDaemon(rootClusterUri, agentProcess);

    return agentProcess;
  }

  /**
   * tryRemoveAgentBinary removes the agent binary but only if all agents are stopped.
   *
   * Rejects on filesystem errors.
   */
  async tryRemoveAgentBinary(): Promise<void> {
    // If we remove the binary while an agent is running, the agent will continue to run but it
    // won't be able to spawn new shells.
    if (!this.areAllAgentsStopped()) {
      this.logger.info(
        'Skipping agent binary removal, not all agents are stopped.'
      );
      return;
    }

    await fs.rm(this.settings.agentBinaryPath, { force: true });
  }

  getState(rootClusterUri: RootClusterUri): AgentProcessState | undefined {
    return this.agentProcesses.get(rootClusterUri)?.state;
  }

  getLogs(rootClusterUri: RootClusterUri): string | undefined {
    return this.agentProcesses.get(rootClusterUri)?.logs;
  }

  async kill(rootClusterUri: RootClusterUri): Promise<void> {
    const agent = this.agentProcesses.get(rootClusterUri);
    if (!agent) {
      return;
    }
    this.logger.info(`Killing agent for ${rootClusterUri}`);
    await terminateWithTimeout(agent.process);
  }

  async killAll(): Promise<void> {
    const agents = Array.from(this.agentProcesses.values());
    await Promise.all(agents.map(agent => terminateWithTimeout(agent.process)));
  }

  private addAgentListeners(
    rootClusterUri: RootClusterUri,
    process: ChildProcess
  ): void {
    let stderrOutput = '';
    this.agentProcesses.get(rootClusterUri).logs = stderrOutput;

    // Teleport logs output to stderr.
    process.stderr.setEncoding('utf-8');
    process.stderr.on('data', (error: string) => {
      stderrOutput += error;
      // TODO(ravicious): Pipe into KeepLastChunks instead.
      stderrOutput = processAgentOutput(stderrOutput);
      this.agentProcesses.get(rootClusterUri).logs = stderrOutput;
    });

    const spawnHandler = () => {
      const { logsDirectory } = generateAgentConfigPaths(
        this.settings,
        rootClusterUri
      );
      createFileLoggerService({
        dev: this.settings.dev,
        dir: logsDirectory,
        name: 'teleport',
        loggerNameColor: LoggerColor.Green,
        passThroughMode: true,
        omitTimestamp: true,
      }).pipeProcessOutputIntoLogger(process);

      this.updateProcessState(rootClusterUri, {
        status: 'running',
      });
    };

    const errorHandler = (error: Error) => {
      process.off('spawn', spawnHandler);
      // close is emitted both when the process ends _and_ after an error on spawn. We have to turn
      // off closeHandler here to make sure that when the agent fails to spawn, we don't override
      // the error state with the close state.
      process.off('close', closeHandler);

      this.updateProcessState(rootClusterUri, {
        status: 'error',
        message: `${error}`,
      });
    };

    const closeHandler = (
      code: number | null,
      signal: NodeJS.Signals | null
    ) => {
      const exitedSuccessfully = code === 0 || signal === 'SIGTERM';

      this.updateProcessState(rootClusterUri, {
        status: 'exited',
        code,
        signal,
        exitedSuccessfully,
        logs: exitedSuccessfully ? undefined : stderrOutput,
      });
    };

    process.once('spawn', spawnHandler);
    process.once('error', errorHandler);
    // Using close instead of exit to ensure stderr has been closed and we captured all logs.
    process.once('close', closeHandler);
  }

  private setupCleanupDaemon(
    rootClusterUri: RootClusterUri,
    agent: ChildProcess
  ) {
    agent.once('spawn', () => {
      const cleanupDaemon = fork(
        this.agentCleanupDaemonPath,
        [
          // agent.pid can in theory be null if the agent gets terminated before the execution gets to
          // this point. In that case, the cleanup daemon is going to exit early.
          agent.pid?.toString(),
          process.pid.toString(),
          rootClusterUri,
          this.settings.logsDir,
        ],
        // Inherit stderr and stdout so that any errors emitted during Node.js process startup will
        // be visible when running Connect from a terminal.
        //
        // In dev mode, stdout from cleanup daemon will be visible in the terminal output but it
        // won't be colored like other logs.
        //
        // It'd be better to pipe stdout and stderr instead and log them (see
        // pipeProcessOutputIntoLogger) but this would require a more elaborate setup (pipe stdout
        // after fork and then stop piping on successful start since agent cleanup daemon has its
        // own logging).
        { stdio: 'inherit' }
      );

      // The cleanup daemon terminates the agent only when the parent (this process) gets
      // unexpectedly killed and loses control over the agent by orphaning it.
      //
      // We must ensure that whenever an agent is running, a cleanup daemon is running as well.
      // That's why we have the listeners on the child processes below.

      // The cleanup daemon failing to start.
      const errorHandler = () => {
        this.logger.error(
          `Cleanup daemon for ${rootClusterUri} has failed to start. Terminating agent.`
        );
        terminateWithTimeout(agent);
      };
      cleanupDaemon.once('error', errorHandler);
      cleanupDaemon.once('spawn', () => {
        // Error handler is no longer needed after the cleanup daemon manages to spawn.
        cleanupDaemon.off('error', errorHandler);
      });

      // The cleanup daemon unexpectedly exiting, without the agent exiting as well.
      const onUnexpectedDaemonExit = () => {
        this.logger.error(
          `Cleanup daemon for ${rootClusterUri} terminated before agent. Terminating agent.`
        );
        terminateWithTimeout(agent);
      };
      cleanupDaemon.once('exit', onUnexpectedDaemonExit);

      // The agent exiting during normal operation.
      agent.once('exit', () => {
        // We're about to consciously terminate the cleanup daemon, so let's remove the unexpected
        // exit handler.
        cleanupDaemon.off('exit', onUnexpectedDaemonExit);

        terminateWithTimeout(cleanupDaemon);
      });
    });
  }

  private updateProcessState(
    rootClusterUri: RootClusterUri,
    state: AgentProcessState
  ): void {
    let loggedState = state;
    if (state.status === 'exited') {
      const { logs, ...rest } = state; // eslint-disable-line @typescript-eslint/no-unused-vars
      loggedState = rest;
    }
    this.logger.info(
      `Updating agent state ${rootClusterUri}: ${JSON.stringify(loggedState)}`
    );

    const agent = this.agentProcesses.get(rootClusterUri);
    agent.state = state;
    this.sendProcessState(rootClusterUri, state);
  }

  private areAllAgentsStopped(): boolean {
    return [...this.agentProcesses.values()].every(
      ({ state: { status } }) =>
        status === 'not-started' || status === 'exited' || status === 'error'
    );
  }
}

/**
 * processAgentOutput limits the output of the agent process to 10 lines and strips ANSI escape
 * codes.
 */
function processAgentOutput(output: string): string {
  // We specifically don't use strip-ansi-stream here because it chunks the output too heavily,
  // resulting in cut off logs at the point of process termination or join timeout.
  return stripAnsi(output).split(os.EOL).slice(-MAX_STDERR_LINES).join(os.EOL);
}
