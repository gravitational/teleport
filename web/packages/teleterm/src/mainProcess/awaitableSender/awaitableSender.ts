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

import Logger from 'teleterm/logger';

export type Message = MessageData | MessageAck;

export interface MessageData {
  id: string;
  type: 'data';
  payload: unknown;
}

export interface MessageAck {
  id: string;
  type: 'ack';
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
  private logger = new Logger('AwaitableSender');
  private messages = new Map<string, { resolve: () => void }>();
  private disposeSignal = Promise.withResolvers<void>();

  constructor(private port: MessagePortMain) {
    this.port.start();
    this.port.on('message', this.processMessage);
    this.port.on('close', this.dispose);
  }

  /** Sends a message and awaits delivery confirmation. */
  send(payload: T): Promise<void> {
    const id = crypto.randomUUID();

    return new Promise(resolve => {
      this.messages.set(id, { resolve });
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
    if (!isMessageAck(message)) {
      return;
    }
    const item = this.messages.get(message.id);
    if (item) {
      item.resolve();
      this.messages.delete(message.id);
    }
  };

  private dispose = (): void => {
    this.port.off('message', this.processMessage);
    this.port.off('close', this.dispose);

    if (this.messages.size) {
      this.logger.warn(
        `Sender was disposed before confirming delivery of ${this.messages.size} message(s).`
      );
    }
    for (const q of this.messages.values()) {
      q.resolve();
    }
    this.disposeSignal.resolve();
  };
}
