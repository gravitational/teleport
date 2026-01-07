import { Box, ButtonWarning, Flex, Text } from 'design';
import { Trash } from 'design/Icon';

import {
  Plugin,
  PluginEntraIdSpec,
  PluginEntraIDStatusDetails,
} from 'teleport/services/integrations';

import { AccessGraphSyncDetails } from './AccessGraphSync';
import { DirectorySyncDetails } from './DirectorySync';
import { GraphApiDetails } from './GraphApi';
import { SsoDetails } from './Sso';

/**
 * StatusDetails displays summary of Entra ID plugin
 * configuration and service status.
 */
export function StatusDetails({
  plugin,
  onDelete,
}: {
  plugin: Plugin<PluginEntraIdSpec, PluginEntraIDStatusDetails>;
  onDelete: () => void;
}) {
  return (
    <Box>
      <Flex
        flexDirection={'row'}
        gap={3}
        mb={3}
        css={`
          @media screen and (max-width: ${p => p.theme.breakpoints.tablet}) {
            gap: ${p => p.theme.space[4]}px;
            margin-bottom: ${p => p.theme.space[4]}px;
            flex-wrap: wrap;
          }
        `}
      >
        <SsoDetails connectorName={plugin.spec.ssoConnectorId} />
        <AccessGraphSyncDetails syncEnabled={plugin.spec.accessGraphEnabled} />
        <GraphApiDetails
          tenantId={plugin.spec.tenantId}
          entraAppId={plugin.spec.entraAppId}
          credentialSource={plugin.spec.credentialSource}
        />
      </Flex>
      <DirectorySyncDetails spec={plugin.spec} status={plugin.status} />

      <ButtonWarning size="large" onClick={onDelete} mt={4}>
        <Trash mr={2} />
        <Text>Delete Plugin</Text>
      </ButtonWarning>
    </Box>
  );
}
