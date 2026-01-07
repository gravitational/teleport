import { useHistory } from 'react-router-dom';

import { Flex, H2, Text } from 'design';
import { CardTile } from 'design/CardTile/CardTile';
import { PlugsConnected } from 'design/Icon';

import { StatusAndOptions } from 'e-teleport/Integrations/IntegrationStatus/Shared';
import useTeleportE from 'e-teleport/useTeleportE';
import cfg from 'teleport/config';

/**
 * SsoDetails displays details related to the SSO connector referenced
 * by the Entra ID plugin.
 */
export function SsoDetails({ connectorName }: { connectorName: string }) {
  const ctx = useTeleportE();
  const history = useHistory();
  const hasSsoAccess =
    ctx.storeUser.getConnectorAccess().list &&
    ctx.storeUser.getConnectorAccess().read;

  const options = [];
  const authConnectorOpt = {
    label: 'View Auth Connector',
    onClick: () => history.push(cfg.routes.sso),
    Icon: PlugsConnected,
    disabled: false,
    tooltip: '',
  };
  if (hasSsoAccess) {
    options.push(authConnectorOpt);
  } else {
    authConnectorOpt.disabled = true;
    authConnectorOpt.tooltip = 'You do not have access to view SSO connectors';
    options.push(authConnectorOpt);
  }

  return (
    <CardTile
      maxWidth="30%"
      css={`
        @media screen and (max-width: ${p => p.theme.breakpoints.medium}) {
          max-width: 100%;
        }
      `}
    >
      <Flex alignItems="center" justifyContent="space-between">
        <H2>SSO Connector</H2>
        <StatusAndOptions options={options} enabled={true} />
      </Flex>
      <Flex flexDirection="column" gap={3} px={1} pt={1}>
        <Text color="text.slightlyMuted">
          Users sign-in to Teleport by authenticating with this SSO Connector.
        </Text>
        <Text color="text.slightlyMuted">
          Connector Name:&nbsp; <b>{connectorName}</b>
        </Text>
      </Flex>
    </CardTile>
  );
}
