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

var $ = require("jQuery");
var session = require('./session');

const api = {

  put(path, data, withToken){
    return api.ajax({url: path, data: JSON.stringify(data), type: 'PUT'}, withToken);
  },

  post(path, data, withToken){
    return api.ajax({url: path, data: JSON.stringify(data), type: 'POST'}, withToken);
  },

  get(path){
    return api.ajax({url: path});
  },

  ajax(cfg, withToken = true){
    var defaultCfg = {
      type: "GET",
      dataType: "json",
      beforeSend: function(xhr) {
        if(withToken){
          var { token } = session.getUserData();
          xhr.setRequestHeader('Authorization','Bearer ' + token);
        }
       }
    }

    return $.ajax($.extend({}, defaultCfg, cfg));
  }
}

module.exports = api;
