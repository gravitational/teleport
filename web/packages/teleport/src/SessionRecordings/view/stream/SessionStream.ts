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

import { parseArrayBuffer } from 'teleport/SessionRecordings/view/stream/decoding';
import {
  isBatchEvent,
  isErrorEvent,
  isStartEvent,
  isStopEvent,
  type BaseEvent,
} from 'teleport/SessionRecordings/view/stream/types';

import { type Player } from '../player/Player';
import { encodeFetchRequest } from './encoding';

export enum PlayerState {
  Loading,
  Paused,
  Playing,
  Stopped,
}

interface SessionStreamEvents {
  state: [PlayerState];
  time: [number];
}

interface Range {
  end: number;
  start: number;
}

const LOAD_THRESHOLD_MS = 5 * 1000; // 5 seconds
const LOAD_CHUNK_MS = 20 * 1000; // 20 seconds

/**
 * SessionStream manages the loading and buffering of events from a WebSocket connection.
 *
 * It handles play, pause, and seek operations, ensuring that events are loaded and played back
 * in a smooth manner. The class uses an internal buffer to store events and manages the state of playback.
 *
 * It passes the events to a Player instance for rendering, so it can be used with different types of
 * session recordings.
 */
export class SessionStream<
  TEvent extends BaseEvent<TEventType>,
  TEventType extends number = number,
  TEndEventType extends TEventType = TEventType,
