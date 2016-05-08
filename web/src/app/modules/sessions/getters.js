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

var { toImmutable } = require('nuclear-js');
var reactor = require('app/reactor');
var cfg = require('app/config');

const sessionsByServer = (serverId) => [['tlpt_sessions'], (sessions) =>{
  return sessions.valueSeq().filter(item=>{
    var parties = item.get('parties') || toImmutable([]);
    var hasServer = parties.find(item2=> item2.get('server_id') === serverId);
    return hasServer;
  }).toList();
}]

const sessionsView = [['tlpt_sessions'], (sessions) =>{
  return sessions.valueSeq().map(createView).toJS();
}];

const sessionViewById = (sid)=> [['tlpt_sessions', sid], (session)=>{
  if(!session){
    return null;
  }

  return createView(session);
}];

const partiesBySessionId = (sid) =>
 [['tlpt_sessions', sid, 'parties'], (parties) =>{

  if(!parties){
    return [];
  }

  return parties.map(item=>{
    return {
      user: item.get('user'),
      serverIp: item.get('remote_addr'),
      serverId: item.get('server_id')
    }
  }).toJS();
}];

function createView(session){
  var sid = session.get('id');
  var serverIp;
  var parties = reactor.evaluate(partiesBySessionId(sid));

  if(parties.length > 0){
    serverIp = parties[0].serverIp;    
  }

  return {
    sid: sid,
    sessionUrl: cfg.getActiveSessionRouteUrl(sid),
    serverIp,
    serverId: session.get('server_id'),
    active: session.get('active'),
    created: session.get('created'),
    lastActive: session.get('last_active'),
    login: session.get('login'),
    parties: parties,
    cols: session.getIn(['terminal_params', 'w']),
    rows: session.getIn(['terminal_params', 'h'])
  }
}

export default {
  partiesBySessionId,
  sessionsByServer,
  sessionsView,
  sessionViewById,
  createView,
  count: [['tlpt_sessions'], sessions => sessions.size ]
}
