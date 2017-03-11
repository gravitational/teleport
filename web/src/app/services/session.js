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
const EMPTY_TOKEN_CONTENT_LENGTH = 20;
const logger = require('app/lib/logger').create('services/sessions');
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
    let userData = null;
    try{
      // first check if user data (with barer token) is embedded in HTML
      userData = this._getUserDataFromHtml();

      // then lookup in the browser local storage
      if(!userData){
        userData = this._getUserDataFromLocalStorage();
      }

    }catch(err){
      logger.error('Cannot retrieve user data', err);
    }

    return userData || {};
  },

  clear(){
    localStorage.clear()
  },

  _getUserDataFromHtml(){
    let $el = $('#bearer_token');
    let userData = null;
    if($el.length !== 0){
      let encodedToken = $el.text() || '';
      if(encodedToken.length > EMPTY_TOKEN_CONTENT_LENGTH){
        let decoded = window.atob(encodedToken);
        let json = JSON.parse(decoded);
        userData = this.setUserData(json);
      }

      // remove initial data from HTML as it will be renewed with a time
      $el.remove();
    }

    return userData;
  },

  _getUserDataFromLocalStorage(){
    let item = localStorage.getItem(AUTH_KEY_DATA);
    if(item){
      return JSON.parse(item);
    }

    return null;
  }
}

module.exports = session;
