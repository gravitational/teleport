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

import moment from 'moment';
import api from 'gravity/services/api';
import cfg from 'gravity/config';
import { map } from 'lodash';

export function fetchSessions(){
  return api.get(cfg.getSiteSshSessionUrl())
    .then(response => {
      if(response && response.sessions){
        return map(response.sessions, makeSession);
      }

      return [];
    })
}

function makeSession(json){
  const parties = map(json.parties, makeParticipant);
  const sid = json.id;
  const namespace = json.namespace;
  const login = json.login;
  const active = json.active;
  const created = new Date(json.created);
  const serverId = json.server_id;
  const siteId = json.siteId;
  const duration = moment(new Date()).diff(created);
  const durationText = moment.duration(duration).humanize();
  return {
    sid,
    namespace,
    login,
    active,
    created,
    durationText,
    serverId,
    siteId,
    parties
  }
}

function makeParticipant(json) {
  const remoteAddr = json.remote_addr || '';
  return {
    user: json.user,
    remoteAddr: remoteAddr.replace(PORT_REGEX, ''),
  }
}

const PORT_REGEX = /:\d+$/;
