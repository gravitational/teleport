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
  SUBSYSTEM_FAILURE: 'T3001E',
  SUBSYSTEM: 'T3001I',
  TERMINAL_RESIZE: 'T2002I',
  USER_LOCAL_LOGIN: 'T1000I',
  USER_LOCAL_LOGINFAILURE: 'T1000W',
  USER_SSO_LOGIN: 'T1001I',
  USER_SSO_LOGINFAILURE: 'T1001W',
  // Gravity Oss
  ALERT_CREATED: 'G1007I',
  ALERT_DELETED: 'G2007I',
  ALERT_TARGET_CREATED: 'G1008I',
  ALERT_TARGET_DELETED: 'G2008I',
  APPLICATION_INSTALL: 'G4000I',
  APPLICATION_ROLLBACK: 'G4002I',
  APPLICATION_UNINSTALL: 'G4003I',
  APPLICATION_UPGRADE: 'G4001I',
  AUTHGATEWAY_UPDATED: 'G1009I',
  AUTHPREFERENCE_UPDATED: 'G1005I',
  CLUSTER_HEALTHY: 'G3001I',
  CLUSTER_UNHEALTHY: 'G3000W',
  GITHUB_CONNECTOR_CREATED: 'G1002I',
  GITHUB_CONNECTOR_DELETED: 'G2002I',
  LOGFORWARDER_CREATED: 'G1003I',
  LOGFORWARDER_DELETED: 'G2003I',
  OPERATION_CONFIG_COMPLETE: 'G0016I',
  OPERATION_CONFIG_FAILURE: 'G0016E',
  OPERATION_CONFIG_START: 'G0015I',
  OPERATION_ENV_COMPLETE: 'G0014I',
  OPERATION_ENV_FAILURE: 'G0014E',
  OPERATION_ENV_START: 'G0013I',
  OPERATION_EXPAND_COMPLETE: 'G0004I',
  OPERATION_EXPAND_FAILURE: 'G0004E',
  OPERATION_EXPAND_START: 'G0003I',
  OPERATION_GC_COMPLETE: 'G0012I',
  OPERATION_GC_FAILURE: 'G0012E',
  OPERATION_GC_START: 'G0011I',
  OPERATION_INSTALL_COMPLETE: 'G0002I',
  OPERATION_INSTALL_FAILURE: 'G0002E',
  OPERATION_INSTALL_START: 'G0001I',
  OPERATION_SHRINK_COMPLETE: 'G0006I',
  OPERATION_SHRINK_FAILURE: 'G0006E',
  OPERATION_SHRINK_START: 'G0005I',
  OPERATION_UNINSTALL_COMPLETE: 'G0010I',
  OPERATION_UNINSTALL_FAILURE: 'G0010E',
  OPERATION_UNINSTALL_START: 'G0009I',
  OPERATION_UPDATE_COMPLETE: 'G0008I',
  OPERATION_UPDATE_FAILURE: 'G0008E',
  OPERATION_UPDATE_START: 'G0007I',
  ROLE_CREATED: 'GE1000I',
  ROLE_DELETED: 'GE2000I',
  SMTPCONFIG_CREATED: 'G1006I',
  SMTPCONFIG_DELETED: 'G2006I',
  TLSKEYPAIR_CREATED: 'G1004I',
  TLSKEYPAIR_DELETED: 'G2004I',
  TOKEN_CREATED: 'G1001I',
  TOKEN_DELETED: 'G2001I',
  USER_CREATED: 'G1000I',
  USER_DELETED: 'G2000I',
  USER_INVITE_CREATED: 'G1010I',
  // Gravity E
  ENDPOINTS_UPDATED: 'GE1003I',
  LICENSE_EXPIRED: 'GE3003I',
  LICENSE_GENERATED: 'GE3002I',
  LICENSE_UPDATED: 'GE3004I',
  OIDC_CONNECTOR_CREATED: 'GE1001I',
  OIDC_CONNECTOR_DELETED: 'GE2001I',
  REMOTE_SUPPORT_DISABLED: 'GE3001I',
  REMOTE_SUPPORT_ENABLED: 'GE3000I',
  SAML_CONNECTOR_CREATED: 'GE1002I',
  SAML_CONNECTOR_DELETED: 'GE2002I',
  UPDATES_DISABLED: 'GE3006I',
  UPDATES_DOWNLOADED: 'GE3007I',
  UPDATES_ENABLED: 'GE3005I',
} as const;

