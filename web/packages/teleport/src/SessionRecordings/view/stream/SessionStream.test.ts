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

import {
  PlayerState,
  SessionStream,
} from 'teleport/SessionRecordings/view/stream/SessionStream';
import {
  ResponseType,
  type BaseEvent,
} from 'teleport/SessionRecordings/view/stream/types';

import { Player } from '../player/Player';

function setup() {
  const ws = new MockWebSocket();
  const player = new MockPlayer();
  const stream = new SessionStream(
    ws,
    player,
    decodeMockEvent,
    MockEventType.End,
    25000
  );

  return { stream, player, ws };
}

describe('play', () => {
  it('should not transition to playing whilst loading', () => {
    const { stream } = setup();

    const stateListener = jest.fn();
    stream.on('state', stateListener);

    stream.play();

    expect(stateListener).not.toHaveBeenCalled();
  });

  it('should transition from loading to paused once the initial events are loaded', async () => {
    const { stream, ws } = setup();

    const statePromise = new Promise<PlayerState>(resolve => {
      stream.once('state', resolve);
    });

    stream.loadInitial();

    ws.receiveMessage(
      createBatchEvent([createMockEvent(MockEventType.Print, 500)])
    );

    ws.receiveMessage(createStopEvent(0, 1000));

    expect(await statePromise).toBe(PlayerState.Paused);
  });

  it('should resume to playing state after initial load if wasPlayingBeforeSeek', async () => {
    const { stream, ws } = setup();

    const pausedPromise = new Promise<PlayerState>(resolve => {
      stream.once('state', resolve);
    });

    stream.loadInitial();

    ws.receiveMessage(
      createBatchEvent([createMockEvent(MockEventType.Print, 500)])
    );
    ws.receiveMessage(createStopEvent(0, 20000));

    expect(await pausedPromise).toBe(PlayerState.Paused);

    const playPromise = new Promise<PlayerState>(resolve => {
      stream.once('state', resolve);
    });

    stream.play();
    stream.seek(22000);

    ws.receiveMessage(
      createBatchEvent([createMockEvent(MockEventType.Print, 22500)], 2)
    );

    expect(await playPromise).toBe(PlayerState.Playing);
  });

  it('should call player.onPlay when transitioning to playing', () => {
    const { stream, player, ws } = setup();

    stream.loadInitial();
    ws.receiveMessage(
      createBatchEvent([createMockEvent(MockEventType.Print, 500)])
    );
    ws.receiveMessage(createStopEvent(0, 1000));

    stream.play();

    expect(player.onPlay).toHaveBeenCalled();
  });
});

describe('pause', () => {
  it('should transition from playing to paused', () => {
    const { stream, ws } = setup();

    const stateListener = jest.fn();
    stream.on('state', stateListener);

    stream.loadInitial();
    ws.receiveMessage(
      createBatchEvent([createMockEvent(MockEventType.Print, 500)])
    );
    ws.receiveMessage(createStopEvent(0, 1000));

    stream.play();

    expect(stateListener).toHaveBeenCalledWith(PlayerState.Playing);

    stream.pause();

    expect(stateListener).toHaveBeenCalledWith(PlayerState.Paused);
  });

  it('should call player.onPause when pausing', () => {
    const { stream, player, ws } = setup();

    stream.loadInitial();
    ws.receiveMessage(
      createBatchEvent([createMockEvent(MockEventType.Print, 500)])
    );
    ws.receiveMessage(createStopEvent(0, 1000));

    stream.play();
    stream.pause();

    expect(player.onPause).toHaveBeenCalled();
  });

  it('should not do anything if already paused', () => {
    const { stream, ws } = setup();

    const stateListener = jest.fn();

    stream.loadInitial();
    ws.receiveMessage(
      createBatchEvent([createMockEvent(MockEventType.Print, 500)])
    );
    ws.receiveMessage(createStopEvent(0, 1000));

    stream.on('state', stateListener);
    stream.pause();

    expect(stateListener).not.toHaveBeenCalled();
  });
});

