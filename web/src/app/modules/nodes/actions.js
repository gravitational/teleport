var reactor = require('app/reactor');
var { TLPT_RECEIVE_NODES }  = require('./actionTypes');
var api = require('app/services/api');
var cfg = require('app/config');

export default {
  fetchNodes(){
    api.get(cfg.api.nodes).done(nodes=>{
      reactor.dispatch(TLPT_RECEIVE_NODES, nodes);
    });
  }
}
