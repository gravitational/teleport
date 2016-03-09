var { Store, toImmutable } = require('nuclear-js');
var { TLPT_TERM_OPEN, TLPT_TERM_CLOSE, TLPT_TERM_CHANGE_SERVER }  = require('./actionTypes');

export default Store({
  getInitialState() {
    return toImmutable(null);
  },

  initialize() {
    this.on(TLPT_TERM_OPEN, setActiveTerminal);
    this.on(TLPT_TERM_CLOSE, close);
    this.on(TLPT_TERM_CHANGE_SERVER, changeServer);
  }
})

function changeServer(state, {serverId, login}){
  return state.set('serverId', serverId)
              .set('login', login);
}

function close(){
  return toImmutable(null);
}

function setActiveTerminal(state, {serverId, login, sid, isNewSession} ){
  return toImmutable({
    serverId,
    login,
    sid,
    isNewSession
  });
}
