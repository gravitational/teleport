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

import React from 'react';
import styled from 'styled-components';
import { Cell } from 'design/DataTable';
import * as Icons from 'design/Icon';

import { eventCodes, Event, EventCode } from 'teleport/services/audit';
import cfg from 'teleport/config';

const EventIconMap: Record<EventCode, any> = {
  [eventCodes.AUTH_ATTEMPT_FAILURE]: Icons.Info,
  [eventCodes.EXEC_FAILURE]: Icons.Cli,
  [eventCodes.EXEC]: Icons.Cli,
  [eventCodes.TRUSTED_CLUSTER_TOKEN_CREATED]: Icons.Info,
  [eventCodes.TRUSTED_CLUSTER_CREATED]: Icons.Info,
  [eventCodes.TRUSTED_CLUSTER_DELETED]: Icons.Info,
  [eventCodes.PROVISION_TOKEN_CREATED]: Icons.Info,
  [eventCodes.GITHUB_CONNECTOR_CREATED]: Icons.Info,
  [eventCodes.GITHUB_CONNECTOR_DELETED]: Icons.Info,
  [eventCodes.OIDC_CONNECTOR_CREATED]: Icons.Info,
  [eventCodes.OIDC_CONNECTOR_DELETED]: Icons.Info,
  [eventCodes.SAML_CONNECTOR_CREATED]: Icons.Info,
  [eventCodes.SAML_CONNECTOR_DELETED]: Icons.Info,
  [eventCodes.ROLE_CREATED]: Icons.Info,
  [eventCodes.ROLE_DELETED]: Icons.Info,
  [eventCodes.SCP_DOWNLOAD_FAILURE]: Icons.Download,
  [eventCodes.SCP_DOWNLOAD]: Icons.Download,
  [eventCodes.SCP_UPLOAD_FAILURE]: Icons.Upload,
  [eventCodes.SCP_UPLOAD]: Icons.Upload,
  [eventCodes.SCP_DISALLOWED]: Icons.FolderPlus,
  [eventCodes.SFTP_OPEN_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_OPEN]: Icons.FolderPlus,
  [eventCodes.SFTP_CLOSE_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_CLOSE]: Icons.FolderPlus,
  [eventCodes.SFTP_READ_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_READ]: Icons.FolderPlus,
  [eventCodes.SFTP_WRITE_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_WRITE]: Icons.FolderPlus,
  [eventCodes.SFTP_LSTAT_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_LSTAT]: Icons.FolderPlus,
  [eventCodes.SFTP_FSTAT_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_FSTAT]: Icons.FolderPlus,
  [eventCodes.SFTP_SETSTAT_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_SETSTAT]: Icons.FolderPlus,
  [eventCodes.SFTP_FSETSTAT_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_FSETSTAT]: Icons.FolderPlus,
  [eventCodes.SFTP_OPENDIR_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_OPENDIR]: Icons.FolderPlus,
  [eventCodes.SFTP_READDIR_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_READDIR]: Icons.FolderPlus,
  [eventCodes.SFTP_REMOVE_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_REMOVE]: Icons.FolderPlus,
  [eventCodes.SFTP_MKDIR_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_MKDIR]: Icons.FolderPlus,
  [eventCodes.SFTP_RMDIR_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_RMDIR]: Icons.FolderPlus,
  [eventCodes.SFTP_REALPATH_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_REALPATH]: Icons.FolderPlus,
  [eventCodes.SFTP_STAT_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_STAT]: Icons.FolderPlus,
  [eventCodes.SFTP_RENAME_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_RENAME]: Icons.FolderPlus,
  [eventCodes.SFTP_READLINK_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_READLINK]: Icons.FolderPlus,
  [eventCodes.SFTP_SYMLINK_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_SYMLINK]: Icons.FolderPlus,
  [eventCodes.SFTP_LINK]: Icons.FolderPlus,
  [eventCodes.SFTP_LINK_FAILURE]: Icons.FolderPlus,
  [eventCodes.SFTP_DISALLOWED]: Icons.FolderPlus,
  [eventCodes.APP_SESSION_CHUNK]: Icons.Info,
  [eventCodes.APP_SESSION_START]: Icons.Info,
  [eventCodes.APP_SESSION_END]: Icons.Info,
  [eventCodes.APP_SESSION_DYNAMODB_REQUEST]: Icons.Database,
  [eventCodes.APP_CREATED]: Icons.Code,
  [eventCodes.APP_UPDATED]: Icons.Code,
  [eventCodes.APP_DELETED]: Icons.Code,
  [eventCodes.SESSION_END]: Icons.Cli,
  [eventCodes.SESSION_JOIN]: Icons.Cli,
  [eventCodes.SESSION_LEAVE]: Icons.Cli,
  [eventCodes.SESSION_START]: Icons.Cli,
  [eventCodes.SESSION_UPLOAD]: Icons.Cli,
  [eventCodes.SESSION_REJECT]: Icons.Cli,
  [eventCodes.TERMINAL_RESIZE]: Icons.Cli,
  [eventCodes.SESSION_DATA]: Icons.Cli,
  [eventCodes.SESSION_NETWORK]: Icons.Cli,
  [eventCodes.SESSION_DISK]: Icons.Cli,
  [eventCodes.SESSION_COMMAND]: Icons.Cli,
  [eventCodes.SESSION_PROCESS_EXIT]: Icons.Cli,
  [eventCodes.SESSION_CONNECT]: Icons.Cli,
  [eventCodes.USER_CREATED]: Icons.Info,
  [eventCodes.USER_UPDATED]: Icons.Info,
  [eventCodes.USER_DELETED]: Icons.Info,
  [eventCodes.RESET_PASSWORD_TOKEN_CREATED]: Icons.Info,
  [eventCodes.USER_PASSWORD_CHANGED]: Icons.Info,
  [eventCodes.ACCESS_REQUEST_CREATED]: Icons.Info,
  [eventCodes.ACCESS_REQUEST_UPDATED]: Icons.Info,
  [eventCodes.ACCESS_REQUEST_REVIEWED]: Icons.Info,
  [eventCodes.ACCESS_REQUEST_DELETED]: Icons.Info,
  [eventCodes.ACCESS_REQUEST_RESOURCE_SEARCH]: Icons.Info,
  [eventCodes.USER_LOCAL_LOGIN]: Icons.Info,
  [eventCodes.USER_LOCAL_LOGINFAILURE]: Icons.Info,
  [eventCodes.USER_SSO_LOGIN]: Icons.Info,
  [eventCodes.USER_SSO_LOGINFAILURE]: Icons.Info,
  [eventCodes.USER_SSO_TEST_FLOW_LOGIN]: Icons.Info,
  [eventCodes.USER_SSO_TEST_FLOW_LOGINFAILURE]: Icons.Info,
  [eventCodes.KUBE_REQUEST]: Icons.Kubernetes,
  [eventCodes.KUBE_CREATED]: Icons.Kubernetes,
  [eventCodes.KUBE_UPDATED]: Icons.Kubernetes,
  [eventCodes.KUBE_DELETED]: Icons.Kubernetes,
  [eventCodes.DATABASE_SESSION_STARTED]: Icons.Database,
  [eventCodes.DATABASE_SESSION_STARTED_FAILURE]: Icons.Database,
  [eventCodes.DATABASE_SESSION_ENDED]: Icons.Database,
  [eventCodes.DATABASE_SESSION_QUERY]: Icons.Database,
  [eventCodes.DATABASE_SESSION_QUERY_FAILURE]: Icons.Database,
  [eventCodes.DATABASE_SESSION_MALFORMED_PACKET]: Icons.Database,
  [eventCodes.DATABASE_CREATED]: Icons.Database,
  [eventCodes.DATABASE_UPDATED]: Icons.Database,
  [eventCodes.DATABASE_DELETED]: Icons.Database,
  [eventCodes.POSTGRES_PARSE]: Icons.Database,
  [eventCodes.POSTGRES_BIND]: Icons.Database,
  [eventCodes.POSTGRES_EXECUTE]: Icons.Database,
  [eventCodes.POSTGRES_CLOSE]: Icons.Database,
  [eventCodes.POSTGRES_FUNCTION_CALL]: Icons.Database,
  [eventCodes.MYSQL_STATEMENT_PREPARE]: Icons.Database,
  [eventCodes.MYSQL_STATEMENT_EXECUTE]: Icons.Database,
  [eventCodes.MYSQL_STATEMENT_SEND_LONG_DATA]: Icons.Database,
  [eventCodes.MYSQL_STATEMENT_CLOSE]: Icons.Database,
  [eventCodes.MYSQL_STATEMENT_RESET]: Icons.Database,
  [eventCodes.MYSQL_STATEMENT_FETCH]: Icons.Database,
  [eventCodes.MYSQL_STATEMENT_BULK_EXECUTE]: Icons.Database,
  [eventCodes.MYSQL_INIT_DB]: Icons.Database,
  [eventCodes.MYSQL_CREATE_DB]: Icons.Database,
  [eventCodes.MYSQL_DROP_DB]: Icons.Database,
  [eventCodes.MYSQL_SHUT_DOWN]: Icons.Database,
  [eventCodes.MYSQL_PROCESS_KILL]: Icons.Database,
  [eventCodes.MYSQL_DEBUG]: Icons.Database,
  [eventCodes.MYSQL_REFRESH]: Icons.Database,
  [eventCodes.SQLSERVER_RPC_REQUEST]: Icons.Database,
  [eventCodes.CASSANDRA_BATCH_EVENT]: Icons.Database,
  [eventCodes.CASSANDRA_EXECUTE_EVENT]: Icons.Database,
  [eventCodes.CASSANDRA_PREPARE_EVENT]: Icons.Database,
  [eventCodes.CASSANDRA_REGISTER_EVENT]: Icons.Database,
  [eventCodes.ELASTICSEARCH_REQUEST]: Icons.Database,
  [eventCodes.ELASTICSEARCH_REQUEST_FAILURE]: Icons.Database,
  [eventCodes.OPENSEARCH_REQUEST]: Icons.Database,
  [eventCodes.OPENSEARCH_REQUEST_FAILURE]: Icons.Database,
  [eventCodes.DYNAMODB_REQUEST]: Icons.Database,
  [eventCodes.DYNAMODB_REQUEST_FAILURE]: Icons.Database,
  [eventCodes.DESKTOP_SESSION_STARTED]: Icons.Desktop,
  [eventCodes.DESKTOP_SESSION_STARTED_FAILED]: Icons.Desktop,
  [eventCodes.DESKTOP_SESSION_ENDED]: Icons.Desktop,
  [eventCodes.DESKTOP_CLIPBOARD_SEND]: Icons.Clipboard,
  [eventCodes.DESKTOP_CLIPBOARD_RECEIVE]: Icons.Clipboard,
  [eventCodes.DESKTOP_SHARED_DIRECTORY_START]: Icons.FolderShared,
  [eventCodes.DESKTOP_SHARED_DIRECTORY_START_FAILURE]: Icons.FolderShared,
  [eventCodes.DESKTOP_SHARED_DIRECTORY_READ]: Icons.FolderShared,
  [eventCodes.DESKTOP_SHARED_DIRECTORY_READ_FAILURE]: Icons.FolderShared,
  [eventCodes.DESKTOP_SHARED_DIRECTORY_WRITE]: Icons.FolderShared,
  [eventCodes.DESKTOP_SHARED_DIRECTORY_WRITE_FAILURE]: Icons.FolderShared,
  [eventCodes.DEVICE_AUTHENTICATE]: Icons.Info,
  [eventCodes.DEVICE_CREATE]: Icons.Info,
  [eventCodes.DEVICE_DELETE]: Icons.Info,
  [eventCodes.DEVICE_ENROLL]: Icons.Info,
  [eventCodes.DEVICE_ENROLL_TOKEN_CREATE]: Icons.Info,
  [eventCodes.DEVICE_ENROLL_TOKEN_SPENT]: Icons.Info,
  [eventCodes.DEVICE_UPDATE]: Icons.Info,
  [eventCodes.MFA_DEVICE_ADD]: Icons.Info,
  [eventCodes.MFA_DEVICE_DELETE]: Icons.Info,
  [eventCodes.BILLING_CARD_CREATE]: Icons.CreditCard,
  [eventCodes.BILLING_CARD_DELETE]: Icons.CreditCard,
  [eventCodes.BILLING_CARD_UPDATE]: Icons.CreditCard,
  [eventCodes.BILLING_INFORMATION_UPDATE]: Icons.CreditCard,
  [eventCodes.CLIENT_DISCONNECT]: Icons.Info,
  [eventCodes.PORTFORWARD]: Icons.Info,
  [eventCodes.PORTFORWARD_FAILURE]: Icons.Info,
  [eventCodes.SUBSYSTEM]: Icons.Info,
  [eventCodes.SUBSYSTEM_FAILURE]: Icons.Info,
  [eventCodes.LOCK_CREATED]: Icons.Lock,
  [eventCodes.LOCK_DELETED]: Icons.Unlock,
  [eventCodes.RECOVERY_TOKEN_CREATED]: Icons.Info,
  [eventCodes.RECOVERY_CODE_GENERATED]: Icons.Keypair,
  [eventCodes.RECOVERY_CODE_USED]: Icons.VpnKey,
  [eventCodes.RECOVERY_CODE_USED_FAILURE]: Icons.VpnKey,
  [eventCodes.PRIVILEGE_TOKEN_CREATED]: Icons.Info,
  [eventCodes.X11_FORWARD]: Icons.Info,
  [eventCodes.X11_FORWARD_FAILURE]: Icons.Info,
  [eventCodes.CERTIFICATE_CREATED]: Icons.Keypair,
  [eventCodes.UPGRADE_WINDOW_UPDATED]: Icons.Info,
  [eventCodes.SESSION_RECORDING_ACCESS]: Icons.Info,
  [eventCodes.SSMRUN_SUCCESS]: Icons.Info,
  [eventCodes.SSMRUN_FAIL]: Icons.Info,
  [eventCodes.BOT_JOIN]: Icons.Info,
  [eventCodes.BOT_JOIN_FAILURE]: Icons.Warning,
  [eventCodes.INSTANCE_JOIN]: Icons.Info,
  [eventCodes.INSTANCE_JOIN_FAILURE]: Icons.Warning,
  [eventCodes.LOGIN_RULE_CREATE]: Icons.Info,
  [eventCodes.LOGIN_RULE_DELETE]: Icons.Info,
  [eventCodes.SAML_IDP_AUTH_ATTEMPT]: Icons.Info,
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_CREATE]: Icons.Info,
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_CREATE_FAILURE]: Icons.Info,
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_UPDATE]: Icons.Info,
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_UPDATE_FAILURE]: Icons.Info,
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE]: Icons.Info,
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE_FAILURE]: Icons.Info,
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE_ALL]: Icons.Info,
  [eventCodes.SAML_IDP_SERVICE_PROVIDER_DELETE_ALL_FAILURE]: Icons.Info,
  [eventCodes.OKTA_GROUPS_UPDATE]: Icons.Info,
  [eventCodes.OKTA_APPLICATIONS_UPDATE]: Icons.Info,
  [eventCodes.OKTA_SYNC_FAILURE]: Icons.Warning,
  [eventCodes.OKTA_ASSIGNMENT_PROCESS]: Icons.Info,
  [eventCodes.OKTA_ASSIGNMENT_PROCESS_FAILURE]: Icons.Warning,
  [eventCodes.OKTA_ASSIGNMENT_CLEANUP]: Icons.Info,
  [eventCodes.OKTA_ASSIGNMENT_CLEANUP_FAILURE]: Icons.Warning,
  [eventCodes.ACCESS_LIST_CREATE]: Icons.Info,
  [eventCodes.ACCESS_LIST_CREATE_FAILURE]: Icons.Warning,
  [eventCodes.ACCESS_LIST_UPDATE]: Icons.Info,
  [eventCodes.ACCESS_LIST_UPDATE_FAILURE]: Icons.Warning,
  [eventCodes.ACCESS_LIST_DELETE]: Icons.Info,
  [eventCodes.ACCESS_LIST_DELETE_FAILURE]: Icons.Warning,
  [eventCodes.ACCESS_LIST_REVIEW]: Icons.Info,
  [eventCodes.ACCESS_LIST_REVIEW_FAILURE]: Icons.Warning,
  [eventCodes.ACCESS_LIST_MEMBER_CREATE]: Icons.User,
  [eventCodes.ACCESS_LIST_MEMBER_CREATE_FAILURE]: Icons.Warning,
  [eventCodes.ACCESS_LIST_MEMBER_UPDATE]: Icons.User,
  [eventCodes.ACCESS_LIST_MEMBER_UPDATE_FAILURE]: Icons.Warning,
  [eventCodes.ACCESS_LIST_MEMBER_DELETE]: Icons.User,
  [eventCodes.ACCESS_LIST_MEMBER_DELETE_FAILURE]: Icons.Warning,
  [eventCodes.ACCESS_LIST_MEMBER_DELETE_ALL_FOR_ACCESS_LIST]: Icons.User,
  [eventCodes.ACCESS_LIST_MEMBER_DELETE_ALL_FOR_ACCESS_LIST_FAILURE]:
    Icons.Warning,
  [eventCodes.SECURITY_REPORT_AUDIT_QUERY_RUN]: Icons.Info,
  [eventCodes.SECURITY_REPORT_RUN]: Icons.Info,
  [eventCodes.EXTERNAL_AUDIT_STORAGE_ENABLE]: Icons.Database,
  [eventCodes.EXTERNAL_AUDIT_STORAGE_DISABLE]: Icons.Database,
  [eventCodes.DISCOVERY_CONFIG_CREATE]: Icons.Info,
  [eventCodes.DISCOVERY_CONFIG_UPDATE]: Icons.Info,
  [eventCodes.DISCOVERY_CONFIG_DELETE]: Icons.Info,
  [eventCodes.DISCOVERY_CONFIG_DELETE_ALL]: Icons.Info,
  [eventCodes.INTEGRATION_CREATE]: Icons.Info,
  [eventCodes.INTEGRATION_UPDATE]: Icons.Info,
  [eventCodes.INTEGRATION_DELETE]: Icons.Info,
  [eventCodes.UNKNOWN]: Icons.Question,
};