describe('seek', () => {
  it('should emit time event when seeking', () => {
    const { stream, ws } = setup();

    const timeListener = jest.fn();
    stream.on('time', timeListener);

    stream.loadInitial();
    ws.receiveMessage(
      createBatchEvent([createMockEvent(MockEventType.Print, 500)])
    );
    ws.receiveMessage(createStopEvent(0, 1000));

    stream.seek(750);

    expect(timeListener).toHaveBeenCalledWith(750);
  });

  it('should call player.onSeek when seeking', () => {
    const { stream, player, ws } = setup();

    stream.loadInitial();
    ws.receiveMessage(
      createBatchEvent([createMockEvent(MockEventType.Print, 500)])
    );
    ws.receiveMessage(createStopEvent(0, 1000));

    stream.seek(750);

    expect(player.onSeek).toHaveBeenCalledWith(750);
  });

  it('should clear player when seeking backwards', () => {
    const { stream, ws, player } = setup();

    stream.loadInitial();
    ws.receiveMessage(
      createBatchEvent([
        createMockEvent(MockEventType.Print, 500),
        createMockEvent(MockEventType.Print, 1000),
      ])
    );
    ws.receiveMessage(createStopEvent(0, 2000));

    stream.seek(1500);
    player.cleared = false;
    stream.seek(250);

    expect(player.cleared).toBe(true);
  });
});

describe('event handling', () => {
  it('should ignore messages with wrong requestId', () => {
    jest.spyOn(console, 'warn').mockImplementation(() => {});

    const { stream, ws, player } = setup();

    stream.loadInitial();

    ws.receiveMessage(
      createBatchEvent([createMockEvent(MockEventType.Print, 100)], 999)
    );

    expect(player.events).toHaveLength(0);
  });

  it('should stop when an end event is encountered', () => {
    const { stream, ws } = setup();

    const stateListener = jest.fn();

    stream.loadInitial();

    ws.receiveMessage(
      createBatchEvent([
        createMockEvent(MockEventType.Print, 100),
        createMockEvent(MockEventType.Print, 200),
        createMockEvent(MockEventType.End, 300),
      ])
    );

    jest.spyOn(performance, 'now').mockReturnValueOnce(0);
    jest.spyOn(performance, 'now').mockReturnValueOnce(400);

    stream.on('state', stateListener);
    stream.play();

    expect(stateListener).toHaveBeenCalledWith(PlayerState.Stopped);
  });
});

describe('time events', () => {
  it('should emit time events during playback', () => {
    const { stream, ws } = setup();

    const timeListener = jest.fn();
    stream.on('time', timeListener);

    stream.loadInitial();
    ws.receiveMessage(
      createBatchEvent([createMockEvent(MockEventType.Print, 500)])
    );
    ws.receiveMessage(createStopEvent(0, 1000));

    stream.play();

    expect(timeListener).toHaveBeenCalled();
  });
});

function createStopEvent(startTime: number, endTime: number, requestId = 1) {
  const buffer = new ArrayBuffer(33);
  const view = new DataView(buffer);
  view.setUint8(0, ResponseType.Stop);
  view.setBigInt64(1, BigInt(0));
  view.setUint32(9, 0);
  view.setUint32(13, requestId);
  view.setBigInt64(17, BigInt(startTime));
  view.setBigInt64(25, BigInt(endTime));
  return buffer;
}

function createBatchEvent(events: ArrayBuffer[], requestId = 1) {
  const totalLength = events.reduce((sum, event) => sum + event.byteLength, 0);
  const buffer = new ArrayBuffer(17 + totalLength);
  const view = new DataView(buffer);

  view.setUint8(0, ResponseType.Batch);
  view.setUint32(1, events.length);
  view.setUint32(5, requestId);
  view.setUint32(9, totalLength);

  let offset = 17;
  for (const event of events) {
    new Uint8Array(buffer, offset, event.byteLength).set(new Uint8Array(event));
    offset += event.byteLength;
  }

  return buffer;
}

