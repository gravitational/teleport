var { Store, toImmutable, Immutable } = require('nuclear-js');
var { TLPT_SESSINS_RECEIVE, TLPT_SESSINS_UPDATE }  = require('./actionTypes');

export default Store({
  getInitialState() {
    return new Immutable.OrderedMap();
  },

  initialize() {
    this.on(TLPT_SESSINS_RECEIVE, receiveSessions);
    this.on(TLPT_SESSINS_UPDATE, updateSession);
  }
})

function updateSession(state, json){
  return state.set(json.id, toImmutable(json));
}

function receiveSessions(state, jsonArray=[]){
  return state.withMutations(state => {
    jsonArray.forEach((item) => {
      item.created = new Date(item.created);
      item.last_active = new Date(item.last_active);
      state.set(item.id, toImmutable(item))
    })
  });
}
