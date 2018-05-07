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

import reactor from 'app/reactor';
import api from 'app/services/api';
import cfg from 'app/config';
import moment from 'moment';
import appGetters from 'app/flux/app/getters';
import Logger from 'app/lib/logger';
import {
  RECEIVE_ACTIVE_SESSIONS,  
  RECEIVE_SITE_EVENTS,  
} from './actionTypes';

const logger = Logger.create('Modules/Sessions');

const actions = {

  fetchStoredSession(sid, siteId) {
    siteId = siteId || reactor.evaluate(appGetters.siteId);
    return api.get(cfg.api.getSessionEventsUrl({ siteId, sid })).then(json=>{
      if (json && json.events) {        
        reactor.dispatch(RECEIVE_SITE_EVENTS, { siteId, json: json.events });
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
      .done(json => {
        if (json && json.events) {          
          reactor.dispatch(RECEIVE_SITE_EVENTS, { siteId, json: json.events });
        }  
      })
      .fail(err => {        
        logger.error('fetchSiteEvents', err);
      });
  },

  fetchActiveSessions() {    
    const siteId = reactor.evaluate(appGetters.siteId);        
    return api.get(cfg.api.getFetchSessionsUrl(siteId))
      .done(json => {
        let sessions = json.sessions || [];                        
        reactor.dispatch(RECEIVE_ACTIVE_SESSIONS, { siteId, json: sessions });        
      })
      .fail(err => {        
        logger.error('fetchActiveSessions', err);
      });
  }
}

export default actions;
