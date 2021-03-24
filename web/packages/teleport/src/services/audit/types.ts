/*
Copyright 2019 Gravitational, Inc.

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

export const CodeEnum = {
  // Teleport
  AUTH_ATTEMPT_FAILURE: 'T3007W',
  CLIENT_DISCONNECT: 'T3006I',
  EXEC_FAILURE: 'T3002E',
  EXEC: 'T3002I',
  PORTFORWARD_FAILURE: 'T3003E',
  PORTFORWARD: 'T3003I',
  SCP_DOWNLOAD_FAILURE: 'T3004E',
  SCP_DOWNLOAD: 'T3004I',
  SCP_UPLOAD_FAILURE: 'T3005E',
  SCP_UPLOAD: 'T3005I',
  SESSION_END: 'T2004I',
  SESSION_JOIN: 'T2001I',
  SESSION_REJECT: 'T1006W',
  SESSION_LEAVE: 'T2003I',
  SESSION_START: 'T2000I',
  SESSION_UPLOAD: 'T2005I',
  SESSION_DATA: 'T2006I',
  SESSION_COMMAND: 'T4000I',
  SESSION_DISK: 'T4001I',
  SESSION_NETWORK: 'T4002I',
  APP_SESSION_START: 'T2007I',
  APP_SESSION_CHUNK: 'T2008I',
  SUBSYSTEM_FAILURE: 'T3001E',
  SUBSYSTEM: 'T3001I',
  KUBE_REQUEST: 'T3009I',
  TERMINAL_RESIZE: 'T2002I',
  USER_LOCAL_LOGIN: 'T1000I',
  USER_LOCAL_LOGINFAILURE: 'T1000W',
  USER_SSO_LOGIN: 'T1001I',
  USER_SSO_LOGINFAILURE: 'T1001W',
  USER_CREATED: 'T1002I',
  USER_DELETED: 'T1004I',
  USER_UPDATED: 'T1003I',
  USER_PASSWORD_CHANGED: 'T1005I',
  RESET_PASSWORD_TOKEN_CREATED: 'T6000I',
  ROLE_CREATED: 'T9000I',
  ROLE_DELETED: 'T9001I',
  GITHUB_CONNECTOR_CREATED: 'T8000I',
  GITHUB_CONNECTOR_DELETED: 'T8001I',
  OIDC_CONNECTOR_CREATED: 'T8100I',
  OIDC_CONNECTOR_DELETED: 'T8101I',
  SAML_CONNECTOR_CREATED: 'T8200I',
  SAML_CONNECTOR_DELETED: 'T8201I',
  ACCESS_REQUEST_CREATED: 'T5000I',
  ACCESS_REQUEST_UPDATED: 'T5001I',
  TRUSTED_CLUSTER_TOKEN_CREATED: 'T7002I',
  TRUSTED_CLUSTER_CREATED: 'T7000I',
  TRUSTED_CLUSTER_DELETED: 'T7001I',
  DATABASE_SESSION_STARTED: 'TDB00I',
  DATABASE_SESSION_STARTED_FAILURE: 'TDB00W',
  DATABASE_SESSION_ENDED: 'TDB01I',
  DATABASE_SESSION_QUERY: 'TDB02I',
  MFA_DEVICE_ADD: 'T1006I',
  MFA_DEVICE_DELETE: 'T1007I',
  BILLING_CARD_CREATE: 'TBL00I',
  BILLING_CARD_DELETE: 'TBL01I',
  BILLING_CARD_UPDATE: 'TBL02I',
  BILLING_ACCOUNT_UPDATE: 'TBL03I',
} as const;

/**
 * Describes all raw event types
 */
