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
import { EventTypeEnum } from 'app/lib/term/enums';
import reactor from 'app/reactor';
import { parseIp } from 'app/lib/objectUtils';
import { getNodeStore } from './../nodes/nodeStore';

/*
** Getters
*/
const activeSessionList = [['tlpt_sessions_active'], ['tlpt', 'siteId'], (sessionList, siteId) => {  
  sessionList = sessionList.filter(n => n.get('siteId') === siteId);    
  return sessionList.valueSeq().map(createActiveListItem).toJS();
}];

const storedSessionList = [['tlpt_sessions_archived'], ['tlpt', 'siteId'], (sessionList, siteId) => {  
  sessionList = sessionList.filter(n => n.get('siteId') === siteId);    
  return sessionList.valueSeq().map(createStoredListItem).toJS();
}];

const nodeIpById = sid => ['tlpt_sessions_events', sid, EventTypeEnum.START, 'addr.local'];
const storedSessionById = sid => ['tlpt_sessions_archived', sid];
const activeSessionById = sid => ['tlpt_sessions_active', sid];
const activePartiesById = sid => [['tlpt_sessions_active', sid, 'parties'], parties => {  
  return parties ? parties.toJS() : [];  
}];

// creates a list of stored sessions which involves collecting the data from other stores
function createStoredListItem(session){
  const sid = session.get('id');      
  const { siteId, nodeIp, created, server_id, parties, last_active } = session;    
  const duration = moment(last_active).diff(created);
  const nodeDisplayText = getNodeIpDisplayText(siteId, server_id, nodeIp);
  const createdDisplayText = getCreatedDisplayText(created);
  const sessionUrl = cfg.getPlayerUrl({
    sid,
    siteId
  });
    
  return {
    active: false,
    parties: createParties(parties),
    sid,    
    duration,    
    siteId,                       
    sessionUrl,        
    created,    
    createdDisplayText,
    nodeDisplayText,
    lastActive: last_active    
  }
}

// creates a list of active sessions which involves collecting the data from other stores
function createActiveListItem(session){
  const sid = session.get('id');  
  const parties = createParties(session.parties);  
  const { siteId, created, login, last_active, server_id } = session;    
  const duration = moment(last_active).diff(created);
  const nodeIp = reactor.evaluate(nodeIpById(sid));
  const nodeDisplayText = getNodeIpDisplayText(siteId, server_id, nodeIp);
  const createdDisplayText = getCreatedDisplayText(created);    
  const sessionUrl = cfg.getTerminalLoginUrl({
    sid,
    siteId,
    login,
    serverId: server_id    
  });
    
  return {
    active: true,
    parties,    
    sid,
    duration,    
    siteId,     
    sessionUrl,
    created,    
    createdDisplayText,    
    nodeDisplayText,
    lastActive: last_active    
  }
}
  
function createParties(partyRecs) {
  const parties = partyRecs.toJS();
  return parties.map(p => {
      let ip = parseIp(p.serverIp);
      return `${p.user} [${ip}]`;
  });
}

function getCreatedDisplayText(date) {
  return moment(date).format(cfg.displayDateFormat);  
}

function getNodeIpDisplayText(siteId, serverId, serverIp) {    
  const server = getNodeStore().findServer(serverId);
  const ipAddress = parseIp(serverIp);

  let displayText = ipAddress;      
  if (server && server.hostname) {
    displayText = server.hostname;
    if (ipAddress) {
      displayText = `${displayText} [${ipAddress}]`;
    }      
  }
  
  return displayText;  
}

export default {
  storedSessionList,
  activeSessionList,
  activeSessionById,
  activePartiesById,
  storedSessionById,  
  createStoredListItem
}