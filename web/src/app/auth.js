var api = require('./services/api');
var session = require('./session');
var cfg = require('app/config');
var $ = require('jQuery');

const refreshRate = 60000 * 1; // 1 min

var refreshTokenTimerId = null;

var auth = {

  signUp(name, password, token, inviteToken){
    var data = {user: name, pass: password, second_factor_token: token, invite_token: inviteToken};
    return api.post(cfg.api.createUserPath, data)
      .then((user)=>{
        session.setUserData(user);
        auth._startTokenRefresher();
        return user;
      });
  },

  login(name, password, token){
    auth._stopTokenRefresher();
    return auth._login(name, password, token).done(auth._startTokenRefresher);
  },

  ensureUser(){
    if(session.getUserData()){
      // refresh timer will not be set in case of browser refresh event
      if(auth._getRefreshTokenTimerId() === null){
        return auth._login().done(auth._startTokenRefresher);
      }

      return $.Deferred().resolve();
    }

    return $.Deferred().reject();
  },

  logout(){
    auth._stopTokenRefresher();
    return session.clear();
  },

  _startTokenRefresher(){
    refreshTokenTimerId = setInterval(auth._refreshToken, refreshRate);
  },

  _stopTokenRefresher(){
    clearInterval(refreshTokenTimerId);
    refreshTokenTimerId = null;
  },

  _getRefreshTokenTimerId(){
    return refreshTokenTimerId;
  },

  _refreshToken(){
    auth._login().fail(()=>{
      auth.logout();
      window.location.reload();
    })
  },

  _login(name, password, token){
    var data = {
      user: name,
      pass: password,
      second_factor_token: token
    };

    return api.post(cfg.api.sessionPath, data).then(data=>{
      session.setUserData(data);
      return data;
    });

  }
}

module.exports = auth;
