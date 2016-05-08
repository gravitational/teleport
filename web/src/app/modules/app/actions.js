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

var reactor = require('app/reactor');
var {fetchActiveSessions} = require('./../sessions/actions');
var {fetchNodes} = require('./../nodes/actions');
var $ = require('jQuery');

const { TLPT_APP_INIT, TLPT_APP_FAILED, TLPT_APP_READY } = require('./actionTypes');

const actions = {

  initApp() {
    reactor.dispatch(TLPT_APP_INIT);    
    actions.fetchNodesAndSessions()
      .done(()=> reactor.dispatch(TLPT_APP_READY) )
      .fail(()=> reactor.dispatch(TLPT_APP_FAILED) );
  },

  fetchNodesAndSessions() {
    return $.when(fetchNodes(), fetchActiveSessions());
  }
}

export default actions;
