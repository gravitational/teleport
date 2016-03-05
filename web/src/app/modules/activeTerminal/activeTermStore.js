var { Store, toImmutable } = require('nuclear-js');
var { TLPT_TERM_OPEN, TLPT_TERM_CLOSE }  = require('./actionTypes');

export default Store({
  getInitialState() {
    return toImmutable(null);
  },

  initialize() {
    this.on(TLPT_TERM_OPEN, setActiveTerminal);
    this.on(TLPT_TERM_CLOSE, close);
  }
})

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
