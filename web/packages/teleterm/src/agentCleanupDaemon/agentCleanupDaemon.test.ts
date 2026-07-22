/**
 * @jest-environment node
 */
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

import childProcess from 'node:child_process';
import fs from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import process from 'node:process';

const stdio = 'pipe'; // Change to 'inherit' for easier debugging.

let logsDir: string;

beforeAll(async () => {
  logsDir = await fs.mkdtemp(
    path.join(tmpdir(), 'agent-cleanup-daemon-test-logs')
  );
});

afterAll(async () => {
  await fs.rm(logsDir, { recursive: true, force: true });
});

describe('agentCleanupDaemon', () => {
  test.each([
    {
      name: 'terminates the agent if the parent gets terminated',
      parentArgs: [],
    },
    {
      name: 'terminates the agent if the parent gets terminated before the cleanup daemon is fully set up',
      parentArgs: ['sendPidsImmediately'],
    },
    {
      name: 'follows up SIGTERM with SIGKILL in case SIGTERM did not cause the agent to terminate',
      parentArgs: ['sendPidsWhenReady', 'ignoreSigterm'],
    },
  ])('$name', async ({ parentArgs }) => {
    await cleanupPids(async addPidToCleanup => {
      const parent = childProcess.fork(
        path.join(__dirname, 'parentTestProcess.mjs'),
        [logsDir, ...parentArgs],
        { stdio }
      );
      addPidToCleanup(parent.pid);

      // parentTestProcess sends PIDs only after it gets a message from both childTestProcess and
      // agentCleanupDaemon. This way we know that both children are actually up and running.
      //
      // Otherwise we might end up killing the parent before the agent cleanup daemon was set up.
      //
      // If sendPidsImmediately is passed as the first arg to the parent process, the PIDs are sent
      // immediately after spawning the children, without waiting for messages.
      const pidsPromise = waitForMessage(parent);
      await expect(pidsPromise).resolves.toMatchObject({
        agentCleanupDaemon: expect.any(Number),
        agent: expect.any(Number),
      });
      const pids = await pidsPromise;
      addPidToCleanup(pids['agent']);
      addPidToCleanup(pids['agentCleanupDaemon']);

      // Make sure that both children are still running.
      expect(isRunning(pids['agent'])).toBe(true);
      expect(isRunning(pids['agentCleanupDaemon'])).toBe(true);

      // Verify that killing the parent results in the eventual termination of both children.
      //
      // Note that when the parent is killed, the child processes become orphans (https://en.wikipedia.org/wiki/Orphan_process),
      // and when orphans are killed, they become zombies (https://en.wikipedia.org/wiki/Zombie_process).
      // In typical UNIX environments, zombies exist only momentarily and are cleaned up by the init process (https://en.wikipedia.org/wiki/Init),
      // however in Docker there is no init process by default (https://blog.phusion.nl/2015/01/20/docker-and-the-pid-1-zombie-reaping-problem/),
      // so zombie processes end up sticking around until the container is stopped. Hence in order for this test to pass in a `docker run`,
      // we need to add the `--init` flag (https://docs.docker.com/engine/reference/run/#specify-an-init-process).
      expect(parent.kill('SIGKILL')).toBe(true);
      await expectPidToEventuallyTerminate(pids['agent']);
      await expectPidToEventuallyTerminate(pids['agentCleanupDaemon']);
    });
  });

  it('exits early if the agent is not running at the start', async () => {
    await cleanupPids(async addPidToCleanup => {
      const parent = childProcess.fork(
        path.join(__dirname, 'parentTestProcess.mjs'),
        [logsDir, 'sendPidsImmediately'],
        { stdio }
      );
      addPidToCleanup(parent.pid);

      const pidsPromise = waitForMessage(parent);
      await expect(pidsPromise).resolves.toMatchObject({
        agentCleanupDaemon: expect.any(Number),
        agent: expect.any(Number),
      });
      const pids = await pidsPromise;
      addPidToCleanup(pids['agent']);
      addPidToCleanup(pids['agentCleanupDaemon']);

      // Make sure that both children are still running.
      expect(isRunning(pids['agent'])).toBe(true);
      expect(isRunning(pids['agentCleanupDaemon'])).toBe(true);

      // Kill the agent before the daemon is set up.
      expect(process.kill(pids['agent'], 'SIGKILL')).toBe(true);

      await expectPidToEventuallyTerminate(pids['agentCleanupDaemon']);
    });
  });

  it('exits on SIGTERM and keeps the agent running', async () => {
    await cleanupPids(async addPidToCleanup => {
      const parent = childProcess.fork(
        path.join(__dirname, 'parentTestProcess.mjs'),
        [logsDir],
        { stdio }
      );
      addPidToCleanup(parent.pid);

      const pidsPromise = waitForMessage(parent);
      await expect(pidsPromise).resolves.toMatchObject({
        agentCleanupDaemon: expect.any(Number),
        agent: expect.any(Number),
      });
      const pids = await pidsPromise;
      addPidToCleanup(pids['agent']);
      addPidToCleanup(pids['agentCleanupDaemon']);

      // Make sure that both children are still running.
      expect(isRunning(pids['agent'])).toBe(true);
      expect(isRunning(pids['agentCleanupDaemon'])).toBe(true);

      // Verify that SIGTERM makes the cleanup daemon terminate.
      expect(process.kill(pids['agentCleanupDaemon'], 'SIGTERM')).toBe(true);
      await expectPidToEventuallyTerminate(pids['agentCleanupDaemon']);

      // Verify that the cleanup daemon doesn't kill the agent when the cleanup daemon receives
      // SIGTERM.
      expect(isRunning(pids['agent'])).toBe(true);
    });
  });
});

describe('isRunning', () => {
  it('reports the status of a process', async () => {
    await cleanupPids(async addPidToCleanup => {
      const child = childProcess.fork(
        path.join(__dirname, 'agentTestProcess.mjs')
      );
      addPidToCleanup(child.pid);

      expect(isRunning(child.pid)).toBe(true);

      child.kill('SIGKILL');
      await expectPidToEventuallyTerminate(child.pid);

      expect(isRunning(child.pid)).toBe(false);
    });
  });
});

const waitForMessage = (process: childProcess.ChildProcess) =>
  new Promise(resolve => {
    process.once('message', resolve);
  });

const expectPidToEventuallyTerminate = async (pid: number) =>
  expect(() => !isRunning(pid)).toEventuallyBeTrue({
    waitFor: 2000,
    tick: 10,
  });

/**
 * isRunning determines whether a process with the given PID is running by sending a special zero
 * signal, as described in process.kill docs.
 *
 * https://nodejs.org/docs/latest-v18.x/api/process.html#processkillpid-signal
 */
const isRunning = (pid: number) => {
  try {
    return process.kill(pid, 0);
  } catch (error) {
    if (error.code === 'ESRCH') {
      return false;
    }

    throw error;
  }
};

const cleanupPids = async (
  func: (addPidToCleanup: (pid: number) => void) => void | Promise<void>
): Promise<void> => {
  const pidsToCleanup = [];
  const addPidToCleanup = (pid: number) => {
    pidsToCleanup.push(pid);
  };

  try {
    await func(addPidToCleanup);
  } finally {
    for (const pid of pidsToCleanup) {
      try {
        process.kill(pid, 'SIGKILL');
      } catch {
        // Ignore errors resulting from the process not existing.
      }
    }
  }
};
