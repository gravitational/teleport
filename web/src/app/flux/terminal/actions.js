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
import session from 'app/services/session';
import api from 'app/services/api';
import cfg from 'app/config';
//import getters from './getters';
import { updateSession } from './../sessions/actions';
import sessionGetters from './../sessions/getters';
//import {showError} from 'app/flux/notifications/actions';

const logger = require('app/lib/logger').create('Current Session');

const { TLPT_TERMINAL_OPEN, TLPT_TERMINAL_CLOSE, TLPT_TERMINAL_SET_STATUS } = require('./actionTypes');

const changeBrowserUrl = newRouteParams => {
  let routeUrl = cfg.getTerminalLoginUrl(newRouteParams);                                    
  session.getHistory().push(routeUrl);      
}

const actions = {

  startNew(routeParams) {
    let newRouteParams = {
      ...routeParams,
      sid: undefined
    }      
    
    changeBrowserUrl(newRouteParams);    
    actions.initTerminal(newRouteParams);
  },
  
  createNewSession(routeParams) {
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

    return api.post(cfg.api.getSiteSessionUrl(siteId), data)
      .then(json => {
        let sid = json.session.id;
        let newRouteParams = {
          ...routeParams,
          sid
        };
    
        reactor.dispatch(TLPT_TERMINAL_OPEN, newRouteParams);
        reactor.dispatch(TLPT_TERMINAL_SET_STATUS, { isReady: true });
        changeBrowserUrl(newRouteParams);                    
      })
      .fail(err => {
        let errorText = api.getErrorText(err);
        reactor.dispatch(TLPT_TERMINAL_SET_STATUS, {
          isError: true,
          errorText,
        });
      });
  },

  initTerminal(routeParams) {
    logger.info('attempt to open a terminal', routeParams);    
    let { sid } = routeParams;

    reactor.dispatch(TLPT_TERMINAL_SET_STATUS, { isLoading: true });
    
    if (sid) {                  
      // look up active session matching given sid      
      let activeSession = reactor.evaluate(sessionGetters.activeSessionById(sid));
      if (activeSession) {        
        reactor.dispatch(TLPT_TERMINAL_OPEN, routeParams);        
        reactor.dispatch(TLPT_TERMINAL_SET_STATUS, { isReady: true });        
      } else {
        reactor.dispatch(TLPT_TERMINAL_SET_STATUS, { isNotFound: true });        
      }
    } else {
      actions.createNewSession(routeParams);      
    }
    
  },

  close() {    
    reactor.dispatch(TLPT_TERMINAL_CLOSE);    
    session.getHistory().push(cfg.routes.nodes);    
  },

  updateSessionFromEventStream(siteId) {
    return data => {
      data.events.forEach(item => {
        if (item.event === 'session.end') {
          actions.close();
        }
      })
      
      updateSession({
        siteId: siteId,
        json: data.session
      });
    }
  }

}

export default actions;