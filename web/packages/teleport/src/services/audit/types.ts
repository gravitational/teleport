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
  SESSION_LEAVE: 'T2003I',
  SESSION_START: 'T2000I',
  SESSION_UPLOAD: 'T2005I',
  SESSION_DATA: 'T2006I',
  SESSION_COMMAND: 'T4000I',
  SESSION_DISK: 'T4001I',
  SESSION_NETWORK: 'T4002I',
  SUBSYSTEM_FAILURE: 'T3001E',
  SUBSYSTEM: 'T3001I',
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

  // Gravity
  G_ALERT_CREATED: 'G1007I',
  G_ALERT_DELETED: 'G2007I',
  G_ALERT_TARGET_CREATED: 'G1008I',
  G_ALERT_TARGET_DELETED: 'G2008I',
  G_APPLICATION_INSTALL: 'G4000I',
  G_APPLICATION_ROLLBACK: 'G4002I',
  G_APPLICATION_UNINSTALL: 'G4003I',
  G_APPLICATION_UPGRADE: 'G4001I',
  G_AUTHGATEWAY_UPDATED: 'G1009I',
  G_AUTHPREFERENCE_UPDATED: 'G1005I',
  G_CLUSTER_HEALTHY: 'G3001I',
  G_CLUSTER_UNHEALTHY: 'G3000W',
  G_LOGFORWARDER_CREATED: 'G1003I',
  G_LOGFORWARDER_DELETED: 'G2003I',
  G_OPERATION_CONFIG_COMPLETE: 'G0016I',
  G_OPERATION_CONFIG_FAILURE: 'G0016E',
  G_OPERATION_CONFIG_START: 'G0015I',
  G_OPERATION_ENV_COMPLETE: 'G0014I',
  G_OPERATION_ENV_FAILURE: 'G0014E',
  G_OPERATION_ENV_START: 'G0013I',
  G_OPERATION_EXPAND_COMPLETE: 'G0004I',
  G_OPERATION_EXPAND_FAILURE: 'G0004E',
  G_OPERATION_EXPAND_START: 'G0003I',
  G_OPERATION_GC_COMPLETE: 'G0012I',
  G_OPERATION_GC_FAILURE: 'G0012E',
  G_OPERATION_GC_START: 'G0011I',
  G_OPERATION_INSTALL_COMPLETE: 'G0002I',
  G_OPERATION_INSTALL_FAILURE: 'G0002E',
  G_OPERATION_INSTALL_START: 'G0001I',
  G_OPERATION_SHRINK_COMPLETE: 'G0006I',
  G_OPERATION_SHRINK_FAILURE: 'G0006E',
  G_OPERATION_SHRINK_START: 'G0005I',
  G_OPERATION_UNINSTALL_COMPLETE: 'G0010I',
  G_OPERATION_UNINSTALL_FAILURE: 'G0010E',
  G_OPERATION_UNINSTALL_START: 'G0009I',
  G_OPERATION_UPDATE_COMPLETE: 'G0008I',
  G_OPERATION_UPDATE_FAILURE: 'G0008E',
  G_OPERATION_UPDATE_START: 'G0007I',
  G_ROLE_CREATED: 'GE1000I',
  G_ROLE_DELETED: 'GE2000I',
  G_SMTPCONFIG_CREATED: 'G1006I',
  G_SMTPCONFIG_DELETED: 'G2006I',
  G_TLSKEYPAIR_CREATED: 'G1004I',
  G_TLSKEYPAIR_DELETED: 'G2004I',
  G_TOKEN_CREATED: 'G1001I',
  G_TOKEN_DELETED: 'G2001I',
  G_USER_CREATED: 'G1000I',
  G_USER_DELETED: 'G2000I',
  G_USER_INVITE_CREATED: 'G1010I',
  G_ENDPOINTS_UPDATED: 'GE1003I',
  G_LICENSE_EXPIRED: 'GE3003I',
  G_LICENSE_GENERATED: 'GE3002I',
  G_LICENSE_UPDATED: 'GE3004I',
  G_GITHUB_CONNECTOR_CREATED: 'G1002I',
  G_GITHUB_CONNECTOR_DELETED: 'G2002I',
  G_OIDC_CONNECTOR_CREATED: 'GE1001I',
  G_OIDC_CONNECTOR_DELETED: 'GE2001I',
  G_REMOTE_SUPPORT_DISABLED: 'GE3001I',
  G_REMOTE_SUPPORT_ENABLED: 'GE3000I',
  G_SAML_CONNECTOR_CREATED: 'GE1002I',
  G_SAML_CONNECTOR_DELETED: 'GE2002I',
  G_UPDATES_DISABLED: 'GE3006I',
  G_UPDATES_DOWNLOADED: 'GE3007I',
  G_UPDATES_ENABLED: 'GE3005I',
} as const;

