var reactor = require('app/reactor');
var api = require('app/services/api');
var cfg = require('app/config');

var { TLPT_SESSINS_RECEIVE } = require('./../sessions/actionTypes');
var { TLPT_NODES_RECEIVE } = require('./../nodes/actionTypes');
var { TLPT_APP_INIT, TLPT_APP_FAILED, TLPT_APP_READY } = require('./actionTypes');

export default {

  initApp() {
    reactor.dispatch(TLPT_APP_INIT);
    module.exports.fetchNodesAndSessions()
      .done(()=>{
        reactor.dispatch(TLPT_APP_READY);
      })
      .fail(()=>{
        reactor.dispatch(TLPT_APP_FAILED);
      });
  },

  fetchNodesAndSessions() {
    return api.get(cfg.api.nodesPath).done(json => {
      var nodeArray = [];
      var sessions = {};

      json.nodes.forEach(item => {
        nodeArray.push(item.node);
        if (item.sessions) {
          item.sessions.forEach(item2 => {
            sessions[item2.id] = item2;
          })
        }
      });

      reactor.batch(() => {
        reactor.dispatch(TLPT_NODES_RECEIVE, nodeArray);
        reactor.dispatch(TLPT_SESSINS_RECEIVE, sessions);
      });

    });
  }
}
