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

import childProcess, { ChildProcess } from 'node:child_process';
import { EventEmitter } from 'node:events';
import { PassThrough } from 'node:stream';

import Logger, { NullService } from 'teleterm/logger';
import { RootClusterUri } from 'teleterm/ui/uri';

import { makeRuntimeSettings } from './fixtures/mocks';

import { AgentRunner } from './agentRunner';
import { AgentProcessState } from './types';

jest.mock('node:child_process');

let eventEmitter: EventEmitter;
let childProcessMock: ChildProcess;
beforeEach(() => {
  Logger.init(new NullService());

  eventEmitter = new EventEmitter();
  childProcessMock = {
    stderr: new PassThrough(),
    once: (event, listener) => {
      eventEmitter.once(event, listener);
      return this;
    },
    on: (event, listener) => {
      eventEmitter.on(event, listener);
      return this;
    },
    off: (event, listener) => {
      eventEmitter.off(event, listener);
      return this;
    },
    kill: jest.fn().mockImplementation(() => {
      eventEmitter.emit('exit', 0);
    }),
  } as unknown as ChildProcess;

  jest.spyOn(childProcess, 'spawn').mockReturnValue(childProcessMock);
});

const userDataDir = '/Users/test/Application Data/Teleport Connect';
const agentBinaryPath = '/Users/test/Caches/Teleport Connect/teleport/teleport';
const rootClusterUri: RootClusterUri = '/clusters/cluster.local';

test('agent process starts with correct arguments', () => {
  const agentRunner = new AgentRunner(
    makeRuntimeSettings({
      agentBinaryPath,
      userDataDir,
    }),
    () => {}
  );
  agentRunner.start(rootClusterUri);

  expect(childProcess.spawn).toHaveBeenCalledWith(
    agentBinaryPath,
    ['start', `--config=${userDataDir}/agents/cluster.local/config.yaml`],
    expect.anything()
  );
});

test('previous agent process is killed when a new one is started', () => {
  const agentRunner = new AgentRunner(
    makeRuntimeSettings({
      agentBinaryPath,
      userDataDir,
    }),
    () => {}
  );
  agentRunner.start(rootClusterUri);
  agentRunner.start(rootClusterUri);

  expect(childProcessMock.kill).toHaveBeenCalledWith('SIGKILL');
});

test('status updates are sent', () => {
  const updateSender = jest.fn();
  const agentRunner = new AgentRunner(
    makeRuntimeSettings({
      agentBinaryPath,
      userDataDir,
    }),
    updateSender
  );

  agentRunner.start(rootClusterUri);
  expect(updateSender).toHaveBeenCalledWith(rootClusterUri, {
    status: 'running',
  } as AgentProcessState);

  const error = new Error('unknown error');
  eventEmitter.emit('error', error);
  expect(updateSender).toHaveBeenCalledWith(rootClusterUri, {
    status: 'error',
    message: `${error}`,
  } as AgentProcessState);

  agentRunner.kill(rootClusterUri);
  expect(updateSender).toHaveBeenCalledWith(rootClusterUri, {
    status: 'exited',
    code: 0,
  } as AgentProcessState);
});
