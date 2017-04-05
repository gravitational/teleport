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
import { nodeHostNameByServerId } from 'app/flux/nodes/getters';
import { parseIp } from 'app/lib/objectUtils';

const activeSessionList = [['tlpt_sessions_active'], ['tlpt', 'siteId'], (sessionList, siteId) => {  
  sessionList = sessionList.filter(n => n.get('siteId') === siteId);    
  return sessionList.valueSeq().map(createActiveListItem).toJS();
}];

const storedSessionList = [['tlpt_sessions_archived'], ['tlpt', 'siteId'], (sessionList, siteId) => {  
  sessionList = sessionList.filter(n => n.get('siteId') === siteId);    
  return sessionList.valueSeq().map(createStoredListItem).toJS();
}];

const activeSessionById = sid => ['tlpt_sessions_active', sid];
const storedSessionById = sid => ['tlpt_sessions_archived', sid];


const activePartiesById = sid => [['tlpt_sessions_active', sid, 'parties'], parties => {
  if (parties) {
    return parties.toJS();
  }

  return [];
}];

const nodeIpById = sid => ['tlpt_sessions_events', sid, EventTypeEnum.START, 'addr.local'];

function createStoredListItem(session){
  let sid = session.get('id');      
  let { siteId, nodeIp, created, server_id, parties, last_active } = session;    
  let duration = moment(last_active).diff(created);
  let nodeDisplayText = getNodeIpDisplayText(server_id, nodeIp);
  let createdDisplayText = getCreatedDisplayText(created);

  let sessionUrl = cfg.getPlayerUrl({
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

function createActiveListItem(session){
  let sid = session.get('id');  
  let parties = createParties(session.parties);
  
  let { siteId, created, login, last_active, server_id } = session;    
  let duration = moment(last_active).diff(created);
  let nodeIp = reactor.evaluate(nodeIpById(sid));
  let nodeDisplayText = getNodeIpDisplayText(server_id, nodeIp);
  let createdDisplayText = getCreatedDisplayText(created);
    
  let sessionUrl = cfg.getTerminalLoginUrl({
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
  let parties = partyRecs.toJS();
  return parties.map(p => {
      let ip = parseIp(p.serverIp);
      return `${p.user} [${ip}]`;
  });
}

function getCreatedDisplayText(date) {
  return moment(date).format(cfg.displayDateFormat);  
}

function getNodeIpDisplayText(serverId, serverIp) {
  let hostname = reactor.evaluate(nodeHostNameByServerId(serverId));
  let ipAddress = parseIp(serverIp);
  let displayText = ipAddress;
    
  if (hostname) {
    displayText = `${hostname}`;
    if (ipAddress) {
      displayText = `${hostname} [${ipAddress}]`;
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