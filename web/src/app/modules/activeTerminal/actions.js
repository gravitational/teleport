var reactor = require('app/reactor');
var {uuid} = require('app/utils');
var { TLPT_TERM_OPEN, TLPT_TERM_CLOSE, TLPT_TERM_CONNECTED, TLPT_TERM_RECEIVE_PARTIES }  = require('./actionTypes');

export default {

  close(){
    reactor.dispatch(TLPT_TERM_CLOSE);
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

  open(addr, login, sid=uuid()){
    /*
    *   {
    *   "addr": "127.0.0.1:5000",
    *   "login": "admin",
    *   "term": {"h": 120, "w": 100},
    *   "sid": "123"
    *  }
    */
    reactor.dispatch(TLPT_TERM_OPEN, {addr, login, sid} );
  }
}
