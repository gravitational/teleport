var reactor = require('app/reactor');
var { TLPT_NODES_RECEIVE }  = require('./actionTypes');
var api = require('app/services/api');
var cfg = require('app/config');
var {showError} = require('app/modules/notifications/actions');

const logger = require('app/common/logger').create('Modules/Nodes');

export default {
  fetchNodes(){
    api.get(cfg.api.nodesPath).done((data=[])=>{
      var nodeArray = data.nodes.map(item=>item.node);
      reactor.dispatch(TLPT_NODES_RECEIVE, nodeArray);
    }).fail((err)=>{      
      showError('Unable to retrieve list of nodes');
      logger.error('fetchNodes', err);
    })
  }
}
