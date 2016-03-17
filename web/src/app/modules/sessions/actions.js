var reactor = require('app/reactor');
var api = require('app/services/api');
var cfg = require('app/config');
var {showError} = require('app/modules/notifications/actions');

const logger = require('app/common/logger').create('Modules/Sessions');
const { TLPT_SESSINS_RECEIVE, TLPT_SESSINS_UPDATE }  = require('./actionTypes');

const actions = {

  fetchSession(sid){
    return api.get(cfg.api.getFetchSessionUrl(sid)).then(json=>{
      if(json && json.session){
        reactor.dispatch(TLPT_SESSINS_UPDATE, json.session);
      }
    });
  },

  fetchSessions({before, sid, limit=cfg.maxSessionLoadSize}){
    let start = before || new Date();
    let params = {
      order: -1,
      limit
    };

    params.start = start.toISOString();

    if(sid){
      params.sessionID = sid;
      params.sessionId = sid;
      params.sid = sid;
    }

    return api.get(cfg.api.getFetchSessionsUrl(params))
      .done((json) => {
        reactor.dispatch(TLPT_SESSINS_RECEIVE, json.sessions);
      })
      .fail((err)=>{
        showError('Unable to retrieve list of sessions');
        logger.error('fetchSessions', err);
      });
  },

  updateSession(json){
    reactor.dispatch(TLPT_SESSINS_UPDATE, json);
  }
}

export default actions;
