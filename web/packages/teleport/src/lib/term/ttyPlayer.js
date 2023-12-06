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

import BufferModule from 'buffer/';
import Logger from 'shared/libs/logger';

import Tty from './tty';
import { TermEvent } from './enums';
import { onlyPrintEvents } from './ttyPlayerEventProvider';

const logger = Logger.create('TtyPlayer');
const STREAM_START_INDEX = 0;
const PLAY_SPEED = 10;

export const Buffer = BufferModule.Buffer;
export const StatusEnum = {
  PLAYING: 'PLAYING',
  ERROR: 'ERROR',
  PAUSED: 'PAUSED',
  LOADING: 'LOADING',
};

export default class TtyPlayer extends Tty {
  constructor(eventProvider) {
    super({});
    this.currentEventIndex = 0;
    this.current = 0;
    this.duration = 0;
    this.status = StatusEnum.LOADING;
    this.statusText = '';

    this._posToEventIndexMap = [];
    this._eventProvider = eventProvider;

    // _chunkQueue is a list of data chunks waiting to be rendered by the term.
    this._chunkQueue = [];
    // _writeInFlight prevents sending more data to xterm while a prior render has not finished yet.
    this._writeInFlight = false;
  }

  // override
  send() {}

  // override
  connect() {
    this.status = StatusEnum.LOADING;
    this._change();
    return this._eventProvider
      .init()
      .then(() => {
        this._init();
        this.status = StatusEnum.PAUSED;
      })
      .catch(err => {
        logger.error('unable to init event provider', err);
        this._handleError(err);
      })
      .finally(this._change.bind(this));
  }

  pauseFlow() {
    this._writeInFlight = true;
  }

  resumeFlow() {
    this._writeInFlight = false;
    this._chunkDequeue();
  }

  move(newPos) {
    if (!this.isReady()) {
      return;
    }

    if (newPos === undefined) {
      newPos = this.current + 1;
    }

    if (newPos < 0) {
      newPos = 0;
    }

    if (newPos > this.duration) {
      this.stop();
    }

    const newEventIndex = this._getEventIndex(newPos) + 1;

    if (newEventIndex === this.currentEventIndex) {
      this.current = newPos;
      this._change();
      return;
    }

    const isRewind = this.currentEventIndex > newEventIndex;

    try {
      // we cannot playback the content within terminal so instead:
      // 1. tell terminal to reset.
      // 2. tell terminal to render 1 huge chunk that has everything up to current
      // location.
      if (isRewind) {
        this._chunkQueue = [];
        this.emit(TermEvent.RESET);
      }

      const from = isRewind ? 0 : this.currentEventIndex;
      const to = newEventIndex;
      const events = this._eventProvider.events.slice(from, to);
      const printEvents = events.filter(onlyPrintEvents);

      this._render(printEvents);
      this.currentEventIndex = newEventIndex;
      this.current = newPos;
      this._change();
    } catch (err) {
      logger.error('move', err);
      this._handleError(err);
    }
  }

  stop() {
    this.status = StatusEnum.PAUSED;
    this.timer = clearInterval(this.timer);
    this._change();
  }

  play() {
    if (this.status === StatusEnum.PLAYING) {
      return;
    }

    this.status = StatusEnum.PLAYING;
    // start from the beginning if reached the end of the session
    if (this.current >= this.duration) {
      this.current = STREAM_START_INDEX;
      this.emit(TermEvent.RESET);
    }

    this.timer = setInterval(this.move.bind(this), PLAY_SPEED);
    this._change();
  }

  getCurrentTime() {
    if (this.currentEventIndex) {
      let { displayTime } =
        this._eventProvider.events[this.currentEventIndex - 1];
      return displayTime;
    } else {
      return '--:--';
    }
  }

  getEventCount() {
    return this._eventProvider.events.length;
  }

  isLoading() {
    return this.status === StatusEnum.LOADING;
  }

  isPlaying() {
    return this.status === StatusEnum.PLAYING;
  }

  isError() {
    return this.status === StatusEnum.ERROR;
  }

  isReady() {
    return (
      this.status !== StatusEnum.LOADING && this.status !== StatusEnum.ERROR
    );
  }

  disconnect() {
    // do nothing
  }

  _init() {
    this.duration = this._eventProvider.getDuration();
    this._eventProvider.events.forEach(item =>
      this._posToEventIndexMap.push(item.msNormalized)
    );
  }

  _chunkDequeue() {
    const chunk = this._chunkQueue.shift();
    if (!chunk) {
      return;
    }

    const str = chunk.data.join('');
    this.emit(TermEvent.RESIZE, { h: chunk.h, w: chunk.w });
    this.emit(TermEvent.DATA, str);
  }

  _render(events) {
    if (!events || events.length === 0) {
      return;
    }

    const groups = [
      {
        data: [events[0].data],
        w: events[0].w,
        h: events[0].h,
      },
    ];

    let cur = groups[0];

    // group events by screen size and construct 1 chunk of data per group
    for (let i = 1; i < events.length; i++) {
      if (cur.w === events[i].w && cur.h === events[i].h) {
        cur.data.push(events[i].data);
      } else {
        cur = {
          data: [events[i].data],
          w: events[i].w,
          h: events[i].h,
        };

        groups.push(cur);
      }
    }

    this._chunkQueue = [...this._chunkQueue, ...groups];
    if (!this._writeInFlight) {
      this._chunkDequeue();
    }
  }

  _getEventIndex(num) {
    const arr = this._posToEventIndexMap;
    var low = 0;
    var hi = arr.length - 1;

    while (hi - low > 1) {
      const mid = Math.floor((low + hi) / 2);
      if (arr[mid] < num) {
        low = mid;
      } else {
        hi = mid;
      }
    }

    if (num - arr[low] <= arr[hi] - num) {
      return low;
    }

    return hi;
  }

  _change() {
    this.emit('change');
  }

  _handleError(err) {
    this.status = StatusEnum.ERROR;
    this.statusText = err.message;
  }
}
