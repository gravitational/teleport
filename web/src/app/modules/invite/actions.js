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
var { TLPT_RECEIVE_USER_INVITE }  = require('./actionTypes');
var { FETCHING_INVITE} = require('app/modules/restApi/constants');
var restApiActions = require('app/modules/restApi/actions');
var api = require('app/services/api');
var cfg = require('app/config');

export default {
  fetchInvite(inviteToken){
    var path = cfg.api.getInviteUrl(inviteToken);
    restApiActions.start(FETCHING_INVITE);
    api.get(path).done(invite=>{
      restApiActions.success(FETCHING_INVITE);
      reactor.dispatch(TLPT_RECEIVE_USER_INVITE, invite);
    }).
    fail((err)=>{
      restApiActions.fail(FETCHING_INVITE, err.responseJSON.message);
    });
  }
}
