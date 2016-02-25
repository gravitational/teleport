var { Store, toImmutable } = require('nuclear-js');
var  { TLPT_RECEIVE_USER } = require('./actionTypes');

export default Store({
  getInitialState() {
    return toImmutable(null);
  },

  initialize() {
    this.on(TLPT_RECEIVE_USER, receiveUser)
  }

})

function receiveUser(state, user){
  return toImmutable(user);
}
