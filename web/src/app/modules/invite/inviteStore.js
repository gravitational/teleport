var { Store, toImmutable } = require('nuclear-js');
var  { TLPT_RECEIVE_USER_INVITE } = require('./actionTypes');

export default Store({
  getInitialState() {
    return toImmutable(null);
  },

  initialize() {
    this.on(TLPT_RECEIVE_USER_INVITE, receiveInvite)
  }
})

function receiveInvite(state, invite){
  return toImmutable(invite);
}
