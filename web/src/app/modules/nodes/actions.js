var reactor = require('app/reactor');
var { TLPT_NODES_RECEIVE }  = require('./actionTypes');
var api = require('app/services/api');
var cfg = require('app/config');
var {showError} = require('app/modules/notifications/actions');
var appGetters = require('app/modules/app/getters')

const logger = require('app/common/logger').create('Modules/Nodes');

export default {
  fetchNodes() {
    let siteId = reactor.evaluate(appGetters.siteId);
    return api.get(cfg.api.getSiteNodesUrl(siteId)).done((data = []) => {
      let nodeArray = data.nodes
        .map(item => item.node);
      
      nodeArray.forEach(item => item.siteId = siteId)            
      reactor.dispatch(TLPT_NODES_RECEIVE, { siteId, nodeArray });

    }).fail((err)=>{
      showError('Unable to retrieve list of nodes');
      logger.error('fetchNodes', err);
    })
  }
}