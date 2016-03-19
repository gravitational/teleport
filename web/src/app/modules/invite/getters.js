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

var {TRYING_TO_SIGN_UP, FETCHING_INVITE} = require('app/modules/restApi/constants');
var {requestStatus} = require('app/modules/restApi/getters');

const invite = [ ['tlpt_invite'], (invite) => invite ];

export default {
  invite,
  attemp: requestStatus(TRYING_TO_SIGN_UP),
  fetchingInvite: requestStatus(FETCHING_INVITE)
}
