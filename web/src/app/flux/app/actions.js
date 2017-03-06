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
import { showError } from 'app/flux/notifications/actions';
import { TLPT_APP_SET_SITE_ID } from './actionTypes';
import { TRYING_TO_INIT_APP } from 'app/flux/restApi/constants';
import { TLPT_SITES_RECEIVE } from './../sites/actionTypes';
import api from 'app/services/api';
import cfg from 'app/config';
import restApiActions from 'app/flux/restApi/actions';
import { fetchNodes } from './../nodes/actions';
import { fetchActiveSessions } from 'app/flux/sessions/actions';
import $ from 'jQuery';

const logger = require('app/lib/logger').create('flux/app');

const actions = {

  setSiteId(siteId) {
    reactor.dispatch(TLPT_APP_SET_SITE_ID, siteId);    
  },

  initApp(nextState) {
    let { siteId } = nextState.params;        
    restApiActions.start(TRYING_TO_INIT_APP);    
    
    // get the list of available clusters        
    return actions.fetchSites()      
      .then(masterSiteId => {         
        siteId = siteId || masterSiteId;
        reactor.dispatch(TLPT_APP_SET_SITE_ID, siteId);
        // fetch nodes and active sessions 
        return $.when(fetchNodes(), fetchActiveSessions()).done(() => {
          restApiActions.success(TRYING_TO_INIT_APP);                                  
        })                
      })
      .fail(err => {        
        let msg = api.getErrorText(err);                
        restApiActions.fail(TRYING_TO_INIT_APP, msg);                        
      });
  },
  
  refresh() {
    actions.fetchSites();
    fetchActiveSessions();          
    fetchNodes();  
  },

  fetchSites(){
    return api.get(cfg.api.sitesBasePath)
      .then(json => {
        let masterSiteId = null;
        let sites = json.sites;     
        if (sites) {
          masterSiteId = sites[0].name;
        }
                
        reactor.dispatch(TLPT_SITES_RECEIVE, sites);
        
        return masterSiteId;
    })
    .fail(err => {
      showError('Unable to retrieve list of clusters ');
      logger.error('fetchSites', err);
    })    
  },

  resetApp() {    
    // reset  reactor
    reactor.reset();
  },
    
  logoutUser(){
    actions.resetApp();
    auth.logout();
  }
}

export default actions;
