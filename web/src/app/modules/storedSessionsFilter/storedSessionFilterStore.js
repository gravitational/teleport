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

var { Store, toImmutable } = require('nuclear-js');
var {weekRange} = require('app/common/dateUtils');
var {
  TLPT_STORED_SESSINS_FILTER_SET_RANGE,
  TLPT_STORED_SESSINS_FILTER_SET_STATUS } = require('./actionTypes');

export default Store({
  getInitialState() {
    let [start, end] = weekRange(new Date());
    let state = {
      start,
      end,
      status: {
        isLoading: false,
        hasMore: false
      }
    }

    return toImmutable(state);
  },

  initialize() {
    this.on(TLPT_STORED_SESSINS_FILTER_SET_RANGE, setRange);
    this.on(TLPT_STORED_SESSINS_FILTER_SET_STATUS, setStatus);
  }
})

function setStatus(state, status){
  return state.mergeIn(['status'], status);
}

function setRange(state, {start, end}){
  return state.set('start', start)
       .set('end', end)
       .set('hasMore', false);
}
