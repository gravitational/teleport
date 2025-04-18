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

import { render } from 'design/utils/testing';
import { makeSuccessAttempt } from 'shared/hooks/useAsync';
import { TdpClient } from 'shared/libs/tdp';
import { wait } from 'shared/utils/wait';

import { DesktopSession } from './DesktopSession';

import 'jest-canvas-mock';

import userEvent from '@testing-library/user-event';

import { TdpTransport } from 'shared/libs/tdp/client';

// Disable WASM in tests.
jest.mock('shared/libs/ironrdp/pkg/ironrdp');

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
    getTransport: async (abortSignal: AbortSignal): Promise<TdpTransport> => {
      abortSignal.onabort = async () => {
        await wait(50);
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

test('reconnect button reinitializes the connection', async () => {
  const transport = getMockTransport();
  const tpdClient = new TdpClient(transport.getTransport);
  jest.spyOn(tpdClient, 'connect');
  jest.spyOn(tpdClient, 'shutdown');
  const { unmount } = render(
    <DesktopSession
      client={tpdClient}
      username="admin"
      desktop="win-lab"
      aclAttempt={aclAttempt}
      hasAnotherSession={hasNoOtherSession}
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
