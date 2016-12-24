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
import auth from 'app/services/auth';
import { showError } from 'app/modules/notifications/actions';
import { TLPT_APP_INIT, TLPT_APP_FAILED, TLPT_APP_READY } from './actionTypes';
import { TLPT_SITES_RECEIVE } from './../sites/actionTypes';
import { TLPT_NODES_RECEIVE } from './../nodes/actionTypes';
import { TLPT_SESSIONS_RECEIVE } from './../sessions/actionTypes';
import api from 'app/services/api';
import cfg from 'app/config';
const logger = require('app/common/logger').create('flux/app');

const actions = {

  initApp(nextState, cb) {
    reactor.dispatch(TLPT_APP_INIT);    
    actions.fetchAllData()
      .done(() => {        
        reactor.dispatch(TLPT_APP_READY);
        cb();
      })
      .fail(()=> reactor.dispatch(TLPT_APP_FAILED) );
  },

  fetchAllData(){
    return api.get(cfg.api.sitesBasePath)
      .then( json => {      
        let sites = json.sites;
        let nodes = [];
        let sessions = [];

        for (let i = 0; i < sites.length; i++){
          let siteId = sites[i].name;
          let siteNodes = sites[i].nodes || [];
          let siteSessions = sites[i].sessions || [];
          
          siteNodes.forEach(n => {
            n.siteId = siteId;
            nodes.push(n);
          });

          siteSessions.forEach(s => {
            s.siteId = siteId;
            sessions.push(s);
          });            
        }

        reactor.batch(() => {
          reactor.dispatch(TLPT_SITES_RECEIVE, sites);
          reactor.dispatch(TLPT_NODES_RECEIVE, nodes);
          reactor.dispatch(TLPT_SESSIONS_RECEIVE, sessions);          
        })              

    })
    .fail(err => {
      showError('Unable to retrieve data ');
      logger.error('fetchAllData', err);
    })    
  },

  resetApp() {
    // set to 'loading state' to notify subscribers
    reactor.dispatch(TLPT_APP_INIT);
    // reset  reactor
    reactor.reset();
  },

  checkIfValidUser() {                
    api.get(cfg.api.userStatus).fail(err => {
      if(err.status == 403){
        actions.logoutUser();
      }
    });
  },
  
  logoutUser(){
    actions.resetApp();
    auth.logout();
  }
}

window.actions = actions;

export default actions;
