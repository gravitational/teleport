var reactor = require('app/reactor');
var { TLPT_RECEIVE_USER_INVITE }  = require('./actionTypes');
var { FETCHING_INVITE} = require('app/modules/restApi/constants');
var restApiActions = require('app/modules/restApi/actions');
var api = require('app/services/api');
var cfg = require('app/config');

export default {
  fetchInvite(inviteToken){
    var path = cfg.api.getInviteUrl(inviteToken);
    restApiActions.start(FETCHING_INVITE);
    api.get(path).done(invite=>{
      restApiActions.success(FETCHING_INVITE);
      reactor.dispatch(TLPT_RECEIVE_USER_INVITE, invite);
    }).
    fail((err)=>{
      restApiActions.fail(FETCHING_INVITE, err.responseJSON.message);
    });
  }
}
