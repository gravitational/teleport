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

import moment from 'moment';
import cfg from 'app/config';
import { EventTypeEnum } from 'app/services/enums';
import reactor from 'app/reactor';

const activeSessions = [['tlpt_sessions_active'], ['tlpt', 'siteId'], (sessionList, siteId) => {  
  sessionList = sessionList.filter(n => n.get('siteId') === siteId);    
  return sessionList.valueSeq().map(createActiveListItem).toJS();
}];

const storedSessions = [['tlpt_sessions_archived'], ['tlpt', 'siteId'], (sessionList, siteId) => {  
  sessionList = sessionList.filter(n => n.get('siteId') === siteId);    
  return sessionList.valueSeq().map(createStoredListItem).toJS();
}];

const activeSessionById = sid => ['tlpt_sessions_active', sid];
const storedSessionById = sid => ['tlpt_sessions_archived', sid];
const nodeIpById = sid => ['tlpt_sessions_events', sid, EventTypeEnum.START, 'addr.local'];

function createStoredListItem(session){
  let sid = session.get('id');      
  let { siteId, nodeIp, created, server_id, parties, last_active } = session;    
  let duration = moment(last_active).diff(created);

  let sessionUrl = cfg.getCurrentSessionRouteUrl({
    sid,
    siteId
  });
    
  return {
    active: false,
    parties: parties.toJS(),
    sid,
    created,    
    duration,    
    siteId,                       
    sessionUrl,
    serverId: server_id,
    nodeIp: nodeIp,
    lastActive: last_active    
  }
}

function createActiveListItem(session){
  let sid = session.get('id');  
  let parties = session.parties.toJS();
  
  let { siteId, created, last_active, server_id } = session;    
  let duration = moment(last_active).diff(created);
  let nodeIp = reactor.evaluate(nodeIpById(sid));
    
  let sessionUrl = cfg.getCurrentSessionRouteUrl({
    sid,
    siteId
  });
    
  return {
    active: true,
    parties,
    serverId: server_id,        
    sid,
    created,    
    duration,    
    siteId,     
    sessionUrl,
    nodeIp: nodeIp,
    lastActive: last_active    
  }
}
  
export default {
  storedSessions,
  activeSessions,
  activeSessionById,
  storedSessionById,  
  createStoredListItem
}
