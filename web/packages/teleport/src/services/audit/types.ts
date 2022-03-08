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

// eventGroupTypes contains a map of events that were grouped under the same
// event type but have different event codes. This is used to filter out duplicate
// event types when listing event filters and provide modified description of event.
export const eventGroupTypes = {
  'db.session.start': 'Database Session Start',
  exec: 'Command Execution',
  port: 'Port Forwarding',
  scp: 'SCP',
  subsystem: 'Subsystem Request',
  'user.login': 'User Logins',
};

/**
 * eventCodes is a map of event codes.
 *
 * After defining an event code:
 *  1: Define fields from JSON response in `RawEvents` object
 *  2: Define formatter in `makeEvent` file which defines *events types and
 *     defines short and long event definitions
 *  * Some events can have same event "type" but have unique "code".
 *    These duplicated event types needs to be defined in `eventGroupTypes` object
 *  3: Define icons for events under `EventTypeCell` file
 *  4: Add an actual JSON event to the fixtures file in `src/Audit` directory to
 *     be used for display and test in storybook.
 */
export const eventCodes = {
  ACCESS_REQUEST_CREATED: 'T5000I',
  ACCESS_REQUEST_REVIEWED: 'T5002I',
  ACCESS_REQUEST_UPDATED: 'T5001I',
  ACCESS_REQUEST_DELETED: 'T5003I',
  APP_SESSION_CHUNK: 'T2008I',
  APP_SESSION_START: 'T2007I',
  AUTH_ATTEMPT_FAILURE: 'T3007W',
  BILLING_INFORMATION_UPDATE: 'TBL03I',
  BILLING_CARD_CREATE: 'TBL00I',
  BILLING_CARD_DELETE: 'TBL01I',
  BILLING_CARD_UPDATE: 'TBL02I',
  CLIENT_DISCONNECT: 'T3006I',
  DATABASE_SESSION_ENDED: 'TDB01I',
  DATABASE_SESSION_QUERY: 'TDB02I',
  DATABASE_SESSION_QUERY_FAILURE: 'TDB02W',
  DATABASE_SESSION_STARTED_FAILURE: 'TDB00W',
  DATABASE_SESSION_STARTED: 'TDB00I',
  DATABASE_CREATED: 'TDB03I',
  DATABASE_UPDATED: 'TDB04I',
  DATABASE_DELETED: 'TDB05I',
  POSTGRES_PARSE: 'TPG00I',
  POSTGRES_BIND: 'TPG01I',
  POSTGRES_EXECUTE: 'TPG02I',
  POSTGRES_CLOSE: 'TPG03I',
  POSTGRES_FUNCTION_CALL: 'TPG04I',
  MYSQL_STATEMENT_PREPARE: 'TMY00I',
  MYSQL_STATEMENT_EXECUTE: 'TMY01I',
  MYSQL_STATEMENT_SEND_LONG_DATA: 'TMY02I',
  MYSQL_STATEMENT_CLOSE: 'TMY03I',
  MYSQL_STATEMENT_RESET: 'TMY04I',
  MYSQL_STATEMENT_FETCH: 'TMY05I',
  MYSQL_STATEMENT_BULK_EXECUTE: 'TMY06I',
  DESKTOP_SESSION_STARTED: 'TDP00I',
  DESKTOP_SESSION_STARTED_FAILED: 'TDP00W',
  DESKTOP_SESSION_ENDED: 'TDP01I',
  DESKTOP_CLIPBOARD_SEND: 'TDP02I',
  DESKTOP_CLIPBOARD_RECEIVE: 'TDP03I',
  EXEC_FAILURE: 'T3002E',
  EXEC: 'T3002I',
  GITHUB_CONNECTOR_CREATED: 'T8000I',
  GITHUB_CONNECTOR_DELETED: 'T8001I',
  KUBE_REQUEST: 'T3009I',
  LOCK_CREATED: 'TLK00I',
  LOCK_DELETED: 'TLK01I',
  MFA_DEVICE_ADD: 'T1006I',
  MFA_DEVICE_DELETE: 'T1007I',
  OIDC_CONNECTOR_CREATED: 'T8100I',
  OIDC_CONNECTOR_DELETED: 'T8101I',
  PORTFORWARD_FAILURE: 'T3003E',
  PORTFORWARD: 'T3003I',
  RECOVERY_TOKEN_CREATED: 'T6001I',
  PRIVILEGE_TOKEN_CREATED: 'T6002I',
  RECOVERY_CODE_GENERATED: 'T1008I',
  RECOVERY_CODE_USED: 'T1009I',
  RECOVERY_CODE_USED_FAILURE: 'T1009W',
  RESET_PASSWORD_TOKEN_CREATED: 'T6000I',
  ROLE_CREATED: 'T9000I',
  ROLE_DELETED: 'T9001I',
  SAML_CONNECTOR_CREATED: 'T8200I',
  SAML_CONNECTOR_DELETED: 'T8201I',
  SCP_DOWNLOAD_FAILURE: 'T3004E',
  SCP_DOWNLOAD: 'T3004I',
  SCP_UPLOAD_FAILURE: 'T3005E',
  SCP_UPLOAD: 'T3005I',
  SESSION_COMMAND: 'T4000I',
  SESSION_DATA: 'T2006I',
  SESSION_DISK: 'T4001I',
  SESSION_END: 'T2004I',
  SESSION_JOIN: 'T2001I',
  SESSION_LEAVE: 'T2003I',
  SESSION_NETWORK: 'T4002I',
  SESSION_PROCESS_EXIT: 'T4003I',
  SESSION_REJECT: 'T1006W',
  SESSION_START: 'T2000I',
  SESSION_UPLOAD: 'T2005I',
  SESSION_CONNECT: 'T2010I',
  SUBSYSTEM_FAILURE: 'T3001E',
  SUBSYSTEM: 'T3001I',
  TERMINAL_RESIZE: 'T2002I',
  TRUSTED_CLUSTER_CREATED: 'T7000I',
  TRUSTED_CLUSTER_DELETED: 'T7001I',
  TRUSTED_CLUSTER_TOKEN_CREATED: 'T7002I',
  UNKNOWN: 'TCC00E',
  USER_CREATED: 'T1002I',
  USER_DELETED: 'T1004I',
  USER_LOCAL_LOGIN: 'T1000I',
  USER_LOCAL_LOGINFAILURE: 'T1000W',
  USER_PASSWORD_CHANGED: 'T1005I',
  USER_SSO_LOGIN: 'T1001I',
  USER_SSO_LOGINFAILURE: 'T1001W',
  USER_UPDATED: 'T1003I',
  X11_FORWARD: 'T3008I',
  X11_FORWARD_FAILURE: 'T3008W',
  CERTIFICATE_CREATED: 'TC000I',
} as const;

