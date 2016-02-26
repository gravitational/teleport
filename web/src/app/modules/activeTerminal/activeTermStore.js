var { Store, toImmutable } = require('nuclear-js');
var { TLPT_TERM_CONNECT, TLPT_TERM_CLOSE }  = require('./actionTypes');

export default Store({
  getInitialState() {
    return toImmutable(null);
  },

  initialize() {
    this.on(TLPT_TERM_CONNECT, connect);
    this.on(TLPT_TERM_CLOSE, close);
  }

})

function close(){
  return toImmutable(null);
}

function connect(state, term){
  return toImmutable(term);
}
