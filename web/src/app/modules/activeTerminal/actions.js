var reactor = require('app/reactor');
var {uuid} = require('app/utils');
var api = require('app/services/api');
var cfg = require('app/config');
var invariant = require('invariant');
var getters = require('./getters');

var { TLPT_TERM_OPEN, TLPT_TERM_CLOSE, TLPT_TERM_CONNECTED, TLPT_TERM_RECEIVE_PARTIES }  = require('./actionTypes');

export default {

  close(){
    reactor.dispatch(TLPT_TERM_CLOSE);
  },

  resize(w, h){
    invariant(w > 5 || h > 5, 'invalid resize parameters');
    let reqData = { terminal_params: { w, h } };
    let {sid} = reactor.evaluate(getters.activeSession);

    api.put(cfg.api.getTerminalSessionUrl(sid), reqData).done(()=>{
      console.log(`resize with ${w} and ${h} - OK`);
    }).fail(()=>{
      console.log(`failed to resize with ${w} and ${h}`);
    })
  },

  connected(){
    reactor.dispatch(TLPT_TERM_CONNECTED);
  },

  receiveParties(json){
    var parties = json.map(item=>{
      return {
        user: item.user,
        lastActive: new Date(item.last_active)
      }
    })

    reactor.dispatch(TLPT_TERM_RECEIVE_PARTIES, parties);
  },

  open(addr, login, sid){
    let isNew = !sid;
    if(isNew){
      sid = uuid();
    }

    reactor.dispatch(TLPT_TERM_OPEN, {addr, login, sid, isNew} );
  }
}
