/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { Envelope } from 'gen-proto-ts/teleport/desktop/v1/tdpb_pb';

import { TdpClient, TdpTransport } from './client';
import { SharedDirectoryAccess } from './sharedDirectoryAccess';

let mockTransport: jest.Mocked<TdpTransport> = {
  send: jest.fn(),
  onMessage: jest.fn(),
  onError: jest.fn(),
  onComplete: jest.fn(),
};

const mockSharedDirectoryAccess: SharedDirectoryAccess = {
  getDirectoryName: jest.fn(),
  stat: jest.fn(),
  readDir: jest.fn(),
  read: jest.fn(),
  write: jest.fn(),
  truncate: jest.fn(),
  create: jest.fn(),
  delete: jest.fn(),
};

// Disable WASM in tests.
jest.mock('shared/libs/ironrdp/pkg/ironrdp');

test('tdp upgrade', async () => {
  let client = new TdpClient(
    () => Promise.resolve(mockTransport),
    () => Promise.resolve(mockSharedDirectoryAccess)
  );

  const transportOpen = new Promise<void>(client.onTransportOpen);

  client.connect({
    screenSpec: { width: 1920, height: 1080, scale: 100 },
    keyboardLayout: 4,
  });

  await transportOpen;

  expect(mockTransport.send).toHaveBeenCalledTimes(2);

  let onMessage = mockTransport.onMessage.mock.calls[0][0] as (
    data: ArrayBufferLike
  ) => void;
  // Hand jam a tdpb upgrade message
  let msg = new Uint8Array(1);
  msg[0] = 38; // Upgrade message type

  // Send the upgrade!
  onMessage(msg.buffer);

  // Expect a client hello
  expect(mockTransport.send).toHaveBeenCalledTimes(3);
  const data = mockTransport.send.mock.calls[2][0] as ArrayBufferLike;
  const buf = new Uint8Array(data);

  // Client hello should contain the same screen spec and keyboard layout
  // that was provided durring 'connect()'
  const envelope = Envelope.fromBinary(buf.slice(4));

  // Should have received a ClientHello
  if (envelope.payload.oneofKind !== 'clientHello') {
    throw Error(
      `Expected kind="clientHello", got ${envelope.payload.oneofKind}`
    );
  }

  const hello = envelope.payload.clientHello;
  expect(hello.screenSpec).toEqual({ width: 1920, height: 1080, scale: 100 });
  expect(hello.keyboardLayout).toEqual(4);
});

test('shared directory management', async () => {
  let client = new TdpClient(
    () => Promise.resolve(mockTransport),
    () => {
      return Promise.resolve(mockSharedDirectoryAccess);
    }
  );

  let manager = client['directoryManager'];
  await Array.from({ length: 10 }, () => manager.shareDirectory());
  // Identifiers begin at 2
  const directories = manager.listSharedDirectories();
  expect(directories).toHaveLength(10);

  // Reached maximum number of shared directories
  await expect(manager.shareDirectory()).rejects.toThrow();
  // unshare all
  manager.reset();

  let shareUnshare = async () => {
    const res = await manager.shareDirectory();
    manager.unshareDirectory(res[0]);
  };

  // 11 more
  await Promise.all(Array.from({ length: 11 }, () => shareUnshare()));

  // The range of valid device identifiers is currently [2, 22]
  // Initial and released identifiers are added to a FIFO, so the
  // next leased identifier should be be 2.
  const res = await manager.shareDirectory();
  expect(res[0]).toEqual(2);

  const warn = jest.spyOn(console, 'warn').mockImplementation();
  // releasing an unknown or unleased identifier does not throw, but
  // should log a warning.
  manager.unshareDirectory(3);
  expect(warn).toHaveBeenCalled();
});
