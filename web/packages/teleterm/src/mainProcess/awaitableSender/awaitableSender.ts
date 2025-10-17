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

import { MessageEvent, MessagePortMain } from 'electron';

export type Message = MessageData | MessageAck;

export interface MessageData {
  id: string;
  type: 'data';
  payload: unknown;
}

export interface MessageAck {
  id: string;
  type: 'ack';
  /**
   * Optional error.
   * Present when the renderer received the message, but failed to process it.
   */
  error?: unknown;
}

function isMessageAck(v: unknown): v is MessageAck {
  return typeof v === 'object' && 'type' in v && v.type === 'ack';
}

/**
 * Enables sending messages from the main process to the renderer
 * and awaiting delivery confirmation.
 *
 * Unlike the standard `webContents.send()` API, which is push-based,
 * `AwaitableSender` is pull-based — the renderer must explicitly subscribe
 * to receive messages.
 */
export class AwaitableSender<T> {
  private messages = new Map<
    string,
    { resolve(): void; reject(reason: unknown): void }
  >();
  private disposeSignal = Promise.withResolvers<void>();

  constructor(private port: MessagePortMain) {
    this.port.start();
    this.port.on('message', this.processMessage);
    this.port.on('close', this.dispose);
  }

  /**
   * Sends a message and awaits delivery confirmation from the receiver.
   *
   * This method returns a promise that resolves once the other side
   * acknowledges receiving and processing the message.
   * If the acknowledgment is not received within the specified timeout
   * (default 10 seconds), the promise rejects with a `MessageAcknowledgementError`.
   *
   * If the renderer received the message, but failed to process it, the promise
   * is also rejected.
   */
  send(
    payload: T,
    { signal = AbortSignal.timeout(10_000) }: { signal?: AbortSignal } = {}
  ): Promise<void> {
    const id = crypto.randomUUID();

    return new Promise((resolve, reject) => {
      const cleanup = () => {
        this.messages.delete(id);
        signal.removeEventListener('abort', abort);
      };

      const abort = () => {
        cleanup();
        reject(new MessageAcknowledgementError(signal.reason));
      };

      if (signal.aborted) {
        return abort();
      }

      signal.addEventListener('abort', abort, { once: true });

      this.messages.set(id, {
        resolve: () => {
          cleanup();
          resolve();
        },
        reject: reason => {
          cleanup();
          reject(reason);
        },
      });

      const message: MessageData = { type: 'data', id, payload };
      this.port.postMessage(message);
    });
  }

  /** Returns a promise that resolves when the sender is disposed. */
  whenDisposed(): Promise<void> {
    return this.disposeSignal.promise;
  }

  private processMessage = (event: MessageEvent): void => {
    const message = event.data;
    // Only to satisfy TypeScript.
    // We don't expect non-ack messages to be received on this port.
    if (!isMessageAck(message)) {
      return;
    }
    const item = this.messages.get(message.id);
    if (!item) {
      return;
    }
    if (message.error) {
      item.reject(message.error);
      return;
    }
    item.resolve();
  };

  private dispose = (): void => {
    this.port.off('message', this.processMessage);
    this.port.off('close', this.dispose);

    for (const { reject } of this.messages.values()) {
      reject(new MessageAcknowledgementError(new Error('Sender was disposed')));
    }
    this.disposeSignal.resolve();
  };
}

/** Error thrown when waiting for message acknowledgement confirmation was abandoned. */
export class MessageAcknowledgementError extends Error {
  constructor(cause?: unknown) {
    super('Failed to receive message acknowledgement from the renderer', {
      cause,
    });
  }
}
