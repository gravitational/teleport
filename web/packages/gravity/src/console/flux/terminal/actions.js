/*
Copyright 2019 Gravitational, Inc.

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
import reactor from 'gravity/reactor';
import api from 'gravity/services/api';
import cfg from 'gravity/config';
import { TLPT_TERMINAL_INIT, TLPT_TERMINAL_UPDATE_SESSION, TLPT_TERMINAL_SET_STATUS } from './actionTypes';

export function fetchSession({ siteId, sid}){
  // because given session might not be available right away,
  // fetch all session to avoid 404 errors.
  return api.get(cfg.getSiteSshSessionUrl({siteId}))
    .then(json => {
      if(!json && !json.sessions){
        return;
      }

      const session = json.sessions.find(s => s.id === sid);
      if(!session){
        return;
      }

      reactor.dispatch(TLPT_TERMINAL_UPDATE_SESSION, {
        parties: session.parties
      })
    })
}

export function createSession({ serverId, siteId, hostname, login, namespace, pod, container }){
  const request = {
    session: {
      login
    }
  };

  return api.post(cfg.getSiteSessionUrl(siteId), request)
    .then(json => {
      const sid = json.session.id;
      reactor.dispatch(TLPT_TERMINAL_INIT, {
        isNew: true,
        serverId,
        siteId,
        login,
        namespace,
        hostname,
        pod,
        container,
        sid
      });

      reactor.dispatch(TLPT_TERMINAL_SET_STATUS, {
        isReady: true
      })

      return sid;
  });
}

export function joinSession(siteId, sid){
  reactor.dispatch(TLPT_TERMINAL_SET_STATUS, {
    isLoading: true
  });

  $.when(
    api.get(cfg.getSiteSessionUrl(siteId, sid)),
    api.get(cfg.getSiteServersUrl(siteId)))
  .then((...responses) => {
      const [session, servers] = responses;
      const server = servers.find(s => s.id === session.server_id);

      reactor.dispatch(TLPT_TERMINAL_INIT, {
        sid,
        siteId,
        login: session.login,
        serverId: session.server_id,
        hostname: server.hostname
      })

      reactor.dispatch(TLPT_TERMINAL_SET_STATUS, {
        isReady: true
      })

    })
   .fail(err => {
      reactor.dispatch(TLPT_TERMINAL_SET_STATUS, {
        isError: true,
        errorText: err.message
      })
   })
}