> extends EventEmitter<SessionStreamEvents> {
  private buffer: TEvent[] = [];
  private currentBufferIndex = 0;
  private maxBufferSize = 2000;

  private loadedRange: Range = {
    end: 0,
    start: 0,
  };
  private loadingRange: Range | null = null;

  private animationFrameId: number | null = null;
  private currentTime = 0;
  private pausedTime = 0;
  private startTime = 0;

  private atEnd = false;
  // we track loading separately from the loading state as we can still play and load at the same time
  private loading = true;
  private requestId = 0;
  private state: PlayerState = PlayerState.Loading;
  private wasPlayingBeforeSeek = false;

  constructor(
    private ws: WebSocket,
    private player: Player<TEvent>,
    private decodeEvent: (data: ArrayBuffer) => TEvent,
    private endEventType: TEndEventType,
    private duration: number
  ) {
    super();

    ws.onmessage = this.handleMessage.bind(this);
  }

  destroy() {
    this.removeAllListeners();
    this.ws.close();

    if (this.animationFrameId) {
      cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = null;
    }

    this.player.destroy();
  }

  loadInitial() {
    this.load(0, LOAD_CHUNK_MS, false);
  }

  play() {
    if (this.loading || this.state === PlayerState.Playing) {
      return;
    }

    if (this.state === PlayerState.Stopped) {
      this.seek(0);
    }

    this.setState(PlayerState.Playing);

    if (this.pausedTime > 0) {
      this.startTime = performance.now() - this.pausedTime;
      this.pausedTime = 0;
    } else if (!this.startTime) {
      this.startTime = performance.now();
      this.currentBufferIndex = 0;
    }

    this.playEvents();
    this.player.onPlay();
  }

  seek(time: number) {
    if (time > this.duration) {
      return;
    }

    this.wasPlayingBeforeSeek = this.state === PlayerState.Playing;

    this.emit('time', time);
    this.player.onSeek(time);

    if (this.state === PlayerState.Stopped) {
      this.setState(PlayerState.Paused);
    }

    const isTimeLoaded =
      time >= this.loadedRange.start && time <= this.loadedRange.end;

    if (!isTimeLoaded) {
      // we do not have the requested time loaded in the buffer, so clear the buffer
      // and request the state of the screen at the requested time, and continue playback
      // from there
      this.clearBuffer();

      if (this.animationFrameId !== null) {
        cancelAnimationFrame(this.animationFrameId);
        this.animationFrameId = null;
      }

      this.startTime = performance.now() - time;
      this.currentTime = time;

      if (
        this.state === PlayerState.Paused ||
        this.state === PlayerState.Stopped
      ) {
        this.pausedTime = time;
      }

      this.setState(PlayerState.Loading);

      this.load(time, time + LOAD_CHUNK_MS, true);

      return;
    }

    if (time < this.currentTime) {
      // we are seeking backwards, so we cannot just apply the events from the current buffer index
      // we need to reset the player state and re-apply all events from the start of the buffer
      this.currentTime = 0;
      this.currentBufferIndex = 0;

      this.player.clear();
    }

    this.startTime = performance.now() - time;

    // play all events up until the requested time
    this.playEventsUpUntil(time);

    if (
      this.state === PlayerState.Paused ||
      this.state === PlayerState.Stopped
    ) {
      // if we are paused or stopped, we need to update the paused time to the requested time
      this.pausedTime = time;
    }
  }

  pause() {
    if (this.state === PlayerState.Paused) {
      return;
    }

    this.setState(PlayerState.Paused);

    if (this.animationFrameId !== null) {
      cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = null;
    }

    this.pausedTime = performance.now() - this.startTime;
    this.currentTime = this.pausedTime;
    this.wasPlayingBeforeSeek = false;

    this.player.onPause();
  }

  private playEvents = () => {
    if (this.state !== PlayerState.Playing) {
      return;
    }

    const currentTime = performance.now() - this.startTime;

    this.currentTime = currentTime;

    this.emit('time', currentTime);

    while (
      this.currentBufferIndex < this.buffer.length &&
      this.buffer[this.currentBufferIndex].timestamp <= currentTime
    ) {
      const event = this.buffer[this.currentBufferIndex];

      this.currentBufferIndex++;

      if (event.type === this.endEventType) {
        this.setState(PlayerState.Stopped);
        this.player.onStop();

        return;
      }

      this.player.applyEvent(event);
    }

    this.checkBufferStatus(currentTime);

    this.animationFrameId = requestAnimationFrame(this.playEvents);
  };

  private playEventsUpUntil(time: number) {
    while (
      this.currentBufferIndex < this.buffer.length &&
      this.buffer[this.currentBufferIndex].timestamp <= time
    ) {
      const event = this.buffer[this.currentBufferIndex];

      this.currentBufferIndex++;

      this.player.applyEvent(event);
    }

    this.currentTime = time;

    this.checkBufferStatus(time);
  }

  private checkBufferStatus(currentTime: number) {
    if (this.loadingRange) {
      return;
    }

    const isWithinLoadedRange =
      currentTime >= this.loadedRange.start &&
      currentTime + LOAD_THRESHOLD_MS <= this.loadedRange.end;

    if (isWithinLoadedRange) {
      return;
    }

    const lastEventTime =
      this.buffer.length > 0
        ? this.buffer[this.buffer.length - 1].timestamp
        : currentTime;

    const remainingBufferTime = lastEventTime - currentTime;

    if (
      !this.atEnd &&
      !this.loading &&
      remainingBufferTime < LOAD_THRESHOLD_MS
    ) {
      const newStartTime = this.loadedRange.end;
      const newEndTime = newStartTime + LOAD_CHUNK_MS;

      this.load(newStartTime, newEndTime);
    }
  }

  private setState(state: PlayerState) {
    this.state = state;
    this.emit('state', state);
  }

  private handleMessage(event: MessageEvent) {
    if (!(event.data instanceof ArrayBuffer)) {
      return;
    }

    const parsed = parseArrayBuffer(event.data, this.decodeEvent);

    if (parsed.requestId !== this.requestId) {
      // eslint-disable-next-line no-console
      console.warn(
        `Ignoring event with requestId ${parsed.requestId}, current requestId is ${this.requestId}`
      );

      return;
    }

    if (isBatchEvent(parsed)) {
      // mark loading as false as soon as we get any batch event so we can start playing again
      this.loading = false;

      this.pushBatchToBuffer(parsed.events);

      if (this.state === PlayerState.Loading) {
        if (this.wasPlayingBeforeSeek) {
          this.play();
        } else {
          this.setState(PlayerState.Paused);
        }
      }

      const hasEnd =
        parsed.events[parsed.events.length - 1].type === this.endEventType;

      if (hasEnd) {
        this.end();
      }

      return;
    }

    if (isStartEvent(parsed)) {
      return;
    }

    if (isStopEvent(parsed)) {
      this.loading = false;
      this.loadingRange = null;

      this.updateLoadedTimes(parsed.startTime, parsed.endTime);

      if (this.state === PlayerState.Loading) {
        if (this.wasPlayingBeforeSeek) {
          this.play();
        } else {
          this.setState(PlayerState.Paused);
        }
      }

      return;
    }

    if (isErrorEvent(parsed)) {
      return;
    }

    // if handleEvent returns true, it means the event has been handled and does not need to be added to the buffer
    if (this.player.handleEvent(parsed)) {
      return;
    }

    this.pushToBuffer(parsed);
  }

  private end() {
    this.atEnd = true;
  }

  private load(
    startTime: number,
    endTime: number,
    requestCurrentScreen = false
  ) {
    if (this.atEnd) {
      return;
    }

    const isAlreadyLoading =
      this.loadingRange &&
      this.loadingRange.start <= startTime &&
      this.loadingRange.end >= endTime;

    if (isAlreadyLoading) {
      return;
    }

    this.requestId += 1;

    this.loading = true;

    this.loadingRange = { end: endTime, start: startTime };

    this.ws.send(
      encodeFetchRequest({
        endTime,
        requestCurrentScreen,
        requestId: this.requestId,
        startTime,
      })
    );
  }

  private pushBatchToBuffer(events: TEvent[]) {
    this.buffer = this.buffer.concat(events);
    this.trimBuffer();
  }

  private pushToBuffer(event: TEvent) {
    this.buffer.push(event);
    this.trimBuffer();
  }

  private trimBuffer() {
    if (this.buffer.length - this.currentBufferIndex > this.maxBufferSize) {
      this.buffer = this.buffer.slice(this.currentBufferIndex);
      this.currentBufferIndex = 0;
      this.updateLoadedTimes(
        this.buffer[0]?.timestamp ?? 0,
        this.buffer[this.buffer.length - 1]?.timestamp ?? 0
      );
    }
  }

  private clearBuffer() {
    this.buffer = [];
    this.currentBufferIndex = 0;
    this.startTime = 0;
    this.pausedTime = 0;
    this.loadedRange = {
      end: 0,
      start: 0,
    };
    this.atEnd = false;
    this.loadingRange = null;
  }

  private updateLoadedTimes(start: number, end: number) {
    if (this.loadedRange.start === 0 && this.loadedRange.end === 0) {
      this.loadedRange = { start, end };

      return;
    }

    this.loadedRange = {
      start: Math.min(this.loadedRange.start, start),
      end: Math.max(this.loadedRange.end, end),
    };
  }
}
