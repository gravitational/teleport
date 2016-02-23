var api = require('./services/api');
var session = require('./session');
var $ = require('jQuery');

const refreshRate = 60000 * 1; // 1 min

var refreshTokenTimerId = null;

var auth = {

  login(email, password){
    auth._stopTokenRefresher();
    return auth._login(email, password).done(auth._startTokenRefresher);
  },

  ensureUser(){
    if(session.getUserData().user){
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

  _login(email, password){
    return api.login(email, password).then(data=>{
      session.setUserData(data);
      return data;
    });
  }
}

module.exports = auth;
