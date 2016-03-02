var { Store, toImmutable } = require('nuclear-js');
var  { TLPT_NODES_RECEIVE } = require('./actionTypes');

export default Store({
  getInitialState() {
    return toImmutable([]);
  },

  initialize() {
    this.on(TLPT_NODES_RECEIVE, receiveNodes)
  }
})

function receiveNodes(state, nodeArray){
  return toImmutable(nodeArray);
}
