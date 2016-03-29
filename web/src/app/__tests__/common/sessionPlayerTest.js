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

var { expect, $, Dfd, spyOn, api } = require('./..');
var TtyPlayer = require('app/common/ttyPlayer')
var sampleData = require('./../sessionPlayerSampleData');

describe('common/ttyPlayer', function () {
  var tty = null;

  afterEach(function () {
    expect.restoreSpies();
  });

  describe('new()', function () {
    it('should create an instance', function () {
      var tty = new TtyPlayer({sid: 'testSid'});
      expect(tty.isReady).toBe(false);
      expect(tty.isPlaying).toBe(false);
      expect(tty.isError).toBe(false);
      expect(tty.isLoading).toBe(true);
      expect(tty.length).toBe(-1);
      expect(tty.current).toBe(1);
    });
  });

  describe('connect()', function (cb) {
    it('should fetch session length', function (cb) {
      spyOn(api, 'get').andReturn($.Deferred().resolve({count:50}));
      var tty = new TtyPlayer({sid: 'testSid'});
      tty.on('change', cb);
      tty.connect();
      expect(tty.isReady).toBe(true);
      expect(tty.length).toBe(50);
    });

    it('should indicate its loading status', function (cb) {
      spyOn(api, 'get').andReturn($.Deferred());
      var tty = new TtyPlayer({sid: 'testSid'});
      tty.on('change', cb);
      tty.connect();
      expect(tty.isLoading).toBe(true);
    });

    it('should indicate its error status', function (cb) {
      spyOn(api, 'get').andReturn($.Deferred().reject());
      var tty = new TtyPlayer({sid: 'testSid'});
      tty.on('change', cb);
      tty.connect();
      expect(tty.isError).toBe(true);
    });

  });

  describe('move()', function () {
    beforeEach(()=>{
      spyOn(api, 'get').andReturn(
        $.Deferred().resolve({count: sampleData.length})
      );

      tty = new TtyPlayer({sid: 'testSid'});
      tty.connect();
      tty.ttyStream = sampleData;
      tty.isReady = true;
    });

    it('should move by 1 position when called w/o params', function (cb) {
      tty.on('data', data=>{
        expect(data).toEqual(sampleData[1].data);
        cb();
      });

      tty.move();
    });

    it('should move from 1 to 5th position (forward)', function (cb) {
      var strArray = outputSlice(1, 5);
      tty.on('data', data=>{
        expect(data).toEqual(strArray.join(''));
        cb();
      });

      tty.move(5);
    });

    it('should move from 5 to 1 position (back)', function (cb) {
      var strArray = outputSlice(1, 5);
      tty.current = 5;
      tty.on('data', data=>{
        expect(data).toEqual(sampleData[1].data);
        cb();
      });

      tty.move(1);
    });

    it('should stop playing if new position is greater than session length', function (cb) {
      let someBigNumber = 1000;
      tty.on('change', cb);
      tty.move(someBigNumber);
      expect(tty.isPlaying).toBe(false);
      expect(tty.current).toBe(tty.length);
    });

    it('should fetch if data for requested position has not been loaded', function (cb) {
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

    });
  });
})

function outputSlice(start, end){
  var tmp = sampleData.slice(start, end);
  var strArray = [];
  tmp.forEach(item=> strArray.push(item.data));
  return strArray;
}
