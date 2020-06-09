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
import Icon, * as Icons from 'design/Icon/Icon';
import { CodeEnum } from 'teleport/services/audit/types';

const EventIconMap = {
  [CodeEnum.AUTH_ATTEMPT_FAILURE]: Icons.VpnKey,
  [CodeEnum.EXEC_FAILURE]: Icons.Code,
  [CodeEnum.EXEC]: Icons.Code,
  [CodeEnum.GITHUB_CONNECTOR_CREATED]: Icons.NoteAdded,
  [CodeEnum.GITHUB_CONNECTOR_DELETED]: Icons.NoteAdded,
  [CodeEnum.OIDC_CONNECTOR_CREATED]: Icons.NoteAdded,
  [CodeEnum.OIDC_CONNECTOR_DELETED]: Icons.NoteAdded,
  [CodeEnum.SAML_CONNECTOR_CREATED]: Icons.NoteAdded,
  [CodeEnum.SAML_CONNECTOR_CREATED]: Icons.NoteAdded,
  [CodeEnum.SAML_CONNECTOR_DELETED]: Icons.NoteAdded,
  [CodeEnum.ROLE_CREATED]: Icons.Person,
  [CodeEnum.ROLE_DELETED]: Icons.Person,
  [CodeEnum.SCP_DOWNLOAD_FAILURE]: Icons.Download,
  [CodeEnum.SCP_DOWNLOAD_FAILURE]: Icons.Upload,
  [CodeEnum.SCP_DOWNLOAD]: Icons.Download,
  [CodeEnum.SCP_UPLOAD]: Icons.Upload,
  [CodeEnum.SESSION_END]: Icons.Cli,
  [CodeEnum.SESSION_JOIN]: Icons.Cli,
  [CodeEnum.SESSION_LEAVE]: Icons.Cli,
  [CodeEnum.SESSION_START]: Icons.Cli,
  [CodeEnum.SESSION_UPLOAD]: Icons.Cli,
  [CodeEnum.TERMINAL_RESIZE]: Icons.Cli,
  [CodeEnum.SESSION_DATA]: Icons.Cli,
  [CodeEnum.SESSION_NETWORK]: Icons.Cli,
  [CodeEnum.SESSION_DISK]: Icons.Cli,
  [CodeEnum.USER_CREATED]: Icons.Person,
  [CodeEnum.USER_DELETED]: Icons.Person,
  [CodeEnum.RESET_PASSWORD_TOKEN_CREATED]: Icons.Person,
  [CodeEnum.USER_PASSWORD_CHANGED]: Icons.Person,
  [CodeEnum.ACCESS_REQUEST_CREATED]: Icons.Person,
  [CodeEnum.ACCESS_REQUEST_UPDATED]: Icons.Person,
  [CodeEnum.USER_LOCAL_LOGIN]: Icons.Person,
  [CodeEnum.USER_LOCAL_LOGINFAILURE]: Icons.Person,
  [CodeEnum.USER_SSO_LOGIN]: Icons.Person,
  [CodeEnum.USER_SSO_LOGINFAILURE]: Icons.Person,
  [CodeEnum.G_ALERT_CREATED]: Icons.NotificationsActive,
  [CodeEnum.G_ALERT_DELETED]: Icons.NotificationsActive,
  [CodeEnum.G_APPLICATION_INSTALL]: Icons.AppInstalled,
  [CodeEnum.G_APPLICATION_ROLLBACK]: Icons.AppRollback,
  [CodeEnum.G_APPLICATION_UNINSTALL]: Icons.PhonelinkErase,
  [CodeEnum.G_APPLICATION_UPGRADE]: Icons.PhonelinkSetup,
  [CodeEnum.G_AUTHGATEWAY_UPDATED]: Icons.Config,
  [CodeEnum.G_LICENSE_EXPIRED]: Icons.NoteAdded,
  [CodeEnum.G_LICENSE_UPDATED]: Icons.NoteAdded,
  [CodeEnum.G_LOGFORWARDER_CREATED]: Icons.ForwarderAdded,
  [CodeEnum.G_LOGFORWARDER_DELETED]: Icons.ForwarderAdded,
  [CodeEnum.G_OPERATION_ENV_COMPLETE]: Icons.Memory,
  [CodeEnum.G_OPERATION_ENV_FAILURE]: Icons.Memory,
  [CodeEnum.G_OPERATION_ENV_START]: Icons.NoteAdded,
  [CodeEnum.G_OPERATION_EXPAND_COMPLETE]: Icons.SettingsOverscan,
  [CodeEnum.G_OPERATION_EXPAND_START]: Icons.SettingsOverscan,
  [CodeEnum.G_OPERATION_INSTALL_COMPLETE]: Icons.Unarchive,
  [CodeEnum.G_OPERATION_INSTALL_FAILURE]: Icons.Unarchive,
  [CodeEnum.G_OPERATION_INSTALL_START]: Icons.Unarchive,
  [CodeEnum.G_OPERATION_SHRINK_COMPLETE]: Icons.Shrink,
  [CodeEnum.G_OPERATION_SHRINK_FAILURE]: Icons.Shrink,
  [CodeEnum.G_OPERATION_SHRINK_START]: Icons.Shrink,
  [CodeEnum.G_REMOTE_SUPPORT_DISABLED]: Icons.LanAlt,
  [CodeEnum.G_REMOTE_SUPPORT_ENABLED]: Icons.LanAlt,
  [CodeEnum.G_SMTPCONFIG_CREATED]: Icons.EmailSolid,
  [CodeEnum.G_SMTPCONFIG_DELETED]: Icons.EmailSolid,
  [CodeEnum.G_TLSKEYPAIR_CREATED]: Icons.Keypair,
  [CodeEnum.G_TLSKEYPAIR_DELETED]: Icons.Keypair,
  [CodeEnum.G_TOKEN_CREATED]: Icons.Stars,
  [CodeEnum.G_TOKEN_DELETED]: Icons.Stars,
  [CodeEnum.G_UPDATES_DISABLED]: Icons.Restore,
  [CodeEnum.G_UPDATES_DOWNLOADED]: Icons.Restore,
  [CodeEnum.G_UPDATES_ENABLED]: Icons.Restore,
  [CodeEnum.G_USER_INVITE_CREATED]: Icons.Person,
};

export default function TypeCell(props) {
  const { rowIndex, data } = props;
  const { codeDesc, code } = data[rowIndex];
  const IconType = EventIconMap[code] || Icons.List;

  return (
    <Cell>
      <StyledEventType>
        <Icon p="1" mr="3" as={IconType} fontSize="3" />
        {codeDesc}
      </StyledEventType>
    </Cell>
  );
}

const StyledEventType = styled.div`
  display: flex;
  align-items: center;
  min-width: 130px;
  font-size: 12px;
  font-weight: 500;
  line-height: 24px;
  white-space: nowrap;
`;
