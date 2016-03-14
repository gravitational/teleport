var reactor = require('app/reactor');
var { TLPT_RECEIVE_USER } = require('./actionTypes');
var { TRYING_TO_SIGN_UP, TRYING_TO_LOGIN} = require('app/modules/restApi/constants');
var restApiActions = require('app/modules/restApi/actions');
var auth = require('app/auth');
var session = require('app/session');
var cfg = require('app/config');

export default {

  ensureUser(nextState, replace, cb){
    auth.ensureUser()
      .done((userData)=> {
        reactor.dispatch(TLPT_RECEIVE_USER, userData.user );
        cb();
      })
      .fail(()=>{
        replace({redirectTo: nextState.location.pathname }, cfg.routes.login);
        cb();
      });
  },

  signUp({name, psw, token, inviteToken}){
    restApiActions.start(TRYING_TO_SIGN_UP);
    auth.signUp(name, psw, token, inviteToken)
      .done((sessionData)=>{
        reactor.dispatch(TLPT_RECEIVE_USER, sessionData.user);
        restApiActions.success(TRYING_TO_SIGN_UP);
        session.getHistory().push({pathname: cfg.routes.app});
      })
      .fail((err)=>{
        restApiActions.fail(TRYING_TO_SIGN_UP, err.responseJSON.message || 'failed to sing up');
      });
  },

  login({user, password, token}, redirect){
    restApiActions.start(TRYING_TO_LOGIN);
    auth.login(user, password, token)
      .done((sessionData)=>{
        restApiActions.success(TRYING_TO_LOGIN);
        reactor.dispatch(TLPT_RECEIVE_USER, sessionData.user);
        session.getHistory().push({pathname: redirect});
      })
      .fail((err)=> restApiActions.fail(TRYING_TO_LOGIN, err.responseJSON.message))
    }
}
