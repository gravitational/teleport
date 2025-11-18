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
import {TdpClient, TdpTransport, } from './client';
import {SharedDirectoryAccess} from './sharedDirectoryAccess';
import {ClientHello} from 'gen-proto-ts/teleport/desktop/tdp_pb'


let mockTransport: jest.Mocked<TdpTransport> = {
  send: jest.fn(),
  onMessage: jest.fn(),
  onError: jest.fn(),
  onComplete: jest.fn(),
}

const mockSharedDirectoryAccess: SharedDirectoryAccess = {
  getDirectoryName: jest.fn(),
  stat: jest.fn(),
  readDir: jest.fn(),
  read: jest.fn(),
  write: jest.fn(),
  truncate: jest.fn(),
  create: jest.fn(),
  delete: jest.fn(),
}

// Disable WASM in tests.
jest.mock('shared/libs/ironrdp/pkg/ironrdp');

test('tdp upgrade', async () => {
  // Create a callback the resoves a promise
  let cb: () => void;
  let pm = new Promise<void>((resolve) => {
    cb = () => {
      resolve();
      return;
    }
  })

  let client = new TdpClient(
    (_signal: AbortSignal) => Promise.resolve(mockTransport),
    () => Promise.resolve(mockSharedDirectoryAccess),
    cb, // Called once transport is set and initial TDP messages sent
  );


  client.connect({screenSpec: { width: 1920, height: 1080}, keyboardLayout: 1})
  await pm; // Transport has been set on client and initial messages were sent
  expect(mockTransport.send).toHaveBeenCalledTimes(2);

  let onMessage = mockTransport.onMessage.mock.calls[0][0] as (data: ArrayBufferLike) => void;
  // Hand jam a tdpb upgrade message
  let msg = new Uint8Array(5)
  let dv = new DataView(msg.buffer);
  msg[0]  = 38 // Upgrade message type
  dv.setUint32(1, 1)
  // Send the upgrade!
  onMessage(msg.buffer)

  // Expect a client hello
  expect(mockTransport.send).toHaveBeenCalledTimes(3);
  let data = mockTransport.send.mock.calls[2][0] as ArrayBufferLike;
  let buf = new Uint8Array(data);

  // Client hello should contain the same screen spec and keyboard layout
  // that was provided durring 'connect()'
  let hello = ClientHello.fromBinary(buf.slice(8))
  expect(hello.screenSpec).toEqual({ width: 1920, height: 1080})
  expect(hello.keyboardLayout).toEqual({keyboardLayout: 1})
})
