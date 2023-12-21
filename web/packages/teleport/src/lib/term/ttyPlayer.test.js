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

import '@gravitational/shared/libs/polyfillFinally';
import api from 'teleport/services/api';

import { TermEvent } from 'teleport/lib/term/enums';

import TtyPlayer, { Buffer } from './ttyPlayer';
import EventProvider, { MAX_SIZE } from './ttyPlayerEventProvider';
import sample from './fixtures/streamData';

describe('lib/term/ttyPlayer/eventProvider', () => {
  afterEach(function () {
    jest.clearAllMocks();
  });

  it('should create an instance', () => {
    const provider = new EventProvider({ url: 'sample.com' });
    expect(provider.events).toEqual([]);
  });

  it('should load events and initialize itself', async () => {
    const provider = new EventProvider({ url: 'sample.com' });

    jest.spyOn(api, 'get').mockImplementation(() => Promise.resolve(sample));
    jest.spyOn(provider, '_createEvents');
    jest.spyOn(provider, '_normalizeEventsByTime');
    jest
      .spyOn(provider, '_fetchContent')
      .mockImplementation(() => Promise.resolve());
    jest.spyOn(provider, '_populatePrintEvents').mockImplementation();

    await provider.init();

    expect(api.get).toHaveBeenCalledWith('sample.com/events');
    expect(provider._createEvents).toHaveBeenCalledWith(sample.events);
    expect(provider._normalizeEventsByTime).toHaveBeenCalled();
    expect(provider._fetchContent).toHaveBeenCalled();
    expect(provider._populatePrintEvents).toHaveBeenCalled();
  });

  it('should create event objects', () => {
    const provider = new EventProvider({ url: 'sample.com' });
    const events = provider._createEvents(sample.events);
    const eventObj = {
      eventType: 'print',
      displayTime: '00:45',
      ms: 4523,
      bytes: 6516,
      offset: 137723,
      data: null,
      w: 115,
      h: 23,
      time: new Date('2016-05-09T14:57:51.238Z'),
      msNormalized: 1744,
    };

    expect(events).toHaveLength(32);
    expect(events[30]).toEqual(eventObj);
  });

  it('should fetch session content', async () => {
    const provider = new EventProvider({ url: 'sample.com' });

    jest
      .spyOn(provider, '_fetchEvents')
      .mockImplementation(() =>
        Promise.resolve(provider._createEvents(sample.events))
      );

    jest
      .spyOn(api, 'fetch')
      .mockImplementation(() => Promise.resolve({ text: () => sample.data }));

    await provider.init();

    expect(api.fetch).toHaveBeenCalledWith(
      `sample.com/stream?offset=0&bytes=${MAX_SIZE}`,
      {
        Accept: 'text/plain',
        'Content-Type': 'text/plain; charset=utf-8',
      }
    );

    const buf = new Buffer(sample.data);
    const lastEvent = provider.events[provider.events.length - 2];
    const expectedChunk = buf
      .slice(lastEvent.offset, lastEvent.offset + lastEvent.bytes)
      .toString('utf8');
    expect(lastEvent.data).toEqual(expectedChunk);
  });
});

describe('lib/ttyPlayer', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  it('should create an instance', () => {
    const ttyPlayer = new TtyPlayer({ url: 'testSid' });
    expect(ttyPlayer.isReady()).toBe(false);
    expect(ttyPlayer.isPlaying()).toBe(false);
    expect(ttyPlayer.isError()).toBe(false);
    expect(ttyPlayer.isLoading()).toBe(true);
    expect(ttyPlayer.duration).toBe(0);
    expect(ttyPlayer.current).toBe(0);
  });

  it('should connect using event provider', async () => {
    const ttyPlayer = new TtyPlayer(new EventProvider({ url: 'testSid' }));

    jest.spyOn(api, 'get').mockImplementation(() => Promise.resolve(sample));
    jest
      .spyOn(ttyPlayer._eventProvider, '_fetchContent')
      .mockImplementation(() => Promise.resolve(sample.data));

    await ttyPlayer.connect();

    expect(ttyPlayer.isReady()).toBe(true);
    expect(ttyPlayer.getEventCount()).toBe(32);
  });

  it('should indicate its loading status', async () => {
    const ttyPlayer = new TtyPlayer(new EventProvider({ url: 'testSid' }));
    jest
      .spyOn(api, 'get')
      .mockImplementation(() => Promise.resolve({ events: [] }));

    ttyPlayer.connect();
    expect(ttyPlayer.isLoading()).toBe(true);
  });

  it('should indicate its error status', async () => {
    jest.spyOn(console, 'error').mockImplementation(() => {});
    jest.spyOn(api, 'get').mockImplementation(() => Promise.reject('!!'));

    const ttyPlayer = new TtyPlayer(new EventProvider({ url: 'testSid' }));

    await ttyPlayer.connect();
    expect(ttyPlayer.isError()).toBe(true);
  });

  describe('move()', () => {
    var tty = null;

    beforeEach(() => {
      tty = new TtyPlayer(new EventProvider({ url: 'testSid' }));
      jest.spyOn(api, 'get').mockImplementation(() => Promise.resolve(sample));
      jest
        .spyOn(tty._eventProvider, '_fetchContent')
        .mockImplementation(() => Promise.resolve(sample.data));
    });

    afterEach(function () {
      jest.clearAllMocks();
    });

    it('should move by 1 position when called w/o params', async () => {
      await tty.connect();

      let renderedData = '';
      tty.on(TermEvent.DATA, data => {
        renderedData = data;
      });

      tty.move();
      expect(renderedData).toHaveLength(42);
    });

    it('should move from 1 to 478 position (forward)', async () => {
      await tty.connect();

      const renderedDataLength = [];
      const resizeEvents = [];

      tty.on(TermEvent.RESIZE, event => {
        resizeEvents.push(event);
      });

      tty.on(TermEvent.DATA, data => {
        renderedDataLength.push(data.length);
        tty.resumeFlow();
      });

      tty.move(478);

      const expected = [
        {
          resize: {
            h: 20,
            w: 147,
          },
          length: 12899,
        },
        {
          resize: {
            h: 29,
            w: 146,
          },
          length: 9415,
        },
        {
          resize: {
            h: 31,
            w: 146,
          },
          length: 10113,
        },
        {
          resize: {
            h: 25,
            w: 146,
          },
          length: 8018,
        },
      ];

      for (let i = 0; i < expected.length; i++) {
        expect(resizeEvents[i]).toEqual(expected[i].resize);
        expect(renderedDataLength[i]).toBe(expected[i].length);
      }
    });

    it('should move from 478 to 1 position (back)', async () => {
      await tty.connect();

      let renderedData = '';
      tty.current = 478;
      tty.on(TermEvent.DATA, data => {
        renderedData = data;
      });

      tty.move(2);
      expect(renderedData).toHaveLength(42);
    });

    it('should stop playing if new position is greater than session length', async () => {
      await tty.connect();
      tty.play();

      const someBigNumber = 20000;
      tty.move(someBigNumber);

      expect(tty.isPlaying()).toBe(false);
    });
  });
});
