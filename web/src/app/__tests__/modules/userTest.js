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

var { reactor, sampleData, expect, Dfd, spyOn } = require('./../');
var {actions, getters} = require('app/modules/user');
var {TLPT_RECEIVE_USER} =  require('app/modules/user/actionTypes');

describe('modules/user', function () {

  afterEach(function () {
    reactor.reset()
  })

  describe('getters', function () {
    beforeEach(function () {
      reactor.dispatch(TLPT_RECEIVE_USER, sampleData.user);
    });

    it('should return "user"', function () {
      var expected = {"name":"alex","logins":["admin","bob"], "shortDisplayName": "a"};
      expect(reactor.evaluateToJS(getters.user)).toEqual(expected);
    });

  });
})
