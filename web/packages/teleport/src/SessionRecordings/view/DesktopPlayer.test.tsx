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

import { EventEmitter } from 'events';

import { act, createRef } from 'react';

import { fireEvent, render, screen } from 'design/utils/testing';

import type { PlayerTimeAnchor } from 'teleport/lib/tdp';
import {
  SessionRecordingEventType,
  type SessionRecordingEvent,
} from 'teleport/services/recordings';
import type { PlayerHandle } from 'teleport/SessionRecordings/view/SshPlayer';

import { DesktopPlayer } from './DesktopPlayer';

class FakePlayerClient {
  private events = new EventEmitter();

  connect = jest.fn().mockResolvedValue(undefined);
  shutdown = jest.fn();
  togglePlayPause = jest.fn();
  setPlaySpeed = jest.fn();
  seekTo = jest.fn((pos: number) => {
    this.emitAnchor({ ms: pos, speed: 1, paused: false });
  });

  onTimeUpdate = this.listenerFor<[PlayerTimeAnchor]>('time');
  onPlayerStatus = this.listenerFor('status');
  onError = this.listenerFor('error');
  onInfo = this.listenerFor('info');
  onTransportOpen = this.listenerFor('open');
  onTransportClose = this.listenerFor('close');
  onPngFrame = this.listenerFor('png');
  onBmpFrame = this.listenerFor('bmp');
  onScreenSpec = this.listenerFor('spec');

  emitTransportOpen() {
    this.events.emit('open');
  }

  emitAnchor(anchor: PlayerTimeAnchor) {
    this.events.emit('time', anchor);
  }

  private listenerFor<T extends unknown[]>(event: string) {
    return (listener: (...args: T) => void) => {
      this.events.on(event, listener);
      return () => this.events.off(event, listener);
    };
  }
}

let mockPlayerClient: FakePlayerClient;

jest.mock('teleport/lib/tdp', () => ({
  PlayerClient: jest.fn().mockImplementation(() => mockPlayerClient),
}));

beforeEach(() => {
  jest.useFakeTimers();
  mockPlayerClient = new FakePlayerClient();
});

afterEach(() => {
  jest.useRealTimers();
});

const INACTIVITY_EVENT: SessionRecordingEvent = {
  type: SessionRecordingEventType.Inactivity,
  startTime: 1000,
  endTime: 5000,
};

function renderPlayer(
  props: Partial<React.ComponentProps<typeof DesktopPlayer>> = {}
) {
  return render(
    <DesktopPlayer
      sid="test-session"
      clusterId="test-cluster"
      durationMs={60_000}
      {...props}
    />
  );
}

function startPlayback() {
  act(() => {
    mockPlayerClient.emitTransportOpen();
  });
}

function advance(ms: number) {
  act(() => {
    jest.advanceTimersByTime(ms);
  });
}

test('emits rAF-smoothed time updates from time anchors', () => {
  const onTimeChange = jest.fn();
  renderPlayer({ onTimeChange });
  startPlayback();

  act(() => {
    mockPlayerClient.emitAnchor({ ms: 1000, speed: 1, paused: false });
  });
  advance(500);

  const times = onTimeChange.mock.calls.map(call => call[0] as number);
  const last = times.at(-1);

  expect(last).toBeGreaterThanOrEqual(1400);
  expect(last).toBeLessThanOrEqual(1600);
  // time advances monotonically across frames
  expect(times).toEqual([...times].sort((a, b) => a - b));
});

test('interpolates at the anchor speed', () => {
  const onTimeChange = jest.fn();
  renderPlayer({ onTimeChange });
  startPlayback();

  act(() => {
    mockPlayerClient.emitAnchor({ ms: 1000, speed: 4, paused: false });
  });
  advance(500);

  const last = onTimeChange.mock.calls.at(-1)[0] as number;

  expect(last).toBeGreaterThanOrEqual(2600);
  expect(last).toBeLessThanOrEqual(3400);
});

test('holds the time while paused', () => {
  const onTimeChange = jest.fn();
  renderPlayer({ onTimeChange });
  startPlayback();

  act(() => {
    mockPlayerClient.emitAnchor({ ms: 2000, speed: 1, paused: true });
  });
  advance(500);

  expect(onTimeChange.mock.calls.at(-1)[0]).toBe(2000);
});

test('shows the skip button during an inactivity period and seeks past it', () => {
  renderPlayer({ events: [INACTIVITY_EVENT] });
  startPlayback();

  act(() => {
    mockPlayerClient.emitAnchor({ ms: 2000, speed: 1, paused: true });
  });
  advance(100);

  fireEvent.click(screen.getByText(/of inactivity/));

  expect(mockPlayerClient.seekTo).toHaveBeenCalledWith(5001);
});

test('does not show the skip button outside inactivity periods', () => {
  renderPlayer({ events: [INACTIVITY_EVENT] });
  startPlayback();

  act(() => {
    mockPlayerClient.emitAnchor({ ms: 8000, speed: 1, paused: true });
  });
  advance(100);

  expect(screen.queryByText(/of inactivity/)).not.toBeInTheDocument();
});

test('seeks the player through the moveToTime handle', () => {
  const ref = createRef<PlayerHandle>();
  renderPlayer({ ref });
  startPlayback();

  act(() => {
    ref.current.moveToTime(10_000);
  });

  expect(mockPlayerClient.seekTo).toHaveBeenCalledWith(10_000);
});

test('renders without metadata props', () => {
  renderPlayer();
  startPlayback();
  advance(100);

  expect(mockPlayerClient.connect).toHaveBeenCalled();
  expect(screen.queryByText(/of inactivity/)).not.toBeInTheDocument();
});
