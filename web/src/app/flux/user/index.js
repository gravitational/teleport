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

import reactor from 'app/reactor';
import * as stsGetters from 'app/flux/status/getters';

const STORE_NAME = 'tlpt_user';

export function getUser() {
  return reactor.evaluate([STORE_NAME])
}

const invite = [ ['tlpt_user_invite'], invite => invite ];
const userName = [STORE_NAME, 'name'];

export const getters = {
  userName,
  invite,
  pswChangeAttempt: stsGetters.changePasswordAttempt,
  loginAttemp: stsGetters.loginAttempt,
  attemp: stsGetters.signupAttempt,
  fetchingInvite: stsGetters.fetchInviteAttempt
}
