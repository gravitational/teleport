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
import { eventCodes, Event, EventCode } from 'teleport/services/audit';
import cfg from 'teleport/config';

const EventIconMap: Record<EventCode, React.FC> = {
  [eventCodes.AUTH_ATTEMPT_FAILURE]: Icons.Info,
  [eventCodes.EXEC_FAILURE]: Icons.Cli,
  [eventCodes.EXEC]: Icons.Cli,
  [eventCodes.TRUSTED_CLUSTER_TOKEN_CREATED]: Icons.Info,
  [eventCodes.TRUSTED_CLUSTER_CREATED]: Icons.Info,
  [eventCodes.TRUSTED_CLUSTER_DELETED]: Icons.Info,
  [eventCodes.GITHUB_CONNECTOR_CREATED]: Icons.Info,
  [eventCodes.GITHUB_CONNECTOR_DELETED]: Icons.Info,
  [eventCodes.OIDC_CONNECTOR_CREATED]: Icons.Info,
  [eventCodes.OIDC_CONNECTOR_DELETED]: Icons.Info,
  [eventCodes.SAML_CONNECTOR_CREATED]: Icons.Info,
  [eventCodes.SAML_CONNECTOR_CREATED]: Icons.Info,
  [eventCodes.SAML_CONNECTOR_DELETED]: Icons.Info,
  [eventCodes.ROLE_CREATED]: Icons.Info,
  [eventCodes.ROLE_DELETED]: Icons.Info,
  [eventCodes.SCP_DOWNLOAD_FAILURE]: Icons.Download,
  [eventCodes.SCP_DOWNLOAD]: Icons.Download,
  [eventCodes.SCP_UPLOAD_FAILURE]: Icons.Upload,
  [eventCodes.SCP_UPLOAD]: Icons.Upload,
  [eventCodes.APP_SESSION_CHUNK]: Icons.Info,
  [eventCodes.APP_SESSION_START]: Icons.Info,
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
  [eventCodes.USER_LOCAL_LOGIN]: Icons.Info,
  [eventCodes.USER_LOCAL_LOGINFAILURE]: Icons.Info,
  [eventCodes.USER_SSO_LOGIN]: Icons.Info,
  [eventCodes.USER_SSO_LOGINFAILURE]: Icons.Info,
  [eventCodes.KUBE_REQUEST]: Icons.Kubernetes,
  [eventCodes.DATABASE_SESSION_STARTED]: Icons.Database,
  [eventCodes.DATABASE_SESSION_STARTED_FAILURE]: Icons.Database,
  [eventCodes.DATABASE_SESSION_ENDED]: Icons.Database,
  [eventCodes.DATABASE_SESSION_QUERY]: Icons.Database,
  [eventCodes.DATABASE_SESSION_QUERY_FAILURE]: Icons.Database,
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
  [eventCodes.DESKTOP_SESSION_STARTED]: Icons.Desktop,
  [eventCodes.DESKTOP_SESSION_STARTED_FAILED]: Icons.Desktop,
  [eventCodes.DESKTOP_SESSION_ENDED]: Icons.Desktop,
  [eventCodes.DESKTOP_CLIPBOARD_SEND]: Icons.Clipboard,
  [eventCodes.DESKTOP_CLIPBOARD_RECEIVE]: Icons.Clipboard,
  [eventCodes.MFA_DEVICE_ADD]: Icons.Info,
  [eventCodes.MFA_DEVICE_DELETE]: Icons.Info,
  [eventCodes.BILLING_CARD_CREATE]: Icons.CreditCardAlt2,
  [eventCodes.BILLING_CARD_DELETE]: Icons.CreditCardAlt2,
  [eventCodes.BILLING_CARD_UPDATE]: Icons.CreditCardAlt2,
  [eventCodes.BILLING_INFORMATION_UPDATE]: Icons.CreditCardAlt2,
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
  [eventCodes.UNKNOWN]: Icons.Question,
};

export default function renderTypeCell(event: Event, clusterId: string) {
  const IconType = EventIconMap[event.code] || Icons.List;

  const iconProps = {
    p: '1',
    mr: '3',
    fontSize: '3',
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
