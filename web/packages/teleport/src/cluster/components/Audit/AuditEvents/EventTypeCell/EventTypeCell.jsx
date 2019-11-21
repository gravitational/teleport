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
import { CodeEnum } from 'teleport/services/events/event';

const EventIconMap = {
  [CodeEnum.ALERT_CREATED]: Icons.NotificationsActive,
  [CodeEnum.ALERT_DELETED]: Icons.NotificationsActive,
  [CodeEnum.APPLICATION_INSTALL]: Icons.AppInstalled,
  [CodeEnum.APPLICATION_ROLLBACK]: Icons.AppRollback,
  [CodeEnum.APPLICATION_UNINSTALL]: Icons.PhonelinkErase,
  [CodeEnum.APPLICATION_UPGRADE]: Icons.PhonelinkSetup,
  [CodeEnum.ROLE_CREATED]: Icons.ClipboardUser,
  [CodeEnum.ROLE_DELETED]: Icons.ClipboardUser,
  [CodeEnum.AUTHGATEWAY_UPDATED]: Icons.Config,
  [CodeEnum.AUTH_ATTEMPT_FAILURE]: Icons.VpnKey,
  [CodeEnum.EXEC]: Icons.Code,
  [CodeEnum.EXEC_FAILURE]: Icons.Code,
  [CodeEnum.OPERATION_EXPAND_COMPLETE]: Icons.SettingsOverscan,
  [CodeEnum.OPERATION_EXPAND_START]: Icons.SettingsOverscan,
  [CodeEnum.GITHUB_CONNECTOR_CREATED]: Icons.NoteAdded,
  [CodeEnum.GITHUB_CONNECTOR_DELETED]: Icons.NoteAdded,
  [CodeEnum.OPERATION_INSTALL_COMPLETE]: Icons.Unarchive,
  [CodeEnum.OPERATION_INSTALL_START]: Icons.Unarchive,
  [CodeEnum.OPERATION_INSTALL_FAILURE]: Icons.Unarchive,
  [CodeEnum.LICENSE_EXPIRED]: Icons.NoteAdded,
  [CodeEnum.LICENSE_UPDATED]: Icons.NoteAdded,
  [CodeEnum.LOGFORWARDER_CREATED]: Icons.ForwarderAdded,
  [CodeEnum.LOGFORWARDER_DELETED]: Icons.ForwarderAdded,
  [CodeEnum.UPDATES_DISABLED]: Icons.Restore,
  [CodeEnum.UPDATES_DOWNLOADED]: Icons.Restore,
  [CodeEnum.UPDATES_ENABLED]: Icons.Restore,
  [CodeEnum.REMOTE_SUPPORT_ENABLED]: Icons.LanAlt,
  [CodeEnum.REMOTE_SUPPORT_DISABLED]: Icons.LanAlt,
  [CodeEnum.OPERATION_ENV_COMPLETE]: Icons.Memory,
  [CodeEnum.OPERATION_ENV_FAILURE]: Icons.Memory,
  [CodeEnum.OPERATION_ENV_START]: Icons.NoteAdded,
  [CodeEnum.SAML_CONNECTOR_DELETED]: Icons.NoteAdded,
  [CodeEnum.SCP_DOWNLOAD]: Icons.Download,
  [CodeEnum.SCP_DOWNLOAD_FAILURE]: Icons.Download,
  [CodeEnum.SCP_UPLOAD]: Icons.Upload,
  [CodeEnum.SCP_DOWNLOAD_FAILURE]: Icons.Upload,
  [CodeEnum.OPERATION_SHRINK_COMPLETE]: Icons.Shrink,
  [CodeEnum.OPERATION_SHRINK_FAILURE]: Icons.Shrink,
  [CodeEnum.OPERATION_SHRINK_START]: Icons.Shrink,
  [CodeEnum.SMTPCONFIG_CREATED]: Icons.EmailSolid,
  [CodeEnum.SMTPCONFIG_DELETED]: Icons.EmailSolid,
  [CodeEnum.TLSKEYPAIR_CREATED]: Icons.Keypair,
  [CodeEnum.TLSKEYPAIR_DELETED]: Icons.Keypair,
  [CodeEnum.TOKEN_CREATED]: Icons.Stars,
  [CodeEnum.TOKEN_DELETED]: Icons.Stars,
  [CodeEnum.USER_CREATE]: Icons.UserCreated,
  [CodeEnum.USER_DELETED]: Icons.UserCreated,
  [CodeEnum.USER_INVITE_CREATED]: Icons.PersonAdd,
  [CodeEnum.USER_LOCAL_LOGIN]: Icons.Person,
  [CodeEnum.USER_LOCAL_LOGINFAILURE]: Icons.Person,
  [CodeEnum.USER_SSO_LOGIN]: Icons.Person,
  [CodeEnum.USER_SSO_LOGINFAILURE]: Icons.Person,
  [CodeEnum.SESSION_START]: Icons.Cli,
  [CodeEnum.SESSION_JOIN]: Icons.Cli,
  [CodeEnum.TERMINAL_RESIZE]: Icons.Cli,
  [CodeEnum.SESSION_LEAVE]: Icons.Cli,
  [CodeEnum.SESSION_END]: Icons.Cli,
  [CodeEnum.SESSION_UPLOAD]: Icons.Cli,
};