function createMockEvent(
  type: MockEventType,
  timestamp: number,
  requestId = 1
): ArrayBuffer {
  const buffer = new ArrayBuffer(17);
  const view = new DataView(buffer);
  view.setUint8(0, type);
  view.setBigInt64(1, BigInt(timestamp));
  view.setUint32(9, 0);
  view.setUint32(13, requestId);
  return buffer;
}

function decodeMockEvent(buffer: ArrayBuffer): MockEvent {
  const view = new DataView(buffer);
  const type = view.getUint8(0) as MockEventType;
  const timestamp = Number(view.getBigInt64(1));
  const requestId = view.getUint32(13);

  return {
    type,
    timestamp,
    requestId,
  } as MockEvent;
}

enum MockEventType {
  Print,
  Resize,
  End,
}

interface MockEventPrint extends BaseEvent<MockEventType.Print> {
  content: string;
}

interface MockEventResize extends BaseEvent<MockEventType.Resize> {
  cols: number;
  rows: number;
}

interface MockEventEnd extends BaseEvent<MockEventType.End> {}

type MockEvent = MockEventPrint | MockEventResize | MockEventEnd;

class MockPlayer extends Player<MockEvent> {
  events: MockEvent[] = [];
  cleared = false;
  destroyed = false;
  handleCalled = false;

  init(): void {}

  destroy(): void {
    this.destroyed = true;
  }

  applyEvent(event: MockEvent): void {
    this.events.push(event);
  }

  handleEvent(event: MockEvent): boolean {
    this.handleCalled = true;

    return event.type === MockEventType.End;
  }

  clear(): void {
    this.cleared = true;
    this.events = [];
  }

  onPlay = jest.fn();
  onPause = jest.fn();
  onSeek = jest.fn();
  onStop = jest.fn();
}

interface WebSocketEventMap {
  close: [CloseEvent];
  error: [Event];
  message: [MessageEvent];
  open: [Event];
}

class MockWebSocket
  extends EventEmitter<WebSocketEventMap>
  implements WebSocket
{
  sentMessages: ArrayBuffer[] = [];
  closed = false;
  binaryType = 'arraybuffer' as const;
  bufferedAmount = 0;
  extensions = '';
  protocol = '';
  readyState = WebSocket.OPEN;
  url = 'ws://localhost';

  send(data: ArrayBuffer): void {
    this.sentMessages.push(data);
  }

  close(): void {
    this.closed = true;
  }

  // Helper to simulate receiving a message
  receiveMessage(data: ArrayBuffer): void {
    const event = new MessageEvent('message', { data });
    if (this.onmessage) {
      this.onmessage(event);
    }
  }

  onmessage: ((event: MessageEvent) => void) | null = null;
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  dispatchEvent(event: Event): boolean {
    return this.emit(event.type as keyof WebSocketEventMap, event as any);
  }

  readonly CLOSED: 3;
  readonly CLOSING: 2;
  readonly CONNECTING: 0;
  readonly OPEN: 1;

  addEventListener<K extends keyof WebSocketEventMap>(
    type: K,
    listener: (this: WebSocket, ev: WebSocketEventMap[K]) => any,
    options?: boolean | AddEventListenerOptions
  ): void;
  addEventListener(
    type: string,
    listener: EventListenerOrEventListenerObject,
    // eslint-disable-next-line unused-imports/no-unused-vars
    options?: boolean | AddEventListenerOptions
  ): void {
    this.on(type as keyof WebSocketEventMap, listener as any);
  }

  removeEventListener<K extends keyof WebSocketEventMap>(
    type: K,
    listener: (this: WebSocket, ev: WebSocketEventMap[K]) => any,
    options?: boolean | EventListenerOptions
  ): void;
  removeEventListener(
    type: string,
    listener: EventListenerOrEventListenerObject,
    // eslint-disable-next-line unused-imports/no-unused-vars
    options?: boolean | EventListenerOptions
  ): void {
    this.off(type as keyof WebSocketEventMap, listener as any);
  }
}