export default function renderTypeCell(event: Event, clusterId: string) {
  const Icon = EventIconMap[event.code] || Icons.ListThin;

  const iconProps = {
    p: 1,
    mr: 3,
  };

  // use button for interactive ssh sessions
  if (
    event.code === eventCodes.SESSION_END &&
    event.raw.interactive &&
    event.raw.session_recording !== 'off'
  ) {
    return (
      <Cell style={{ verticalAlign: 'inherit' }}>
        <StyledEventType>
          <a
            title="Open Session Player"
            href={cfg.getPlayerRoute(
              {
                clusterId,
                sid: event.raw.sid,
              },
              {
                recordingType: 'ssh',
              }
            )}
            target="_blank"
            style={{ textDecoration: 'none' }}
          >
            <StyledCliIcon {...iconProps} />
          </a>
          {event.codeDesc}
        </StyledEventType>
      </Cell>
    );
  }

  return (
    <Cell style={{ verticalAlign: 'inherit' }}>
      <StyledEventType>
        <Icon {...iconProps} size="medium" />
        {event.codeDesc}
      </StyledEventType>
    </Cell>
  );
}

const StyledCliIcon = styled(Icons.Cli)(
  props => `
  background: ${props.theme.colors.levels.deep};
  border: 2px solid ${props.theme.colors.brand};
  color: ${props.theme.colors.text.slightlyMuted};
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  border-radius: 100px;
  transition: all 0.3s;

  &:hover,
  &:active,
  &:focus {
    background: ${props.theme.colors.levels.sunken};
    color: ${props.theme.colors.text.main};
  }

  &:active {
    box-shadow: none;
    opacity: 0.56;
  }
`
);

const StyledEventType = styled.div`
  display: flex;
  align-items: center;
  min-width: 130px;
  font-size: 12px;
  font-weight: 500;
  line-height: 24px;
  white-space: nowrap;
`;
