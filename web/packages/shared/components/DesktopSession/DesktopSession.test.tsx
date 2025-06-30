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

import { screen } from '@testing-library/react';
import { act } from 'react';

import { render } from 'design/utils/testing';
import { makeSuccessAttempt } from 'shared/hooks/useAsync';
import { BrowserFileSystem, MessageType, TdpClient } from 'shared/libs/tdp';

import { DesktopSession } from './DesktopSession';

import 'jest-canvas-mock';

import userEvent from '@testing-library/user-event';

import { TdpTransport } from 'shared/libs/tdp/client';

// Disable WASM in tests.
jest.mock('shared/libs/ironrdp/pkg/ironrdp');

// Matches codec.decodePngFrame.
function encodePngFrame(): ArrayBuffer {
  const buffer = new ArrayBuffer(21);
  const view = new DataView(buffer);
  view.setUint8(0, MessageType.PNG_FRAME);
  view.setUint32(1, 0);
  view.setUint32(5, 0);
  view.setUint32(9, 0);
  view.setUint32(13, 0);
  view.setUint32(17, 0);
  return buffer;
}

const hasNoOtherSession = jest.fn().mockResolvedValue(false);
const aclAttempt = makeSuccessAttempt({
  clipboardSharingEnabled: true,
  directorySharingEnabled: true,
});
const getMockTransport = () => {
  const emitter = new EventEmitter();
  return {
    emitTransportError: () =>
      emitter.emit('error', new Error('Could not send bytes')),
    emitPngFrameMessage: () => emitter.emit('message', encodePngFrame()),
    getTransport: async (abortSignal: AbortSignal): Promise<TdpTransport> => {
      abortSignal.onabort = () => {
        emitter.emit('complete');
      };
      return {
        send: () => {},
        onMessage: callback => {
          emitter.on('message', callback);
          return () => emitter.off('message', callback);
        },
        onComplete: callback => {
          emitter.on('complete', callback);
          return () => emitter.off('complete', callback);
        },
        onError: callback => {
          emitter.on('error', callback);
          return () => emitter.off('error', callback);
        },
      };
    },
  };
};

let originalQuery: typeof navigator.permissions.query;

beforeEach(() => {
  originalQuery = navigator.permissions.query;

  navigator.permissions.query = jest.fn().mockResolvedValue({
    state: 'granted',
    onchange: null,
  });
});

afterEach(() => {
  navigator.permissions.query = originalQuery;
});

test('reconnect button reinitializes the connection', async () => {
  const transport = getMockTransport();
  const tpdClient = new TdpClient(
    transport.getTransport,
    new BrowserFileSystem()
  );
  jest.spyOn(tpdClient, 'connect');
  jest.spyOn(tpdClient, 'shutdown');
  const { unmount } = render(
    <DesktopSession
      client={tpdClient}
      username="admin"
      desktop="win-lab"
      aclAttempt={aclAttempt}
      hasAnotherSession={hasNoOtherSession}
      browserSupportsSharing
    />
  );

  // The session is initializing.
  expect(await screen.findByTestId('indicator')).toBeInTheDocument();

  // An error occurred, the connection has been closed.
  transport.emitTransportError();

  expect(
    await screen.findByText('The desktop session is offline.')
  ).toBeInTheDocument();
  expect(await screen.findByText('Could not send bytes')).toBeInTheDocument();
  const reconnect = await screen.findByRole('button', { name: 'Reconnect' });

  await userEvent.click(reconnect);

  // The session is initializing again.
  expect(
    screen.queryByText('The desktop session is offline.')
  ).not.toBeInTheDocument();
  expect(await screen.findByTestId('indicator')).toBeInTheDocument();

  expect(hasNoOtherSession).toHaveBeenCalledTimes(2);
  expect(tpdClient.connect).toHaveBeenCalledTimes(2);
  unmount();
  // Called 2 times: the first one during reconnecting, the second one after unmounting.
  expect(tpdClient.shutdown).toHaveBeenCalledTimes(2);
});

test('ensure sharing remains enabled if the initial desktop connection attempt fails', async () => {
  const transport = getMockTransport();
  const tpdClient = new TdpClient(
    transport.getTransport,
    new BrowserFileSystem()
  );
  render(
    <DesktopSession
      client={tpdClient}
      username="admin"
      desktop="win-lab"
      aclAttempt={aclAttempt}
      hasAnotherSession={hasNoOtherSession}
      browserSupportsSharing
    />
  );

  // The session is initializing.
  expect(await screen.findByTestId('indicator')).toBeInTheDocument();

  // An error occurred, the connection has been closed.
  transport.emitTransportError();

  expect(
    await screen.findByText('The desktop session is offline.')
  ).toBeInTheDocument();
  const reconnect = await screen.findByRole('button', { name: 'Reconnect' });

  await userEvent.click(reconnect);
  // This time the connection succeeded.
  await act(() => transport.emitPngFrameMessage());

  expect(await screen.findByTitle('More actions')).toBeVisible();
  await userEvent.click(screen.getByTitle('More actions'));
  expect(await screen.findByText('Share Directory')).toBeVisible();
});
