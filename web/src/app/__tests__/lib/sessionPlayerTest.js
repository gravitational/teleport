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

var { expect, $, spyOn, api, Dfd } = require('./..');
var {EventProvider, TtyPlayer, Buffer} = require('app/lib/term/ttyPlayer');
var sample = require('./streamData')

describe('lib/term/ttyPlayer/eventProvider', function(){

  afterEach(function () {
    expect.restoreSpies();
  });

  describe('new()', function(){
    it('should create an instance', function () {
      var provider = new EventProvider({url: 'sample.com'});
      expect(provider.events).toEqual([]);
      expect(provider.getLength()).toBe(0);
    });
  });

  describe('init()', function(){
    it('should load events and initialize itself', function () {
      var provider = new EventProvider({url: 'sample.com'});
      spyOn(api, 'get').andReturn(Dfd().resolve(sample))
      spyOn(provider, '_createPrintEvents');
      spyOn(provider, '_normalizeEventsByTime');

      provider.init();

      expect(api.get).toHaveBeenCalledWith('sample.com/events');
      expect(provider._createPrintEvents).toHaveBeenCalledWith(sample.events);
      expect(provider._normalizeEventsByTime).toHaveBeenCalled();
    });
  });

  describe('_createPrintEvents()', function(){
    it('should filter json data for print events and create event objects', function () {
      var provider = new EventProvider({url: 'sample.com'});
      var eventObj = {
        displayTime: "00:45",
        ms: 4524,
        msNormalized: 4524,
        bytes: 183,
        offset: 144056,
        data: null,
        w: 115,
        h: 23,
        time: new Date("2016-05-09T14:57:51.243Z")
      };

      provider._createPrintEvents(sample.events);
      expect(provider.events.length).toBe(101);
      expect(provider.events[100]).toEqual(eventObj);
    });
  });

  describe('_normalizeEventsByTime()', function(){
    it('should adjust time for a better replay by shortening delays between events', function () {
      var provider = new EventProvider({url: 'sample.com'});
      provider._createPrintEvents(sample.events);
      provider._normalizeEventsByTime();

      expect(provider.events.length).toBe(31);
      expect(provider.events[30].msNormalized).toBe(1780);
    });
  });

  describe('getEventsWithByteStream(start, end)', function(){
    it('should check if event data needs to be fetched and return true', function () {
      var provider = new EventProvider({url: 'sample.com'});
      spyOn(api, 'get').andReturn(Dfd().resolve(sample))
      provider.init();
      expect(provider._shouldFetch(0, 3)).toBe(true);
    });

    it('should fetch data stream with the right URL', function () {
      spyOn(api, 'ajax').andReturn(Dfd());
      spyOn(api, 'get').andReturn(Dfd().resolve(sample))

      var provider = new EventProvider({url: 'sample.com'});
      var expected = {
        dataType: 'text',
        processData: true,
        url: 'sample.com/stream?offset=0&bytes=144239'
      }

      provider.init();
      provider.getEventsWithByteStream(0, 1);

      expect(api.ajax).toHaveBeenCalledWith(expected);
    });

    it('should be able to fetch and then procces the byte stream', function () {
      var actual = null;
      var provider = new EventProvider({url: 'sample.com'});
      provider._createPrintEvents(sample.events);
      provider._normalizeEventsByTime();

      spyOn(api, 'ajax').andReturn(Dfd().resolve(sample.data))

      var {bytes, offset} = provider.events[10];
      var buf = new Buffer(sample.data);
      var expected = buf.slice(0, offset + bytes).toString('utf8');

      provider.getEventsWithByteStream(0, 11).done(events=>{
        actual = events.map(ev => ev.data).join('');
      });

      expect(actual).toEqual(expected);

    });
  });
});

describe('lib/ttyPlayer', function () {

  afterEach(function () {
    expect.restoreSpies();
  });

  describe('new()', function () {
    it('should create an instance', function () {
      var ttyPlayer = new TtyPlayer({url: 'testSid'});
      expect(ttyPlayer.isReady).toBe(false);
      expect(ttyPlayer.isPlaying).toBe(false);
      expect(ttyPlayer.isError).toBe(false);
      expect(ttyPlayer.isLoading).toBe(true);
      expect(ttyPlayer.length).toBe(-1);
      expect(ttyPlayer.current).toBe(0);
    });
  });

  describe('connect()', function () {
    it('should initialize event provider', function (cb) {
      var ttyPlayer = new TtyPlayer({url: 'testSid'});
      spyOn(ttyPlayer._eventProvider, 'init').andReturn(Dfd().resolve(sample));
      ttyPlayer.on('change', cb);
      ttyPlayer.connect();
      expect(ttyPlayer.isReady).toBe(true);
      expect(ttyPlayer.length).toBe(sample.events.length);
    });

    it('should indicate its loading status', function (cb) {
      spyOn(api, 'get').andReturn($.Deferred());
      var ttyPlayer = new TtyPlayer({url: 'testSid'});
      ttyPlayer.on('change', cb);
      ttyPlayer.connect();
      expect(ttyPlayer.isLoading).toBe(true);
    });

    it('should indicate its error status', function (cb) {      
      spyOn(api, 'get').andReturn($.Deferred().reject(new Error('!!!')));
      var ttyPlayer = new TtyPlayer({url: 'testSid'});
      ttyPlayer.on('change', cb);
      ttyPlayer.connect();      
      expect(ttyPlayer.isError).toBe(true);
    });

  });

  describe('move()', function () {
    var tty = null;

    beforeEach(()=>{
      tty = new TtyPlayer({url: 'sample.com'});
      spyOn(api, 'ajax').andReturn(Dfd().resolve(sample.data));
      spyOn(api, 'get').andReturn(Dfd().resolve(sample));
      tty.connect();
      tty.isReady = true;
    });

    it('should move by 1 position when called w/o params', function (cb) {
      tty.on('data', data=>{
        expect(data.length).toBe(42);
        cb();
      });

      tty.move();
    });

    it('should move from 1 to 478 position (forward)', function (cb) {
      tty.on('data', data=>{
        cb();
        expect(data.length).toBe(11246);
      });

      tty.move(478);
    });

    it('should move from 478 to 1 position (back)', function (cb) {
      tty.current = 478;
      tty.on('data', data=>{
        cb();
        expect(data.length).toEqual(42);
      });

      tty.move(2);
    });

    it('should stop playing if new position is greater than session length', function (cb) {
      let someBigNumber = 1000;
      tty.on('change', cb);
      tty.move(someBigNumber);
      expect(tty.isPlaying).toBe(false);
      expect(tty.current).toBe(tty.length);
    });
  });
})

