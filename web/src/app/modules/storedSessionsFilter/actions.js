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
var sessionModule = require('./../sessions');
var {filter} = require('./getters');
var {maxSessionLoadSize} = require('app/config');
var moment = require('moment');

const {
  TLPT_STORED_SESSINS_FILTER_SET_RANGE,
  TLPT_STORED_SESSINS_FILTER_SET_STATUS }  = require('./actionTypes');

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

  setTimeRange(start, end){
    reactor.batch(()=>{
      reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_RANGE, {start, end});
      _fetch(end);
    });
  }
}

function _fetch(before, sid){
  reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_STATUS, {isLoading: true, hasMore: false});
  sessionModule.actions.fetchSessions({sid, before, limit: maxSessionLoadSize})
    .done((json) => {
      let {start} = reactor.evaluate(filter);
      let {sessions } = json;
      let status = {
        hasMore: false,
        isLoading: false
      }

      if (sessions.length === maxSessionLoadSize) {
        let {id, created} = sessions[sessions.length-1];
        status.sid = id;
        status.hasMore = moment(start).isBefore(created)
      }

      reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_STATUS, status);
    });
}

export default actions;
