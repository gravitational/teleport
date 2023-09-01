/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { ChangedUserAuthn, RecoveryCodes } from './types';

// makeChangedUserAuthn makes the response from a successful user reset or invite.
// Only teleport cloud and users with valid emails as username will receive
// recovery codes.
export function makeChangedUserAuthn(json: any): ChangedUserAuthn {
  json = json || {};

  return {
    recovery: makeRecoveryCodes(json.recovery),
  };
}

export function makeRecoveryCodes(json: any): RecoveryCodes {
  json = json || {};

  return {
    codes: json.codes || [],
    createdDate: json.created ? new Date(json.created) : null,
  };
}
