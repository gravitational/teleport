var { Store, toImmutable } = require('nuclear-js');
var  { TLPT_RECEIVE_NODES } = require('./actionTypes');

export default Store({
  getInitialState() {
    return toImmutable([]);
  },

  initialize() {
    this.on(TLPT_RECEIVE_NODES, receiveNodes)
  }
})

function receiveNodes(state, nodeArray){
  return toImmutable(nodeArray);
}
