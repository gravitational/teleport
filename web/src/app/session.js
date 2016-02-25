var { browserHistory } = require('react-router');

const AUTH_KEY_DATA = 'authData';

var _history = null;

var session = {

  init(history=browserHistory){
    _history = history;
  },

  getHistory(){
    return _history;
  },

  setUserData(userData){
    sessionStorage.setItem(AUTH_KEY_DATA, JSON.stringify(userData));
  },

  getUserData(){
    var item = sessionStorage.getItem(AUTH_KEY_DATA);
    if(item){
      return JSON.parse(item);
    }

    return {};
  },

  clear(){
    sessionStorage.clear()
  }

}

module.exports = session;
