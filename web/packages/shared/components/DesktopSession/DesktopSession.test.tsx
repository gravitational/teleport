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

import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { act } from 'react';

import { render } from 'design/utils/testing';
import { Envelope } from 'gen-proto-ts/teleport/desktop/v1/tdpb_pb';
import { makeSuccessAttempt } from 'shared/hooks/useAsync';
import {
  MessageType,
  selectDirectoryInBrowser,
  SharedDirectoryAccess,
  TdpClient,
} from 'shared/libs/tdp';
import { TdpTransport } from 'shared/libs/tdp/client';

import { DesktopSessionWithSharing } from './DesktopSessionWithSharing';

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

// Matches codec.decodePngFrame.
function encodePngFrameTDPB(): ArrayBuffer {
  const msg = Envelope.toBinary(
    Envelope.create({
      payload: {
        oneofKind: 'pngFrame',
        pngFrame: {
          coordinates: {
            left: 0,
            top: 0,
            right: 0,
            bottom: 0,
          },
          data: new Uint8Array(),
        },
      },
    })
  );
  const arraybuf = new Uint8Array(msg.length + 4);
  const view = new DataView(arraybuf.buffer);
  view.setUint32(0, msg.length);
  arraybuf.set(msg, 4);
  return arraybuf.buffer;
}

function encodeServerHello(canRemoveDirectory: boolean): ArrayBuffer {
  const msg = Envelope.toBinary(
    Envelope.create({
      payload: {
        oneofKind: 'serverHello',
        serverHello: {
          activationSpec: {
            ioChannelId: 1,
            userChannelId: 2,
            screenHeight: 100,
            screenWidth: 100,
          },
          clipboardEnabled: true,
          directoryRemoveSupported: canRemoveDirectory,
          hidpiSupported: false,
          sessions: [],
        },
      },
    })
  );
  const arraybuf = new Uint8Array(msg.length + 4);
  const view = new DataView(arraybuf.buffer);
  view.setUint32(0, msg.length);
  arraybuf.set(msg, 4);
  return arraybuf.buffer;
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
    emitPngFrameTDPBMessage: () =>
      emitter.emit('message', encodePngFrameTDPB()),
    emitServerCapabilities: (opts: { canRemove: boolean }) =>
      emitter.emit('message', encodeServerHello(opts.canRemove)),
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
    selectDirectoryInBrowser
  );
  jest.spyOn(tpdClient, 'connect');
  jest.spyOn(tpdClient, 'shutdown');
  const { unmount } = render(
    <DesktopSessionWithSharing
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

test('directory sharing menu', async () => {
  const transport = getMockTransport();
  const dirName = 'some directory';
  const tpdClient = new TdpClient(
    transport.getTransport,
    async () => mockDirectoryAccess(dirName),
    { mode: 'tdpb' }
  );
  render(
    <DesktopSessionWithSharing
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

  // Successfully initialize the connection.
  await act(() => transport.emitPngFrameTDPBMessage());
  await act(() => transport.emitServerCapabilities({ canRemove: true }));

  // The menu icon should also be visible
  const shareButton = await screen.findByRole('button', {
    name: 'Share local directories with the desktop',
  });
  expect(shareButton).toBeVisible();

  // Clicking the new menu icon should open a menu
  await userEvent.click(shareButton);

  // Share a couple directories via this menu
  let shareMenu = await screen.findByTestId('shared-directory-menu');
  await userEvent.click(
    within(shareMenu).getByRole('button', { name: 'Share a directory' })
  );
  await userEvent.click(
    within(shareMenu).getByRole('button', { name: 'Share a directory' })
  );

  // retrieve the directories
  const directories = await getSharedDirectoryEntries(shareMenu);
  expect(directories).toHaveLength(2);
  expect(directories[0].name).toEqual(dirName);
  expect(directories[1].name).toEqual(dirName);

  // Clicking the eject button unshares the directory and removes
  // it from the menu.
  expect(directories[0].ejectButton).toBeEnabled();
  await userEvent.click(directories[0].ejectButton);

  // Only one should remain
  const updatedDirectories = await getSharedDirectoryEntries(shareMenu);
  expect(updatedDirectories).toHaveLength(1);

  // Server reports that it cannot remove directories
  // unshare/eject button should be disabled.
  await act(() => transport.emitServerCapabilities({ canRemove: false }));
  expect(updatedDirectories[0].ejectButton).toBeDisabled();
});

async function getSharedDirectoryEntries(menu: HTMLElement) {
  const list = within(menu).getByRole('list', { name: 'Shared directories' });
  const entries = await within(list).findAllByRole('listitem');

  return entries.map(elem => {
    return {
      name: elem.textContent,
      ejectButton: within(elem).getByRole('button', {
        name: /disconnect shared directory/i,
      }),
    };
  });
}

test('ensure sharing remains enabled if the initial desktop connection attempt fails', async () => {
  const transport = getMockTransport();
  const tpdClient = new TdpClient(
    transport.getTransport,
    selectDirectoryInBrowser
  );
  render(
    <DesktopSessionWithSharing
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

test('re-sharing directory is possible after a reconnect', async () => {
  const transport = getMockTransport();

  const mockFsSpy = jest.fn(async () => mockDirectoryAccess());
  const tpdClient = new TdpClient(transport.getTransport, mockFsSpy);
  render(
    <DesktopSessionWithSharing
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

  // Successfully initialize the connection.
  await act(() => transport.emitPngFrameMessage());

  // Share a directory.
  await testSharingDirectory();

  // An error occurred, the connection has been closed.
  transport.emitTransportError();
  expect(
    await screen.findByText('The desktop session is offline.')
  ).toBeInTheDocument();

  // Reconnect.
  const reconnect = await screen.findByRole('button', { name: 'Reconnect' });
  await userEvent.click(reconnect);
  await act(() => transport.emitPngFrameMessage());

  // Share the directory again.
  await testSharingDirectory();
  expect(mockFsSpy).toHaveBeenCalledTimes(2);
});

async function testSharingDirectory() {
  expect(await screen.findByTitle('More actions')).toBeVisible();
  await userEvent.click(screen.getByTitle('More actions'));
  await userEvent.click(await screen.findByText('Share Directory'));
}

function mockDirectoryAccess(name: string = ''): SharedDirectoryAccess {
  return {
    getDirectoryName: () => name,
    create: () => undefined,
    read: () => undefined,
    stat: () => undefined,
    delete: () => undefined,
    readDir: () => undefined,
    truncate: () => undefined,
    write: () => undefined,
  };
}
