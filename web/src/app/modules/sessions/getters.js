var { toImmutable } = require('nuclear-js');
var reactor = require('app/reactor');

const sessionsByServer = (serverId) => [['tlpt_sessions'], (sessions) =>{
  return sessions.valueSeq().filter(item=>{
    var parties = item.get('parties') || toImmutable([]);
    var hasServer = parties.find(item2=> item2.get('server_id') === serverId);
    return hasServer;
  }).toList();
}]

const sessionsView = [['tlpt_sessions'], (sessions) =>{
  return sessions.valueSeq().map(item=>{
    var sid = item.get('id');
    var parties = reactor.evaluate(partiesBySessionId(sid));
    return {
      sid: sid,
      serverIp: parties[0].serverIp,
      serverId: parties[0].serverId,
      login: item.get('login'),
      parties: parties
    }
  }).toJS();
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
  return parties.sortBy(item=> new Date(item.get('lastActive'))).first();
}

export default {
  partiesBySessionId,
  sessionsByServer,
  sessionsView
}
