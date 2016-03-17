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

var reactor = require('app/reactor');

var {
  TLPT_REST_API_START,
  TLPT_REST_API_SUCCESS,
  TLPT_REST_API_FAIL } = require('./actionTypes');

export default {

  start(reqType){
    reactor.dispatch(TLPT_REST_API_START, {type: reqType});
  },

  fail(reqType, message){
    reactor.dispatch(TLPT_REST_API_FAIL,  {type: reqType, message});
  },

  success(reqType){
    reactor.dispatch(TLPT_REST_API_SUCCESS, {type: reqType});
  }

}
