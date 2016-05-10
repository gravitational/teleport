/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

var { browserHistory, createMemoryHistory } = require('react-router');
var $ = require('jQuery');

const logger = require('app/common/logger').create('services/sessions');
const AUTH_KEY_DATA = 'authData';

var _history = createMemoryHistory();

var UserData = function(json){
  $.extend(this, json);
  this.created = new Date().getTime();
}

var session = {

  init(history=browserHistory){
    _history = history;
  },

  getHistory(){
    return _history;
  },

  setUserData(data){
    var userData = new UserData(data);
    localStorage.setItem(AUTH_KEY_DATA, JSON.stringify(userData));
    return userData;
  },

  getUserData(){
    try{
      var item = localStorage.getItem(AUTH_KEY_DATA);
      if(item){
        return JSON.parse(item);
      }

      // for sso use-cases, try to grab the token from HTML
      var hiddenDiv = document.getElementById("bearer_token");
      if(hiddenDiv !== null ){
          let json = window.atob(hiddenDiv.textContent);
          let data = JSON.parse(json);
          if(data.token){
            // put it into the session
            var userData = this.setUserData(data);
            // remove the element
            hiddenDiv.remove();
            return userData;
          }
      }
    }catch(err){
      logger.error('error trying to read user auth data:', err);
    }

    return {};
  },

  clear(){
    localStorage.clear()
  }

}

module.exports = session;
