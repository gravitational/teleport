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
var cfg = require('app/config');
var {showError} = require('app/modules/notifications/actions');
var moment = require('moment');
var appGetters = require('app/modules/app/getters')

const logger = require('app/common/logger').create('Modules/Sessions');
const { TLPT_SESSIONS_UPDATE, TLPT_SESSIONS_UPDATE_WITH_EVENTS, TLPT_SESSIONS_RECEIVE }  = require('./actionTypes');

const actions = {

  fetchStoredSession(sid) {
    let siteId = reactor.evaluate(appGetters.siteId);
    return api.get(cfg.api.getSessionEventsUrl({ siteId, sid })).then(json=>{
      if(json && json.events){
        reactor.dispatch(TLPT_SESSIONS_UPDATE_WITH_EVENTS, { siteId, json: json.events });
      }
    });
  },

  fetchSiteEvents(start, end){
    // default values
    start = start || moment(new Date()).endOf('day').toDate();
    end = end || moment(end).subtract(3, 'day').startOf('day').toDate();

    start = start.toISOString();
    end = end.toISOString();

    let siteId = reactor.evaluate(appGetters.siteId);
    return api.get(cfg.api.getSiteEventsFilterUrl({ start, end, siteId }))
      .done( json => {
        if (json && json.events) {
          reactor.dispatch(TLPT_SESSIONS_UPDATE_WITH_EVENTS, { siteId, json: json.events });
        }  
      })
      .fail( err => {
        showError('Unable to retrieve site events');
        logger.error('fetchSiteEvents', err);
      });
  },

  fetchActiveSessions() {    
    let siteId = reactor.evaluate(appGetters.siteId);        
    return api.get(cfg.api.getFetchSessionsUrl(siteId))
      .done( json => {
        let sessions = json.sessions || [];        
        reactor.dispatch(TLPT_SESSIONS_RECEIVE, { siteId, json: sessions });
      })
      .fail( err => {
        showError('Unable to retrieve list of sessions');
        logger.error('fetchActiveSessions', err);
      });
  },
  
  updateSession({ siteId, json }){
    reactor.dispatch(TLPT_SESSIONS_UPDATE, { siteId, json });
  }
}

export default actions;
