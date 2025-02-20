/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// eventGroupTypes contains a map of events that were grouped under the same
// event type but have different event codes. This is used to filter out duplicate
// event types when listing event filters and provide modified description of event.
export const eventGroupTypes = {
  'db.session.start': 'Database Session Start',
  exec: 'Command Execution',
  port: 'Port Forwarding',
  scp: 'SCP',
  sftp: 'SFTP',
  subsystem: 'Subsystem Request',
  'user.login': 'User Logins',
  'spiffe.svid.issued': 'SPIFFE SVID Issuance',
};

/**
 * eventCodes is a map of event codes.
 *
 * After defining an event code:
 *  1: Define fields from JSON response in `RawEvents` object (in this file)
 *  2: Define formatter in `makeEvent.ts` file which defines *events types and
 *     defines short and long event definitions
 *  * Some events can have same event "type" but have unique "code".
 *    These duplicated event types needs to be defined in `eventGroupTypes` object
 *  3: Define icons for events under `EventTypeCell.tsx` file
 *  4: Add an actual JSON event to the fixtures file in `src/Audit/fixtures/index.ts`.
 *  5: Check fixture is rendered in storybook, then update snapshot for `Audit.story.test.tsx`
 */
export const eventCodes = {
  ACCESS_REQUEST_CREATED: 'T5000I',
  ACCESS_REQUEST_REVIEWED: 'T5002I',
  ACCESS_REQUEST_UPDATED: 'T5001I',
  ACCESS_REQUEST_DELETED: 'T5003I',
  ACCESS_REQUEST_RESOURCE_SEARCH: 'T5004I',
  APP_SESSION_CHUNK: 'T2008I',
  APP_SESSION_START: 'T2007I',
  APP_SESSION_END: 'T2011I',
  APP_SESSION_DYNAMODB_REQUEST: 'T2013I',
  APP_CREATED: 'TAP03I',
  APP_UPDATED: 'TAP04I',
  APP_DELETED: 'TAP05I',
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
  DATABASE_SESSION_MALFORMED_PACKET: 'TDB06I',
  DATABASE_SESSION_PERMISSIONS_UPDATE: 'TDB07I',
  DATABASE_SESSION_USER_CREATE: 'TDB08I',
  DATABASE_SESSION_USER_CREATE_FAILURE: 'TDB08W',
  DATABASE_SESSION_USER_DEACTIVATE: 'TDB09I',
  DATABASE_SESSION_USER_DEACTIVATE_FAILURE: 'TDB09W',
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
  MYSQL_INIT_DB: 'TMY07I',
  MYSQL_CREATE_DB: 'TMY08I',
  MYSQL_DROP_DB: 'TMY09I',
  MYSQL_SHUT_DOWN: 'TMY10I',
  MYSQL_PROCESS_KILL: 'TMY11I',
  MYSQL_DEBUG: 'TMY12I',
  MYSQL_REFRESH: 'TMY13I',
  SQLSERVER_RPC_REQUEST: 'TMS00I',
  CASSANDRA_BATCH_EVENT: 'TCA01I',
  CASSANDRA_PREPARE_EVENT: 'TCA02I',
  CASSANDRA_EXECUTE_EVENT: 'TCA03I',
  CASSANDRA_REGISTER_EVENT: 'TCA04I',
  ELASTICSEARCH_REQUEST: 'TES00I',
  ELASTICSEARCH_REQUEST_FAILURE: 'TES00E',
  OPENSEARCH_REQUEST: 'TOS00I',
  OPENSEARCH_REQUEST_FAILURE: 'TOS00E',
  DYNAMODB_REQUEST: 'TDY01I',
  DYNAMODB_REQUEST_FAILURE: 'TDY01E',
  DESKTOP_SESSION_STARTED: 'TDP00I',
  DESKTOP_SESSION_STARTED_FAILED: 'TDP00W',
  DESKTOP_SESSION_ENDED: 'TDP01I',
  DESKTOP_CLIPBOARD_SEND: 'TDP02I',
  DESKTOP_CLIPBOARD_RECEIVE: 'TDP03I',
  DESKTOP_SHARED_DIRECTORY_START: 'TDP04I',
  DESKTOP_SHARED_DIRECTORY_START_FAILURE: 'TDP04W',
  DESKTOP_SHARED_DIRECTORY_READ: 'TDP05I',
  DESKTOP_SHARED_DIRECTORY_READ_FAILURE: 'TDP05W',
  DESKTOP_SHARED_DIRECTORY_WRITE: 'TDP06I',
  DESKTOP_SHARED_DIRECTORY_WRITE_FAILURE: 'TDP06W',
  DEVICE_CREATE: 'TV001I',
  DEVICE_DELETE: 'TV002I',
  DEVICE_ENROLL_TOKEN_CREATE: 'TV003I',
  DEVICE_ENROLL_TOKEN_SPENT: 'TV004I',
  DEVICE_ENROLL: 'TV005I',
  DEVICE_AUTHENTICATE: 'TV006I',
  DEVICE_UPDATE: 'TV007I',
  DEVICE_WEB_TOKEN_CREATE: 'TV008I',
  DEVICE_AUTHENTICATE_CONFIRM: 'TV009I',
  EXEC_FAILURE: 'T3002E',
  EXEC: 'T3002I',
  GITHUB_CONNECTOR_CREATED: 'T8000I',
  GITHUB_CONNECTOR_DELETED: 'T8001I',
  GITHUB_CONNECTOR_UPDATED: 'T80002I', // extra 0 is intentional
  KUBE_REQUEST: 'T3009I',
  KUBE_CREATED: 'T3010I',
  KUBE_UPDATED: 'T3011I',
  KUBE_DELETED: 'T3012I',
  LOCK_CREATED: 'TLK00I',
  LOCK_DELETED: 'TLK01I',
  MFA_DEVICE_ADD: 'T1006I',
  MFA_DEVICE_DELETE: 'T1007I',
  OIDC_CONNECTOR_CREATED: 'T8100I',
  OIDC_CONNECTOR_DELETED: 'T8101I',
  OIDC_CONNECTOR_UPDATED: 'T8102I',
  PORTFORWARD_FAILURE: 'T3003E',
  PORTFORWARD_STOP: 'T3003S',
  PORTFORWARD: 'T3003I',
  RECOVERY_TOKEN_CREATED: 'T6001I',
  PRIVILEGE_TOKEN_CREATED: 'T6002I',
  RECOVERY_CODE_GENERATED: 'T1008I',
  RECOVERY_CODE_USED: 'T1009I',
  RECOVERY_CODE_USED_FAILURE: 'T1009W',
  RESET_PASSWORD_TOKEN_CREATED: 'T6000I',
  ROLE_CREATED: 'T9000I',
  ROLE_DELETED: 'T9001I',
  ROLE_UPDATED: 'T9002I',
  SAML_CONNECTOR_CREATED: 'T8200I',
  SAML_CONNECTOR_DELETED: 'T8201I',
  SAML_CONNECTOR_UPDATED: 'T8202I',
  SCP_DOWNLOAD_FAILURE: 'T3004E',
  SCP_DOWNLOAD: 'T3004I',
  SCP_UPLOAD_FAILURE: 'T3005E',
  SCP_UPLOAD: 'T3005I',
  SCP_DISALLOWED: 'T3010E',
  SFTP_OPEN_FAILURE: 'TS001E',
  SFTP_OPEN: 'TS001I',
  SFTP_SETSTAT_FAILURE: 'TS007E',
  SFTP_SETSTAT: 'TS007I',
  SFTP_OPENDIR_FAILURE: 'TS009E',
  SFTP_OPENDIR: 'TS009I',
  SFTP_READDIR_FAILURE: 'TS010E',
  SFTP_READDIR: 'TS010I',
  SFTP_REMOVE_FAILURE: 'TS011E',
  SFTP_REMOVE: 'TS011I',
  SFTP_MKDIR_FAILURE: 'TS012E',
  SFTP_MKDIR: 'TS012I',
  SFTP_RMDIR_FAILURE: 'TS013E',
  SFTP_RMDIR: 'TS013I',
  SFTP_RENAME_FAILURE: 'TS016E',
  SFTP_RENAME: 'TS016I',
  SFTP_SYMLINK_FAILURE: 'TS018E',
  SFTP_SYMLINK: 'TS018I',
  SFTP_LINK: 'TS019I',
  SFTP_LINK_FAILURE: 'TS019E',
  SFTP_DISALLOWED: 'TS020E',
  SFTP_SUMMARY: 'TS021I',
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
  SESSION_RECORDING_ACCESS: 'T2012I',
  SSMRUN_FAIL: 'TDS00W',
  SSMRUN_SUCCESS: 'TDS00I',
  SUBSYSTEM_FAILURE: 'T3001E',
  SUBSYSTEM: 'T3001I',
  TERMINAL_RESIZE: 'T2002I',
  TRUSTED_CLUSTER_CREATED: 'T7000I',
  TRUSTED_CLUSTER_DELETED: 'T7001I',
  TRUSTED_CLUSTER_TOKEN_CREATED: 'T7002I',
  PROVISION_TOKEN_CREATED: 'TJT00I',
  UNKNOWN: 'TCC00E',
  USER_CREATED: 'T1002I',
  USER_DELETED: 'T1004I',
  USER_LOCAL_LOGIN: 'T1000I',
  USER_LOCAL_LOGINFAILURE: 'T1000W',
  USER_PASSWORD_CHANGED: 'T1005I',
  USER_SSO_LOGIN: 'T1001I',
  USER_SSO_LOGINFAILURE: 'T1001W',
  USER_SSO_TEST_FLOW_LOGIN: 'T1010I',
  USER_SSO_TEST_FLOW_LOGINFAILURE: 'T1011W',
  USER_HEADLESS_LOGIN_REQUESTED: 'T1012I',
  USER_HEADLESS_LOGIN_APPROVED: 'T1013I',
  USER_HEADLESS_LOGIN_APPROVEDFAILURE: 'T1013W',
  USER_HEADLESS_LOGIN_REJECTED: 'T1014W',
  CREATE_MFA_AUTH_CHALLENGE: 'T1015I',
  VALIDATE_MFA_AUTH_RESPONSE: 'T1016I',
  VALIDATE_MFA_AUTH_RESPONSEFAILURE: 'T1016W',
  USER_UPDATED: 'T1003I',
  X11_FORWARD: 'T3008I',
  X11_FORWARD_FAILURE: 'T3008W',
  CERTIFICATE_CREATED: 'TC000I',
  UPGRADE_WINDOW_UPDATED: 'TUW01I',
  BOT_JOIN: 'TJ001I',
  BOT_JOIN_FAILURE: 'TJ001E',
  INSTANCE_JOIN: 'TJ002I',
  INSTANCE_JOIN_FAILURE: 'TJ002E',
  BOT_CREATED: 'TB001I',
  BOT_UPDATED: 'TB002I',
  BOT_DELETED: 'TB003I',
  WORKLOAD_IDENTITY_CREATE: `WID001I`,
  WORKLOAD_IDENTITY_UPDATE: `WID002I`,
  WORKLOAD_IDENTITY_DELETE: `WID003I`,
  LOGIN_RULE_CREATE: 'TLR00I',
  LOGIN_RULE_DELETE: 'TLR01I',
  SAML_IDP_AUTH_ATTEMPT: 'TSI000I',
  SAML_IDP_SERVICE_PROVIDER_CREATE: 'TSI001I',
  SAML_IDP_SERVICE_PROVIDER_CREATE_FAILURE: 'TSI001W',
  SAML_IDP_SERVICE_PROVIDER_UPDATE: 'TSI002I',
  SAML_IDP_SERVICE_PROVIDER_UPDATE_FAILURE: 'TSI002W',
  SAML_IDP_SERVICE_PROVIDER_DELETE: 'TSI003I',
  SAML_IDP_SERVICE_PROVIDER_DELETE_FAILURE: 'TSI003W',
  SAML_IDP_SERVICE_PROVIDER_DELETE_ALL: 'TSI004I',
  SAML_IDP_SERVICE_PROVIDER_DELETE_ALL_FAILURE: 'TSI004W',
  OKTA_GROUPS_UPDATE: 'TOK001I',
  OKTA_APPLICATIONS_UPDATE: 'TOK002I',
  OKTA_SYNC_FAILURE: 'TOK003E',
  OKTA_ASSIGNMENT_PROCESS: 'TOK004I',
  OKTA_ASSIGNMENT_PROCESS_FAILURE: 'TOK004E',
  OKTA_ASSIGNMENT_CLEANUP: 'TOK005I',
  OKTA_ASSIGNMENT_CLEANUP_FAILURE: 'TOK005E',
  OKTA_ACCESS_LIST_SYNC: 'TOK006I',
  OKTA_ACCESS_LIST_SYNC_FAILURE: 'TOK006E',
  OKTA_USER_SYNC: 'TOK007I',
  OKTA_USER_SYNC_FAILURE: 'TOK007E',
  ACCESS_LIST_CREATE: 'TAL001I',
  ACCESS_LIST_CREATE_FAILURE: 'TAL001E',
  ACCESS_LIST_UPDATE: 'TAL002I',
  ACCESS_LIST_UPDATE_FAILURE: 'TAL002E',
  ACCESS_LIST_DELETE: 'TAL003I',
  ACCESS_LIST_DELETE_FAILURE: 'TAL003E',
  ACCESS_LIST_REVIEW: 'TAL004I',
  ACCESS_LIST_REVIEW_FAILURE: 'TAL004E',
  ACCESS_LIST_MEMBER_CREATE: 'TAL005I',
  ACCESS_LIST_MEMBER_CREATE_FAILURE: 'TAL005E',
  ACCESS_LIST_MEMBER_UPDATE: 'TAL006I',
  ACCESS_LIST_MEMBER_UPDATE_FAILURE: 'TAL006E',
  ACCESS_LIST_MEMBER_DELETE: 'TAL007I',
  ACCESS_LIST_MEMBER_DELETE_FAILURE: 'TAL007E',
  ACCESS_LIST_MEMBER_DELETE_ALL_FOR_ACCESS_LIST: 'TAL008I',
  ACCESS_LIST_MEMBER_DELETE_ALL_FOR_ACCESS_LIST_FAILURE: 'TAL008E',
  USER_LOGIN_INVALID_ACCESS_LIST: 'TAL009W',
  SECURITY_REPORT_AUDIT_QUERY_RUN: 'SRE001I',
  SECURITY_REPORT_RUN: 'SRE002I',
  EXTERNAL_AUDIT_STORAGE_ENABLE: 'TEA001I',
  EXTERNAL_AUDIT_STORAGE_DISABLE: 'TEA002I',
  SPIFFE_SVID_ISSUED: 'TSPIFFE000I',
  SPIFFE_SVID_ISSUED_FAILURE: 'TSPIFFE000E',
  AUTH_PREFERENCE_UPDATE: 'TCAUTH001I',
  CLUSTER_NETWORKING_CONFIG_UPDATE: 'TCNET002I',
  SESSION_RECORDING_CONFIG_UPDATE: 'TCREC003I',
  ACCESS_GRAPH_PATH_CHANGED: 'TAG001I',
  SPANNER_RPC: 'TSPN001I',
  SPANNER_RPC_DENIED: 'TSPN001W',
  DISCOVERY_CONFIG_CREATE: 'DC001I',
  DISCOVERY_CONFIG_UPDATE: 'DC002I',
  DISCOVERY_CONFIG_DELETE: 'DC003I',
  DISCOVERY_CONFIG_DELETE_ALL: 'DC004I',
  INTEGRATION_CREATE: 'IG001I',
  INTEGRATION_UPDATE: 'IG002I',
  INTEGRATION_DELETE: 'IG003I',
  STATIC_HOST_USER_CREATE: 'SHU001I',
  STATIC_HOST_USER_UPDATE: 'SHU002I',
  STATIC_HOST_USER_DELETE: 'SHU003I',
  CROWN_JEWEL_CREATE: 'CJ001I',
  CROWN_JEWEL_UPDATE: 'CJ002I',
  CROWN_JEWEL_DELETE: 'CJ003I',
  USER_TASK_CREATE: 'UT001I',
  USER_TASK_UPDATE: 'UT002I',
  USER_TASK_DELETE: 'UT003I',
  PLUGIN_CREATE: 'PG001I',
  PLUGIN_UPDATE: 'PG002I',
  PLUGIN_DELETE: 'PG003I',
  CONTACT_CREATE: 'TCTC001I',
  CONTACT_DELETE: 'TCTC002I',
  GIT_COMMAND: 'TGIT001I',
  GIT_COMMAND_FAILURE: 'TGIT001E',
  STABLE_UNIX_USER_CREATE: 'TSUU001I',
  AWS_IC_RESOURCE_SYNC_SUCCESS: 'TAIC001I',
  AWS_IC_RESOURCE_SYNC_FAILURE: 'TAIC001E',
  AWSIC_ACCOUNT_ASSIGNMENT_CREATE: 'TAIC002I',
  AWSIC_ACCOUNT_ASSIGNMENT_DELETE: 'TAIC003I',
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
  [eventCodes.ACCESS_REQUEST_RESOURCE_SEARCH]: RawEvent<
    typeof eventCodes.ACCESS_REQUEST_RESOURCE_SEARCH,
    { resource_type: string; search_as_roles: string[] }
  >;
  [eventCodes.AUTH_ATTEMPT_FAILURE]: RawEventAuthFailure<
    typeof eventCodes.AUTH_ATTEMPT_FAILURE
  >;
  [eventCodes.APP_CREATED]: RawEvent<
    typeof eventCodes.APP_CREATED,
    {
      name: string;
    }
  >;
  [eventCodes.APP_UPDATED]: RawEvent<
    typeof eventCodes.APP_UPDATED,
    {
      name: string;
    }
  >;
  [eventCodes.APP_DELETED]: RawEvent<
    typeof eventCodes.APP_DELETED,
    {
      name: string;
    }
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
  [eventCodes.GITHUB_CONNECTOR_UPDATED]: RawEventConnector<
    typeof eventCodes.GITHUB_CONNECTOR_UPDATED
  >;
  [eventCodes.OIDC_CONNECTOR_CREATED]: RawEventConnector<
    typeof eventCodes.OIDC_CONNECTOR_CREATED
  >;
  [eventCodes.OIDC_CONNECTOR_DELETED]: RawEventConnector<
    typeof eventCodes.OIDC_CONNECTOR_DELETED
  >;
  [eventCodes.OIDC_CONNECTOR_UPDATED]: RawEventConnector<
    typeof eventCodes.OIDC_CONNECTOR_UPDATED
  >;
  [eventCodes.PORTFORWARD]: RawEvent<typeof eventCodes.PORTFORWARD>;
  [eventCodes.PORTFORWARD_STOP]: RawEvent<typeof eventCodes.PORTFORWARD_STOP>;
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
  [eventCodes.SAML_CONNECTOR_UPDATED]: RawEventConnector<
    typeof eventCodes.SAML_CONNECTOR_UPDATED
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
  [eventCodes.SCP_DISALLOWED]: RawEvent<
    typeof eventCodes.SCP_DISALLOWED,
    {
      user: string;
    }
  >;
  [eventCodes.SFTP_OPEN]: RawEventSFTP<typeof eventCodes.SFTP_OPEN>;
  [eventCodes.SFTP_OPEN_FAILURE]: RawEventSFTP<
    typeof eventCodes.SFTP_OPEN_FAILURE
  >;
  [eventCodes.SFTP_SETSTAT]: RawEventSFTP<typeof eventCodes.SFTP_SETSTAT>;
  [eventCodes.SFTP_SETSTAT_FAILURE]: RawEventSFTP<
    typeof eventCodes.SFTP_SETSTAT_FAILURE
  >;
  [eventCodes.SFTP_OPENDIR]: RawEventSFTP<typeof eventCodes.SFTP_OPENDIR>;
  [eventCodes.SFTP_OPENDIR_FAILURE]: RawEventSFTP<
    typeof eventCodes.SFTP_OPENDIR_FAILURE
  >;
  [eventCodes.SFTP_READDIR]: RawEventSFTP<typeof eventCodes.SFTP_READDIR>;
  [eventCodes.SFTP_READDIR_FAILURE]: RawEventSFTP<
    typeof eventCodes.SFTP_READDIR_FAILURE
  >;
  [eventCodes.SFTP_REMOVE]: RawEventSFTP<typeof eventCodes.SFTP_REMOVE>;
  [eventCodes.SFTP_REMOVE_FAILURE]: RawEventSFTP<
    typeof eventCodes.SFTP_REMOVE_FAILURE
  >;
  [eventCodes.SFTP_MKDIR]: RawEventSFTP<typeof eventCodes.SFTP_MKDIR>;
  [eventCodes.SFTP_MKDIR_FAILURE]: RawEventSFTP<
    typeof eventCodes.SFTP_MKDIR_FAILURE
  >;
  [eventCodes.SFTP_RMDIR]: RawEventSFTP<typeof eventCodes.SFTP_RMDIR>;
  [eventCodes.SFTP_RMDIR_FAILURE]: RawEventSFTP<
    typeof eventCodes.SFTP_RMDIR_FAILURE
  >;
  [eventCodes.SFTP_RENAME]: RawEventSFTP<typeof eventCodes.SFTP_RENAME>;
  [eventCodes.SFTP_RENAME_FAILURE]: RawEventSFTP<
    typeof eventCodes.SFTP_RENAME_FAILURE
  >;
  [eventCodes.SFTP_SYMLINK]: RawEventSFTP<typeof eventCodes.SFTP_SYMLINK>;
  [eventCodes.SFTP_SYMLINK_FAILURE]: RawEventSFTP<
    typeof eventCodes.SFTP_SYMLINK_FAILURE
  >;
  [eventCodes.SFTP_LINK]: RawEventSFTP<typeof eventCodes.SFTP_LINK>;
  [eventCodes.SFTP_LINK_FAILURE]: RawEventSFTP<
    typeof eventCodes.SFTP_LINK_FAILURE
  >;
  [eventCodes.SFTP_DISALLOWED]: RawEventSFTP<typeof eventCodes.SFTP_DISALLOWED>;
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
      kubernetes_cluster: string;
      proto: string;
      server_hostname: string;
      server_addr: string;
      server_id: string;
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
    {
      sid: string;
      aws_role_arn: string;
      app_name: string;
    }
  >;
  [eventCodes.APP_SESSION_END]: RawEvent<
    typeof eventCodes.APP_SESSION_END,
    {
      sid: string;
      app_name: string;
    }
  >;
  [eventCodes.APP_SESSION_CHUNK]: RawEvent<
    typeof eventCodes.APP_SESSION_CHUNK,
    {
      sid: string;
      aws_role_arn: string;
      app_name: string;
    }
  >;
  [eventCodes.APP_SESSION_DYNAMODB_REQUEST]: RawEvent<
    typeof eventCodes.APP_SESSION_DYNAMODB_REQUEST,
    {
      target: string;
      app_name: string;
    }
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
  [eventCodes.USER_SSO_TEST_FLOW_LOGIN]: RawEvent<
    typeof eventCodes.USER_SSO_TEST_FLOW_LOGIN
  >;
  [eventCodes.USER_SSO_TEST_FLOW_LOGINFAILURE]: RawEvent<
    typeof eventCodes.USER_SSO_TEST_FLOW_LOGINFAILURE,
    {
      error: string;
    }
  >;
  [eventCodes.USER_HEADLESS_LOGIN_REQUESTED]: RawEvent<
    typeof eventCodes.USER_HEADLESS_LOGIN_REQUESTED
  >;
  [eventCodes.USER_HEADLESS_LOGIN_APPROVED]: RawEvent<
    typeof eventCodes.USER_HEADLESS_LOGIN_APPROVED
  >;
  [eventCodes.USER_HEADLESS_LOGIN_APPROVEDFAILURE]: RawEvent<
    typeof eventCodes.USER_HEADLESS_LOGIN_APPROVEDFAILURE,
    {
      error: string;
    }
  >;
  [eventCodes.USER_HEADLESS_LOGIN_REJECTED]: RawEvent<
    typeof eventCodes.USER_HEADLESS_LOGIN_REJECTED
  >;
  [eventCodes.ROLE_CREATED]: RawEvent<typeof eventCodes.ROLE_CREATED, HasName>;
  [eventCodes.ROLE_DELETED]: RawEvent<typeof eventCodes.ROLE_DELETED, HasName>;
  [eventCodes.ROLE_UPDATED]: RawEvent<typeof eventCodes.ROLE_UPDATED, HasName>;
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
  [eventCodes.PROVISION_TOKEN_CREATED]: RawEvent<
    typeof eventCodes.PROVISION_TOKEN_CREATED,
    {
      roles: string[];
      join_method: string;
    }
  >;
  [eventCodes.KUBE_REQUEST]: RawEvent<
    typeof eventCodes.KUBE_REQUEST,
    {
      kubernetes_cluster: string;
      verb: string;
      request_path: string;
      response_code: string;
    }
  >;
  [eventCodes.KUBE_CREATED]: RawEvent<
    typeof eventCodes.KUBE_CREATED,
    {
      name: string;
    }
  >;
  [eventCodes.KUBE_UPDATED]: RawEvent<
    typeof eventCodes.KUBE_UPDATED,
    {
      name: string;
    }
  >;
  [eventCodes.KUBE_DELETED]: RawEvent<
    typeof eventCodes.KUBE_DELETED,
    {
      name: string;
    }
  >;
  [eventCodes.DATABASE_SESSION_STARTED]: RawEvent<
    typeof eventCodes.DATABASE_SESSION_STARTED,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
      db_roles: string[];
    }
  >;
  [eventCodes.DATABASE_SESSION_STARTED_FAILURE]: RawEvent<
    typeof eventCodes.DATABASE_SESSION_STARTED_FAILURE,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
      db_roles: string[];
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
  [eventCodes.DATABASE_SESSION_MALFORMED_PACKET]: RawEvent<
    typeof eventCodes.DATABASE_SESSION_MALFORMED_PACKET,
    {
      name: string;
      db_service: string;
      db_name: string;
    }
  >;
  [eventCodes.DATABASE_SESSION_PERMISSIONS_UPDATE]: RawEvent<
    typeof eventCodes.DATABASE_SESSION_PERMISSIONS_UPDATE,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
      permission_summary: {
        permission: string;
        counts: { [key: string]: number };
      }[];
    }
  >;
  [eventCodes.DATABASE_SESSION_USER_CREATE]: RawDatabaseSessionEvent<
    typeof eventCodes.DATABASE_SESSION_USER_CREATE,
    {
      roles: string[];
    }
  >;
  [eventCodes.DATABASE_SESSION_USER_CREATE_FAILURE]: RawDatabaseSessionEvent<
    typeof eventCodes.DATABASE_SESSION_USER_CREATE_FAILURE,
    {
      error: string;
      message: string;
      roles: string[];
    }
  >;
  [eventCodes.DATABASE_SESSION_USER_DEACTIVATE]: RawDatabaseSessionEvent<
    typeof eventCodes.DATABASE_SESSION_USER_DEACTIVATE,
    {
      delete: boolean;
    }
  >;
  [eventCodes.DATABASE_SESSION_USER_DEACTIVATE_FAILURE]: RawDatabaseSessionEvent<
    typeof eventCodes.DATABASE_SESSION_USER_DEACTIVATE_FAILURE,
    {
      error: string;
      message: string;
      delete: boolean;
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
  [eventCodes.MYSQL_INIT_DB]: RawEvent<
    typeof eventCodes.MYSQL_INIT_DB,
    {
      db_service: string;
      schema_name: string;
    }
  >;
  [eventCodes.MYSQL_CREATE_DB]: RawEvent<
    typeof eventCodes.MYSQL_CREATE_DB,
    {
      db_service: string;
      schema_name: string;
    }
  >;
  [eventCodes.MYSQL_DROP_DB]: RawEvent<
    typeof eventCodes.MYSQL_DROP_DB,
    {
      db_service: string;
      schema_name: string;
    }
  >;
  [eventCodes.MYSQL_SHUT_DOWN]: RawEvent<
    typeof eventCodes.MYSQL_SHUT_DOWN,
    {
      db_service: string;
    }
  >;
  [eventCodes.MYSQL_PROCESS_KILL]: RawEvent<
    typeof eventCodes.MYSQL_PROCESS_KILL,
    {
      db_service: string;
      process_id: number;
    }
  >;
  [eventCodes.MYSQL_DEBUG]: RawEvent<
    typeof eventCodes.MYSQL_DEBUG,
    {
      db_service: string;
    }
  >;
  [eventCodes.MYSQL_REFRESH]: RawEvent<
    typeof eventCodes.MYSQL_REFRESH,
    {
      db_service: string;
      subcommand: string;
    }
  >;
  [eventCodes.SQLSERVER_RPC_REQUEST]: RawEvent<
    typeof eventCodes.SQLSERVER_RPC_REQUEST,
    {
      name: string;
      db_service: string;
      db_name: string;
      proc_name: string;
    }
  >;
  [eventCodes.CASSANDRA_BATCH_EVENT]: RawEvent<
    typeof eventCodes.CASSANDRA_BATCH_EVENT,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
    }
  >;
  [eventCodes.CASSANDRA_PREPARE_EVENT]: RawEvent<
    typeof eventCodes.CASSANDRA_PREPARE_EVENT,
    {
      name: string;
      query: string;
      db_service: string;
      db_name: string;
      db_user: string;
    }
  >;
  [eventCodes.CASSANDRA_EXECUTE_EVENT]: RawEvent<
    typeof eventCodes.CASSANDRA_EXECUTE_EVENT,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
    }
  >;
  [eventCodes.CASSANDRA_REGISTER_EVENT]: RawEvent<
    typeof eventCodes.CASSANDRA_REGISTER_EVENT,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
    }
  >;
  [eventCodes.ELASTICSEARCH_REQUEST]: RawEvent<
    typeof eventCodes.ELASTICSEARCH_REQUEST,
    {
      name: string;
      db_service: string;
      db_name: string;
      category: number;
      target: string;
      query: string;
      path: string;
    }
  >;
  [eventCodes.ELASTICSEARCH_REQUEST_FAILURE]: RawEvent<
    typeof eventCodes.ELASTICSEARCH_REQUEST_FAILURE,
    {
      name: string;
      db_service: string;
      db_name: string;
      category: number;
      target: string;
      query: string;
      path: string;
    }
  >;
  [eventCodes.OPENSEARCH_REQUEST]: RawEvent<
    typeof eventCodes.OPENSEARCH_REQUEST,
    {
      name: string;
      db_service: string;
      db_name: string;
      category: number;
      target: string;
      query: string;
      path: string;
    }
  >;
  [eventCodes.OPENSEARCH_REQUEST_FAILURE]: RawEvent<
    typeof eventCodes.OPENSEARCH_REQUEST_FAILURE,
    {
      name: string;
      db_service: string;
      db_name: string;
      category: number;
      target: string;
      query: string;
      path: string;
    }
  >;
  [eventCodes.DYNAMODB_REQUEST]: RawEvent<
    typeof eventCodes.DYNAMODB_REQUEST,
    {
      target: string;
      db_service: string;
    }
  >;
  [eventCodes.DYNAMODB_REQUEST_FAILURE]: RawEvent<
    typeof eventCodes.DYNAMODB_REQUEST_FAILURE,
    {
      target: string;
      db_service: string;
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
      desktop_name: string;
      sid: string;
      windows_user: string;
      windows_domain: string;
    }
  >;
  [eventCodes.DESKTOP_SESSION_STARTED_FAILED]: RawEvent<
    typeof eventCodes.DESKTOP_SESSION_STARTED_FAILED,
    {
      desktop_addr: string;
      desktop_name: string;
      windows_user: string;
      windows_domain: string;
    }
  >;
  [eventCodes.DESKTOP_SESSION_ENDED]: RawEvent<
    typeof eventCodes.DESKTOP_SESSION_ENDED,
    {
      desktop_addr: string;
      desktop_name: string;
      sid: string;
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
  [eventCodes.DESKTOP_SHARED_DIRECTORY_START]: RawEvent<
    typeof eventCodes.DESKTOP_SHARED_DIRECTORY_START,
    {
      desktop_addr: string;
      directory_name: string;
      windows_domain: string;
    }
  >;
  [eventCodes.DESKTOP_SHARED_DIRECTORY_START_FAILURE]: RawEvent<
    typeof eventCodes.DESKTOP_SHARED_DIRECTORY_START_FAILURE,
    {
      desktop_addr: string;
      directory_name: string;
      windows_domain: string;
    }
  >;
  [eventCodes.DESKTOP_SHARED_DIRECTORY_READ]: RawEvent<
    typeof eventCodes.DESKTOP_SHARED_DIRECTORY_READ,
    {
      desktop_addr: string;
      directory_name: string;
      windows_domain: string;
      file_path: string;
      length: number;
    }
  >;
  [eventCodes.DESKTOP_SHARED_DIRECTORY_READ_FAILURE]: RawEvent<
    typeof eventCodes.DESKTOP_SHARED_DIRECTORY_READ_FAILURE,
    {
      desktop_addr: string;
      directory_name: string;
      windows_domain: string;
      file_path: string;
      length: number;
    }
  >;
  [eventCodes.DESKTOP_SHARED_DIRECTORY_WRITE]: RawEvent<
    typeof eventCodes.DESKTOP_SHARED_DIRECTORY_WRITE,
    {
      desktop_addr: string;
      directory_name: string;
      windows_domain: string;
      file_path: string;
      length: number;
    }
  >;
  [eventCodes.DESKTOP_SHARED_DIRECTORY_WRITE_FAILURE]: RawEvent<
    typeof eventCodes.DESKTOP_SHARED_DIRECTORY_WRITE_FAILURE,
    {
      desktop_addr: string;
      directory_name: string;
      windows_domain: string;
      file_path: string;
      length: number;
    }
  >;
  [eventCodes.DEVICE_CREATE]: RawDeviceEvent<typeof eventCodes.DEVICE_CREATE>;
  [eventCodes.DEVICE_DELETE]: RawDeviceEvent<typeof eventCodes.DEVICE_DELETE>;
  [eventCodes.DEVICE_ENROLL]: RawDeviceEvent<typeof eventCodes.DEVICE_ENROLL>;
  [eventCodes.DEVICE_ENROLL_TOKEN_CREATE]: RawDeviceEvent<
    typeof eventCodes.DEVICE_ENROLL_TOKEN_CREATE
  >;
  [eventCodes.DEVICE_ENROLL_TOKEN_SPENT]: RawDeviceEvent<
    typeof eventCodes.DEVICE_ENROLL_TOKEN_SPENT
  >;
  [eventCodes.DEVICE_AUTHENTICATE]: RawDeviceEvent<
    typeof eventCodes.DEVICE_AUTHENTICATE
  >;
  [eventCodes.DEVICE_UPDATE]: RawDeviceEvent<typeof eventCodes.DEVICE_UPDATE>;
  [eventCodes.DEVICE_WEB_TOKEN_CREATE]: RawDeviceEvent<
    typeof eventCodes.DEVICE_WEB_TOKEN_CREATE
  >;
  [eventCodes.DEVICE_AUTHENTICATE_CONFIRM]: RawDeviceEvent<
    typeof eventCodes.DEVICE_AUTHENTICATE_CONFIRM
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
  [eventCodes.UPGRADE_WINDOW_UPDATED]: RawEvent<
    typeof eventCodes.UPGRADE_WINDOW_UPDATED,
    {
      upgrade_window_start: string;
    }
  >;
  [eventCodes.SESSION_RECORDING_ACCESS]: RawEvent<
    typeof eventCodes.SESSION_RECORDING_ACCESS,
    {
      sid: string;
      user: string;
    }
  >;
  [eventCodes.SSMRUN_SUCCESS]: RawEvent<
    typeof eventCodes.SSMRUN_SUCCESS,
    {
      account_id: string;
      instance_id: string;
      command_id: string;
      region: string;
      status: string;
      exit_code: number;
    }
  >;
  [eventCodes.SSMRUN_FAIL]: RawEvent<
    typeof eventCodes.SSMRUN_FAIL,
    {
      account_id: string;
      instance_id: string;
      command_id: string;
      region: string;
      status: string;
      exit_code: number;
    }
  >;
  [eventCodes.BOT_JOIN]: RawEvent<
    typeof eventCodes.BOT_JOIN,
    {
      bot_name: string;
      method: string;
    }
  >;
  [eventCodes.BOT_JOIN_FAILURE]: RawEvent<
    typeof eventCodes.BOT_JOIN,
    {
      bot_name: string;
      method: string;
    }
  >;
  [eventCodes.INSTANCE_JOIN]: RawEvent<
    typeof eventCodes.INSTANCE_JOIN,
    {
      node_name: string;
      method: string;
      role: string;
    }
  >;
  [eventCodes.INSTANCE_JOIN_FAILURE]: RawEvent<
    typeof eventCodes.INSTANCE_JOIN,
    {
      node_name: string;
      method: string;
      role: string;
    }
  >;
  [eventCodes.BOT_CREATED]: RawEvent<typeof eventCodes.BOT_CREATED, HasName>;
  [eventCodes.BOT_UPDATED]: RawEvent<typeof eventCodes.BOT_UPDATED, HasName>;
  [eventCodes.BOT_DELETED]: RawEvent<typeof eventCodes.BOT_DELETED, HasName>;
  [eventCodes.WORKLOAD_IDENTITY_CREATE]: RawEvent<
    typeof eventCodes.WORKLOAD_IDENTITY_CREATE,
    HasName
  >;
  [eventCodes.WORKLOAD_IDENTITY_UPDATE]: RawEvent<
    typeof eventCodes.WORKLOAD_IDENTITY_UPDATE,
    HasName
  >;
  [eventCodes.WORKLOAD_IDENTITY_DELETE]: RawEvent<
    typeof eventCodes.WORKLOAD_IDENTITY_DELETE,
    HasName
  >;
  [eventCodes.LOGIN_RULE_CREATE]: RawEvent<
    typeof eventCodes.LOGIN_RULE_CREATE,
    HasName
  >;
  [eventCodes.LOGIN_RULE_DELETE]: RawEvent<
    typeof eventCodes.LOGIN_RULE_DELETE,
    HasName
  >;
  [eventCodes.SAML_IDP_AUTH_ATTEMPT]: RawEvent<
    typeof eventCodes.SAML_IDP_AUTH_ATTEMPT,
    {
      success: boolean;
      service_provider_entity_id: string;
      service_provider_shortcut: string;
    }
  >;
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_CREATE]: RawEvent<
    typeof eventCodes.SAML_IDP_SERVICE_PROVIDER_CREATE,
    {
      name: string;
      updated_by: string;
      service_provider_entity_id: string;
    }
  >;
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_CREATE_FAILURE]: RawEvent<
    typeof eventCodes.SAML_IDP_SERVICE_PROVIDER_CREATE_FAILURE,
    {
      name: string;
      updated_by: string;
      service_provider_entity_id: string;
    }
  >;
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_UPDATE]: RawEvent<
    typeof eventCodes.SAML_IDP_SERVICE_PROVIDER_UPDATE,
    {
      name: string;
      updated_by: string;
      service_provider_entity_id: string;
    }
  >;
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_UPDATE_FAILURE]: RawEvent<
    typeof eventCodes.SAML_IDP_SERVICE_PROVIDER_UPDATE_FAILURE,
    {
      name: string;
      updated_by: string;
      service_provider_entity_id: string;
    }
  >;
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE]: RawEvent<
    typeof eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE,
    {
      name: string;
      updated_by: string;
      service_provider_entity_id: string;
    }
  >;
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE_FAILURE]: RawEvent<
    typeof eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE_FAILURE,
    {
      name: string;
      updated_by: string;
      service_provider_entity_id: string;
    }
  >;
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE_ALL]: RawEvent<
    typeof eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE_ALL,
    {
      name: string;
      updated_by: string;
    }
  >;
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE_ALL_FAILURE]: RawEvent<
    typeof eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE_ALL_FAILURE,
    {
      name: string;
      updated_by: string;
    }
  >;
  [eventCodes.OKTA_GROUPS_UPDATE]: RawEvent<
    typeof eventCodes.OKTA_GROUPS_UPDATE,
    {
      added: number;
      updated: number;
      deleted: number;
    }
  >;
  [eventCodes.OKTA_APPLICATIONS_UPDATE]: RawEvent<
    typeof eventCodes.OKTA_APPLICATIONS_UPDATE,
    {
      added: number;
      updated: number;
      deleted: number;
    }
  >;
  [eventCodes.OKTA_SYNC_FAILURE]: RawEvent<typeof eventCodes.OKTA_SYNC_FAILURE>;
  [eventCodes.OKTA_ASSIGNMENT_PROCESS]: RawEvent<
    typeof eventCodes.OKTA_ASSIGNMENT_PROCESS,
    {
      name: string;
      source: string;
    }
  >;
  [eventCodes.OKTA_ASSIGNMENT_PROCESS_FAILURE]: RawEvent<
    typeof eventCodes.OKTA_ASSIGNMENT_PROCESS_FAILURE,
    {
      name: string;
      source: string;
    }
  >;
  [eventCodes.OKTA_ASSIGNMENT_CLEANUP]: RawEvent<
    typeof eventCodes.OKTA_ASSIGNMENT_PROCESS,
    {
      name: string;
      source: string;
    }
  >;
  [eventCodes.OKTA_ASSIGNMENT_CLEANUP_FAILURE]: RawEvent<
    typeof eventCodes.OKTA_ASSIGNMENT_CLEANUP_FAILURE,
    {
      name: string;
      source: string;
    }
  >;
  [eventCodes.OKTA_USER_SYNC]: RawEvent<
    typeof eventCodes.OKTA_USER_SYNC,
    {
      num_users_created: number;
      num_users_modified: number;
      num_users_deleted: number;
    }
  >;
  [eventCodes.OKTA_USER_SYNC_FAILURE]: RawEvent<
    typeof eventCodes.OKTA_USER_SYNC_FAILURE
  >;
  [eventCodes.OKTA_ACCESS_LIST_SYNC]: RawEvent<
    typeof eventCodes.OKTA_ACCESS_LIST_SYNC
  >;
  [eventCodes.OKTA_ACCESS_LIST_SYNC_FAILURE]: RawEvent<
    typeof eventCodes.OKTA_ACCESS_LIST_SYNC_FAILURE
  >;
  [eventCodes.ACCESS_LIST_CREATE]: RawEvent<
    typeof eventCodes.ACCESS_LIST_CREATE,
    {
      name: string;
      updated_by: string;
    }
  >;
  [eventCodes.ACCESS_LIST_CREATE_FAILURE]: RawEvent<
    typeof eventCodes.ACCESS_LIST_CREATE_FAILURE,
    {
      name: string;
      updated_by: string;
    }
  >;
  [eventCodes.ACCESS_LIST_UPDATE]: RawEvent<
    typeof eventCodes.ACCESS_LIST_UPDATE,
    {
      name: string;
      updated_by: string;
    }
  >;
  [eventCodes.ACCESS_LIST_UPDATE_FAILURE]: RawEvent<
    typeof eventCodes.ACCESS_LIST_UPDATE_FAILURE,
    {
      name: string;
      updated_by: string;
    }
  >;
  [eventCodes.ACCESS_LIST_DELETE]: RawEvent<
    typeof eventCodes.ACCESS_LIST_DELETE,
    {
      name: string;
      updated_by: string;
    }
  >;
  [eventCodes.ACCESS_LIST_DELETE_FAILURE]: RawEvent<
    typeof eventCodes.ACCESS_LIST_DELETE_FAILURE,
    {
      name: string;
      updated_by: string;
    }
  >;
  [eventCodes.ACCESS_LIST_REVIEW]: RawEvent<
    typeof eventCodes.ACCESS_LIST_REVIEW,
    {
      name: string;
      updated_by: string;
    }
  >;
  [eventCodes.ACCESS_LIST_REVIEW_FAILURE]: RawEvent<
    typeof eventCodes.ACCESS_LIST_REVIEW_FAILURE,
    {
      name: string;
      updated_by: string;
    }
  >;
  [eventCodes.ACCESS_LIST_MEMBER_CREATE]: RawEventAccessList<
    typeof eventCodes.ACCESS_LIST_MEMBER_CREATE
  >;
  [eventCodes.ACCESS_LIST_MEMBER_CREATE_FAILURE]: RawEventAccessList<
    typeof eventCodes.ACCESS_LIST_MEMBER_CREATE_FAILURE
  >;
  [eventCodes.ACCESS_LIST_MEMBER_UPDATE]: RawEventAccessList<
    typeof eventCodes.ACCESS_LIST_MEMBER_UPDATE
  >;
  [eventCodes.ACCESS_LIST_MEMBER_UPDATE_FAILURE]: RawEventAccessList<
    typeof eventCodes.ACCESS_LIST_MEMBER_UPDATE_FAILURE
  >;
  [eventCodes.ACCESS_LIST_MEMBER_DELETE]: RawEventAccessList<
    typeof eventCodes.ACCESS_LIST_MEMBER_DELETE
  >;
  [eventCodes.ACCESS_LIST_MEMBER_DELETE_FAILURE]: RawEventAccessList<
    typeof eventCodes.ACCESS_LIST_MEMBER_DELETE_FAILURE
  >;
  [eventCodes.ACCESS_LIST_MEMBER_DELETE_ALL_FOR_ACCESS_LIST]: RawEvent<
    typeof eventCodes.ACCESS_LIST_MEMBER_DELETE_ALL_FOR_ACCESS_LIST,
    {
      access_list_name: string;
      updated_by: string;
    }
  >;
  [eventCodes.ACCESS_LIST_MEMBER_DELETE_ALL_FOR_ACCESS_LIST_FAILURE]: RawEvent<
    typeof eventCodes.ACCESS_LIST_MEMBER_DELETE_ALL_FOR_ACCESS_LIST_FAILURE,
    {
      access_list_name: string;
      updated_by: string;
    }
  >;
  [eventCodes.USER_LOGIN_INVALID_ACCESS_LIST]: RawEvent<
    typeof eventCodes.USER_LOGIN_INVALID_ACCESS_LIST,
    {
      access_list_name: string;
      user: string;
      missing_roles: string[];
    }
  >;
  [eventCodes.SECURITY_REPORT_AUDIT_QUERY_RUN]: RawEvent<
    typeof eventCodes.SECURITY_REPORT_AUDIT_QUERY_RUN,
    {
      query: string;
      total_execution_time_in_millis: string;
      total_data_scanned_in_bytes: string;
    }
  >;
  [eventCodes.SECURITY_REPORT_RUN]: RawEvent<
    typeof eventCodes.SECURITY_REPORT_RUN,
    {
      name: string;
      total_execution_time_in_millis: string;
      total_data_scanned_in_bytes: string;
    }
  >;
  [eventCodes.EXTERNAL_AUDIT_STORAGE_ENABLE]: RawEvent<
    typeof eventCodes.EXTERNAL_AUDIT_STORAGE_ENABLE,
    {
      updated_by: string;
    }
  >;
  [eventCodes.EXTERNAL_AUDIT_STORAGE_DISABLE]: RawEvent<
    typeof eventCodes.EXTERNAL_AUDIT_STORAGE_DISABLE,
    {
      updated_by: string;
    }
  >;
  [eventCodes.CREATE_MFA_AUTH_CHALLENGE]: RawEvent<
    typeof eventCodes.CREATE_MFA_AUTH_CHALLENGE,
    {
      user: string;
    }
  >;
  [eventCodes.VALIDATE_MFA_AUTH_RESPONSE]: RawEvent<
    typeof eventCodes.VALIDATE_MFA_AUTH_RESPONSE,
    {
      user: string;
    }
  >;
  [eventCodes.VALIDATE_MFA_AUTH_RESPONSEFAILURE]: RawEvent<
    typeof eventCodes.VALIDATE_MFA_AUTH_RESPONSEFAILURE,
    {
      user: string;
    }
  >;
  [eventCodes.SPIFFE_SVID_ISSUED]: RawEvent<
    typeof eventCodes.SPIFFE_SVID_ISSUED,
    {
      spiffe_id: string;
    }
  >;
  [eventCodes.SPIFFE_SVID_ISSUED_FAILURE]: RawEvent<
    typeof eventCodes.SPIFFE_SVID_ISSUED_FAILURE,
    {
      spiffe_id: string;
    }
  >;
  [eventCodes.AUTH_PREFERENCE_UPDATE]: RawEvent<
    typeof eventCodes.AUTH_PREFERENCE_UPDATE,
    {
      user: string;
    }
  >;
  [eventCodes.CLUSTER_NETWORKING_CONFIG_UPDATE]: RawEvent<
    typeof eventCodes.CLUSTER_NETWORKING_CONFIG_UPDATE,
    {
      user: string;
    }
  >;
  [eventCodes.SESSION_RECORDING_CONFIG_UPDATE]: RawEvent<
    typeof eventCodes.SESSION_RECORDING_CONFIG_UPDATE,
    {
      user: string;
    }
  >;
  [eventCodes.ACCESS_GRAPH_PATH_CHANGED]: RawEvent<
    typeof eventCodes.ACCESS_GRAPH_PATH_CHANGED,
    {
      change_id: string;
      affected_resource_name: string;
      affected_resource_source: string;
      affected_resource_kind: string;
    }
  >;
  [eventCodes.SPANNER_RPC]: RawSpannerRPCEvent<typeof eventCodes.SPANNER_RPC>;
  [eventCodes.SPANNER_RPC_DENIED]: RawSpannerRPCEvent<
    typeof eventCodes.SPANNER_RPC_DENIED
  >;
  [eventCodes.DISCOVERY_CONFIG_CREATE]: RawEvent<
    typeof eventCodes.DISCOVERY_CONFIG_CREATE,
    HasName
  >;
  [eventCodes.DISCOVERY_CONFIG_UPDATE]: RawEvent<
    typeof eventCodes.DISCOVERY_CONFIG_UPDATE,
    HasName
  >;
  [eventCodes.DISCOVERY_CONFIG_DELETE]: RawEvent<
    typeof eventCodes.DISCOVERY_CONFIG_DELETE,
    HasName
  >;
  [eventCodes.DISCOVERY_CONFIG_DELETE_ALL]: RawEvent<
    typeof eventCodes.DISCOVERY_CONFIG_DELETE_ALL
  >;
  [eventCodes.INTEGRATION_CREATE]: RawEvent<
    typeof eventCodes.INTEGRATION_CREATE,
    HasName
  >;
  [eventCodes.INTEGRATION_UPDATE]: RawEvent<
    typeof eventCodes.INTEGRATION_UPDATE,
    HasName
  >;
  [eventCodes.INTEGRATION_DELETE]: RawEvent<
    typeof eventCodes.INTEGRATION_DELETE,
    HasName
  >;
  [eventCodes.STATIC_HOST_USER_CREATE]: RawEvent<
    typeof eventCodes.STATIC_HOST_USER_CREATE,
    HasName
  >;
  [eventCodes.STATIC_HOST_USER_UPDATE]: RawEvent<
    typeof eventCodes.STATIC_HOST_USER_UPDATE,
    HasName
  >;
  [eventCodes.STATIC_HOST_USER_DELETE]: RawEvent<
    typeof eventCodes.STATIC_HOST_USER_DELETE,
    HasName
  >;
  [eventCodes.CROWN_JEWEL_CREATE]: RawEvent<
    typeof eventCodes.CROWN_JEWEL_CREATE,
    HasName
  >;
  [eventCodes.CROWN_JEWEL_UPDATE]: RawEvent<
    typeof eventCodes.CROWN_JEWEL_UPDATE,
    HasName
  >;
  [eventCodes.CROWN_JEWEL_DELETE]: RawEvent<
    typeof eventCodes.CROWN_JEWEL_DELETE,
    HasName
  >;
  [eventCodes.USER_TASK_CREATE]: RawEvent<
    typeof eventCodes.USER_TASK_CREATE,
    HasName
  >;
  [eventCodes.USER_TASK_UPDATE]: RawEvent<
    typeof eventCodes.USER_TASK_UPDATE,
    HasName
  >;
  [eventCodes.USER_TASK_DELETE]: RawEvent<
    typeof eventCodes.USER_TASK_DELETE,
    HasName
  >;
  [eventCodes.PLUGIN_CREATE]: RawEvent<
    typeof eventCodes.PLUGIN_CREATE,
    Merge<HasName, { plugin_type: string }>
  >;
  [eventCodes.PLUGIN_UPDATE]: RawEvent<
    typeof eventCodes.PLUGIN_UPDATE,
    Merge<HasName, { plugin_type: string }>
  >;
  [eventCodes.PLUGIN_DELETE]: RawEvent<
    typeof eventCodes.PLUGIN_DELETE,
    Merge<HasName, { user: string }>
  >;
  [eventCodes.SFTP_SUMMARY]: RawEvent<
    typeof eventCodes.SFTP_SUMMARY,
    {
      user: string;
      server_hostname: string;
    }
  >;
  [eventCodes.CONTACT_CREATE]: RawEvent<
    typeof eventCodes.CONTACT_CREATE,
    {
      email: string;
      contact_type: number;
    }
  >;
  [eventCodes.CONTACT_DELETE]: RawEvent<
    typeof eventCodes.CONTACT_DELETE,
    {
      email: string;
      contact_type: number;
    }
  >;
  [eventCodes.GIT_COMMAND]: RawEvent<
    typeof eventCodes.GIT_COMMAND,
    {
      service: string;
      path: string;
      actions?: {
        action: string;
        reference: string;
        new?: string;
        old?: string;
      }[];
    }
  >;
  [eventCodes.GIT_COMMAND_FAILURE]: RawEvent<
    typeof eventCodes.GIT_COMMAND_FAILURE,
    {
      service: string;
      path: string;
      exitError: string;
    }
  >;
  [eventCodes.STABLE_UNIX_USER_CREATE]: RawEvent<
    typeof eventCodes.STABLE_UNIX_USER_CREATE,
    {
      stable_unix_user: {
        username: string;
        uid: number;
      };
    }
  >;
  [eventCodes.AWS_IC_RESOURCE_SYNC_SUCCESS]: RawEventAwsIcResourceSync<
    typeof eventCodes.AWS_IC_RESOURCE_SYNC_SUCCESS
  >;
  [eventCodes.AWS_IC_RESOURCE_SYNC_FAILURE]: RawEventAwsIcResourceSync<
    typeof eventCodes.AWS_IC_RESOURCE_SYNC_FAILURE
  >;
  [eventCodes.AWSIC_ACCOUNT_ASSIGNMENT_CREATE]: RawEventAwsIcAccountAssignment<
    typeof eventCodes.AWSIC_ACCOUNT_ASSIGNMENT_CREATE
  >;
  [eventCodes.AWSIC_ACCOUNT_ASSIGNMENT_DELETE]: RawEventAwsIcAccountAssignment<
    typeof eventCodes.AWSIC_ACCOUNT_ASSIGNMENT_DELETE
  >;
};