export type RawEvents = {
  [CodeEnum.ACCESS_REQUEST_CREATED]: RawEventAccess<
    typeof CodeEnum.ACCESS_REQUEST_CREATED
  >;
  [CodeEnum.ACCESS_REQUEST_UPDATED]: RawEventAccess<
    typeof CodeEnum.ACCESS_REQUEST_UPDATED
  >;
  [CodeEnum.AUTH_ATTEMPT_FAILURE]: RawEventAuthFailure<
    typeof CodeEnum.AUTH_ATTEMPT_FAILURE
  >;
  [CodeEnum.CLIENT_DISCONNECT]: RawEvent<
    typeof CodeEnum.CLIENT_DISCONNECT,
    { reason: string }
  >;
  [CodeEnum.EXEC]: RawEvent<
    typeof CodeEnum.EXEC,
    {
      proto: 'kube';
      kubernetes_cluster: string;
    }
  >;
  [CodeEnum.EXEC_FAILURE]: RawEvent<
    typeof CodeEnum.EXEC_FAILURE,
    { exitError: string }
  >;
  [CodeEnum.BILLING_CARD_CREATE]: RawEvent<typeof CodeEnum.BILLING_CARD_CREATE>;
  [CodeEnum.BILLING_CARD_DELETE]: RawEvent<typeof CodeEnum.BILLING_CARD_DELETE>;
  [CodeEnum.BILLING_CARD_UPDATE]: RawEvent<typeof CodeEnum.BILLING_CARD_UPDATE>;
  [CodeEnum.BILLING_ACCOUNT_UPDATE]: RawEvent<
    typeof CodeEnum.BILLING_ACCOUNT_UPDATE
  >;
  [CodeEnum.GITHUB_CONNECTOR_CREATED]: RawEventConnector<
    typeof CodeEnum.GITHUB_CONNECTOR_CREATED
  >;
  [CodeEnum.GITHUB_CONNECTOR_DELETED]: RawEventConnector<
    typeof CodeEnum.GITHUB_CONNECTOR_DELETED
  >;
  [CodeEnum.OIDC_CONNECTOR_CREATED]: RawEventConnector<
    typeof CodeEnum.OIDC_CONNECTOR_CREATED
  >;
  [CodeEnum.OIDC_CONNECTOR_DELETED]: RawEventConnector<
    typeof CodeEnum.OIDC_CONNECTOR_DELETED
  >;
  [CodeEnum.PORTFORWARD]: RawEvent<typeof CodeEnum.PORTFORWARD>;
  [CodeEnum.PORTFORWARD_FAILURE]: RawEvent<
    typeof CodeEnum.PORTFORWARD_FAILURE,
    {
      error: string;
    }
  >;
  [CodeEnum.SAML_CONNECTOR_CREATED]: RawEventConnector<
    typeof CodeEnum.SAML_CONNECTOR_CREATED
  >;
  [CodeEnum.SAML_CONNECTOR_DELETED]: RawEventConnector<
    typeof CodeEnum.SAML_CONNECTOR_DELETED
  >;
  [CodeEnum.SCP_DOWNLOAD]: RawEvent<
    typeof CodeEnum.SCP_DOWNLOAD,
    {
      path: string;
      ['addr_local']: string;
    }
  >;
  [CodeEnum.SCP_DOWNLOAD_FAILURE]: RawEvent<
    typeof CodeEnum.SCP_DOWNLOAD_FAILURE,
    {
      exitError: string;
    }
  >;
  [CodeEnum.SCP_UPLOAD]: RawEvent<
    typeof CodeEnum.SCP_UPLOAD,
    {
      path: string;
      ['addr.local']: string;
    }
  >;
  [CodeEnum.SCP_UPLOAD_FAILURE]: RawEvent<
    typeof CodeEnum.SCP_UPLOAD_FAILURE,
    {
      exitError: string;
    }
  >;

  [CodeEnum.SESSION_COMMAND]: RawEventCommand<typeof CodeEnum.SESSION_COMMAND>;

  [CodeEnum.SESSION_DISK]: RawDiskEvent<typeof CodeEnum.SESSION_DISK>;

  [CodeEnum.SESSION_NETWORK]: RawEventNetwork<typeof CodeEnum.SESSION_NETWORK>;

  [CodeEnum.SESSION_DATA]: RawEventData<typeof CodeEnum.SESSION_DATA>;

  [CodeEnum.SESSION_JOIN]: RawEvent<
    typeof CodeEnum.SESSION_JOIN,
    {
      sid: string;
    }
  >;
  [CodeEnum.SESSION_END]: RawEvent<
    typeof CodeEnum.SESSION_END,
    {
      sid: string;
      server_id: string;
      server_addr: string;
      session_start: string;
      session_stop: string;
      participants?: string[];
      server_hostname: string;
      interactive: boolean;
      proto: string;
      kubernetes_cluster: string;
      kubernetes_pod_namespace: string;
      kubernetes_pod_name: string;
    }
  >;
  [CodeEnum.SESSION_LEAVE]: RawEvent<
    typeof CodeEnum.SESSION_LEAVE,
    {
      sid: string;
    }
  >;
  [CodeEnum.SESSION_START]: RawEvent<
    typeof CodeEnum.SESSION_START,
    {
      sid: string;
    }
  >;
  [CodeEnum.SESSION_REJECT]: RawEvent<
    typeof CodeEnum.SESSION_REJECT,
    {
      login: string;
      server_id: string;
      reason: string;
    }
  >;
  [CodeEnum.SESSION_UPLOAD]: RawEvent<
    typeof CodeEnum.SESSION_UPLOAD,
    {
      sid: string;
    }
  >;
  [CodeEnum.APP_SESSION_START]: RawEvent<
    typeof CodeEnum.APP_SESSION_START,
    { sid: string }
  >;
  [CodeEnum.APP_SESSION_CHUNK]: RawEvent<
    typeof CodeEnum.APP_SESSION_CHUNK,
    { sid: string }
  >;
  [CodeEnum.SUBSYSTEM]: RawEvent<
    typeof CodeEnum.SUBSYSTEM,
    {
      name: string;
    }
  >;
  [CodeEnum.SUBSYSTEM_FAILURE]: RawEvent<
    typeof CodeEnum.SUBSYSTEM_FAILURE,
    {
      name: string;
      exitError: string;
    }
  >;
  [CodeEnum.TERMINAL_RESIZE]: RawEvent<
    typeof CodeEnum.TERMINAL_RESIZE,
    { sid: string }
  >;
  [CodeEnum.USER_CREATED]: RawEventUser<typeof CodeEnum.USER_CREATED>;
  [CodeEnum.USER_DELETED]: RawEventUser<typeof CodeEnum.USER_DELETED>;
  [CodeEnum.USER_UPDATED]: RawEventUser<typeof CodeEnum.USER_UPDATED>;
  [CodeEnum.USER_PASSWORD_CHANGED]: RawEvent<
    typeof CodeEnum.USER_PASSWORD_CHANGED,
    HasName
  >;
  [CodeEnum.RESET_PASSWORD_TOKEN_CREATED]: RawEventPasswordToken<
    typeof CodeEnum.RESET_PASSWORD_TOKEN_CREATED
  >;
  [CodeEnum.USER_LOCAL_LOGIN]: RawEvent<typeof CodeEnum.USER_LOCAL_LOGIN>;
  [CodeEnum.USER_LOCAL_LOGINFAILURE]: RawEvent<
    typeof CodeEnum.USER_LOCAL_LOGINFAILURE,
    {
      error: string;
    }
  >;
  [CodeEnum.USER_SSO_LOGIN]: RawEvent<typeof CodeEnum.USER_SSO_LOGIN>;
  [CodeEnum.USER_SSO_LOGINFAILURE]: RawEvent<
    typeof CodeEnum.USER_SSO_LOGINFAILURE,
    {
      error: string;
    }
  >;
  [CodeEnum.ROLE_CREATED]: RawEvent<typeof CodeEnum.ROLE_CREATED, HasName>;
  [CodeEnum.ROLE_DELETED]: RawEvent<typeof CodeEnum.ROLE_DELETED, HasName>;
  [CodeEnum.TRUSTED_CLUSTER_TOKEN_CREATED]: RawEvent<
    typeof CodeEnum.TRUSTED_CLUSTER_TOKEN_CREATED
  >;
  [CodeEnum.TRUSTED_CLUSTER_CREATED]: RawEvent<
    typeof CodeEnum.TRUSTED_CLUSTER_CREATED,
    {
      name: string;
    }
  >;
  [CodeEnum.TRUSTED_CLUSTER_DELETED]: RawEvent<
    typeof CodeEnum.TRUSTED_CLUSTER_DELETED,
    {
      name: string;
    }
  >;
  [CodeEnum.KUBE_REQUEST]: RawEvent<
    typeof CodeEnum.KUBE_REQUEST,
    {
      kubernetes_cluster: string;
    }
  >;
  [CodeEnum.DATABASE_SESSION_STARTED]: RawEvent<
    typeof CodeEnum.DATABASE_SESSION_STARTED,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
    }
  >;
  [CodeEnum.DATABASE_SESSION_STARTED_FAILURE]: RawEvent<
    typeof CodeEnum.DATABASE_SESSION_STARTED_FAILURE,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
    }
  >;
  [CodeEnum.DATABASE_SESSION_ENDED]: RawEvent<
    typeof CodeEnum.DATABASE_SESSION_ENDED,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
    }
  >;
  [CodeEnum.DATABASE_SESSION_QUERY]: RawEvent<
    typeof CodeEnum.DATABASE_SESSION_QUERY,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
      db_query: string;
    }
  >;
  [CodeEnum.MFA_DEVICE_ADD]: RawEvent<
    typeof CodeEnum.MFA_DEVICE_ADD,
    {
      mfa_device_name: string;
      mfa_device_uuid: string;
      mfa_device_type: string;
    }
  >;
  [CodeEnum.MFA_DEVICE_DELETE]: RawEvent<
    typeof CodeEnum.MFA_DEVICE_DELETE,
    {
      mfa_device_name: string;
      mfa_device_uuid: string;
      mfa_device_type: string;
    }
  >;
};

