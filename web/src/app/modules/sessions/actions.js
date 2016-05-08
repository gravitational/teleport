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
var api = require('app/services/api');
var apiUtils = require('app/services/apiUtils');
var cfg = require('app/config');
var {showError} = require('app/modules/notifications/actions');
var moment = require('moment');

const logger = require('app/common/logger').create('Modules/Sessions');
const { TLPT_SESSINS_RECEIVE, TLPT_SESSINS_UPDATE, TLPT_SESSINS_UPDATE_WITH_EVENTS }  = require('./actionTypes');

const actions = {

  fetchStoredSession(sid){
    return api.get(cfg.api.getSessionEventsUrl(sid)).then(json=>{
      if(json && json.events){
        reactor.dispatch(TLPT_SESSINS_UPDATE_WITH_EVENTS, json.events);
      }
    });
  },

  fetchSiteEvents(start, end){
    // default values
    start = start || moment(new Date()).endOf('day').toDate();
    end = end || moment(end).subtract(3, 'day').startOf('day').toDate();

    start = start.toISOString();
    end = end.toISOString();

    return api.get(cfg.api.getSiteEventsFilterUrl(start, end))
      .done((json) => {
        let {events=[]} = json;
        reactor.dispatch(TLPT_SESSINS_UPDATE_WITH_EVENTS, events);
      })
      .fail((err)=>{
        showError('Unable to retrieve site events');
        logger.error('fetchSiteEvents', err);
      });
  },

  fetchActiveSessions({end, sid, limit=cfg.maxSessionLoadSize}={}){
    let start = end || new Date();
    let params = {
      order: -1,
      limit,
      start,
      sid
    };

    return apiUtils.filterSessions(params)
      .done((json) => {
        reactor.dispatch(TLPT_SESSINS_RECEIVE, json.sessions);
      })
      .fail((err)=>{
        showError('Unable to retrieve list of sessions');
        logger.error('fetchActiveSessions', err);
      });
  },

  updateSession(json){
    reactor.dispatch(TLPT_SESSINS_UPDATE, json);
  }
}

export default actions;
