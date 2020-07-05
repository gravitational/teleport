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
import { Event } from 'teleport/services/audit';
import cfg from 'teleport/config';

const EventIconMap = {
  [CodeEnum.AUTH_ATTEMPT_FAILURE]: Icons.Info,
  [CodeEnum.EXEC_FAILURE]: Icons.Cli,
  [CodeEnum.EXEC]: Icons.Cli,
  [CodeEnum.TRUSTED_CLUSTER_TOKEN_CREATED]: Icons.Info,
  [CodeEnum.GITHUB_CONNECTOR_CREATED]: Icons.Info,
  [CodeEnum.GITHUB_CONNECTOR_DELETED]: Icons.Info,
  [CodeEnum.OIDC_CONNECTOR_CREATED]: Icons.Info,
  [CodeEnum.OIDC_CONNECTOR_DELETED]: Icons.Info,
  [CodeEnum.SAML_CONNECTOR_CREATED]: Icons.Info,
  [CodeEnum.SAML_CONNECTOR_CREATED]: Icons.Info,
  [CodeEnum.SAML_CONNECTOR_DELETED]: Icons.Info,
  [CodeEnum.ROLE_CREATED]: Icons.Info,
  [CodeEnum.ROLE_DELETED]: Icons.Info,
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
  [CodeEnum.SESSION_COMMAND]: Icons.Cli,
  [CodeEnum.USER_CREATED]: Icons.Info,
  [CodeEnum.USER_UPDATED]: Icons.Info,
  [CodeEnum.USER_DELETED]: Icons.Info,
  [CodeEnum.RESET_PASSWORD_TOKEN_CREATED]: Icons.Info,
  [CodeEnum.USER_PASSWORD_CHANGED]: Icons.Info,
  [CodeEnum.ACCESS_REQUEST_CREATED]: Icons.Info,
  [CodeEnum.ACCESS_REQUEST_UPDATED]: Icons.Info,
  [CodeEnum.USER_LOCAL_LOGIN]: Icons.Info,
  [CodeEnum.USER_LOCAL_LOGINFAILURE]: Icons.Info,
  [CodeEnum.USER_SSO_LOGIN]: Icons.Info,
  [CodeEnum.USER_SSO_LOGINFAILURE]: Icons.Info,
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
  [CodeEnum.G_USER_INVITE_CREATED]: Icons.Info,
};

export default function TypeCell(props) {
  const { rowIndex, data } = props;
  const event: Event = data[rowIndex];
  let IconType = EventIconMap[event.code] || Icons.List;

  const iconProps = {
    p: '1',
    mr: '3',
    fontSize: '3',
  };

  // use button for interactive ssh sessions
  if (event.code === CodeEnum.SESSION_END && event.raw.interactive) {
    return (
      <Cell>
        <StyledEventType>
          <a
            title="Open Session Player"
            href={cfg.getPlayerRoute({ sid: event.raw.sid })}
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
    <Cell>
      <StyledEventType>
        <Icon {...iconProps} as={IconType} />
        {event.codeDesc}
      </StyledEventType>
    </Cell>
  );
}

const StyledCliIcon = styled(Icons.Cli)(
  props => `
  background: ${props.theme.colors.dark};
  border: 2px solid ${props.theme.colors.accent};
  color: ${props.theme.colors.text.primary};
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
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.24);
    color: ${props.theme.colors.light};
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
