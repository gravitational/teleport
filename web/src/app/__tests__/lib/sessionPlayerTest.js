/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
import api from 'app/services/api';
import $ from 'jQuery';
import expect, { spyOn } from 'expect';
import { EventProvider, TtyPlayer, MAX_SIZE, Buffer } from 'app/lib/term/ttyPlayer';
import { TermEventEnum } from 'app/lib/term/enums';
import sample from './streamData';

const Dfd = $.Deferred;

describe('lib/term/ttyPlayer/eventProvider', () => {

  afterEach(function () {
    expect.restoreSpies();
  });

  describe('new()', () => {
    it('should create an instance', () => {
      const provider = new EventProvider({url: 'sample.com'});
      expect(provider.events).toEqual([]);
    });
  });

  describe('init()', () => {
    it('should load events and initialize itself', function () {
      const provider = new EventProvider({url: 'sample.com'});
      spyOn(api, 'get').andReturn(Dfd().resolve(sample))
      spyOn(provider, '_createEvents').andCallThrough();
      spyOn(provider, '_normalizeEventsByTime').andCallThrough();
      spyOn(provider, '_fetchContent').andReturn(Dfd().resolve());
      spyOn(provider, '_populatePrintEvents');

      provider.init();
      expect(api.get).toHaveBeenCalledWith('sample.com/events');
      expect(provider._createEvents).toHaveBeenCalledWith(sample.events);
      expect(provider._normalizeEventsByTime).toHaveBeenCalled();
      expect(provider._fetchContent).toHaveBeenCalled();
      expect(provider._populatePrintEvents).toHaveBeenCalled();
    });
  });

  describe('_createEvents()', () => {
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
        time: new Date("2016-05-09T14:57:51.238Z"),
        msNormalized: 1744,
      };

      expect(events.length).toBe(32);
      expect(events[30]).toEqual(eventObj);
    });
  });


  describe('fetchContent()', () => {
    it('should fetch session content', () => {
      const provider = new EventProvider({url: 'sample.com'});
      const expectedReq = {
        dataType: 'text',
        processData: true,
        url: `sample.com/stream?offset=0&bytes=${MAX_SIZE}`
      }

      spyOn(provider, '_fetchEvents').andReturn(Dfd().resolve(provider._createEvents(sample.events)));
      spyOn(api, 'ajax').andReturn(Dfd().resolve(sample.data))
      provider.init();
      expect(api.ajax).toHaveBeenCalledWith(expectedReq);

      const buf = new Buffer(sample.data);
      const lastEvent = provider.events[provider.events.length-2];
      const expectedChunk = buf.slice(
        lastEvent.offset,
        lastEvent.offset + lastEvent.bytes).toString('utf8');
      expect(lastEvent.data).toEqual(expectedChunk);
    });
  });
});

describe('lib/ttyPlayer', () => {

  afterEach(() => {
    expect.restoreSpies();
  });

  describe('new()', () => {
    it('should create an instance', () => {
      const ttyPlayer = new TtyPlayer({url: 'testSid'});
      expect(ttyPlayer.isReady).toBe(false);
      expect(ttyPlayer.isPlaying).toBe(false);
      expect(ttyPlayer.isError).toBe(false);
      expect(ttyPlayer.isLoading).toBe(true);
      expect(ttyPlayer.duration).toBe(0);
      expect(ttyPlayer.current).toBe(0);
    });
  });

  describe('connect()', () => {
    it('should initialize event provider', cb => {
      const ttyPlayer = new TtyPlayer({url: 'testSid'});
      spyOn(ttyPlayer._eventProvider, 'init').andReturn(Dfd().resolve(sample));
      ttyPlayer.on('change', cb);
      ttyPlayer.connect();
      expect(ttyPlayer.isReady).toBe(true);
      expect(ttyPlayer.length).toBe(sample.events.length);
    });

    it('should indicate its loading status', cb => {
      spyOn(api, 'get').andReturn($.Deferred());
      const ttyPlayer = new TtyPlayer({url: 'testSid'});
      ttyPlayer.on('change', cb);
      ttyPlayer.connect();
      expect(ttyPlayer.isLoading).toBe(true);
    });

    it('should indicate its error status', cb => {
      spyOn(api, 'get').andReturn($.Deferred().reject(new Error('!!!')));
      const ttyPlayer = new TtyPlayer({url: 'testSid'});
      ttyPlayer.on('change', cb);
      ttyPlayer.connect();
      expect(ttyPlayer.isError).toBe(true);
    });

  });

  describe('move()', () => {
    var tty = null;

    beforeEach(()=>{
      tty = new TtyPlayer({url: 'sample.com'});
      spyOn(api, 'ajax').andReturn(Dfd().resolve(sample.data));
      spyOn(api, 'get').andReturn(Dfd().resolve(sample));
      tty.connect();
      tty.isReady = true;
    });

    it('should move by 1 position when called w/o params', cb => {
      tty.on(TermEventEnum.DATA, data=>{
        expect(data.length).toBe(42);
        cb();
      });

      tty.move();
    });

    it('should move from 1 to 478 position (forward)', cb => {
      tty.on(TermEventEnum.DATA, data=>{
        cb();
        expect(data.length).toBe(11246);
      });

      tty.move(478);
    });

    it('should move from 478 to 1 position (back)', cb => {
      tty.current = 478;
      tty.on(TermEventEnum.DATA, data=>{
        cb();
        expect(data.length).toEqual(42);
      });

      tty.move(2);
    });

    it('should stop playing if new position is greater than session length', cb => {
      const someBigNumber = 1000;
      tty.on('change', cb);
      tty.move(someBigNumber);
      expect(tty.isPlaying).toBe(false);
      expect(tty.current).toBe(tty.length);
    });
  });
})