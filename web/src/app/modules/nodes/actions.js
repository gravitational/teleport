var reactor = require('app/reactor');
var { TLPT_RECEIVE_NODES }  = require('./actionTypes');
var api = require('app/services/api');
var cfg = require('app/config');

export default {
  fetchNodes(){
    api.get(cfg.api.nodesPath).done(data=>{
      reactor.dispatch(TLPT_RECEIVE_NODES, data.nodes);
    });
  }
}
