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

import { ChildProcess } from 'child_process';

const TCP_PORT_MATCH = /\{CONNECT_GRPC_PORT:\s(\d+)}/;
// Both the shared process and the tshd output the string no matter if TCP or UDS is used as a
// transport method.
const UDS_MATCH = /\{CONNECT_GRPC_PORT:/;

/**
 * Waits for the process to start a gRPC server and log the address used by the gRPC server.
 *
 * @return {Promise<string>} The address used by the gRPC server started from the process.
 */
export async function resolveNetworkAddress(
  requestedAddress: string,
  process: ChildProcess,
  timeoutMs = 15_000 // 15s; needs to be larger than other timeouts in the processes.
): Promise<string> {
  const protocol = new URL(requestedAddress).protocol;

  switch (protocol) {
    case 'unix:': {
      // In case of UDS, we know the address upfront. Still, we wait for the process to emit the
      // message so that we know the server was started and accepts connections.
      await waitForMatchInStdout(
        UDS_MATCH,
        requestedAddress,
        process,
        timeoutMs
      );
      return requestedAddress;
    }
    case 'tcp:': {
      const matchResult = await waitForMatchInStdout(
        TCP_PORT_MATCH,
        requestedAddress,
        process,
        timeoutMs
      );
      return `localhost:${matchResult[1]}`;
    }
    default: {
      throw new Error(`Unknown protocol ${protocol}`);
    }
  }
}

function waitForMatchInStdout(
  regex: RegExp,
  requestedAddress: string,
  process: ChildProcess,
  timeoutMs: number
): Promise<RegExpMatchArray> {
  return new Promise((resolve, reject) => {
    process.stdout.setEncoding('utf-8');
    let chunks = '';

    const timeout = setTimeout(() => {
      rejectOnError(
        new ResolveError(requestedAddress, process, 'the operation timed out')
      );
    }, timeoutMs);

    const removeListeners = () => {
      process.stdout.off('data', findAddressInChunk);
      process.off('error', rejectOnError);
      process.off('close', rejectOnClose);
      clearTimeout(timeout);
    };

    const findAddressInChunk = (chunk: string) => {
      chunks += chunk;
      const matchResult = chunks.match(regex);
      if (matchResult) {
        resolve(matchResult);
        removeListeners();
      }
    };

    const rejectOnError = (error: Error) => {
      reject(error);
      removeListeners();
    };

    const rejectOnClose = (code: number, signal: NodeJS.Signals) => {
      const codeOrSignal = [
        // code can be 0, so we cannot just check it the same way as the signal.
        code != null && `code ${code}`,
        signal && `signal ${signal}`,
      ]
        .filter(Boolean)
        .join(' ');
      const details = codeOrSignal ? ` with ${codeOrSignal}` : '';
      rejectOnError(
        new ResolveError(
          requestedAddress,
          process,
          `the process exited${details}`
        )
      );
    };

    process.stdout.on('data', findAddressInChunk);
    process.on('error', rejectOnError);
    // Listen for close instead of exit. This doesn't make much difference in prod usage, but it's
    // meaningful in tests. testProcess.mjs exits soon after printing to stdout, so we have to make
    // sure that stdio streams are closed to avoid process.on('exit') being processed before
    // stdout.on('data').
    process.on('close', rejectOnClose);
  });
}

export class ResolveError extends Error {
  constructor(requestedAddress: string, process: ChildProcess, reason: string) {
    super(
      `Could not resolve address (${requestedAddress}) for process ${process.spawnfile}: ${reason}.`
    );
  }
}
