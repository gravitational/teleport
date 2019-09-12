/*
Copyright 2019 Gravitational, Inc.

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

import $ from "jQuery";
import Logger from 'shared/libs/logger';
import localStorage from './localStorage';

const logger = Logger.create('services/api');

const api = {

  delete(path, data, withToken){
    return api.ajax({url: path, data: JSON.stringify(data), type: 'DELETE'}, withToken);
  },

  post(path, data, withToken){
    return api.ajax({url: path, data: JSON.stringify(data), type: 'POST'}, withToken);
  },

  put(path, data, withToken){
    return api.ajax({url: path, data: JSON.stringify(data), type: 'PUT'}, withToken);
  },

  patch(path, data, withToken){
    return api.ajax({
      contentType: 'application/merge-patch+json',
      url: path, data: JSON.stringify(data), type: 'PATCH'
     }, withToken);
  },

  get(path){
    return api.ajax({url: path});
  },

  ajax(cfg, withAuth = true){
    const defaultCfg = {
      type: 'GET',
      contentType: 'application/json; charset=utf-8',
      cache: false,
      dataType: 'json',
      beforeSend: function(xhr) {
        xhr.setRequestHeader('X-CSRF-Token', getXCSRFToken());
        if(withAuth){
          const accessToken = getAccessToken();
          xhr.setRequestHeader('Authorization',`Bearer ${accessToken}`);
        }
       }
    }

    const ajaxPromise = $.ajax($.extend({}, defaultCfg, cfg));

    // abort mechanism
    const signal = cfg.signal || new Signal();
    const abortCb = () => {
      ajaxPromise.abort();
    }
    signal.subscribe(abortCb);

    const dfd = $.Deferred();
    ajaxPromise
      .then(response => dfd.resolve(response))
      .fail(err => {
        const msg = this.getErrorText(err);
        const error = new Error(msg);

        // preserve state
        error.state = err.state;
        error.status = err.status;
        error.readyState = err.readyState;
        dfd.reject(error);
      })
      .always(() => {
        signal.unsubscribe(abortCb)
      })

    return dfd;
  },

  // Because there are several ways backend can return errors,
  // we are going to try all of them to get an error text
  getErrorText(err){
    let msg = 'Unknown error';

    if (err instanceof Error) {
      return err.message || msg;
    }

    if(err.responseJSON && err.responseJSON.message){
      return err.responseJSON.message;
    }

    if (err.responseJSON && err.responseJSON.error) {
      return err.responseJSON.error.message || msg;
    }

    if (err.responseText) {
      return err.responseText;
    }

    return msg;
  }
}

// Signal allows to cancel on-goining HTTP requests
export function Signal(){
  const subs = [];

  return {
    subscribe(cb){
      subs.push(cb)
    },

    unsubscribe(cb){
      const index = subs.indexOf(cb);
      if (index > -1) {
        subs.splice(index, 1)
      }
    },

    abort(){
      const tmp = [...subs];
      subs.length = 0;
      tmp.forEach(cb => {
        try{
          cb();
        }
        catch(err){
          logger.error('abort', err);
        }
      })
    }
  }
}

export function getAuthHeaders() {
  const accessToken = getAccessToken();
  const csrfToken = getXCSRFToken();
  return {
    'X-CSRF-Token': csrfToken,
    'Authorization': `Bearer ${accessToken}`
  }
}

export function getNoCacheHeaders() {
  return {
    'cache-control': 'max-age=0',
    'expires': '0',
    'pragma': 'no-cache'
  }
}

export const getXCSRFToken = () => {
  const metaTag = document.querySelector('[name=grv_csrf_token]');
  return metaTag ? metaTag.content : '';
}

export function getAccessToken() {
  const bearerToken = localStorage.getBearerToken() || {};
  return bearerToken.accessToken;
}

export default api;

