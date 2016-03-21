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

var api = require('./api');
var cfg = require('../config');

const apiUtils = {
    filterSessions({start, end, sid, limit, order=-1}){
      let params = {
        start: start.toISOString(),
        end,
        order,
        limit
      }

      if(sid){
        params.session_id = sid;
      }

      return api.get(cfg.api.getFetchSessionsUrl(params))
    }
}

module.exports = apiUtils;
