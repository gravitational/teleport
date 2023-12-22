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

import api from 'teleport/services/api';

import { EventType } from './enums';

const URL_PREFIX_EVENTS = '/events';
const Buffer = BufferModule.Buffer;

export const MAX_SIZE = 5242880; // 5mg

export default class EventProvider {
  constructor({ url }) {
    this.url = url;
    this.events = [];
  }

  getDuration() {
    const eventCount = this.events.length;
    if (eventCount === 0) {
      return 0;
    }

    return this.events[eventCount - 1].msNormalized;
  }

  init() {
    return this._fetchEvents().then(events => {
      this.events = events;
      const printEvents = this.events.filter(onlyPrintEvents);
      if (printEvents.length === 0) {
        return;
      }

      return this._fetchContent(printEvents).then(buffer => {
        this._populatePrintEvents(buffer, printEvents);
      });
    });
  }

  _fetchEvents() {
    const url = this.url + URL_PREFIX_EVENTS;
    return api.get(url).then(json => {
      if (!json.events) {
        return [];
      }

      return this._createEvents(json.events);
    });
  }

  _fetchContent(events) {
    // calculate the size of the session in bytes to know how many
    // chunks to load due to maximum chunk size limitation.
    let offset = events[0].offset;
    const end = events.length - 1;
    const totalSize = events[end].offset - offset + events[end].bytes;
    const chunkCount = Math.ceil(totalSize / MAX_SIZE);

    // create a fetch request for each chunk
    const promises = [];
    for (let i = 0; i < chunkCount; i++) {
      const url = `${this.url}/stream?offset=${offset}&bytes=${MAX_SIZE}`;
      promises.push(
        api
          .fetch(url, {
            Accept: 'text/plain',
            'Content-Type': 'text/plain; charset=utf-8',
          })
          .then(response => response.text())
      );
      offset = offset + MAX_SIZE;
    }

    // fetch all chunks and then merge
    return Promise.all(promises).then(responses => {
      const allBytes = responses.reduce((byteStr, r) => byteStr + r, '');
      return new Buffer(allBytes);
    });
  }

  // assign a slice of tty stream to corresponding print event
  _populatePrintEvents(buffer, events) {
    let byteStrOffset = events[0].bytes;
    events[0].data = buffer.slice(0, byteStrOffset).toString('utf8');
    for (var i = 1; i < events.length; i++) {
      let { bytes } = events[i];
      events[i].data = buffer
        .slice(byteStrOffset, byteStrOffset + bytes)
        .toString('utf8');
      byteStrOffset += bytes;
    }
  }

  _createEvents(json) {
    let w, h;
    let events = [];

    // filter print events and ensure that each has the right screen size and valid values
    for (let i = 0; i < json.length; i++) {
      const { ms, event, offset, time, bytes } = json[i];

      // grab new screen size for the next events
      if (event === EventType.RESIZE || event === EventType.START) {
        [w, h] = json[i].size.split(':');
      }

      // session has ended, stop here
      if (event === EventType.END) {
        const start = new Date(events[0].time);
        const end = new Date(time);
        const duration = end.getTime() - start.getTime();
        events.push({
          eventType: event,
          ms: duration,
          time: new Date(time),
        });

        break;
      }

      // process only PRINT events
      if (event !== EventType.PRINT) {
        continue;
      }

      events.push({
        eventType: EventType.PRINT,
        ms,
        bytes,
        offset,
        data: null,
        w: Number(w),
        h: Number(h),
        time: new Date(time),
      });
    }

    return this._normalizeEventsByTime(events);
  }

  _normalizeEventsByTime(events) {
    if (!events || events.length === 0) {
      return [];
    }

    events.forEach(e => {
      e.displayTime = formatDisplayTime(e.ms);
      e.ms = e.ms > 0 ? Math.floor(e.ms / 10) : 0;
      e.msNormalized = e.ms;
    });

    let cur = events[0];
    let tmp = [];
    for (let i = 1; i < events.length; i++) {
      const sameSize = cur.w === events[i].w && cur.h === events[i].h;
      const delay = events[i].ms - cur.ms;

      // merge events with tiny delay
      if (delay < 2 && sameSize) {
        cur.bytes += events[i].bytes;
        continue;
      }

      // avoid long delays between chunks
      events[i].msNormalized = cur.msNormalized + shortenTime(delay);

      tmp.push(cur);
      cur = events[i];
    }

    if (tmp.indexOf(cur) === -1) {
      tmp.push(cur);
    }

    return tmp;
  }
}

function shortenTime(value) {
  if (value >= 25 && value < 50) {
    return 25;
  } else if (value >= 50 && value < 100) {
    return 50;
  } else if (value >= 100) {
    return 100;
  } else {
    return value;
  }
}

function formatDisplayTime(ms) {
  if (ms <= 0) {
    return '00:00';
  }

  let totalSec = Math.floor(ms / 1000);
  let totalDays = (totalSec % 31536000) % 86400;
  let h = Math.floor(totalDays / 3600);
  let m = Math.floor((totalDays % 3600) / 60);
  let s = (totalDays % 3600) % 60;

  m = m > 9 ? m : '0' + m;
  s = s > 9 ? s : '0' + s;
  h = h > 0 ? h + ':' : '';

  return `${h}${m}:${s}`;
}

export function onlyPrintEvents(e) {
  return e.eventType === EventType.PRINT;
}
