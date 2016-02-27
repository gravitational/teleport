var reactor = require('app/reactor');
var {uuid} = require('app/utils');
var { TLPT_TERM_CONNECT, TLPT_TERM_CLOSE, TLPT_TERM_CONNECTED }  = require('./actionTypes');

export default {

  close(){
    reactor.dispatch(TLPT_TERM_CLOSE);
  },

  connected(){
    reactor.dispatch(TLPT_TERM_CONNECTED);
  },

  openSession(addr, login, sid=uuid()){
    /*
    *   {
    *   "addr": "127.0.0.1:5000",
    *   "login": "admin",
    *   "term": {"h": 120, "w": 100},
    *   "sid": "123"
    *  }
    */
    reactor.dispatch(TLPT_TERM_CONNECT, {addr, login, sid} );
  }
}
