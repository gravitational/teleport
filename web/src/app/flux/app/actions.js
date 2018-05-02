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

import $ from 'jQuery';
import reactor from 'app/reactor';
import { SET_SITE_ID, ADD_NAV_ITEM } from './actionTypes';
import { RECEIVE_CLUSTERS } from './../sites/actionTypes';
import { RECEIVE_USER } from './../user/actionTypes';
import { RECEIVE_USERACL } from './../userAcl/actionTypes';
import api from 'app/services/api';
import cfg from 'app/config';
import { initAppStatus } from 'app/flux/status/actions';
import { fetchNodes } from './../nodes/actions';
import { fetchActiveSessions } from 'app/flux/sessions/actions';
import Logger from 'app/lib/logger';

const logger = Logger.create('flux/app');

export function addNavItem(item) {
  reactor.dispatch(ADD_NAV_ITEM, item);
}

export function setSiteId(siteId) {
  reactor.dispatch(SET_SITE_ID, siteId);    
}

export function initApp(siteId, featureActivator) {         
  initAppStatus.start();  
  // get the list of available clusters        
  return fetchInitData(siteId)  
    .done(() => {
      featureActivator.onload();
      initAppStatus.success();      
    })
    .fail(err => {      
      let msg = api.getErrorText(err);
      initAppStatus.fail(msg);
    })      
}

export function refresh() {
  return $.when(      
    fetchActiveSessions(),
    fetchNodes()
  )    
}

export function fetchInitData(siteId) {
  return $.when(fetchSites(), fetchUserContext())
    .then(masterSiteId => {
      const selectedCluster = siteId || masterSiteId;
      setSiteId(selectedCluster);
      return $.when(fetchNodes(), fetchActiveSessions());
    });  
}

export function fetchSites(){
  return api.get(cfg.api.sitesBasePath)
    .then(json => {
      let masterSiteId = null;
      let sites = json.sites;     
      if (sites) {
        masterSiteId = sites[0].name;
      }
              
      reactor.dispatch(RECEIVE_CLUSTERS, sites);
      
      return masterSiteId;
  })
  .fail(err => {      
    logger.error('fetchSites', err);
  })    
}

export function  fetchUserContext(){
  return api.get(cfg.api.userContextPath).done(json=>{      
    reactor.dispatch(RECEIVE_USER, { name: json.userName, authType: json.authType });
    reactor.dispatch(RECEIVE_USERACL, json.userAcl);
  })    
}      
