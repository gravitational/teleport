var { Store, toImmutable } = require('nuclear-js');
var { TLPT_TERM_OPEN, TLPT_TERM_CLOSE, TLPT_TERM_CONNECTED, TLPT_TERM_RECEIVE_PARTIES }  = require('./actionTypes');

export default Store({
  getInitialState() {
    return toImmutable(null);
  },

  initialize() {
    this.on(TLPT_TERM_CONNECTED, connected);
    this.on(TLPT_TERM_OPEN, setActiveTerminal);
    this.on(TLPT_TERM_CLOSE, close);
    this.on(TLPT_TERM_RECEIVE_PARTIES, receiveParties);
  }

})

function close(){
  return toImmutable(null);
}

function receiveParties(state, parties){
  return state.set('parties', toImmutable(parties));
}

function setActiveTerminal(state, settings){
  return toImmutable({      
      isConnecting: true,
      ...settings
  });
}

function connected(state){
  return state.set('isConnected', true)
              .set('isConnecting', false);
}
