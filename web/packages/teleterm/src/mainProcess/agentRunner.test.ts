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

import { makeRuntimeSettings } from './fixtures/mocks';

import { AgentRunner } from './agentRunner';
import { AgentProcessState } from './types';

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
    const agentProcess = await agentRunner.start(rootClusterUri);
    await new Promise(resolve => agentProcess.on('spawn', resolve));
    expect(updateSender).toHaveBeenCalledWith(rootClusterUri, {
      status: 'running',
    } as AgentProcessState);

    await agentRunner.kill(rootClusterUri);
    expect(updateSender).toHaveBeenCalledWith(rootClusterUri, {
      status: 'exited',
      code: null,
      signal: 'SIGTERM',
    } as AgentProcessState);

    expect(updateSender).toHaveBeenCalledTimes(2);
  } finally {
    await agentRunner.killAll();
  }
});

test('status updates are sent on a failed start', async () => {
  const updateSender = jest.fn();
  const nonExisingPath = path.join(__dirname, 'agentTestProcess-nonExisting.mjs');
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
    expect(updateSender).toHaveBeenCalledWith(rootClusterUri, {
      status: 'error',
      message: `Error: spawn ${nonExisingPath} ENOENT`,
    } as AgentProcessState);
  } finally {
    await agentRunner.killAll();
  }
});