/**
 * Describes all raw event types
 */
export type RawEvents = {
  [eventCodes.ACCESS_REQUEST_CREATED]: RawEventAccess<
    typeof eventCodes.ACCESS_REQUEST_CREATED
  >;
  [eventCodes.ACCESS_REQUEST_UPDATED]: RawEventAccess<
    typeof eventCodes.ACCESS_REQUEST_UPDATED
  >;
  [eventCodes.ACCESS_REQUEST_REVIEWED]: RawEventAccess<
    typeof eventCodes.ACCESS_REQUEST_REVIEWED
  >;
  [eventCodes.ACCESS_REQUEST_DELETED]: RawEventAccess<
    typeof eventCodes.ACCESS_REQUEST_DELETED
  >;
  [eventCodes.AUTH_ATTEMPT_FAILURE]: RawEventAuthFailure<
    typeof eventCodes.AUTH_ATTEMPT_FAILURE
  >;
  [eventCodes.CLIENT_DISCONNECT]: RawEvent<
    typeof eventCodes.CLIENT_DISCONNECT,
    { reason: string }
  >;
  [eventCodes.EXEC]: RawEvent<
    typeof eventCodes.EXEC,
    {
      proto: 'kube';
      kubernetes_cluster: string;
    }
  >;
  [eventCodes.EXEC_FAILURE]: RawEvent<
    typeof eventCodes.EXEC_FAILURE,
    { exitError: string }
  >;
  [eventCodes.BILLING_CARD_CREATE]: RawEvent<
    typeof eventCodes.BILLING_CARD_CREATE
  >;
  [eventCodes.BILLING_CARD_DELETE]: RawEvent<
    typeof eventCodes.BILLING_CARD_DELETE
  >;
  [eventCodes.BILLING_CARD_UPDATE]: RawEvent<
    typeof eventCodes.BILLING_CARD_UPDATE
  >;
  [eventCodes.BILLING_INFORMATION_UPDATE]: RawEvent<
    typeof eventCodes.BILLING_INFORMATION_UPDATE
  >;
  [eventCodes.GITHUB_CONNECTOR_CREATED]: RawEventConnector<
    typeof eventCodes.GITHUB_CONNECTOR_CREATED
  >;
  [eventCodes.GITHUB_CONNECTOR_DELETED]: RawEventConnector<
    typeof eventCodes.GITHUB_CONNECTOR_DELETED
  >;
  [eventCodes.OIDC_CONNECTOR_CREATED]: RawEventConnector<
    typeof eventCodes.OIDC_CONNECTOR_CREATED
  >;
  [eventCodes.OIDC_CONNECTOR_DELETED]: RawEventConnector<
    typeof eventCodes.OIDC_CONNECTOR_DELETED
  >;
  [eventCodes.PORTFORWARD]: RawEvent<typeof eventCodes.PORTFORWARD>;
  [eventCodes.PORTFORWARD_FAILURE]: RawEvent<
    typeof eventCodes.PORTFORWARD_FAILURE,
    {
      error: string;
    }
  >;
  [eventCodes.SAML_CONNECTOR_CREATED]: RawEventConnector<
    typeof eventCodes.SAML_CONNECTOR_CREATED
  >;
  [eventCodes.SAML_CONNECTOR_DELETED]: RawEventConnector<
    typeof eventCodes.SAML_CONNECTOR_DELETED
  >;
  [eventCodes.SCP_DOWNLOAD]: RawEvent<
    typeof eventCodes.SCP_DOWNLOAD,
    {
      path: string;
      ['addr_local']: string;
    }
  >;
  [eventCodes.SCP_DOWNLOAD_FAILURE]: RawEvent<
    typeof eventCodes.SCP_DOWNLOAD_FAILURE,
    {
      exitError: string;
    }
  >;
  [eventCodes.SCP_UPLOAD]: RawEvent<
    typeof eventCodes.SCP_UPLOAD,
    {
      path: string;
      ['addr.local']: string;
    }
  >;
  [eventCodes.SCP_UPLOAD_FAILURE]: RawEvent<
    typeof eventCodes.SCP_UPLOAD_FAILURE,
    {
      exitError: string;
    }
  >;

  [eventCodes.SESSION_COMMAND]: RawEventCommand<
    typeof eventCodes.SESSION_COMMAND
  >;

  [eventCodes.SESSION_DISK]: RawDiskEvent<typeof eventCodes.SESSION_DISK>;

  [eventCodes.SESSION_NETWORK]: RawEventNetwork<
    typeof eventCodes.SESSION_NETWORK
  >;

  [eventCodes.SESSION_PROCESS_EXIT]: RawEventProcessExit<
    typeof eventCodes.SESSION_PROCESS_EXIT
  >;

  [eventCodes.SESSION_DATA]: RawEventData<typeof eventCodes.SESSION_DATA>;

  [eventCodes.SESSION_JOIN]: RawEvent<
    typeof eventCodes.SESSION_JOIN,
    {
      sid: string;
    }
  >;
  [eventCodes.SESSION_END]: RawEvent<
    typeof eventCodes.SESSION_END,
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
      session_recording: 'off' | 'node' | 'proxy' | 'node-sync' | 'proxy-sync';
    }
  >;
  [eventCodes.SESSION_LEAVE]: RawEvent<
    typeof eventCodes.SESSION_LEAVE,
    {
      sid: string;
    }
  >;
  [eventCodes.SESSION_START]: RawEvent<
    typeof eventCodes.SESSION_START,
    {
      sid: string;
    }
  >;
  [eventCodes.SESSION_REJECT]: RawEvent<
    typeof eventCodes.SESSION_REJECT,
    {
      login: string;
      server_id: string;
      reason: string;
    }
  >;
  [eventCodes.SESSION_UPLOAD]: RawEvent<
    typeof eventCodes.SESSION_UPLOAD,
    {
      sid: string;
    }
  >;
  [eventCodes.APP_SESSION_START]: RawEvent<
    typeof eventCodes.APP_SESSION_START,
    { sid: string }
  >;
  [eventCodes.APP_SESSION_CHUNK]: RawEvent<
    typeof eventCodes.APP_SESSION_CHUNK,
    { sid: string }
  >;
  [eventCodes.SUBSYSTEM]: RawEvent<
    typeof eventCodes.SUBSYSTEM,
    {
      name: string;
    }
  >;
  [eventCodes.SUBSYSTEM_FAILURE]: RawEvent<
    typeof eventCodes.SUBSYSTEM_FAILURE,
    {
      name: string;
      exitError: string;
    }
  >;
  [eventCodes.TERMINAL_RESIZE]: RawEvent<
    typeof eventCodes.TERMINAL_RESIZE,
    { sid: string }
  >;
  [eventCodes.USER_CREATED]: RawEventUser<typeof eventCodes.USER_CREATED>;
  [eventCodes.USER_DELETED]: RawEventUser<typeof eventCodes.USER_DELETED>;
  [eventCodes.USER_UPDATED]: RawEventUser<typeof eventCodes.USER_UPDATED>;
  [eventCodes.USER_PASSWORD_CHANGED]: RawEvent<
    typeof eventCodes.USER_PASSWORD_CHANGED,
    HasName
  >;
  [eventCodes.RESET_PASSWORD_TOKEN_CREATED]: RawEventUserToken<
    typeof eventCodes.RESET_PASSWORD_TOKEN_CREATED
  >;
  [eventCodes.USER_LOCAL_LOGIN]: RawEvent<typeof eventCodes.USER_LOCAL_LOGIN>;
  [eventCodes.USER_LOCAL_LOGINFAILURE]: RawEvent<
    typeof eventCodes.USER_LOCAL_LOGINFAILURE,
    {
      error: string;
    }
  >;
  [eventCodes.USER_SSO_LOGIN]: RawEvent<typeof eventCodes.USER_SSO_LOGIN>;
  [eventCodes.USER_SSO_LOGINFAILURE]: RawEvent<
    typeof eventCodes.USER_SSO_LOGINFAILURE,
    {
      error: string;
    }
  >;
  [eventCodes.ROLE_CREATED]: RawEvent<typeof eventCodes.ROLE_CREATED, HasName>;
  [eventCodes.ROLE_DELETED]: RawEvent<typeof eventCodes.ROLE_DELETED, HasName>;
  [eventCodes.TRUSTED_CLUSTER_TOKEN_CREATED]: RawEvent<
    typeof eventCodes.TRUSTED_CLUSTER_TOKEN_CREATED
  >;
  [eventCodes.TRUSTED_CLUSTER_CREATED]: RawEvent<
    typeof eventCodes.TRUSTED_CLUSTER_CREATED,
    {
      name: string;
    }
  >;
  [eventCodes.TRUSTED_CLUSTER_DELETED]: RawEvent<
    typeof eventCodes.TRUSTED_CLUSTER_DELETED,
    {
      name: string;
    }
  >;
  [eventCodes.KUBE_REQUEST]: RawEvent<
    typeof eventCodes.KUBE_REQUEST,
    {
      kubernetes_cluster: string;
    }
  >;
  [eventCodes.DATABASE_SESSION_STARTED]: RawEvent<
    typeof eventCodes.DATABASE_SESSION_STARTED,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
    }
  >;
  [eventCodes.DATABASE_SESSION_STARTED_FAILURE]: RawEvent<
    typeof eventCodes.DATABASE_SESSION_STARTED_FAILURE,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
    }
  >;
  [eventCodes.DATABASE_SESSION_ENDED]: RawEvent<
    typeof eventCodes.DATABASE_SESSION_ENDED,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
    }
  >;
  [eventCodes.DATABASE_SESSION_QUERY]: RawEvent<
    typeof eventCodes.DATABASE_SESSION_QUERY,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
      db_query: string;
    }
  >;
  [eventCodes.DATABASE_SESSION_QUERY_FAILURE]: RawEvent<
    typeof eventCodes.DATABASE_SESSION_QUERY_FAILURE,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
      db_query: string;
    }
  >;
  [eventCodes.DATABASE_CREATED]: RawEvent<
    typeof eventCodes.DATABASE_CREATED,
    {
      name: string;
    }
  >;
  [eventCodes.DATABASE_UPDATED]: RawEvent<
    typeof eventCodes.DATABASE_UPDATED,
    {
      name: string;
    }
  >;
  [eventCodes.DATABASE_DELETED]: RawEvent<
    typeof eventCodes.DATABASE_DELETED,
    {
      name: string;
    }
  >;
  [eventCodes.POSTGRES_PARSE]: RawEvent<
    typeof eventCodes.POSTGRES_PARSE,
    {
      name: string;
      db_service: string;
      statement_name: string;
      query: string;
    }
  >;
  [eventCodes.POSTGRES_BIND]: RawEvent<
    typeof eventCodes.POSTGRES_BIND,
    {
      name: string;
      db_service: string;
      statement_name: string;
      portal_name: string;
    }
  >;
  [eventCodes.POSTGRES_EXECUTE]: RawEvent<
    typeof eventCodes.POSTGRES_EXECUTE,
    {
      name: string;
      db_service: string;
      portal_name: string;
    }
  >;
  [eventCodes.POSTGRES_CLOSE]: RawEvent<
    typeof eventCodes.POSTGRES_CLOSE,
    {
      name: string;
      db_service: string;
      statement_name: string;
      portal_name: string;
    }
  >;
  [eventCodes.POSTGRES_FUNCTION_CALL]: RawEvent<
    typeof eventCodes.POSTGRES_FUNCTION_CALL,
    {
      name: string;
      db_service: string;
      function_oid: string;
    }
  >;
  [eventCodes.MYSQL_STATEMENT_PREPARE]: RawEvent<
    typeof eventCodes.MYSQL_STATEMENT_PREPARE,
    {
      db_service: string;
      db_name: string;
      query: string;
    }
  >;
  [eventCodes.MYSQL_STATEMENT_EXECUTE]: RawEvent<
    typeof eventCodes.MYSQL_STATEMENT_EXECUTE,
    {
      db_service: string;
      db_name: string;
      statement_id: number;
    }
  >;
  [eventCodes.MYSQL_STATEMENT_SEND_LONG_DATA]: RawEvent<
    typeof eventCodes.MYSQL_STATEMENT_SEND_LONG_DATA,
    {
      db_service: string;
      db_name: string;
      statement_id: number;
      parameter_id: number;
      data_size: number;
    }
  >;
  [eventCodes.MYSQL_STATEMENT_CLOSE]: RawEvent<
    typeof eventCodes.MYSQL_STATEMENT_CLOSE,
    {
      db_service: string;
      db_name: string;
      statement_id: number;
    }
  >;
  [eventCodes.MYSQL_STATEMENT_RESET]: RawEvent<
    typeof eventCodes.MYSQL_STATEMENT_RESET,
    {
      db_service: string;
      db_name: string;
      statement_id: number;
    }
  >;
  [eventCodes.MYSQL_STATEMENT_FETCH]: RawEvent<
    typeof eventCodes.MYSQL_STATEMENT_FETCH,
    {
      db_service: string;
      db_name: string;
      rows_count: number;
      statement_id: number;
    }
  >;
  [eventCodes.MYSQL_STATEMENT_BULK_EXECUTE]: RawEvent<
    typeof eventCodes.MYSQL_STATEMENT_BULK_EXECUTE,
    {
      db_service: string;
      db_name: string;
      statement_id: number;
    }
  >;
  [eventCodes.MFA_DEVICE_ADD]: RawEvent<
    typeof eventCodes.MFA_DEVICE_ADD,
    {
      mfa_device_name: string;
      mfa_device_uuid: string;
      mfa_device_type: string;
    }
  >;
  [eventCodes.MFA_DEVICE_DELETE]: RawEvent<
    typeof eventCodes.MFA_DEVICE_DELETE,
    {
      mfa_device_name: string;
      mfa_device_uuid: string;
      mfa_device_type: string;
    }
  >;
  [eventCodes.LOCK_CREATED]: RawEvent<
    typeof eventCodes.LOCK_CREATED,
    { name: string }
  >;
  [eventCodes.LOCK_DELETED]: RawEvent<
    typeof eventCodes.LOCK_DELETED,
    { name: string }
  >;
  [eventCodes.PRIVILEGE_TOKEN_CREATED]: RawEventUserToken<
    typeof eventCodes.PRIVILEGE_TOKEN_CREATED
  >;
  [eventCodes.RECOVERY_TOKEN_CREATED]: RawEventUserToken<
    typeof eventCodes.RECOVERY_TOKEN_CREATED
  >;
  [eventCodes.RECOVERY_CODE_GENERATED]: RawEvent<
    typeof eventCodes.RECOVERY_CODE_GENERATED
  >;
  [eventCodes.RECOVERY_CODE_USED]: RawEvent<
    typeof eventCodes.RECOVERY_CODE_USED
  >;
  [eventCodes.RECOVERY_CODE_USED_FAILURE]: RawEvent<
    typeof eventCodes.RECOVERY_CODE_USED_FAILURE
  >;
  [eventCodes.DESKTOP_SESSION_STARTED]: RawEvent<
    typeof eventCodes.DESKTOP_SESSION_STARTED,
    {
      desktop_addr: string;
      windows_user: string;
      windows_domain: string;
    }
  >;
  [eventCodes.DESKTOP_SESSION_STARTED_FAILED]: RawEvent<
    typeof eventCodes.DESKTOP_SESSION_STARTED_FAILED,
    {
      desktop_addr: string;
      windows_user: string;
      windows_domain: string;
    }
  >;
  [eventCodes.DESKTOP_SESSION_ENDED]: RawEvent<
    typeof eventCodes.DESKTOP_SESSION_ENDED,
    {
      desktop_addr: string;
      windows_user: string;
      windows_domain: string;
    }
  >;
  [eventCodes.DESKTOP_CLIPBOARD_RECEIVE]: RawEvent<
    typeof eventCodes.DESKTOP_CLIPBOARD_RECEIVE,
    {
      desktop_addr: string;
      length: number;
      windows_domain: string;
    }
  >;
  [eventCodes.DESKTOP_CLIPBOARD_SEND]: RawEvent<
    typeof eventCodes.DESKTOP_CLIPBOARD_SEND,
    {
      desktop_addr: string;
      length: number;
      windows_domain: string;
    }
  >;
  [eventCodes.UNKNOWN]: RawEvent<
    typeof eventCodes.UNKNOWN,
    {
      unknown_type: string;
      unknown_code: string;
      data: string;
    }
  >;
  [eventCodes.X11_FORWARD]: RawEvent<typeof eventCodes.X11_FORWARD>;
  [eventCodes.X11_FORWARD_FAILURE]: RawEvent<
    typeof eventCodes.X11_FORWARD_FAILURE
  >;
  [eventCodes.SESSION_CONNECT]: RawEvent<
    typeof eventCodes.SESSION_CONNECT,
    { server_addr: string }
  >;
  [eventCodes.CERTIFICATE_CREATED]: RawEvent<
    typeof eventCodes.CERTIFICATE_CREATED,
    {
      cert_type: 'user';
      identity: { user: string };
    }
  >;
};

