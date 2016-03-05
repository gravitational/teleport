var reactor = require('app/reactor');
var {fetchSessions} = require('./../sessions/actions');
var {fetchNodes } = require('./../nodes/actions');
var $ = require('jQuery');

var { TLPT_APP_INIT, TLPT_APP_FAILED, TLPT_APP_READY } = require('./actionTypes');

var actions = {

  initApp() {
    reactor.dispatch(TLPT_APP_INIT);
    actions.fetchNodesAndSessions()
      .done(()=>{ reactor.dispatch(TLPT_APP_READY); })
      .fail(()=>{ reactor.dispatch(TLPT_APP_FAILED); });

    //api.get(`/v1/webapi/sites/-current-/sessions/03d3e11d-45c1-4049-bceb-b233605666e4/chunks?start=0&end=100`).done(() => {
    //});
  },

  fetchNodesAndSessions() {
    return $.when(fetchNodes(), fetchSessions());
  }
}

export default actions;
