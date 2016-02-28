var { toImmutable } = require('nuclear-js');

const sessionsByServer = (addr) => [['tlpt_sessions'], (sessions) =>{
  return sessions.valueSeq().filter(item=>{
    var parties = item.get('parties') || toImmutable([]);
    var hasServer = parties.find(item2=> item2.get('server_addr') === addr);
    return hasServer;
  }).toList();
}]

const partiesBySessionId = (sid) =>
 [['tlpt_sessions', sid, 'parties'], (parties) =>{

  if(!parties){
    return [];
  }

  var lastActiveUsrName = getLastActiveUser(parties);

  return parties.map(item=>{
    var user = item.get('user');
    return {
      user: item.get('user'),
      isActive: lastActiveUsrName === user
    }
  }).toJS();
}];

function getLastActiveUser(parties){
  return parties.sortBy(item=> new Date(item.get('lastActive'))).first().get('user');
}

export default {
  partiesBySessionId,
  sessionsByServer
}
