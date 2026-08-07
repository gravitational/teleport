/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { TdpCodec } from 'shared/libs/tdp';

import { StatusEnum } from 'teleport/lib/player';

import { PlayerClient, type PlayerTimeAnchor } from './playerClient';

const encoder = new TextEncoder();
const codec = new TdpCodec();

function makeClient() {
  const client = new PlayerClient({ url: 'wss://localhost/playback' });
  // connect() is what normally initializes the codec and transport
  (client as unknown as { codec: TdpCodec }).codec = new TdpCodec();
  const send = jest.fn();
  jest
    .spyOn(client as unknown as { send: (data: string) => void }, 'send')
    .mockImplementation(send);

  const anchors: PlayerTimeAnchor[] = [];
  const statuses: StatusEnum[] = [];
  client.onTimeUpdate(anchor => anchors.push(anchor));
  client.onPlayerStatus(status => statuses.push(status));

  return { client, anchors, statuses, send };
}

function toBase64(buffer: ArrayBufferLike) {
  return btoa(String.fromCharCode(...new Uint8Array(buffer)));
}

// recordingMessage builds a playback message as sent by the server: a JSON wrapper
// with the playback timestamp and a base64-encoded TDP message (a real client screen
// spec, the simplest message that round-trips without a transport).
function recordingMessage(ms: number) {
  const screenSpec = codec.encodeClientScreenSpec({
    width: 1920,
    height: 1080,
    scale: 1,
  });

  return encoder.encode(JSON.stringify({ ms, message: toBase64(screenSpec) }))
    .buffer as ArrayBuffer;
}

function controlMessage(json: object) {
  return encoder.encode(JSON.stringify(json)).buffer as ArrayBuffer;
}

test('emits a time anchor for every server message', async () => {
  const { client, anchors } = makeClient();

  await client.processMessage(recordingMessage(1000));
  await client.processMessage(recordingMessage(2000));

  expect(anchors).toEqual([
    { ms: 1000, speed: 1, paused: false },
    { ms: 2000, speed: 1, paused: false },
  ]);
});

test('emits an optimistic anchor on seek and suppresses anchors until the replay catches up', async () => {
  const { client, anchors } = makeClient();

  await client.processMessage(recordingMessage(1000));

  client.seekTo(5000);

  await client.processMessage(recordingMessage(2000));
  await client.processMessage(recordingMessage(5000));
  await client.processMessage(recordingMessage(6000));

  expect(anchors.map(a => a.ms)).toEqual([1000, 5000, 5000, 6000]);
});

test('suppresses anchors during a backwards seek until the replay reaches the target', async () => {
  const { client, anchors } = makeClient();

  await client.processMessage(recordingMessage(5000));

  client.seekTo(2000);

  await client.processMessage(recordingMessage(100));
  await client.processMessage(recordingMessage(2000));

  expect(anchors.map(a => a.ms)).toEqual([5000, 2000, 2000]);
});

test('emits player status instead of time anchors on play/pause', () => {
  const { client, anchors, statuses } = makeClient();

  client.togglePlayPause();
  client.togglePlayPause();

  expect(statuses).toEqual([StatusEnum.PAUSED, StatusEnum.PLAYING]);
  expect(anchors).toHaveLength(0);
});

test('does not emit time anchors on speed changes', () => {
  const { client, anchors } = makeClient();

  client.setPlaySpeed(2);

  expect(anchors).toHaveLength(0);
});

test('reflects the paused state and speed in subsequent anchors', async () => {
  const { client, anchors } = makeClient();

  client.togglePlayPause();
  client.setPlaySpeed(2);

  await client.processMessage(recordingMessage(1000));

  expect(anchors).toEqual([{ ms: 1000, speed: 2, paused: true }]);
});

test('emits a complete status on the end message', async () => {
  const { client, statuses } = makeClient();

  await client.processMessage(controlMessage({ message: 'end' }));

  expect(statuses).toEqual([StatusEnum.COMPLETE]);
});

test('emits an error on the error message', async () => {
  const { client } = makeClient();

  const errors: Error[] = [];
  client.onError(error => errors.push(error));

  await client.processMessage(
    controlMessage({ message: 'error', errorText: 'playback failed' })
  );

  expect(errors).toEqual([new Error('playback failed')]);
});

test('sends actions over the transport', () => {
  const { client, send } = makeClient();

  client.togglePlayPause();
  client.setPlaySpeed(2);
  client.seekTo(1000);

  expect(send.mock.calls.map(call => JSON.parse(call[0]))).toEqual([
    { action: 'play/pause' },
    { action: 'speed', speed: 2 },
    { action: 'seek', pos: 1000 },
  ]);
});
