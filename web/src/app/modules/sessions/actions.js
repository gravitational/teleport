var reactor = require('app/reactor');
var api = require('app/services/api');
var cfg = require('app/config');

var { TLPT_SESSINS_RECEIVE, TLPT_SESSINS_UPDATE }  = require('./actionTypes');

export default {

  fetchSession(sid){
    return api.get(cfg.api.getFetchSessionUrl(sid)).then(json=>{
      if(json && json.session){
        reactor.dispatch(TLPT_SESSINS_UPDATE, json.session);
      }
    });
  },

  fetchSessions(){
    return api.get(cfg.api.getFetchSessionsUrl()).done((json) => {
      reactor.dispatch(TLPT_SESSINS_RECEIVE, json.sessions);
    });
  },

  updateSession(json){
    reactor.dispatch(TLPT_SESSINS_UPDATE, json);
  }  
}
