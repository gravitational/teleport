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

import auth from "app/services/auth";
import api from "app/services/api";
import session from "app/services/session";
import Logger from "app/lib/logger";
import * as status from "./../status/actions";

const logger = Logger.create("flux/settingsAccount/actions");

export function changePasswordWithU2f(oldPsw, newPsw) {
  const promise = auth.changePasswordWithU2f(oldPsw, newPsw);
  _handleChangePasswordPromise(promise);
}

export function changePassword(oldPass, newPass, token) {
  const promise = auth.changePassword(oldPass, newPass, token);
  _handleChangePasswordPromise(promise);
}

export function resetPasswordChangeAttempt() {
  status.changePasswordStatus.clear();
}

function _handleChangePasswordPromise(promise) {
  status.changePasswordStatus.start();
  return promise
    .done(() => {
      status.changePasswordStatus.success();
    })
    .fail(err => {            
      const msg = api.getErrorText(err);
      logger.error("change password", err);
      status.changePasswordStatus.fail(msg);
      // logout a user in case of access denied error 
      // (most likely a user exceeded a number of allowed attempts)
      if(err.status == 403){
        session.logout();
      }
    });
}
