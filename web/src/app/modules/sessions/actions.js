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

const logger = require('app/common/logger').create('Modules/Sessions');
const { TLPT_SESSINS_RECEIVE, TLPT_SESSINS_UPDATE }  = require('./actionTypes');

const actions = {

  fetchSession(sid){
    return api.get(cfg.api.getFetchSessionUrl(sid)).then(json=>{
      if(json && json.session){
        reactor.dispatch(TLPT_SESSINS_UPDATE, json.session);
      }
    });
  },

  fetchSessions({before, sid, limit=cfg.maxSessionLoadSize}){
    let start = before || new Date();
    let params = {
      order: -1,
      limit
    };

    params.start = start.toISOString();

    if(sid){
      params.sessionID = sid;
      params.sessionId = sid;
      params.sid = sid;
    }

    return api.get(cfg.api.getFetchSessionsUrl(params))
      .done((json) => {
        reactor.dispatch(TLPT_SESSINS_RECEIVE, json.sessions);
      })
      .fail((err)=>{
        showError('Unable to retrieve list of sessions');
        logger.error('fetchSessions', err);
      });
  },

  updateSession(json){
    reactor.dispatch(TLPT_SESSINS_UPDATE, json);
  }
}

export default actions;
