var reactor = require('app/reactor');
var { TLPT_RECEIVE_USER_INVITE }  = require('./actionTypes');
var api = require('app/services/api');
var cfg = require('app/config');

export default {
  fetchInvite(inviteToken){
    var path = cfg.api.getInviteUrl(inviteToken);
    api.get(path).done(invite=>{
      reactor.dispatch(TLPT_RECEIVE_USER_INVITE, invite);
    });
  }
}
