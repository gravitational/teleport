var reactor = require('app/reactor');
var session = require('app/session');
var uuid = require('app/common/uuid');
var api = require('app/services/api');
var cfg = require('app/config');
var getters = require('./getters');
var sessionModule = require('./../sessions');

const logger = require('app/common/logger').create('Current Session');
const { TLPT_TERM_OPEN, TLPT_TERM_CLOSE, TLPT_TERM_CHANGE_SERVER } = require('./actionTypes');

const actions = {

  changeServer(serverId, login){
    reactor.dispatch(TLPT_TERM_CHANGE_SERVER, {
      serverId,
      login
    });
  },

  close(){
    let {isNewSession} = reactor.evaluate(getters.activeSession);

    reactor.dispatch(TLPT_TERM_CLOSE);

    if(isNewSession){
      session.getHistory().push(cfg.routes.nodes);
    }else{
      session.getHistory().push(cfg.routes.sessions);
    }
  },

  resize(w, h){
    // some min values
    w = w < 5 ? 5 : w;
    h = h < 5 ? 5 : h;

    let reqData = { terminal_params: { w, h } };
    let {sid} = reactor.evaluate(getters.activeSession);

    logger.info('resize', `w:${w} and h:${h}`);
    api.put(cfg.api.getTerminalSessionUrl(sid), reqData)
      .done(()=> logger.info('resized'))
      .fail((err)=> logger.error('failed to resize', err));
  },

  openSession(sid){
    logger.info('attempt to open session', {sid});
    sessionModule.actions.fetchSession(sid)
      .done(()=>{
        let sView = reactor.evaluate(sessionModule.getters.sessionViewById(sid));
        let { serverId, login } = sView;
        logger.info('open session', 'OK');
        reactor.dispatch(TLPT_TERM_OPEN, {
            serverId,
            login,
            sid,
            isNewSession: false
          });
      })
      .fail((err)=>{
        logger.error('open session', err);
        session.getHistory().push(cfg.routes.pageNotFound);
      })
  },

  createNewSession(serverId, login){
    var sid = uuid();
    var routeUrl = cfg.getActiveSessionRouteUrl(sid);
    var history = session.getHistory();

    logger.info('createNewSession', {serverId, login});
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
