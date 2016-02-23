var createHistory = require('history/lib/createBrowserHistory');

const AUTH_KEY_DATA = 'authData';

var _history = null;

var session = {

  init(){
    _history = createHistory();
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
