import { Flex, H2, Text } from 'design';
import { CardTile } from 'design/CardTile/CardTile';

import { StatusAndOptions } from 'e-teleport/Integrations/IntegrationStatus/Shared';

export function GraphApiDetails({
  tenantId,
  entraAppId,
  credentialSource,
}: {
  tenantId: string;
  entraAppId: string;
  credentialSource: string;
}) {
  return (
    <CardTile
      maxWidth="40%"
      css={`
        @media screen and (max-width: ${p => p.theme.breakpoints.medium}) {
          max-width: 100%;
        }
      `}
    >
      <Flex alignItems="center" justifyContent="space-between" gap={2}>
        <H2>Microsoft Graph API Integration</H2>

        <StatusAndOptions enabled={true} />
      </Flex>
      <Flex flexDirection="column" gap={3} px={1} pt={1} height="100%">
        <Text color="text.slightlyMuted">
          <span>
            Teleport authenticates to the Microsoft Graph API using these
            credentials.
          </span>
        </Text>
        <Flex flexDirection="column" gap={1}>
          <Text color="text.slightlyMuted">
            Credential Source:&nbsp;{' '}
            <b>{friendlyCredentialSource(credentialSource)}</b>
          </Text>
          <Text color="text.slightlyMuted">
            Tenant ID:&nbsp; <b>{tenantId}</b>
          </Text>
          <Text color="text.slightlyMuted">
            Application ID:&nbsp; <b>{entraAppId}</b>
          </Text>
        </Flex>
      </Flex>
    </CardTile>
  );
}

function friendlyCredentialSource(source: string): string {
  if (source === 'ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS') {
    return 'System credentials';
  }

  return 'Azure OIDC';
}
