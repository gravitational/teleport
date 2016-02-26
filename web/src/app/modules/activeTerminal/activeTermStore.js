var { Store, toImmutable } = require('nuclear-js');
var { TLPT_TERM_CONNECT, TLPT_TERM_CLOSE, TLPT_TERM_CONNECTED }  = require('./actionTypes');

export default Store({
  getInitialState() {
    return toImmutable(null);
  },

  initialize() {
    this.on(TLPT_TERM_CONNECTED, connected);
    this.on(TLPT_TERM_CONNECT, connect);
    this.on(TLPT_TERM_CLOSE, close);
  }

})

function close(){
  return toImmutable(null);
}

function connect(state, term){
  return toImmutable({
      isConnecting: true,
      term
  });
}

function connected(state){
  return state.set('isConnected', true)
              .set('isConnecting', false);
}