/**
 * Describes all raw event types
 */
export type RawEvents = {
  [CodeEnum.ALERT_CREATED]: RawEventAlert<typeof CodeEnum.ALERT_CREATED>;
  [CodeEnum.ALERT_DELETED]: RawEventAlert<typeof CodeEnum.ALERT_DELETED>;
  [CodeEnum.ALERT_TARGET_CREATED]: RawEventAlert<
    typeof CodeEnum.ALERT_TARGET_CREATED
  >;
  [CodeEnum.ALERT_TARGET_DELETED]: RawEvent<
    typeof CodeEnum.ALERT_TARGET_DELETED
  >;
  [CodeEnum.APPLICATION_INSTALL]: RawEventApplication<
    typeof CodeEnum.APPLICATION_INSTALL
  >;
  [CodeEnum.APPLICATION_UPGRADE]: RawEventApplication<
    typeof CodeEnum.APPLICATION_UPGRADE
  >;
  [CodeEnum.APPLICATION_ROLLBACK]: RawEventApplication<
    typeof CodeEnum.APPLICATION_ROLLBACK
  >;
  [CodeEnum.APPLICATION_UNINSTALL]: RawEventApplication<
    typeof CodeEnum.APPLICATION_UNINSTALL
  >;

  [CodeEnum.AUTH_ATTEMPT_FAILURE]: RawEventAuthFailure<
    typeof CodeEnum.AUTH_ATTEMPT_FAILURE
  >;
  [CodeEnum.AUTHGATEWAY_UPDATED]: RawEvent<typeof CodeEnum.AUTHGATEWAY_UPDATED>;
  [CodeEnum.AUTHPREFERENCE_UPDATED]: RawEvent<
    typeof CodeEnum.AUTHPREFERENCE_UPDATED
  >;
  [CodeEnum.CLIENT_DISCONNECT]: RawEvent<
    typeof CodeEnum.CLIENT_DISCONNECT,
    { reason: string }
  >;
  [CodeEnum.CLUSTER_HEALTHY]: RawEvent<
    typeof CodeEnum.CLUSTER_HEALTHY,
    { reason: string }
  >;
  [CodeEnum.CLUSTER_UNHEALTHY]: RawEvent<
    typeof CodeEnum.CLUSTER_UNHEALTHY,
    { reason: string }
  >;
  [CodeEnum.ENDPOINTS_UPDATED]: RawEvent<typeof CodeEnum.ENDPOINTS_UPDATED>;
  [CodeEnum.EXEC]: RawEvent<typeof CodeEnum.EXEC>;
  [CodeEnum.EXEC_FAILURE]: RawEvent<
    typeof CodeEnum.EXEC_FAILURE,
    { exitError: string }
  >;
  [CodeEnum.GITHUB_CONNECTOR_CREATED]: RawEvent<
    typeof CodeEnum.GITHUB_CONNECTOR_CREATED,
    HasName
  >;
  [CodeEnum.GITHUB_CONNECTOR_DELETED]: RawEvent<
    typeof CodeEnum.GITHUB_CONNECTOR_DELETED,
    HasName
  >;
  [CodeEnum.LICENSE_GENERATED]: RawEvent<
    typeof CodeEnum.LICENSE_GENERATED,
    { maxNodes: number }
  >;
  [CodeEnum.LICENSE_EXPIRED]: RawEvent<typeof CodeEnum.LICENSE_EXPIRED>;
  [CodeEnum.LICENSE_UPDATED]: RawEvent<typeof CodeEnum.LICENSE_UPDATED>;
  [CodeEnum.LOGFORWARDER_CREATED]: RawEvent<
    typeof CodeEnum.LOGFORWARDER_CREATED,
    HasName
  >;

  [CodeEnum.LOGFORWARDER_DELETED]: RawEvent<
    typeof CodeEnum.LOGFORWARDER_DELETED,
    HasName
  >;
  [CodeEnum.OIDC_CONNECTOR_CREATED]: RawEvent<
    typeof CodeEnum.OIDC_CONNECTOR_CREATED,
    HasName
  >;
  [CodeEnum.OIDC_CONNECTOR_DELETED]: RawEvent<
    typeof CodeEnum.OIDC_CONNECTOR_DELETED,
    HasName
  >;
  [CodeEnum.OPERATION_CONFIG_COMPLETE]: RawEvent<
    typeof CodeEnum.OPERATION_CONFIG_COMPLETE
  >;
  [CodeEnum.OPERATION_CONFIG_FAILURE]: RawEvent<
    typeof CodeEnum.OPERATION_CONFIG_FAILURE
  >;
  [CodeEnum.OPERATION_CONFIG_START]: RawEvent<
    typeof CodeEnum.OPERATION_CONFIG_START
  >;
  [CodeEnum.OPERATION_ENV_COMPLETE]: RawEvent<
    typeof CodeEnum.OPERATION_ENV_COMPLETE
  >;
  [CodeEnum.OPERATION_ENV_FAILURE]: RawEvent<
    typeof CodeEnum.OPERATION_ENV_FAILURE
  >;
  [CodeEnum.OPERATION_ENV_START]: RawEvent<typeof CodeEnum.OPERATION_ENV_START>;
  [CodeEnum.OPERATION_EXPAND_START]: RawEventOperation<
    typeof CodeEnum.OPERATION_EXPAND_START
  >;
  [CodeEnum.OPERATION_EXPAND_COMPLETE]: RawEventOperation<
    typeof CodeEnum.OPERATION_EXPAND_COMPLETE
  >;
  [CodeEnum.OPERATION_EXPAND_FAILURE]: RawEventOperation<
    typeof CodeEnum.OPERATION_EXPAND_FAILURE
  >;
  [CodeEnum.OPERATION_GC_START]: RawEvent<typeof CodeEnum.OPERATION_GC_START>;
  [CodeEnum.OPERATION_GC_COMPLETE]: RawEvent<
    typeof CodeEnum.OPERATION_GC_COMPLETE
  >;
  [CodeEnum.OPERATION_GC_FAILURE]: RawEvent<
    typeof CodeEnum.OPERATION_GC_FAILURE
  >;
  [CodeEnum.OPERATION_INSTALL_START]: RawEvent<
    typeof CodeEnum.OPERATION_INSTALL_START,
    HasCluster
  >;
  [CodeEnum.OPERATION_INSTALL_COMPLETE]: RawEvent<
    typeof CodeEnum.OPERATION_INSTALL_COMPLETE,
    HasCluster
  >;
  [CodeEnum.OPERATION_INSTALL_FAILURE]: RawEvent<
    typeof CodeEnum.OPERATION_INSTALL_FAILURE,
    HasCluster
  >;
  [CodeEnum.OPERATION_SHRINK_START]: RawEvent<
    typeof CodeEnum.OPERATION_SHRINK_START,
    {
      hostname: string;
      ip: string;
      role: string;
    }
  >;
  [CodeEnum.OPERATION_SHRINK_COMPLETE]: RawEvent<
    typeof CodeEnum.OPERATION_SHRINK_COMPLETE,
    {
      hostname: string;
      ip: string;
      role: string;
    }
  >;
  [CodeEnum.OPERATION_SHRINK_FAILURE]: RawEvent<
    typeof CodeEnum.OPERATION_SHRINK_FAILURE,
    {
      hostname: string;
      ip: string;
      role: string;
    }
  >;
  [CodeEnum.OPERATION_UNINSTALL_START]: RawEvent<
    typeof CodeEnum.OPERATION_UNINSTALL_START
  >;
  [CodeEnum.OPERATION_UNINSTALL_COMPLETE]: RawEvent<
    typeof CodeEnum.OPERATION_UNINSTALL_COMPLETE
  >;
  [CodeEnum.OPERATION_UNINSTALL_FAILURE]: RawEvent<
    typeof CodeEnum.OPERATION_UNINSTALL_FAILURE
  >;
  [CodeEnum.OPERATION_UPDATE_COMPLETE]: RawEvent<
    typeof CodeEnum.OPERATION_UPDATE_COMPLETE,
    {
      version: string;
    }
  >;
  [CodeEnum.OPERATION_UPDATE_FAILURE]: RawEvent<
    typeof CodeEnum.OPERATION_UPDATE_FAILURE,
    {
      version: string;
    }
  >;
  [CodeEnum.OPERATION_UPDATE_START]: RawEvent<
    typeof CodeEnum.OPERATION_UPDATE_START,
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
  [CodeEnum.REMOTE_SUPPORT_ENABLED]: RawEvent<
    typeof CodeEnum.REMOTE_SUPPORT_ENABLED,
    HasHub
  >;
  [CodeEnum.REMOTE_SUPPORT_DISABLED]: RawEvent<
    typeof CodeEnum.REMOTE_SUPPORT_DISABLED,
    HasHub
  >;
  [CodeEnum.SAML_CONNECTOR_CREATED]: RawEvent<
    typeof CodeEnum.SAML_CONNECTOR_CREATED,
    HasName
  >;
  [CodeEnum.SAML_CONNECTOR_DELETED]: RawEvent<
    typeof CodeEnum.SAML_CONNECTOR_DELETED,
    HasName
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
      participants?: string[];
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
  [CodeEnum.SMTPCONFIG_CREATED]: RawEvent<typeof CodeEnum.SMTPCONFIG_CREATED>;
  [CodeEnum.SMTPCONFIG_DELETED]: RawEvent<typeof CodeEnum.SMTPCONFIG_DELETED>;
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
  [CodeEnum.TERMINAL_RESIZE]: RawEvent<typeof CodeEnum.TERMINAL_RESIZE>;
  [CodeEnum.TLSKEYPAIR_CREATED]: RawEvent<typeof CodeEnum.TLSKEYPAIR_CREATED>;
  [CodeEnum.TLSKEYPAIR_DELETED]: RawEvent<typeof CodeEnum.TLSKEYPAIR_DELETED>;
  [CodeEnum.TOKEN_CREATED]: RawEvent<
    typeof CodeEnum.TOKEN_CREATED,
    {
      owner: string;
    }
  >;
  [CodeEnum.TOKEN_DELETED]: RawEvent<
    typeof CodeEnum.TOKEN_DELETED,
    {
      owner: string;
    }
  >;
  [CodeEnum.UPDATES_ENABLED]: RawEvent<typeof CodeEnum.UPDATES_ENABLED, HasHub>;
  [CodeEnum.UPDATES_DISABLED]: RawEvent<
    typeof CodeEnum.UPDATES_DISABLED,
    HasHub
  >;
  [CodeEnum.UPDATES_DOWNLOADED]: RawEvent<
    typeof CodeEnum.UPDATES_DOWNLOADED,
    {
      hub: string;
      name: string;
      version: string;
    }
  >;
  [CodeEnum.USER_CREATED]: RawEvent<typeof CodeEnum.USER_CREATED, HasName>;
  [CodeEnum.USER_DELETED]: RawEvent<typeof CodeEnum.USER_DELETED, HasName>;
  [CodeEnum.USER_INVITE_CREATED]: RawEvent<
    typeof CodeEnum.USER_INVITE_CREATED,
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

type RawEventAlert<T extends Code> = RawEvent<
  T,
  {
    name: string;
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
  [key in keyof RawEvents]: {
    desc: string;
    format: (json: RawEvents[key]) => string;
  };
};

export type Events = {
  [key in keyof RawEvents]: {
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
