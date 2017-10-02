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

import Logger from 'app/lib/logger';
import cfg from 'app/config';
import $ from 'jQuery';
import history from './history';
import localStorage, { KeysEnum } from './localStorage';
import api from './api';

const EMPTY_TOKEN_CONTENT_LENGTH = 20;
const TOKEN_CHECKER_INTERVAL = 15 * 1000; //  every 15 sec
const logger = Logger.create('services/sessions');

export class BearerToken {  
  constructor(json){
    this.accessToken = json.token;
    this.expiresIn = json.expires_in;  
    this.created = new Date().getTime();
  }
}

let sesstionCheckerTimerId = null;

const session = {

  logout() {        
    api.delete(cfg.api.sessionPath).always(() => {
      history.push(cfg.routes.login, true);
    });
    
    this.clear();    
  },

  clear(){
    this._stopSessionChecker();        
    localStorage.unsubscribe(receiveMessage)    
    localStorage.setBearerToken(null);
    localStorage.clear();
  },
  
  ensureSession(){
    this._stopSessionChecker();
    this._ensureLocalStorageSubscription();

    const token = this._getBearerToken();
    if(!token){
      return $.Deferred().reject();
    }

    if(this._shouldRenewToken()){
      return this._renewToken().done(this._startSessionChecker.bind(this));
    }

    this._startSessionChecker();
    return $.Deferred().resolve(token)
  },
  
  _getBearerToken(){
    let token = null;
    try{      
      token = this._extractBearerTokenFromHtml();
      if (token) {
        localStorage.setBearerToken(token)
      } else {                          
        token = localStorage.getBearerToken();
      }

    }catch(err){
      logger.error('Cannot find bearer token', err);
    }

    return token;
  },

  _extractBearerTokenFromHtml() {
    const el = document.querySelector('[name=grv_bearer_token]');        
    let token = null;
    if (el !== null) {
      let encodedToken = el.content || '';      
      if (encodedToken.length > EMPTY_TOKEN_CONTENT_LENGTH) {
        let decoded = window.atob(encodedToken);
        let json = JSON.parse(decoded);
        token = new BearerToken(json);
      }

      // remove initial data from HTML as it will be renewed with a time
      el.parentNode.removeChild(el);    
    }

    return token;
  },

  _shouldRenewToken(){
    if(this._getIsRenewing()){
      return false;
    }

    return this._timeLeft() < TOKEN_CHECKER_INTERVAL * 1.5;    
  },

  _shouldCheckStatus(){
    if(this._getIsRenewing()){
      return false;
    }
    
    /* 
    * double the threshold value for slow connections to avoid 
    * access-denied response due to concurrent renew token request 
    * made from other tab
    */
    return this._timeLeft() > TOKEN_CHECKER_INTERVAL * 2;
  },

  _renewToken(){        
    this._setAndBroadcastIsRenewing(true);        
    return api.post(cfg.api.renewTokenPath)
      .then(this._receiveBearerToken.bind(this))
      .fail(this.logout.bind(this))
      .always(()=>{        
        this._setAndBroadcastIsRenewing(false);        
      })
  },

  _receiveBearerToken(json){
    const token = new BearerToken(json);
    localStorage.setBearerToken(token);        
  },

  _fetchStatus(){                
    api.get(cfg.api.userStatusPath)    
    .fail( err => {    
      // indicates that session is no longer valid (caused by server restarts or updates)
      if(err.status == 403){
        this.logout();
      }
    });
  },

  _setAndBroadcastIsRenewing(value){
    this._setIsRenewing(value);
    localStorage.broadcast(KeysEnum.TOKEN_RENEW, value);        
  },

  _setIsRenewing(value){    
    this._isRenewing = value;     
  },

  _getIsRenewing(){
    return !!this._isRenewing;
  },

  _timeLeft(){
    const token = this._getBearerToken();
    if (!token) {
      return 0;
    }
    
    let { expiresIn, created } = token;
    if(!created || !expiresIn){     
      return 0;
    }

    expiresIn = expiresIn * 1000;
    let delta = created + expiresIn - new Date().getTime();
    return delta;          
  },
    
  // detects localStorage changes from other tabs
  _ensureLocalStorageSubscription(){
    localStorage.subscribe(receiveMessage);
  },
  
  _startSessionChecker(){
    this._stopSessionChecker();
    sesstionCheckerTimerId = setInterval(()=> {          
      // calling ensureSession() will again invoke _startSessionChecker              
      this.ensureSession(); 

      // check if server has a valid session in case of server restarts
      if(this._shouldCheckStatus()){        
        this._fetchStatus();
      }        
    }, TOKEN_CHECKER_INTERVAL);            
  },

  _stopSessionChecker(){
    clearInterval(sesstionCheckerTimerId);
    sesstionCheckerTimerId = null;
  }  
}

function receiveMessage(event){      
  const { key, newValue } = event;
  
  // check if local storage has been cleared from another tab
  if(localStorage.getBearerToken() === null){
    session.logout();
  }

  // renewToken has been invoked from another tab
  if(key === KeysEnum.TOKEN_RENEW && !!newValue){
    session._setIsRenewing(JSON.parse(newValue));    
  }        
}

export default session;