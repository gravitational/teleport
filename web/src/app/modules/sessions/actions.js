var reactor = require('app/reactor');
var { TLPT_SESSINS_RECEIVE, TLPT_SESSINS_UPDATE }  = require('./actionTypes');

export default {
  updateSession(json){
    reactor.dispatch(TLPT_SESSINS_UPDATE, json);
  },

  receive(json){
    reactor.dispatch(TLPT_SESSINS_RECEIVE, json);
  }
}
