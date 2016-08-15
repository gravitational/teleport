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
var {filter} = require('./getters');
var {fetchSiteEvents} = require('./../sessions/actions');
var {showError} = require('app/modules/notifications/actions');

const logger = require('app/common/logger').create('Modules/Sessions');

const { TLPT_STORED_SESSINS_FILTER_SET_RANGE }  = require('./actionTypes');

const { TLPT_SESSINS_REMOVE_STORED }  = require('./../sessions/actionTypes');

const actions = {

  fetch(){
    let { start, end } = reactor.evaluate(filter);
    _fetch(start, end);
  },

  removeStoredSessions(){
    reactor.dispatch(TLPT_SESSINS_REMOVE_STORED);
  },

  setTimeRange(start, end){
    reactor.batch(()=>{
      reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_RANGE, {start, end});
      _fetch(start, end);
    });
  }
}

function _fetch(start, end){
  return fetchSiteEvents(start, end)
    .fail((err)=>{
      showError('Unable to retrieve list of sessions for a given time range');
      logger.error('fetching filtered set of sessions', err);
    });
}

export default actions;