function getColor(severity, code) {
  // first pick the color based on event code
  switch (code) {
    case CodeEnum.SESSION_START:
    case CodeEnum.SESSION_JOIN:
    case CodeEnum.TERMINAL_RESIZE:
    case CodeEnum.SESSION_UPLOAD:
    case CodeEnum.EXEC:
      return 'bgTerminal';
    case CodeEnum.REMOTE_SUPPORT_DISABLED:
    case CodeEnum.SESSION_END:
    case CodeEnum.SESSION_LEAVE:
      return 'grey.600';
    case CodeEnum.ALERT_CREATED:
    case CodeEnum.GITHUB_CONNECTOR_CREATED:
    case CodeEnum.LOGFORWARDER_CREATED:
    case CodeEnum.SMTPCONFIG_CREATED:
    case CodeEnum.TLSKEYPAIR_CREATED:
    case CodeEnum.TOKEN_CREATED:
    case CodeEnum.USER_CREATE:
    case CodeEnum.ROLE_CREATED:
    case CodeEnum.USER_INVITE_CREATED:
      return 'success';
    case CodeEnum.AUTH_ATTEMPT_FAILURE:
    case CodeEnum.EXEC_FAILURE:
    case CodeEnum.OPERATION_INSTALL_FAILURE:
    case CodeEnum.OPERATION_ENV_FAILURE:
    case CodeEnum.SCP_DOWNLOAD_FAILURE:
    case CodeEnum.OPERATION_SHRINK_FAILURE:
    case CodeEnum.USER_LOCAL_LOGINFAILURE:
    case CodeEnum.USER_SSO_LOGINFAILURE:
    case CodeEnum.OPERATION_UPDATE_FAILURE:
    case CodeEnum.OPERATION_UNINSTALL_FAILURE:
    case CodeEnum.PORTFORWARD_FAILURE:
    case CodeEnum.SCP_UPLOAD_FAILURE:
    case CodeEnum.SUBSYSTEM_FAILURE:
      return 'danger';
    case CodeEnum.ALERT_DELETED:
    case CodeEnum.ROLE_DELETED:
    case CodeEnum.GITHUB_CONNECTOR_DELETED:
    case CodeEnum.OIDC_CONNECTOR_DELETED:
    case CodeEnum.LOGFORWARDER_DELETED:
    case CodeEnum.SAML_CONNECTOR_DELETED:
    case CodeEnum.SMTPCONFIG_DELETED:
    case CodeEnum.TLSKEYPAIR_DELETED:
    case CodeEnum.TOKEN_DELETED:
    case CodeEnum.USER_DELETED:
      return 'warning';
  }

  return 'info';
}

export default function TypeCell({ rowIndex, data }) {
  const { codeDesc, code, severity } = data[rowIndex];
  const IconType = EventIconMap[code] || Icons.Stars;
  const bgColor = getColor(severity, code);

  return (
    <Cell style={{ fontSize: '14px' }}>
      <StyledEventType>
        <StyledIcon p="1" mr="3" bg={bgColor} as={IconType} fontSize="4" />
        {codeDesc}
      </StyledEventType>
    </Cell>
  );
}

const StyledIcon = styled(Icon)`
  border-radius: 50%;
`;

const StyledEventType = styled.div`
  display: flex;
  align-items: center;
  min-width: 130px;
  font-size: 12px;
  font-weight: 500;
  line-height: 24px;
  white-space: nowrap;
`;
