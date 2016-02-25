var { Store, toImmutable } = require('nuclear-js');
var  { TLPT_RECEIVE_NODES } = require('./actionTypes');

export default Store({
  getInitialState() {
    return toImmutable({});
  },

  initialize() {
    this.on(TLPT_RECEIVE_NODES, receiveNodes)
  }
})

function receiveNodes(state, nodeArrayData){
  return state.withMutations(state => {
    nodeArrayData.forEach((item) => {
        state.set(item.id, toImmutable(item))
      })
   });
}
