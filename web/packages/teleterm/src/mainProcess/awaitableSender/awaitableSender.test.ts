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

import { EventEmitter } from 'events';

import Logger, { NullService } from 'teleterm/logger';

import {
  AwaitableSender,
  MessageAcknowledgementError,
} from './awaitableSender';

beforeAll(() => {
  Logger.init(new NullService());
});

test('starts port and add event listeners in constructor', () => {
  const port = new MockMessagePortMain();
  new AwaitableSender(port);
  expect(port.listeners('message')).toHaveLength(1);
  expect(port.listeners('close')).toHaveLength(1);
});

test('send posts message and returns a promise that resolves on ack', async () => {
  const port = new MockMessagePortMain();
  const sender = new AwaitableSender(port);

  const payload = { foo: 'bar' };
  const promise = sender.send(payload);

  expect(port.postMessage).toHaveBeenCalledWith({
    type: 'data',
    id: expect.any(String),
    payload,
  });

  // The other side responds: first emit ack for other id and then an empty message.
  port.emitMessage({ id: 'wrong-id', type: 'ack' });
  port.emitMessage(undefined);
  // Wait a short time and make sure the promise hasn't resolved.
  const result = await Promise.race([
    promise.then(() => 'resolved'),
    new Promise(resolve => setTimeout(() => resolve('pending'), 50)),
  ]);

  expect(result).toBe('pending');

  // The other side responds again: now emit ack for our send request.
  const correctId = port.postMessage.mock.calls[0][0].id;
  port.emitMessage({ id: correctId, type: 'ack' });

  await expect(promise).resolves.toBeUndefined();
});

test('dispose removes listeners, resolves pending messages, clears map, and resolves disposeSignal', async () => {
  const port = new MockMessagePortMain();
  const sender = new AwaitableSender(port);
  let sendPromise = sender.send(undefined);

  port.close();
  expect(port.listeners('message')).toHaveLength(0);
  expect(port.listeners('close')).toHaveLength(0);

  const disposedPromise = sender.whenDisposed();

  // The pending send promise should resolve after dispose
  await expect(sendPromise).rejects.toThrow(MessageAcknowledgementError);
  await expect(disposedPromise).resolves.toBeUndefined();
});

class MockMessagePortMain extends EventEmitter {
  public postMessage = jest.fn();
  public start = jest.fn();

  constructor() {
    super();
  }

  emitMessage(data: unknown): void {
    this.emit('message', { data });
  }

  close(): void {
    this.emit('close');
  }
}
