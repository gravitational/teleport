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
import { TRYING_TO_LOGIN, TRYING_TO_SIGN_UP, FETCHING_INVITE, TRYING_TO_CHANGE_PSW} from 'app/flux/restApi/constants';
import { requestStatus } from 'app/flux/restApi/getters';

const STORE_NAME = 'tlpt_user';

export function getUser() {
  return reactor.evaluate([STORE_NAME])
}

const invite = [ ['tlpt_user_invite'], invite => invite ];
const userName = [STORE_NAME, 'name'];

export const getters = {
  userName,
  invite,
  pswChangeAttempt: requestStatus(TRYING_TO_CHANGE_PSW),
  loginAttemp: requestStatus(TRYING_TO_LOGIN),
  attemp: requestStatus(TRYING_TO_SIGN_UP),
  fetchingInvite: requestStatus(FETCHING_INVITE)
}
