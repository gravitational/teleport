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
import * as AT from './actionTypes';
import * as RT from './constants';

export function makeStatus(reqType) {
  return {
    start() {
      reactor.dispatch(AT.START, {type: reqType});
    },

    success(message) {
      reactor.dispatch(AT.SUCCESS, {type: reqType, message});
    },

    fail(message) {
      reactor.dispatch(AT.FAIL,  {type: reqType, message});
    },

    clear(){
      reactor.dispatch(AT.CLEAR, {type: reqType});
    }
  }
}

export const initAppStatus = makeStatus(RT.TRYING_TO_INIT_APP);  
export const loginStatus = makeStatus(RT.TRYING_TO_LOGIN);
export const fetchInviteStatus = makeStatus(RT.FETCHING_INVITE);
export const signupStatus = makeStatus(RT.TRYING_TO_SIGN_UP);
export const initSettingsStatus = makeStatus(RT.TRYING_TO_INIT_SETTINGS);
export const changePasswordStatus = makeStatus(RT.TRYING_TO_CHANGE_PSW);