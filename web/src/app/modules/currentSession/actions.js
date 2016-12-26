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
var session = require('app/services/session');
var api = require('app/services/api');
var cfg = require('app/config');
var getters = require('./getters');
var { fetchStoredSession, updateSession } = require('./../sessions/actions');
var sessionGetters = require('./../sessions/getters');
var $ = require('jQuery');

const logger = require('app/common/logger').create('Current Session');
const { TLPT_CURRENT_SESSION_OPEN, TLPT_CURRENT_SESSION_CLOSE } = require('./actionTypes');

const actions = {

  createNewSession(siteId, serverId, login){
    let data = { 'session': { 'terminal_params': { 'w': 45, 'h': 5 }, login } }        
    api.post(cfg.api.getSiteSessionUrl(siteId), data).then(json=>{
      let sid = json.session.id;
      let routeUrl = cfg.getCurrentSessionRouteUrl({ siteId, sid });
      let history = session.getHistory();

      reactor.dispatch(TLPT_CURRENT_SESSION_OPEN, {
        siteId,
        serverId,
        login,
        sid,
        isNewSession: true
      });

      history.push(routeUrl);
   });
 },

  openSession(nextState) {   
   let { sid } = nextState.params;
   let currentSession = reactor.evaluate(getters.currentSession);
   if(currentSession){
     return;
   }

   logger.info('attempt to open session', {sid});
   $.when(fetchStoredSession(sid))
     .done(()=>{
       let sView = reactor.evaluate(sessionGetters.sessionViewById(sid));
       if (!sView) {
         reactor.dispatch(TLPT_CURRENT_SESSION_OPEN, null);
         return;
       }

       let { serverId, login, siteId } = sView;
       logger.info('open session', 'OK');
       reactor.dispatch(TLPT_CURRENT_SESSION_OPEN, {
          siteId,
          serverId,
          login,
          sid,
          isNewSession: false
        });
     })
     .fail((err)=>{
       logger.error('open session', err);
       reactor.dispatch(TLPT_CURRENT_SESSION_OPEN, null);
     })
 },

  close(){
    let {isNewSession} = reactor.evaluate(getters.currentSession);

    reactor.dispatch(TLPT_CURRENT_SESSION_CLOSE);

    if(isNewSession){
      session.getHistory().push(cfg.routes.nodes);
    }else{
      session.getHistory().push(cfg.routes.sessions);
    }
  },

  processSessionEventStream(data){
    data.events.forEach(item=> {
      if(item.event === 'session.end'){
        actions.close();
      }
    })

    updateSession(data.session);
  }

}

export default actions;