/**
 * Event Code
 */
export type Code = typeof CodeEnum[keyof typeof CodeEnum];

type HasName = {
  name: string;
};

/**
 * Merges properties of 2 types and returns a new "clean" type (using "infer")
 */
type Merge<A, B> = Omit<A, keyof B> & B extends infer O
  ? { [K in keyof O]: O[K] }
  : never;

/**
 * Describes common properties of the raw events (backend data)
 */
export type RawEvent<T extends Code, K = {}> = Merge<
  {
    code: T;
    user: string;
    time: string;
    uid: string;
    event: string;
  },
  K
>;

type RawEventData<T extends Code> = RawEvent<
  T,
  {
    login: string;
    rx: number;
    server_id: string;
    sid: string;
    tx: number;
    user: string;
  }
>;

type RawEventCommand<T extends Code> = RawEvent<
  T,
  {
    login: string;
    namespace: string;
    path: string;
    pid: number;
    ppid: number;
    program: string;
    return_code: number;
    server_id: string;
    sid: string;
  }
>;

type RawEventNetwork<T extends Code> = RawEvent<
  T,
  {
    login: string;
    namespace: string;
    pid: number;
    cgroup_id: number;
    program: string;
    server_id: string;
    flags: number;
    sid: string;
    src_addr: string;
    dst_addr: string;
    dst_port: string;
  }
