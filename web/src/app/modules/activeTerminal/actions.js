var reactor = require('app/reactor');
var { TLPT_TERM_CONNECT, TLPT_TERM_CLOSE }  = require('./actionTypes');

export default {

  close(){
    reactor.dispatch(TLPT_TERM_CLOSE);
  },

  connect(addr, login){
    /*
    *   {
    *   "addr": "127.0.0.1:5000",
    *   "login": "admin",
    *   "term": {"h": 120, "w": 100},
    *   "sid": "123"
    *  }
    */
    reactor.dispatch(TLPT_TERM_CONNECT, {addr, login});
  }
}
