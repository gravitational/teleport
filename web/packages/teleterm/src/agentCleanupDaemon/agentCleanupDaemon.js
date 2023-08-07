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

// agentCleanupDaemon.mjs is a JavaScript file and not a TypeScript file on purpose.
//
// If it was written in TypeScript, we'd have to run it through something like ts-node in tests.
// Unfortunately, tools like ts-node mess up with the actual processes being executed [1] and in
// general modify the behavior of a regular Node.js process [2].
//
// To keep things in tests as close to production as possible, we opted to use a regular JS module
// that can be simply forked.
//
// What's more, it cannot be an ESM module (.mjs). Electron doesn't support them well yet [3],
// particularly when ESM modules are forked from asar.
//
// [1] https://github.com/TypeStrong/ts-node/tree/47d4f45f35e824a2515e17383a563be7dba7d8ff#native-ecmascript-modules
// [2] https://github.com/TypeStrong/ts-node/tree/47d4f45f35e824a2515e17383a563be7dba7d8ff#how-it-works
// [3] https://github.com/electron/electron/issues/21457

const { setTimeout } = require('node:timers/promises');

const agentPid = parseInt(process.argv[2], 10);
// Pass ppid over argv rather than reading process.ppid, as ppid can change when the cleanup deamon
// gets orphaned.
const parentPid = parseInt(process.argv[3], 10);
// For debugging purposes only.
const rootClusterUri = process.argv[4];
const timeToSigkill = parseInt(process.argv[5], 10) || 5_000;

if (!agentPid) {
  throw new Error('Agent PID must be passed over argv as the first argument');
}
if (!parentPid) {
  throw new Error('Parent PID must be passed over argv as the second argument');
}
if (!rootClusterUri) {
  throw new Error(
    'Root cluster URI must be passed over argv as the third argument'
  );
}
if (!process.send) {
  // https://nodejs.org/docs/latest-v18.x/api/child_process.html#optionsstdio
  // https://nodejs.org/docs/latest-v18.x/api/process.html#processsendmessage-sendhandle-options-callback
  throw new Error(
    'Agent cleanup daemon must be spawned from a Node.js parent that establishes an IPC channel'
  );
}

const logLine = `parent=${parentPid} agent=${agentPid} self=${process.pid} cluster=${rootClusterUri}`;

// disconnect will be emitted when the IPC channel between the cleanup daemon and the parent gets
// closed. Since we don't explicitly close the channel at any point, this means that the parent got
// unexpectedly terminated.
process.on('disconnect', async () => {
  log(`Disconnected from the parent.`);
  await terminateAgent();
});

// Signal readiness to the parent by sending an empty message.
// In some cases, such as in tests, we need to know that the cleanup daemon is ready and operating
// before we terminate the parent.
process.send(null, undefined, undefined, () => {
  // Ignore errors. In a situation where the parent got killed before the execution got to this
  // point, process.send will fail with ERR_IPC_CHANNEL_CLOSED.
  //
  // We handle the IPC channel being closed below with process.connected.
});

log(`Spawned and ready.`);

postLaunchChecks();

async function postLaunchChecks() {
  // Terminate the agent if the parent got terminated before the cleanup daemon was able to fully
  // set up.
  //
  // In that scenario, the 'disconnect' event will never be fired and the event loop will no longer
  // have any work to perform.
  if (!process.connected) {
    logError(`The parent got terminated during setup.`);
    await terminateAgent();
    process.exitCode = 41;
    return;
  }

  if (!isRunning(agentPid)) {
    logError(`The agent got terminated during setup, exiting.`);
    process.removeAllListeners('disconnect');
    process.exitCode = 42;
    return;
  }
}

async function terminateAgent() {
  try {
    log(`Sending SIGTERM to the agent.`);
    // SIGTERM should cause a fast shutdown of the agent.
    process.kill(agentPid, 'SIGTERM');

    await setTimeout(timeToSigkill);
    if (!isRunning(agentPid)) {
      log('The agent was gracefully terminated with SIGTERM.');
      return;
    }

    // Follow up with SIGKILL in case the agent is still running after receiving SIGTERM.
    log(`Sending SIGKILL to the agent.`);
    process.kill(agentPid, 'SIGKILL');
  } catch (error) {
    if (error.code === 'ESRCH') {
      logError(`No agent process found.`);
      return;
    }
    throw error;
  }
}

/**
 * isRunning determines whether a process with the given PID is running by sending a special zero
 * signal, as described in process.kill docs.
 *
 * https://nodejs.org/docs/latest-v18.x/api/process.html#processkillpid-signal
 */
function isRunning(pid) {
  try {
    return process.kill(pid, 0);
  } catch (error) {
    if (error.code === 'ESRCH') {
      return false;
    }

    throw error;
  }
}

function log(...args) {
  console.log(...args, logLine);
}

function logError(...args) {
  console.error(...args, logLine);
}
