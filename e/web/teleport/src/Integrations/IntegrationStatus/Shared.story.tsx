import { Box, Flex, Text } from 'design';
import { NewTab, PlugsConnected } from 'design/Icon';

import { StatusAndOptions } from './Shared';

export default {
  title: 'TeleportE/Integrations/Status/Shared/StatusAndOptions',
};

export const Default = () => {
  const options = [
    {
      label: 'View Auth Connector',
      onClick: () => {},
      Icon: PlugsConnected,
    },
    {
      label: "Open Okta's SAML App",
      onClick: () => {},
      Icon: NewTab,
    },
    {
      label: 'Disabled menu',
      onClick: () => {},
      Icon: NewTab,
      disabled: true,
      tooltip: 'missing permission',
    },
  ];

  return (
    <Flex gap={3}>
      <Flex flexDirection={'column'}>
        <Text mb={1}>Enabled Status</Text>
        <Box bg="levels.surface" padding={3} width="150px">
          <StatusAndOptions options={options} enabled={true} />
        </Box>
      </Flex>

      <Flex flexDirection={'column'}>
        <Text mb={1}>Disabled Status</Text>
        <Box bg="levels.surface" padding={3} width="150px">
          <StatusAndOptions options={options} enabled={false} />
        </Box>
      </Flex>
    </Flex>
  );
};
