import React from 'react';
import { useTheme } from 'styled-components';
import { Box, Card, Flex, Text, Link } from 'design';
import { DevicesIcon } from 'design/SVGIcon';

export const EmptyList = () => {
  const theme = useTheme();
  return (
    <Card maxWidth="700px" p={4} as={Flex} alignItems="center">
      <Box style={{ textAlign: 'center' }} mr={5}>
        <DevicesIcon size={150} fill={theme.colors.spotBackground[2]} />
      </Box>

      <Box>
        <Text typography="h6" mb={3} caps>
          Register Trusted Device
        </Text>
        <Text typography="subtitle1" mb={3}>
          Device Trust enables authenticated device access. Resources protected
          by the Device Trust mode "required" will enforce the use of a Trusted
          Device, in addition to establishing the user's identity and enforcing
          the necessary roles.
        </Text>
        <Text typography="subtitle1" mb={3}>
          Trusted Devices can be registered manually using{' '}
          <Link
            color="text.main"
            href="https://goteleport.com/docs/access-controls/device-trust/guide/?scope=enterprise#step-12-register-a-trusted-device"
            target="_blank"
          >
            tctl client
          </Link>{' '}
          or synced automatically from MDM services like{' '}
          <Link
            color="text.main"
            href="https://goteleport.com/docs/access-controls/device-trust/jamf-integration/?scope=enterprise"
            target="_blank"
          >
            Jamf
          </Link>
          .
        </Text>
        <Text typography="subtitle1">
          Please{' '}
          <Link
            color="text.main"
            href="https://goteleport.com/docs/access-controls/guides/device-trust/"
            target="_blank"
          >
            view our documentation
          </Link>{' '}
          on how to get started with Device Trust.
        </Text>
      </Box>
    </Card>
  );
};
