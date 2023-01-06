/**
 * Copyright 2022 Gravitational, Inc.
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

export type RenewSessionRequest = {
  requestId?: string;
  switchback?: boolean;
  reloadUser?: boolean;
};

export type BearerToken = {
  accessToken: string;
  expiresIn: string;
  created: number;
  sessionExpires: Date;
  sessionInactiveTimeout: number;
};

export enum KeysEnum {
  TOKEN = 'grv_teleport_token',
  TOKEN_RENEW = 'grv_teleport_token_renew',
  LAST_ACTIVE = 'grv_teleport_last_active',
}

export enum EventKeys {
  CLEAR = 'bc_clear_session',
  GET_TOKEN = 'bc_get_session_token',
  UPSERT_TOKEN = 'bc_upsert_session_token',
  GET_TOKEN_IS_RENEWING = 'bc_get_token_is_renewing',
  UPSERT_TOKEN_IS_RENEWING = 'bc_upsert_token_is_renewing',
  GET_LAST_ACTIVE = 'bc_get_last_active',
  UPSERT_LAST_ACTIVE = 'bc_upsert_last_active',
}

export type BroadcastChannelMessage = {
  key: EventKeys;
  token?: BearerToken;
  isTokenRenewing?: boolean;
  lastActive?: number;
};
