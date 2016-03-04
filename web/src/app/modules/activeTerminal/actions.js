var reactor = require('app/reactor');
var session = require('app/session');
var {uuid} = require('app/utils');
var api = require('app/services/api');
var cfg = require('app/config');
var getters = require('./getters');
var sessionModule = require('./../sessions');

var { TLPT_TERM_OPEN, TLPT_TERM_CLOSE } = require('./actionTypes');

var actions = {

  close(){
    reactor.dispatch(TLPT_TERM_CLOSE);
    session.getHistory().push(cfg.routes.sessions);
  },

  resize(w, h){
    // some min values
    w = w < 5 ? 5 : w;
    h = h < 5 ? 5 : h;

    let reqData = { terminal_params: { w, h } };
    let {sid} = reactor.evaluate(getters.activeSession);

    api.put(cfg.api.getTerminalSessionUrl(sid), reqData)
      .done(()=>{
        console.log(`resize with w:${w} and h:${h} - OK`);
      })
      .fail(()=>{
        console.log(`failed to resize with w:${w} and h:${h}`);
    })
  },

  openSession(sid){
    sessionModule.actions.fetchSession(sid)
      .done(()=>{
        let sView = reactor.evaluate(sessionModule.getters.sessionViewById(sid));
        let { serverId, login } = sView;
        reactor.dispatch(TLPT_TERM_OPEN, {
            serverId,
            login,
            sid,
            isNewSession: false
          });
      })
      .fail(()=>{
        session.getHistory().push(cfg.routes.pageNotFound);
      })
  },

  createNewSession(serverId, login){
    var sid = uuid();
    var routeUrl = cfg.getActiveSessionRouteUrl(sid);
    var history = session.getHistory();

    reactor.dispatch(TLPT_TERM_OPEN, {
      serverId,
      login,
      sid,
      isNewSession: true
    });

    history.push(routeUrl);
  }

}

export default actions;
