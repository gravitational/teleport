/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Logger } from 'design/logger';
import { TdpTransport } from 'shared/libs/tdp/client';

const logger = new Logger('WebSocketTdpTransport');

/**
 * Adapts WebSocket to TDP transport.
 * Returns a promise that fulfills with an object once the socket connection is successfully opened.
 * The promise rejects if the WebSocket connection fails during the setup.
 */
export async function adaptWebSocketToTdpTransport(
  socket: WebSocket,
  signal: AbortSignal
): Promise<TdpTransport> {
  if (signal.aborted) {
    throw new DOMException('Websocket was aborted.', 'AbortError');
  }
  // WebsocketCloseCode.NORMAL
  signal.addEventListener('abort', () => socket.close(1000));
  socket.binaryType = 'arraybuffer';

  try {
    await waitToOpen(socket);
  } catch (e) {
    logger.error('Could not open WebSocket', e);
    throw e;
  }
  logger.info('WebSocket is open');

  return {
    send: data => {
      // WebSocket only throws when we try to send a message while it is non-OPEN state.
      if (socket.readyState !== WebSocket.OPEN) {
        logger.info('WebSocket is not open, cannot send message.');
        return;
      }
      socket.send(data);
    },
    onMessage: callback => {
      const handler = (e: MessageEvent) => {
        callback(e.data);
      };
      socket.addEventListener('message', handler);
      return () => socket.removeEventListener('message', handler);
    },
    onComplete: callback => {
      const handler = (e: CloseEvent) => {
        if (e.wasClean) {
          callback();
        }
      };
      socket.addEventListener('close', handler);
      return () => socket.removeEventListener('close', handler);
    },
    onError: callback => {
      const handler = (e: CloseEvent) => {
        if (!e.wasClean) {
          logger.error(`Websocket closed with error (${e.code})`);
          callback(new Error(`Connection closed with websocket error`));
        }
      };
      // The socket will only ever emit the socket 'error' event
      // prior to a socket 'close' event (https://stackoverflow.com/a/40084550/6277051).
      // Therefore, we can rely on 'close' events to account for any websocket errors.
      socket.addEventListener('close', handler);
      return () => socket.removeEventListener('close', handler);
    },
  };
}

async function waitToOpen(socket: WebSocket): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    const handleOpen = () => {
      cleanup();
      resolve();
    };

    const handleError = (event: Event) => {
      cleanup();
      reject(
        new Error(
          `WebSocket error (type=${event.type}, readyState=${socket.readyState})`
        )
      );
    };

    function cleanup() {
      socket.removeEventListener('open', handleOpen);
      socket.removeEventListener('error', handleError);
    }

    socket.addEventListener('open', handleOpen);
    socket.addEventListener('error', handleError);
  });
}
