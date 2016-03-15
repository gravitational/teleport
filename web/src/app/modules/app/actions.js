var reactor = require('app/reactor');
var {fetchSessions} = require('./../sessions/actions');
var {fetchNodes} = require('./../nodes/actions');
var {monthRange} = require('app/common/dateUtils');
var $ = require('jQuery');

const { TLPT_APP_INIT, TLPT_APP_FAILED, TLPT_APP_READY } = require('./actionTypes');

const actions = {

  initApp() {
    reactor.dispatch(TLPT_APP_INIT);
    actions.fetchNodesAndSessions()
      .done(()=> reactor.dispatch(TLPT_APP_READY) )
      .fail(()=> reactor.dispatch(TLPT_APP_FAILED) );
  },

  fetchNodesAndSessions() {
    var [start, end ] = monthRange();
    return $.when(fetchNodes(), fetchSessions(start, end));
  }
}

export default actions;
