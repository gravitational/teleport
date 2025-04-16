/*
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

import WS from 'jest-websocket-mock';

import { StatusEnum } from 'teleport/lib/player';
import { TermEvent } from 'teleport/lib/term/enums';

import TtyPlayer from './ttyPlayer';

describe('lib/ttyPlayer', () => {
  let server;
  const url = 'ws://localhost:3088';
  beforeEach(() => {
    server = new WS(url);
  });
  afterEach(() => {
    WS.clean();

    jest.clearAllMocks();
  });

  it('connects to a websocket', async () => {
    const setPlayerStatus = jest.fn();
    const setStatusText = () => {};
    const setTime = () => {};

    const ttyPlayer = new TtyPlayer({
      url,
      setPlayerStatus,
      setStatusText,
      setTime,
    });
    const emit = jest.spyOn(ttyPlayer, 'emit');

    ttyPlayer.connect();
    await server.connected;

    expect(setPlayerStatus.mock.calls).toHaveLength(1);
    expect(setPlayerStatus.mock.calls[0][0]).toBe(StatusEnum.LOADING);

    expect(emit.mock.calls).toHaveLength(1);
    expect(emit.mock.calls[0][0]).toBe('open');

    server.close();
    await server.closed;

    expect(emit.mock.calls).toHaveLength(2);
    expect(emit.mock.calls[1][0]).toBe(TermEvent.CONN_CLOSE);
  });

  it('emits resize events', async () => {
    const setPlayerStatus = () => {};
    const setStatusText = () => {};
    const setTime = () => {};

    const ttyPlayer = new TtyPlayer({
      url,
      setPlayerStatus,
      setStatusText,
      setTime,
    });
    const emit = jest.spyOn(ttyPlayer, 'emit');

    ttyPlayer.connect();
    await server.connected;

    const resizeMessage = new Uint8Array([
      5, // message type = Resize
      0,
      4, // size
      0,
      80, // width
      0,
      60, // height
    ]);

    server.send(resizeMessage.buffer);

    expect(emit.mock.lastCall).toBeDefined();
    expect(emit.mock.lastCall[0]).toBe(TermEvent.RESIZE);
    expect(emit.mock.lastCall[1]).toStrictEqual({ w: 80, h: 60 });
  });

  it('plays PTY data', async () => {
    const setPlayerStatus = jest.fn();
    const setStatusText = jest.fn();
    const setTime = jest.fn();

    const ttyPlayer = new TtyPlayer({
      url,
      setPlayerStatus,
      setStatusText,
      setTime,
    });
    const emit = jest.spyOn(ttyPlayer, 'emit');

    ttyPlayer.connect();
    await server.connected;

    const data = new TextEncoder('utf-8').encode('~/test $');
    const len = data.length + 8;
    const ptyMessage = new Uint8Array([
      1 /* message type = PTY */,
      len >> 8,
      len & 0xff /* length */,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      123 /* timestamp (123ms) */,
      ...data,
    ]);

    server.send(ptyMessage.buffer);

    expect(emit.mock.lastCall).toBeDefined();
    expect(emit.mock.lastCall[0]).toBe(TermEvent.DATA);

    expect(emit.mock.lastCall[1]).toStrictEqual(Uint8Array.from(data).buffer);
    expect(setTime.mock.lastCall).toBeDefined();
    expect(setTime.mock.lastCall[0]).toBe(123);
  });
});
