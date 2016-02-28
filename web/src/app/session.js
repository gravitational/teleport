var { browserHistory, createMemoryHistory } = require('react-router');

const AUTH_KEY_DATA = 'authData';

var _history = createMemoryHistory();

var session = {

  init(history=browserHistory){
    _history = history;
  },

  getHistory(){
    return _history;
  },

  setUserData(userData){
    localStorage.setItem(AUTH_KEY_DATA, JSON.stringify(userData));
  },

  getUserData(){
    var item = localStorage.getItem(AUTH_KEY_DATA);
    if(item){
      return JSON.parse(item);
    }

    return {};
  },

  clear(){
    localStorage.clear()
  }

}

module.exports = session;
