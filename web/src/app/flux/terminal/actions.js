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
import history from 'app/services/history';
import api from 'app/services/api';
import cfg from 'app/config';
import Logger from 'app/lib/logger'; 
import { getNodeStore } from './../nodes/nodeStore';
import sessionGetters from './../sessions/getters';
import { TLPT_TERMINAL_INIT, TLPT_TERMINAL_CLOSE, TLPT_TERMINAL_SET_STATUS } from './actionTypes';

const logger = Logger.create('flux/terminal');

const setStatus = json => reactor.dispatch(TLPT_TERMINAL_SET_STATUS, json);    

const initStore = params => {
  const { serverId } = params;
  const server = getNodeStore().findServer(serverId);  
  const hostname = server ? server.hostname : '';
  reactor.dispatch(TLPT_TERMINAL_INIT, {
    ...params,
    hostname
  });
}

const createSid = routeParams => {
  let { login, siteId } = routeParams;
  let data = {
    session: {
      terminal_params: {
        w: 45,
        h: 5
      },
      login
    }
  };

  return api.post(cfg.api.getSiteSessionUrl(siteId), data);    
}
    
export const initTerminal = routeParams => {
  logger.info('attempt to open a terminal', routeParams);    
  
  let { sid } = routeParams;
  
  setStatus({ isLoading: true });
  
  if (sid) {                  
    let activeSession = reactor.evaluate(sessionGetters.activeSessionById(sid));
    if (activeSession) {      
      // init store with existing sid
      initStore(routeParams);
      setStatus({ isReady: true });
    } else {
      setStatus({ isNotFound: true });              
    }   

    return;
  } 
  
  createSid(routeParams)
    .done(json => {
      let sid = json.session.id;
      let newRouteParams = {
        ...routeParams,
        sid
      };        
      initStore(newRouteParams)
      setStatus({ isReady: true });
      updateRoute(newRouteParams);                    
    })
    .fail(err => {
      let errorText = api.getErrorText(err);
      setStatus({ isError: true, errorText });
    });  
}
    
export const close = () => {    
  reactor.dispatch(TLPT_TERMINAL_CLOSE);      
  history.push(cfg.routes.nodes);      
}

export const updateRoute = newRouteParams => {    
  let routeUrl = cfg.getTerminalLoginUrl(newRouteParams);                                    
  history.push(routeUrl);      
}  