>;

type RawDiskEvent<T extends Code> = RawEvent<
  T,
  {
    login: string;
    namespace: string;
    pid: number;
    cgroup_id: number;
    program: string;
    path: string;
    return_code: number;
    server_id: string;
    flags: number;
    sid: string;
  }
>;

type RawEventAccess<T extends Code> = RawEvent<
  T,
  {
    id: string;
    user: string;
    roles: string[];
    state: string;
  }
>;

type RawEventPasswordToken<T extends Code> = RawEvent<
  T,
  {
    name: string;
    ttl: string;
  }
>;

type RawEventUser<T extends Code> = RawEvent<
  T,
  {
    name: string;
  }
>;

type RawEventConnector<T extends Code> = RawEvent<
  T,
  {
    name: string;
    user: string;
  }
>;

type RawEventAuthFailure<T extends Code> = RawEvent<
  T,
  {
    error: string;
  }
>;

/**
 * A map of event formatters that provide short and long description
 */
export type Formatters = {
  [key in Code]: {
    desc: string;
    format: (json: RawEvents[key]) => string;
  };
};

export type Events = {
  [key in Code]: {
    id: string;
    time: Date;
    user: string;
    message: string;
    code: key;
    codeDesc: string;
    raw: RawEvents[key];
  };
};

export type Event = Events[Code];

export type SessionEnd = Events[typeof CodeEnum.SESSION_END];