/**
 * Event Code
 */
export type EventCode = typeof eventCodes[keyof typeof eventCodes];

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
export type RawEvent<T extends EventCode, K = unknown> = Merge<
  {
    code: T;
    user: string;
    time: string;
    uid: string;
    event: string;
  },
  K
>;

type RawEventData<T extends EventCode> = RawEvent<
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

type RawEventCommand<T extends EventCode> = RawEvent<
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

type RawEventNetwork<T extends EventCode> = RawEvent<
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

type RawEventProcessExit<T extends EventCode> = RawEvent<
  T,
  {
    login: string;
    namespace: string;
    pid: number;
    program: string;
    exit_status: number;
    server_id: string;
    sid: string;
  }
>;

type RawDiskEvent<T extends EventCode> = RawEvent<
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

type RawEventAccess<T extends EventCode> = RawEvent<
  T,
  {
    id: string;
    user: string;
    roles: string[];
    state: string;
    reviewer: string;
  }
>;

type RawEventUserToken<T extends EventCode> = RawEvent<
  T,
  {
    name: string;
    ttl: string;
  }
>;

type RawEventUser<T extends EventCode> = RawEvent<
  T,
  {
    name: string;
  }
>;

type RawEventConnector<T extends EventCode> = RawEvent<
  T,
  {
    name: string;
    user: string;
  }
>;

type RawEventAuthFailure<T extends EventCode> = RawEvent<
  T,
  {
    error: string;
  }
>;

/**
 * A map of event formatters that provide short and long description
 */
export type Formatters = {
  [key in EventCode]: {
    type: string;
    desc: string;
    format: (json: RawEvents[key]) => string;
  };
};

export type Events = {
  [key in EventCode]: {
    id: string;
    time: Date;
    user: string;
    message: string;
    code: key;
    codeDesc: string;
    raw: RawEvents[key];
  };
};

export type Event = Events[EventCode];

export type SessionEnd = Events[typeof eventCodes.SESSION_END];

export type EventQuery = {
  from: Date;
  to: Date;
  limit?: number;
  startKey?: string;
  filterBy?: string;
};

export type EventResponse = {
  events: Event[];
  startKey: string;
};
