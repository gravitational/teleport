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

  var lastActiveUsrName = getLastActiveUser(parties).get('user');

  return parties.map(item=>{
    var user = item.get('user');
    return {
      user: item.get('user'),
      serverIp: item.get('remote_addr'),
      serverId: item.get('server_id'),
      isActive: lastActiveUsrName === user
    }
  }).toJS();
}];

function getLastActiveUser(parties){
  return parties.sortBy(item=> new Date(item.get('lastActive'))).last();
}

function createView(session){
  var sid = session.get('id');
  var serverIp, serverId;
  var parties = reactor.evaluate(partiesBySessionId(sid));

  if(parties.length > 0){
    serverIp = parties[0].serverIp;
    serverId = parties[0].serverId;
  }

  return {
    sid: sid,
    sessionUrl: cfg.getActiveSessionRouteUrl(sid),
    serverIp,
    serverId,
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
  createView  
}
