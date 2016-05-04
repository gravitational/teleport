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

/*var { expect, $, spyOn, api } = require('./..');
var TtyPlayer = require('app/common/ttyPlayer').default;
var {EventProvider} = require('app/common/ttyPlayer');
var sampleData = require('./../sessionPlayerSampleData');
*/

var { expect, $, spyOn, api, Dfd } = require('./..');
var {EventProvider, TtyPlayer} = require('app/common/term/ttyPlayer');

var sampleStreamData = {bytes: "G10wO2Frb250c2V2b3lAeDIyMDogfgdha29udHNldm95QHgyMjA6fiQgDRtbSxtdMDtha29udHNldm95QHgyMjA6IH4HYWtvbnRzZXZveUB4MjIwOn4kIGENCmE6IGNvbW1hbmQgbm90IGZvdW5kDQobXTA7YWtvbnRzZXZveUB4MjIwOiB+B2Frb250c2V2b3lAeDIyMDp+JCBiDQpiOiBjb21tYW5kIG5vdCBmb3VuZA0KG10wO2Frb250c2V2b3lAeDIyMDogfgdha29udHNldm95QHgyMjA6fiQgYw0KYzogY29tbWFuZCBub3QgZm91bmQNChtdMDtha29udHNldm95QHgyMjA6IH4HYWtvbnRzZXZveUB4MjIwOn4kIGV4aXQNCmxvZ291dA0K"};
var sampleEvents = {"events":[{"addr.local":"127.0.0.1:3022","addr.remote":"127.0.0.1:52238","event":"session.start","login":"akontsevoy","offset":0,"sec":0,"time":"2016-04-30T01:07:25Z","user":"akontsevoy"},{"event":"resize","login":"akontsevoy","offset":0,"sec":0,"size":"130:20","time":"2016-04-30T01:07:25Z","user":"akontsevoy"},{"bytes":42,"event":"print","offset":0,"sec":0},{"bytes":46,"event":"print","offset":42,"sec":4},{"bytes":1,"event":"print","offset":88,"sec":4},{"bytes":2,"event":"print","offset":89,"sec":5},{"bytes":22,"event":"print","offset":91,"sec":5},{"bytes":42,"event":"print","offset":113,"sec":5},{"bytes":1,"event":"print","offset":155,"sec":6},{"bytes":2,"event":"print","offset":156,"sec":6},{"bytes":22,"event":"print","offset":158,"sec":6},{"bytes":42,"event":"print","offset":180,"sec":6},{"bytes":1,"event":"print","offset":222,"sec":6},{"bytes":2,"event":"print","offset":223,"sec":7},{"bytes":22,"event":"print","offset":225,"sec":7},{"bytes":42,"event":"print","offset":247,"sec":7},{"bytes":1,"event":"print","offset":289,"sec":8},{"bytes":1,"event":"print","offset":290,"sec":9},{"bytes":1,"event":"print","offset":291,"sec":9},{"bytes":1,"event":"print","offset":292,"sec":9},{"bytes":2,"event":"print","offset":293,"sec":10},{"bytes":8,"event":"print","offset":295,"sec":10},{"event":"session.end","offset":303,"sec":10,"time":"2016-04-30T01:07:35Z","user":"akontsevoy"}]};

describe('common/term/ttyPlayer/eventProvider', function(){

  afterEach(function () {
    expect.restoreSpies();
  });

  describe('new()', function(){
    it('should create an instance', function () {
      var provider = new EventProvider({url: 'sample.com'});
      expect(provider.events).toEqual([]);
    });
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
      spyOn(api, 'get').andReturn(Dfd().resolve(sampleEvents))
      var provider = new EventProvider({url: 'sample.com'});
      var eventCount = sampleEvents.events.length;
      provider.init();
      expect(provider.getLength()).toBe(eventCount);
      expect(provider.events[eventCount-1].w).toEqual("130");
      expect(provider.events[eventCount-1].h).toEqual("20");
    });
  });

  describe('getEventsWithByteStream(start, end)', function(){
    it('should check if event data needs to be fetched and return true', function () {
      var provider = new EventProvider({url: 'sample.com'});
      provider._init(sampleEvents);
      provider.init();
      expect(provider._shouldFetch(0, 4000)).toBe(true);
    });

    it('should fetch data stream with the right URL', function () {
      spyOn(api, 'get').andReturn(Dfd());
      var provider = new EventProvider({url: 'sample.com'});
      provider._init(sampleEvents);
      provider.getEventsWithByteStream(0, 4000);
      expect(api.get).toHaveBeenCalledWith('sample.com/stream?offset=0&bytes=303');
    });

    it('should be able to fetch and then procces the byte stream', function () {
      spyOn(api, 'get').andReturn(Dfd().resolve(sampleStreamData));
      var provider = new EventProvider({url: 'sample.com'});
      var actualStreamData = '';
      var expectedStreamData = window.atob(sampleStreamData.bytes);

      provider._init(sampleEvents);
      provider.getEventsWithByteStream(0, 4000).done(events=>{
        actualStreamData = events.map(ev => ev.data).join('');
      });

      expect(actualStreamData).toEqual(expectedStreamData);
    });
  });
});