/**
 * Event Code
 */
export type EventCode = (typeof eventCodes)[keyof typeof eventCodes];

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

type RawDeviceEvent<T extends EventCode> = RawEvent<
  T,
  {
    device: { asset_tag: string; device_id: string; os_type: number };
    success?: boolean;
    user?: string;
    // status from "legacy" event format.
    status?: { success: boolean };
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
    action: number;
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

type RawEventAccessList<T extends EventCode> = RawEvent<
  T,
  {
    access_list_name: string;
    members: { member_name: string }[];
    updated_by: string;
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

type RawEventSFTP<T extends EventCode> = RawEvent<
  T,
  {
    path: string;
    error: string;
    ['addr.local']: string;
  }
>;

type RawDatabaseSessionEvent<T extends EventCode, K = unknown> = Merge<
  RawEvent<
    T,
    {
      name: string;
      db_service: string;
      db_name: string;
      db_user: string;
      username: string;
    }
  >,
  K
>;

type RawSpannerRPCEvent<T extends EventCode> = RawEvent<
  T,
  {
    procedure: string;
    db_service: string;
    db_name: string;
    args: { sql?: string };
  }
>;

/**
 * RawEventAwsIcResourceSync extends RawEvent with custom fields
 * present in the AWS Identity Center resource sync event.
 */
type RawEventAwsIcResourceSync<T extends EventCode> = RawEvent<
  T,
  {
    total_accounts: number;
    total_account_assignments: number;
    total_user_groups: number;
    total_permission_sets: number;
    status: boolean;
    /* message contains user message for both success and failed status */
    message: string;
  }
>;

/**
 * RawEventAwsIcAccountAssignment extends RawEvent with fields
 * present in the AWS Identity Center account assignment event.
 */
type RawEventAwsIcAccountAssignment<T extends EventCode> = RawEvent<
  T,
  {
    principal_metadata: {
      name: string;
      friendly_name: string;
      external_id: string;
      principal_type: string;
    };
    principal_assignments: { account_id: string; permission_set_arn: string }[];
  }
>;

/**
 * A map of event formatters that provide short and long description
 */
export type Formatters = {
  [key in EventCode]: {
    type: string;
    desc: string | ((json: RawEvents[key]) => string);
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
