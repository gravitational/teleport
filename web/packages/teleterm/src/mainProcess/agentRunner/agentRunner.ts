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

import { spawn, fork, ChildProcess } from 'node:child_process';
import os from 'node:os';

import stripAnsi from 'strip-ansi';

import Logger from 'teleterm/logger';
import { RootClusterUri } from 'teleterm/ui/uri';
import { createFileLoggerService, LoggerColor } from 'teleterm/services/logger';

import { generateAgentConfigPaths } from '../createAgentConfigFile';
import { AgentProcessState, RuntimeSettings } from '../types';
import { terminateWithTimeout } from '../terminateWithTimeout';

const MAX_STDERR_LINES = 10;
// https://github.com/gravitational/teleport/blob/1212306cff9be443286cdf0e8cbdf6471fc392d7/lib/service/signals.go#L59
const AGENT_GRACEFUL_SHUTDOWN_SIGNAL = 'SIGQUIT';

export class AgentRunner {
  private logger = new Logger('AgentRunner');
  private agentProcesses = new Map<
    RootClusterUri,
    {
      process: ChildProcess;
      state: AgentProcessState;
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
    });
    this.addAgentListeners(rootClusterUri, agentProcess);
    this.setupCleanupDaemon(rootClusterUri, agentProcess);

    return agentProcess;
  }

  getState(rootClusterUri: RootClusterUri): AgentProcessState | undefined {
    return this.agentProcesses.get(rootClusterUri)?.state;
  }

  async kill(rootClusterUri: RootClusterUri): Promise<void> {
    const agent = this.agentProcesses.get(rootClusterUri);
    if (!agent) {
      this.logger.warn(`Cannot get an agent to kill for ${rootClusterUri}`);
      return;
    }
    this.logger.info(`Killing agent for ${rootClusterUri}`);
    await terminateAgent(agent.process);
  }

  async killAll(): Promise<void> {
    const agents = Array.from(this.agentProcesses.values());
    await Promise.all(agents.map(agent => terminateAgent(agent.process)));
  }

  private addAgentListeners(
    rootClusterUri: RootClusterUri,
    process: ChildProcess
  ): void {
    // Teleport logs output to stderr.
    let stderrOutput = '';
    process.stderr.setEncoding('utf-8');
    process.stderr.on('data', (error: string) => {
      stderrOutput += error;
      stderrOutput = processAgentOutput(stderrOutput);
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
      const exitedSuccessfully =
        code === 0 || signal === AGENT_GRACEFUL_SHUTDOWN_SIGNAL;

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
      const cleanupDaemon = fork(this.agentCleanupDaemonPath, [
        // agent.pid can in theory be null if the agent gets terminated before the execution gets to
        // this point. In that case, the cleanup daemon is going to exit early.
        agent.pid?.toString(),
        process.pid.toString(),
        rootClusterUri,
        this.settings.logsDir,
      ]);

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
        terminateAgent(agent);
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
        terminateAgent(agent);
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

function terminateAgent(process: ChildProcess): Promise<void> {
  return terminateWithTimeout(process, undefined, process =>
    process.kill(AGENT_GRACEFUL_SHUTDOWN_SIGNAL)
  );
}