describe('common/ttyPlayer', function () {
  var tty = null;

  afterEach(function () {
    expect.restoreSpies();
  });

  describe('new()', function () {
    it('should create an instance', function () {
      var tty = new TtyPlayer({url: 'testSid'});
      expect(tty.isReady).toBe(false);
      expect(tty.isPlaying).toBe(false);
      expect(tty.isError).toBe(false);
      expect(tty.isLoading).toBe(true);
      expect(tty.length).toBe(-1);
      expect(tty.current).toBe(0);
    });
  });

  describe('connect()', function () {
    it('should fetch session length', function (cb) {
      spyOn(api, 'get').andReturn($.Deferred().resolve(sampleEvents));
      var tty = new TtyPlayer({url: 'testSid'});
      tty.on('change', cb);
      tty.connect();
      expect(tty.isReady).toBe(true);
      expect(tty.length).toBe(sampleEvents.events.length);
    });

    it('should indicate its loading status', function (cb) {
      spyOn(api, 'get').andReturn($.Deferred());
      var tty = new TtyPlayer({url: 'testSid'});
      tty.on('change', cb);
      tty.connect();
      expect(tty.isLoading).toBe(true);
    });

    it('should indicate its error status', function (cb) {
      spyOn(api, 'get').andReturn($.Deferred().reject());
      var tty = new TtyPlayer({url: 'testSid'});
      tty.on('change', cb);
      tty.connect();
      expect(tty.isError).toBe(true);
    });

  });

  describe('move()', function () {
    beforeEach(()=>{
      tty = new TtyPlayer({url: 'sample.com'});
      spyOn(api, 'get').andReturn(Dfd().resolve(sampleStreamData));
      spyOn(tty._eventProvider, 'init').andReturn(Dfd().resolve());


      tty._eventProvider._init(sampleEvents);
      tty.connect();
      tty.isReady = true;
    });

    it('should move by 1 position when called w/o params', function (cb) {
      tty.on('data', data=>{
        expect(data).toEqual('');
        cb();
      });

      tty.move();
    });

    it('should move from 1 to 5th position (forward)', function () {
      var expected = outputSlice(0, 4);
      var actualData = [];
      tty.on('data', data=>{
        actualData.push(data);
      });

      tty.move(5);
      expect(actualData[0]).toEqual('');
      expect(actualData[1]).toEqual(expected);
    });

    it('should move from 5 to 1 position (back)', function () {
      var expected = outputSlice(0, 2);
      var actualData = [];
      tty.current = 5;
      tty.on('data', data=>{
        actualData.push(data);
      });

      tty.move(3);
      expect(actualData[1]).toEqual(expected);
    });

    it('should stop playing if new position is greater than session length', function (cb) {
      let someBigNumber = 1000;
      tty.on('change', cb);
      tty.move(someBigNumber);
      expect(tty.isPlaying).toBe(false);
      expect(tty.current).toBe(tty.length);
    });

/*    it('should fetch if data for requested position has not been loaded', function (cb) {
      var expectedUrl = '/v1/webapi/sites/-current-/sessions/testSid/chunks?start=1&end=21';
      var apiResponse = sampleData.slice(1, sampleData.length).map(item=> {
        var {w, h, data} = item;
        data = btoa(data);
        return { data, term: {w, h} };

      });

      tty.ttyStream = tty.ttyStream.slice(0, 3);
      spyOn(api, 'get').andCall((url)=>{
        expect(url).toBe(expectedUrl);
        expect(tty.isLoading).toBe(true);
        expect(tty.isReady).toBe(false);
        return $.Deferred().resolve({
            chunks: apiResponse
          });
      });

      tty.on('data', data=>{
        var strArray = outputSlice(1, 5);
        expect(data).toEqual(strArray.join(''));
        cb();
      });

      tty.move(5);
      expect(tty.isLoading).toBe(false);
      expect(tty.isReady).toBe(true);

    });*/
  });
})

function outputSlice(start, end){
  var tmp = window.atob(sampleStreamData.bytes);
  var {events} = sampleEvents;
  var offset = events[start].offset;
  var bytes = events[start].bytes + offset;
  for(var i = start+1; i <= end; i++){
    bytes += events[i].bytes;
  }

  return tmp.slice(offset, bytes);
}
