/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import path from 'node:path';

import Logger, { NullService } from 'teleterm/logger';
import { RootClusterUri } from 'teleterm/ui/uri';

import { makeRuntimeSettings } from '../fixtures/mocks';
import { AgentProcessState } from '../types';

import { AgentRunner } from './agentRunner';

beforeEach(() => {
  Logger.init(new NullService());
});

const userDataDir = '/Users/test/Application Data/Teleport Connect';
const agentBinaryPath = path.join(__dirname, 'agentTestProcess.mjs');
const rootClusterUri: RootClusterUri = '/clusters/cluster.local';

test('agent process starts with correct arguments', async () => {
  const agentRunner = new AgentRunner(
    makeRuntimeSettings({
      agentBinaryPath,
      userDataDir,
    }),
    () => {}
  );

  try {
    const agentProcess = await agentRunner.start(rootClusterUri);

    expect(agentProcess.spawnargs).toEqual([
      agentBinaryPath,
      'start',
      `--config=${userDataDir}/agents/cluster.local/config.yaml`,
    ]);
  } finally {
    await agentRunner.killAll();
  }
});

test('previous agent process is killed when a new one is started', async () => {
  const agentRunner = new AgentRunner(
    makeRuntimeSettings({
      agentBinaryPath,
      userDataDir,
    }),
    () => {}
  );

  try {
    const firstProcess = await agentRunner.start(rootClusterUri);
    await agentRunner.start(rootClusterUri);

    expect(firstProcess.killed).toBeTruthy();
  } finally {
    await agentRunner.killAll();
  }
});

test('status updates are sent on a successful start', async () => {
  const updateSender = jest.fn();
  const agentRunner = new AgentRunner(
    makeRuntimeSettings({
      agentBinaryPath,
      userDataDir,
    }),
    updateSender
  );

  try {
    expect(agentRunner.getState(rootClusterUri)).toBeUndefined();
    const agentProcess = await agentRunner.start(rootClusterUri);
    expect(agentRunner.getState(rootClusterUri)).toStrictEqual({
      status: 'not-started',
    } as AgentProcessState);
    await new Promise((resolve, reject) => {
      const timeout = setTimeout(
        () => reject('Process start timed out.'),
        4_000
      );
      agentProcess.once('spawn', () => {
        resolve(undefined);
        clearTimeout(timeout);
      });
    });
    const runningState: AgentProcessState = { status: 'running' };
    expect(agentRunner.getState(rootClusterUri)).toStrictEqual(runningState);
    expect(updateSender).toHaveBeenCalledWith(rootClusterUri, runningState);

    await agentRunner.kill(rootClusterUri);
    const exitedState: AgentProcessState = {
      status: 'exited',
      code: null,
      stackTrace: undefined,
      exitedSuccessfully: true,
      signal: 'SIGTERM',
    };
    expect(agentRunner.getState(rootClusterUri)).toStrictEqual(exitedState);
    expect(updateSender).toHaveBeenCalledWith(rootClusterUri, exitedState);

    expect(updateSender).toHaveBeenCalledTimes(2);
  } finally {
    await agentRunner.killAll();
  }
});

test('status updates are sent on a failed start', async () => {
  const updateSender = jest.fn();
  const nonExisingPath = path.join(
    __dirname,
    'agentTestProcess-nonExisting.mjs'
  );
  const agentRunner = new AgentRunner(
    makeRuntimeSettings({
      agentBinaryPath: nonExisingPath,
      userDataDir,
    }),
    updateSender
  );

  try {
    const agentProcess = await agentRunner.start(rootClusterUri);
    await new Promise(resolve => agentProcess.on('error', resolve));

    expect(updateSender).toHaveBeenCalledTimes(1);
    const errorState: AgentProcessState = {
      status: 'error',
      message: expect.stringContaining('ENOENT'),
    };
    expect(agentRunner.getState(rootClusterUri)).toStrictEqual(errorState);
    expect(updateSender).toHaveBeenCalledWith(rootClusterUri, errorState);
  } finally {
    await agentRunner.killAll();
  }
});
