var { toImmutable } = require('nuclear-js');
var reactor = require('app/reactor');

const sessionsByServer = (addr) => [['tlpt_sessions'], (sessions) =>{
  return sessions.valueSeq().filter(item=>{
    var parties = item.get('parties') || toImmutable([]);
    var hasServer = parties.find(item2=> item2.get('server_addr') === addr);
    return hasServer;
  }).toList();
}]

const sessionsView = [['tlpt_sessions'], (sessions) =>{
  return sessions.valueSeq().map(item=>{
    var sid = item.get('id');
    var parties = reactor.evaluate(partiesBySessionId(sid));
    return {
      sid: sid,
      addr: parties[0].addr,
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
      addr: item.get('server_addr'),
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