/**
 * Describes all raw event types
 */
export type RawEvents = {
  [CodeEnum.G_ALERT_CREATED]: RawEventAlert<typeof CodeEnum.G_ALERT_CREATED>;
  [CodeEnum.G_ALERT_DELETED]: RawEventAlert<typeof CodeEnum.G_ALERT_DELETED>;
  [CodeEnum.ACCESS_REQUEST_CREATED]: RawEventAccess<
    typeof CodeEnum.ACCESS_REQUEST_CREATED
  >;
  [CodeEnum.ACCESS_REQUEST_UPDATED]: RawEventAccess<
    typeof CodeEnum.ACCESS_REQUEST_UPDATED
  >;
  [CodeEnum.G_ALERT_TARGET_CREATED]: RawEventAlert<
    typeof CodeEnum.G_ALERT_TARGET_CREATED
  >;
  [CodeEnum.G_ALERT_TARGET_DELETED]: RawEvent<
    typeof CodeEnum.G_ALERT_TARGET_DELETED
  >;
  [CodeEnum.G_APPLICATION_INSTALL]: RawEventApplication<
    typeof CodeEnum.G_APPLICATION_INSTALL
  >;
  [CodeEnum.G_APPLICATION_UPGRADE]: RawEventApplication<
    typeof CodeEnum.G_APPLICATION_UPGRADE
  >;
  [CodeEnum.G_APPLICATION_ROLLBACK]: RawEventApplication<
    typeof CodeEnum.G_APPLICATION_ROLLBACK
  >;
  [CodeEnum.G_APPLICATION_UNINSTALL]: RawEventApplication<
    typeof CodeEnum.G_APPLICATION_UNINSTALL
  >;

  [CodeEnum.AUTH_ATTEMPT_FAILURE]: RawEventAuthFailure<
    typeof CodeEnum.AUTH_ATTEMPT_FAILURE
  >;
  [CodeEnum.G_AUTHGATEWAY_UPDATED]: RawEvent<
    typeof CodeEnum.G_AUTHGATEWAY_UPDATED
  >;
  [CodeEnum.G_AUTHPREFERENCE_UPDATED]: RawEvent<
    typeof CodeEnum.G_AUTHPREFERENCE_UPDATED
  >;
  [CodeEnum.CLIENT_DISCONNECT]: RawEvent<
    typeof CodeEnum.CLIENT_DISCONNECT,
    { reason: string }
  >;
  [CodeEnum.G_CLUSTER_HEALTHY]: RawEvent<
    typeof CodeEnum.G_CLUSTER_HEALTHY,
    { reason: string }
  >;
  [CodeEnum.G_CLUSTER_UNHEALTHY]: RawEvent<
    typeof CodeEnum.G_CLUSTER_UNHEALTHY,
    { reason: string }
  >;
  [CodeEnum.G_ENDPOINTS_UPDATED]: RawEvent<typeof CodeEnum.G_ENDPOINTS_UPDATED>;
  [CodeEnum.EXEC]: RawEvent<typeof CodeEnum.EXEC>;
  [CodeEnum.EXEC_FAILURE]: RawEvent<
    typeof CodeEnum.EXEC_FAILURE,
    { exitError: string }
  >;
  [CodeEnum.GITHUB_CONNECTOR_CREATED]: RawEventConnector<
    typeof CodeEnum.GITHUB_CONNECTOR_CREATED
  >;
  [CodeEnum.GITHUB_CONNECTOR_DELETED]: RawEventConnector<
    typeof CodeEnum.GITHUB_CONNECTOR_DELETED
  >;
  [CodeEnum.G_GITHUB_CONNECTOR_CREATED]: RawEventConnector<
    typeof CodeEnum.G_GITHUB_CONNECTOR_CREATED
  >;
  [CodeEnum.G_GITHUB_CONNECTOR_DELETED]: RawEventConnector<
    typeof CodeEnum.G_GITHUB_CONNECTOR_DELETED
  >;
  [CodeEnum.G_LICENSE_GENERATED]: RawEvent<
    typeof CodeEnum.G_LICENSE_GENERATED,
    { maxNodes: number }
  >;
  [CodeEnum.G_LICENSE_EXPIRED]: RawEvent<typeof CodeEnum.G_LICENSE_EXPIRED>;
  [CodeEnum.G_LICENSE_UPDATED]: RawEvent<typeof CodeEnum.G_LICENSE_UPDATED>;
  [CodeEnum.G_LOGFORWARDER_CREATED]: RawEvent<
    typeof CodeEnum.G_LOGFORWARDER_CREATED,
    HasName
  >;

  [CodeEnum.G_LOGFORWARDER_DELETED]: RawEvent<
    typeof CodeEnum.G_LOGFORWARDER_DELETED,
    HasName
  >;
  [CodeEnum.OIDC_CONNECTOR_CREATED]: RawEventConnector<
    typeof CodeEnum.OIDC_CONNECTOR_CREATED
  >;
  [CodeEnum.OIDC_CONNECTOR_DELETED]: RawEventConnector<
    typeof CodeEnum.OIDC_CONNECTOR_DELETED
  >;
  [CodeEnum.G_OIDC_CONNECTOR_CREATED]: RawEventConnector<
    typeof CodeEnum.G_OIDC_CONNECTOR_CREATED
  >;
  [CodeEnum.G_OIDC_CONNECTOR_DELETED]: RawEventConnector<
    typeof CodeEnum.G_OIDC_CONNECTOR_DELETED
  >;
  [CodeEnum.G_OPERATION_CONFIG_COMPLETE]: RawEvent<
    typeof CodeEnum.G_OPERATION_CONFIG_COMPLETE
  >;
  [CodeEnum.G_OPERATION_CONFIG_FAILURE]: RawEvent<
    typeof CodeEnum.G_OPERATION_CONFIG_FAILURE
  >;
  [CodeEnum.G_OPERATION_CONFIG_START]: RawEvent<
    typeof CodeEnum.G_OPERATION_CONFIG_START
  >;
  [CodeEnum.G_OPERATION_ENV_COMPLETE]: RawEvent<
    typeof CodeEnum.G_OPERATION_ENV_COMPLETE
  >;
  [CodeEnum.G_OPERATION_ENV_FAILURE]: RawEvent<
    typeof CodeEnum.G_OPERATION_ENV_FAILURE
  >;
  [CodeEnum.G_OPERATION_ENV_START]: RawEvent<
    typeof CodeEnum.G_OPERATION_ENV_START
  >;
  [CodeEnum.G_OPERATION_EXPAND_START]: RawEventOperation<
    typeof CodeEnum.G_OPERATION_EXPAND_START
  >;
  [CodeEnum.G_OPERATION_EXPAND_COMPLETE]: RawEventOperation<
    typeof CodeEnum.G_OPERATION_EXPAND_COMPLETE
  >;
  [CodeEnum.G_OPERATION_EXPAND_FAILURE]: RawEventOperation<
    typeof CodeEnum.G_OPERATION_EXPAND_FAILURE
  >;
  [CodeEnum.G_OPERATION_GC_START]: RawEvent<
    typeof CodeEnum.G_OPERATION_GC_START
  >;
  [CodeEnum.G_OPERATION_GC_COMPLETE]: RawEvent<
    typeof CodeEnum.G_OPERATION_GC_COMPLETE
  >;
  [CodeEnum.G_OPERATION_GC_FAILURE]: RawEvent<
    typeof CodeEnum.G_OPERATION_GC_FAILURE
  >;
  [CodeEnum.G_OPERATION_INSTALL_START]: RawEvent<
    typeof CodeEnum.G_OPERATION_INSTALL_START,
    HasCluster
  >;
  [CodeEnum.G_OPERATION_INSTALL_COMPLETE]: RawEvent<
    typeof CodeEnum.G_OPERATION_INSTALL_COMPLETE,
    HasCluster
  >;
  [CodeEnum.G_OPERATION_INSTALL_FAILURE]: RawEvent<
    typeof CodeEnum.G_OPERATION_INSTALL_FAILURE,
    HasCluster
  >;
  [CodeEnum.G_OPERATION_SHRINK_START]: RawEvent<
    typeof CodeEnum.G_OPERATION_SHRINK_START,
    {
      hostname: string;
      ip: string;
      role: string;
    }
  >;
  [CodeEnum.G_OPERATION_SHRINK_COMPLETE]: RawEvent<
    typeof CodeEnum.G_OPERATION_SHRINK_COMPLETE,
    {
      hostname: string;
      ip: string;
      role: string;
    }
  >;
  [CodeEnum.G_OPERATION_SHRINK_FAILURE]: RawEvent<
    typeof CodeEnum.G_OPERATION_SHRINK_FAILURE,
    {
      hostname: string;
      ip: string;
      role: string;
    }
  >;
  [CodeEnum.G_OPERATION_UNINSTALL_START]: RawEvent<
    typeof CodeEnum.G_OPERATION_UNINSTALL_START
  >;
  [CodeEnum.G_OPERATION_UNINSTALL_COMPLETE]: RawEvent<
    typeof CodeEnum.G_OPERATION_UNINSTALL_COMPLETE
  >;
  [CodeEnum.G_OPERATION_UNINSTALL_FAILURE]: RawEvent<
    typeof CodeEnum.G_OPERATION_UNINSTALL_FAILURE
  >;
  [CodeEnum.G_OPERATION_UPDATE_COMPLETE]: RawEvent<
    typeof CodeEnum.G_OPERATION_UPDATE_COMPLETE,
    {
      version: string;
    }
  >;
  [CodeEnum.G_OPERATION_UPDATE_FAILURE]: RawEvent<
    typeof CodeEnum.G_OPERATION_UPDATE_FAILURE,
    {
      version: string;
    }
  >;
  [CodeEnum.G_OPERATION_UPDATE_START]: RawEvent<
    typeof CodeEnum.G_OPERATION_UPDATE_START,
    {
      version: string;
    }
  >;
  [CodeEnum.PORTFORWARD]: RawEvent<typeof CodeEnum.PORTFORWARD>;
  [CodeEnum.PORTFORWARD_FAILURE]: RawEvent<
    typeof CodeEnum.PORTFORWARD_FAILURE,
    {
      error: string;
    }
  >;
  [CodeEnum.G_REMOTE_SUPPORT_ENABLED]: RawEvent<
    typeof CodeEnum.G_REMOTE_SUPPORT_ENABLED,
    HasHub
  >;
  [CodeEnum.G_REMOTE_SUPPORT_DISABLED]: RawEvent<
    typeof CodeEnum.G_REMOTE_SUPPORT_DISABLED,
    HasHub
  >;
  [CodeEnum.SAML_CONNECTOR_CREATED]: RawEventConnector<
    typeof CodeEnum.SAML_CONNECTOR_CREATED
  >;
  [CodeEnum.SAML_CONNECTOR_DELETED]: RawEventConnector<
    typeof CodeEnum.SAML_CONNECTOR_DELETED
  >;
  [CodeEnum.G_SAML_CONNECTOR_CREATED]: RawEventConnector<
    typeof CodeEnum.G_SAML_CONNECTOR_CREATED
  >;
  [CodeEnum.G_SAML_CONNECTOR_DELETED]: RawEventConnector<
    typeof CodeEnum.G_SAML_CONNECTOR_DELETED
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
  [CodeEnum.SESSION_UPLOAD]: RawEvent<
    typeof CodeEnum.SESSION_UPLOAD,
    {
      sid: string;
    }
  >;
  [CodeEnum.G_SMTPCONFIG_CREATED]: RawEvent<
    typeof CodeEnum.G_SMTPCONFIG_CREATED
  >;
  [CodeEnum.G_SMTPCONFIG_DELETED]: RawEvent<
    typeof CodeEnum.G_SMTPCONFIG_DELETED
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
  [CodeEnum.G_TLSKEYPAIR_CREATED]: RawEvent<
    typeof CodeEnum.G_TLSKEYPAIR_CREATED
  >;
  [CodeEnum.G_TLSKEYPAIR_DELETED]: RawEvent<
    typeof CodeEnum.G_TLSKEYPAIR_DELETED
  >;
  [CodeEnum.G_TOKEN_CREATED]: RawEvent<
    typeof CodeEnum.G_TOKEN_CREATED,
    {
      owner: string;
    }
  >;
  [CodeEnum.G_TOKEN_DELETED]: RawEvent<
    typeof CodeEnum.G_TOKEN_DELETED,
    {
      owner: string;
    }
  >;
  [CodeEnum.G_UPDATES_ENABLED]: RawEvent<
    typeof CodeEnum.G_UPDATES_ENABLED,
    HasHub
  >;
  [CodeEnum.G_UPDATES_DISABLED]: RawEvent<
    typeof CodeEnum.G_UPDATES_DISABLED,
    HasHub
  >;
  [CodeEnum.G_UPDATES_DOWNLOADED]: RawEvent<
    typeof CodeEnum.G_UPDATES_DOWNLOADED,
    {
      hub: string;
      name: string;
      version: string;
    }
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
  [CodeEnum.G_USER_CREATED]: RawEvent<typeof CodeEnum.G_USER_CREATED, HasName>;
  [CodeEnum.G_USER_DELETED]: RawEvent<typeof CodeEnum.G_USER_DELETED, HasName>;
  [CodeEnum.G_USER_INVITE_CREATED]: RawEvent<
    typeof CodeEnum.G_USER_INVITE_CREATED,
    {
      name: string;
      roles: string;
    }
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
  [CodeEnum.G_ROLE_CREATED]: RawEvent<typeof CodeEnum.G_ROLE_CREATED, HasName>;
  [CodeEnum.G_ROLE_DELETED]: RawEvent<typeof CodeEnum.G_ROLE_DELETED, HasName>;
  [CodeEnum.TRUSTED_CLUSTER_TOKEN_CREATED]: RawEvent<
    typeof CodeEnum.TRUSTED_CLUSTER_TOKEN_CREATED
  >;
};

/**
 * Event Code
 */
export type Code = typeof CodeEnum[keyof typeof CodeEnum];

type HasName = {
  name: string;
};

type HasHub = {
  hub: string;
};

type HasCluster = {
  cluster: string;
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

type RawEventAlert<T extends Code> = RawEvent<
  T,
  {
    name: string;
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

type RawEventApplication<T extends Code> = RawEvent<
  T,
  {
    releaseName: string;
    name: string;
    version: string;
  }
>;

type RawEventAuthFailure<T extends Code> = RawEvent<
  T,
  {
    error: string;
  }
>;

type RawEventOperation<T extends Code> = RawEvent<
  T,
  {
    hostname: string;
    ip: string;
    role: string;
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
