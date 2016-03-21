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
var {maxSessionLoadSize} = require('app/config');
var moment = require('moment');
var apiUtils = require('app/services/apiUtils');

var {showError} = require('app/modules/notifications/actions');

const logger = require('app/common/logger').create('Modules/Sessions');

const {
  TLPT_STORED_SESSINS_FILTER_SET_RANGE,
  TLPT_STORED_SESSINS_FILTER_SET_STATUS }  = require('./actionTypes');

const {TLPT_SESSINS_RECEIVE, TLPT_SESSINS_REMOVE_STORED }  = require('./../sessions/actionTypes');

/**
* Due to current limitations of the backend API, the filtering logic for the Archived list of Session
* works as follows:
*  1) each time a new date range is set, all previously retrieved inactive sessions get deleted.
*  2) hasMore flag will be determine after a consequent fetch request with new date range values.
*/

const actions = {

  fetch(){
    let { end } = reactor.evaluate(filter);
    _fetch(end);
  },

  fetchMore(){
    let {status, end } = reactor.evaluate(filter);
    if(status.hasMore === true && !status.isLoading){
      _fetch(end, status.sid);
    }
  },

  removeStoredSessions(){
    reactor.dispatch(TLPT_SESSINS_REMOVE_STORED);
  },

  setTimeRange(start, end){
    reactor.batch(()=>{
      reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_RANGE, {start, end, hasMore: false});
      reactor.dispatch(TLPT_SESSINS_REMOVE_STORED);
      _fetch(end);
    });
  }
}

function _fetch(end, sid){
  let status = {
    hasMore: false,
    isLoading: true
  }

  reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_STATUS, status);

  let start = end || new Date();
  let params = {
    order: -1,
    limit: maxSessionLoadSize,
    start,
    sid
  };

  return apiUtils.filterSessions(params).done((json) => {
    let {sessions} = json;
    let {start} = reactor.evaluate(filter);

    status.hasMore = false;
    status.isLoading = false;

    if (sessions.length === maxSessionLoadSize) {
      let {id, created} = sessions[sessions.length-1];
      status.sid = id;
      status.hasMore = moment(start).isBefore(created);

      /**
      * remove at least 1 item before storing the sessions, this way we ensure that
      * there always will be at least one new item on the next 'fetchMore' request.
      */
      sessions = sessions.slice(0, maxSessionLoadSize-1);
    }

    reactor.batch(()=>{
      reactor.dispatch(TLPT_SESSINS_RECEIVE, sessions);
      reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_STATUS, status);
    });

  })
  .fail((err)=>{
    showError('Unable to retrieve list of sessions');
    logger.error('fetching filtered set of sessions', err);
  });

}

export default actions;
