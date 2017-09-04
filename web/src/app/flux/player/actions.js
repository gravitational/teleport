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
import { fetchStoredSession } from './../sessions/actions';
import sessionGetters from './../sessions/getters';
import { getAcl } from './../userAcl/store';

const logger = require('app/lib/logger').create('app/flux/player');

import {  
  TLPT_PLAYER_INIT,
  TLPT_PLAYER_CLOSE,
  TLPT_PLAYER_SET_STATUS
} from './actionTypes';

const actions = {

  openPlayer(routeParams) {
    let routeUrl = cfg.getPlayerUrl(routeParams);
    history.push(routeUrl);      
  },

  initPlayer(routeParams) {
    logger.info('initPlayer()', routeParams);         
    let {sid, siteId } = routeParams;    
    reactor.dispatch(TLPT_PLAYER_SET_STATUS, { isLoading: true });
    fetchStoredSession(sid, siteId)
      .done(() => {
        let storedSession = reactor.evaluate(sessionGetters.storedSessionById(sid));
        if (!storedSession) {
          reactor.dispatch(TLPT_PLAYER_SET_STATUS, {
            isError: true,
            errorText: 'Cannot find archived session'
          });                    
        } else {
          let { siteId } = storedSession;    
          reactor.dispatch(TLPT_PLAYER_INIT, {
            siteId,          
            sid          
          });          
        }                  
      })
      .fail(err => {          
        logger.error('open session', err);
        let errorText = api.getErrorText(err);
        reactor.dispatch(TLPT_PLAYER_SET_STATUS, {
          isError: true, 
          errorText,
         });                        
      })
  },

  close() {    
    reactor.dispatch(TLPT_PLAYER_CLOSE);    
    const canListSessions = getAcl().getSessionAccess().read;
    const redirect = canListSessions ? cfg.routes.sessions : cfg.routes.app;
    history.push(redirect);    
  }
}

export default actions;